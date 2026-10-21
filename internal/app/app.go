package app

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"strings"
	"syscall"
	"time"

	"GoArchivist/internal/config"
	"GoArchivist/internal/logging"
	"GoArchivist/internal/web"
	"github.com/gogpu/systray"
)

//go:embed assets/tray.png
var trayIcon []byte

// Run starts the web server and keeps the process alive through its system-tray icon.
func Run() error {
	cfg, paths, err := config.Load()
	if err != nil {
		return fmt.Errorf("загрузка конфигурации: %w", err)
	}

	logFile, err := logging.Init(paths.LogsDir)
	if err != nil {
		return fmt.Errorf("настройка журналирования: %w", err)
	}
	defer logFile.Close()

	server, err := web.NewServer(
		cfg.Web.Address,
		logging.LogFilePath(paths.LogsDir),
	)
	if err != nil {
		return err
	}

	interfaceURL, err := webURL(cfg.Web.Address)
	if err != nil {
		return fmt.Errorf("адрес веб-интерфейса: %w", err)
	}
	openInterface := func() {
		if err := openBrowser(interfaceURL); err != nil {
			log.Printf("не удалось открыть браузер: %v", err)
		}
	}

	serverErr := make(chan error, 1)
	go func() {
		serverErr <- server.ListenAndServe()
	}()
	log.Printf("GoArchivist запущен: %s", interfaceURL)

	quitRequested := make(chan struct{}, 1)
	requestQuit := func() {
		select {
		case quitRequested <- struct{}{}:
		default:
		}
	}

	menu := systray.NewMenu()
	menu.Add("GoArchivist", openInterface)
	menu.Add("Выход", requestQuit)

	tray := systray.New()
	tray.SetIcon(trayIcon).
		SetTooltip("GoArchivist — почтовый архив").
		SetMenu(menu).
		OnClick(openInterface).
		Show()

	signalCtx, stopSignals := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
	)
	defer stopSignals()

	shutdownDone := make(chan error, 1)
	go func() {
		var runErr error
		serverAlreadyStopped := false

		select {
		case <-quitRequested:
			log.Println("Запрошено завершение из меню трея")
		case <-signalCtx.Done():
			log.Println("Получен сигнал завершения")
		case err := <-serverErr:
			serverAlreadyStopped = true
			if err != nil && !errors.Is(err, http.ErrServerClosed) {
				runErr = fmt.Errorf("HTTP-сервер: %w", err)
			}
		}

		if !serverAlreadyStopped {
			shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			if err := server.Shutdown(shutdownCtx); err != nil {
				_ = server.Close()
				runErr = errors.Join(runErr, fmt.Errorf("корректное завершение HTTP-сервера: %w", err))
			}
			cancel()

			if err := <-serverErr; err != nil && !errors.Is(err, http.ErrServerClosed) {
				runErr = errors.Join(runErr, fmt.Errorf("HTTP-сервер: %w", err))
			}
		}

		tray.Remove()
		shutdownDone <- runErr
	}()

	trayErr := tray.Run()
	// Also initiate shutdown if the native tray loop exits unexpectedly.
	requestQuit()
	shutdownErr := <-shutdownDone

	log.Println("GoArchivist остановлен")
	return errors.Join(trayErr, shutdownErr)
}

func webURL(address string) (string, error) {
	candidate := strings.TrimSpace(address)
	if !strings.Contains(candidate, "://") {
		candidate = "http://" + candidate
	}

	parsed, err := url.Parse(candidate)
	if err != nil {
		return "", err
	}
	if (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return "", fmt.Errorf("ожидался адрес с http или https, получено %q", address)
	}
	return parsed.String(), nil
}

func openBrowser(target string) error {
	var command string
	var args []string
	switch runtime.GOOS {
	case "windows":
		command = "rundll32"
		args = []string{"url.dll,FileProtocolHandler", target}
	case "darwin":
		command = "open"
		args = []string{target}
	default:
		command = "xdg-open"
		args = []string{target}
	}

	cmd := exec.Command(command, args...)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("запуск %s: %w", command, err)
	}
	go func() {
		if err := cmd.Wait(); err != nil {
			log.Printf("команда открытия браузера завершилась с ошибкой: %v", err)
		}
	}()
	return nil
}
