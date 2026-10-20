package web

import (
	"html/template"
	"net/http"
	"os"
)

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
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			renderErrorPage(w, page, "405.html", http.StatusMethodNotAllowed)
			return
		}

		section := r.URL.Query().Get("section")
		if section != "logs" {
			section = "general"
		}

		data := struct {
			Section string
			Logs    string
		}{
			Section: section,
		}

		if section == "logs" {
			logs, err := os.ReadFile(logFilePath)
			if err != nil {
				http.Error(
					w,
					"Не удалось прочитать файл журнала",
					http.StatusInternalServerError,
				)
				return
			}
			data.Logs = string(logs)
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")

		if err := page.ExecuteTemplate(w, "settings.html", data); err != nil {
			http.Error(
				w,
				"Не удалось отобразить страницу настроек",
				http.StatusInternalServerError,
			)
		}
	}
}

func clearLogsHandler(logFilePath string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", "POST")
			http.Error(w, "Метод не поддерживается", http.StatusMethodNotAllowed)
			return
		}

		if err := os.WriteFile(logFilePath, nil, 0o644); err != nil {
			http.Error(
				w,
				"Не удалось очистить файл журнала",
				http.StatusInternalServerError,
			)
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
