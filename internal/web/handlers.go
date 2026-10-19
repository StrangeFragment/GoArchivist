package web

import (
	"html/template"
	"net/http"
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

func settingsHandler(page *template.Template) http.HandlerFunc {
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

		w.Header().Set("Content-Type", "text/html; charset=utf-8")

		if err := page.ExecuteTemplate(w, "settings.html", struct {
			Section string
		}{Section: section}); err != nil {
			http.Error(w, "Не удалось отобразить страницу настроек", http.StatusInternalServerError)
		}
	}
}

func renderErrorPage(w http.ResponseWriter, page *template.Template, name string, status int) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)

	if err := page.ExecuteTemplate(w, name, nil); err != nil {
		http.Error(w, http.StatusText(status), status)
	}
}
