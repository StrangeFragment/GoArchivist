package app

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"GoArchivist/internal/web"
)

func Run() error {
	const address = ":8080"

	server, err := web.NewServer(address)
	if err != nil {
		return err
	}

	signalCtx, stop := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
	)
	defer stop()

	serverErr := make(chan error, 1)
	go func() {
		serverErr <- server.ListenAndServe()
	}()

	log.Printf("GoArchivist запущен: http://localhost%s", address)

	select {
	case err := <-serverErr:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("HTTP-сервер: %w", err)
		}
		return nil

	case <-signalCtx.Done():
		log.Println("Получен сигнал завершения; ожидаю окончания запросов...")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		_ = server.Close()
		return fmt.Errorf("корректное завершение HTTP-сервера: %w", err)
	}

	<-serverErr
	log.Println("HTTP-сервер остановлен")
	return nil
}
