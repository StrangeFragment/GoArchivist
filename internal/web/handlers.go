package web

import (
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"os"
	"regexp"
	"time"

	"GoArchivist/internal/archive"
	"GoArchivist/internal/config"
)

var pinTimePattern = regexp.MustCompile(`^(?:[01][0-9]|2[0-3])[0-5][0-9]$`)

func homeHandler(page *template.Template) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			renderErrorPage(w, page, "404.html", http.StatusNotFound)
			return
		}
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			renderErrorPage(w, page, "405.html", http.StatusMethodNotAllowed)
			return
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := page.ExecuteTemplate(w, "index.html", nil); err != nil {
			http.Error(w, "Не удалось отобразить страницу", http.StatusInternalServerError)
		}
	}
}

func settingsHandler(page *template.Template, logFilePath string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("section") == "logs" {
			logsHandler(page, logFilePath)(w, r)
			return
		}
		if r.Method != http.MethodGet && r.Method != http.MethodHead && r.Method != http.MethodPost {
			w.Header().Set("Allow", "GET, HEAD, POST")
			renderErrorPage(w, page, "405.html", http.StatusMethodNotAllowed)
			return
		}

		w.Header().Set("Cache-Control", "no-store")
		if r.Method == http.MethodPost {
			if err := r.ParseMultipartForm(1 << 20); err != nil {
				writePINError(w, r, page, "Не удалось прочитать введённый код.", http.StatusBadRequest)
				return
			}

			pin := r.FormValue("pin")
			clientTime := r.FormValue("client_time")
			if !pinTimePattern.MatchString(clientTime) || pin != clientTime {
				writePINError(
					w,
					r,
					page,
					"Код неверен или срок его действия истёк. Введите текущий код.",
					http.StatusForbidden,
				)
				return
			}

			renderSettingsPage(w, page, "", "")
			return
		}

		renderPINPage(w, page, "", http.StatusOK)
	}
}

func archiveDirectoryHandler(page *template.Template) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", "POST")
			renderErrorPage(w, page, "405.html", http.StatusMethodNotAllowed)
			return
		}

		w.Header().Set("Cache-Control", "no-store")
		if err := r.ParseForm(); err != nil {
			renderSettingsPage(w, page, "Не удалось прочитать введённый путь.", "error")
			return
		}

		directory, err := archive.ValidateDirectory(r.FormValue("archive_directory"))
		if err != nil {
			renderSettingsPage(w, page, err.Error(), "error")
			return
		}

		switch r.FormValue("action") {
		case "check":
			renderSettingsPage(
				w,
				page,
				fmt.Sprintf("Папка доступна для чтения и записи: %s", directory),
				"success",
			)
			return

		case "save":
			cfg, paths, err := config.Load()
			if err != nil {
				renderSettingsPage(
					w,
					page,
					fmt.Sprintf("Папка проверена, но конфигурацию загрузить не удалось: %v", err),
					"error",
				)
				return
			}

			cfg.Archive.Directory = directory
			if err := config.Save(paths.ConfigFile, cfg); err != nil {
				renderSettingsPage(
					w,
					page,
					fmt.Sprintf("Папка проверена, но сохранить настройку не удалось: %v", err),
					"error",
				)
				return
			}

			renderSettingsPage(w, page, "Путь к папке архива сохранён.", "success")
			return

		default:
			renderSettingsPage(w, page, "Неизвестное действие.", "error")
		}
	}
}

func logsHandler(page *template.Template, logFilePath string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			renderErrorPage(w, page, "405.html", http.StatusMethodNotAllowed)
			return
		}

		logs, err := os.ReadFile(logFilePath)
		if err != nil {
			http.Error(w, "Не удалось прочитать файл журнала", http.StatusInternalServerError)
			return
		}

		data := struct {
			Section string
			Logs    string
		}{
			Section: "logs",
			Logs:    string(logs),
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := page.ExecuteTemplate(w, "settings.html", data); err != nil {
			http.Error(w, "Не удалось отобразить страницу журнала", http.StatusInternalServerError)
		}
	}
}

func renderSettingsPage(
	w http.ResponseWriter,
	page *template.Template,
	message string,
	messageType string,
) {
	cfg, _, err := config.Load()
	if err != nil {
		http.Error(w, "Не удалось загрузить настройки приложения", http.StatusInternalServerError)
		return
	}

	data := struct {
		Section          string
		Logs             string
		ArchiveDirectory string
		Message          string
		MessageType      string
	}{
		Section:          "general",
		ArchiveDirectory: cfg.Archive.Directory,
		Message:          message,
		MessageType:      messageType,
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := page.ExecuteTemplate(w, "settings.html", data); err != nil {
		http.Error(w, "Не удалось отобразить страницу настроек", http.StatusInternalServerError)
	}
}

func renderPINPage(w http.ResponseWriter, page *template.Template, message string, status int) {
	data := struct {
		Error string
	}{
		Error: message,
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	if err := page.ExecuteTemplate(w, "pin.html", data); err != nil {
		http.Error(w, http.StatusText(status), status)
	}
}

func writePINError(
	w http.ResponseWriter,
	r *http.Request,
	page *template.Template,
	message string,
	status int,
) {
	if r.Header.Get("X-Requested-With") == "XMLHttpRequest" {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(struct {
			Error string `json:"error"`
		}{
			Error: message,
		})
		return
	}

	renderPINPage(w, page, message, status)
}

func clearLogsHandler(logFilePath string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", "POST")
			http.Error(w, "Метод не поддерживается", http.StatusMethodNotAllowed)
			return
		}
		if err := os.WriteFile(logFilePath, nil, 0o644); err != nil {
			http.Error(w, "Не удалось очистить файл журнала", http.StatusInternalServerError)
			return
		}

		http.Redirect(w, r, "/settings?section=logs", http.StatusSeeOther)
	}
}

func renderErrorPage(w http.ResponseWriter, page *template.Template, name string, status int) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	if err := page.ExecuteTemplate(w, name, nil); err != nil {
		http.Error(w, http.StatusText(status), status)
	}
}

// Оставлено на случай использования в других файлах пакета.
var _ = time.Local
