package web

import (
	"html/template"
	"net/http"
)

func homeHandler(page *template.Template) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")

		if err := page.ExecuteTemplate(w, "index.html", nil); err != nil {
			http.Error(w, "Не удалось отобразить страницу", http.StatusInternalServerError)
		}
	}
}
