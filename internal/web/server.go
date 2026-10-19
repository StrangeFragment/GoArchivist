package web

import (
	"context"
	"embed"
	"fmt"
	"html/template"
	"io/fs"
	"net/http"
)

//go:embed templates/*.html static/css/* static/js/*
var assets embed.FS

type Server struct {
	httpServer *http.Server
}

func NewServer(address string) (*Server, error) {
	page, err := template.ParseFS(assets, "templates/*.html")
	if err != nil {
		return nil, fmt.Errorf("загрузка HTML-шаблонов: %w", err)
	}

	staticFiles, err := fs.Sub(assets, "static")
	if err != nil {
		return nil, fmt.Errorf("подготовка статических файлов: %w", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/settings", settingsHandler(page))
	mux.HandleFunc("/", homeHandler(page))
	mux.Handle(
		"/static/",
		http.StripPrefix("/static/", http.FileServer(http.FS(staticFiles))),
	)

	return &Server{
		httpServer: &http.Server{
			Addr:    address,
			Handler: mux,
		},
	}, nil
}

func (s *Server) ListenAndServe() error {
	return s.httpServer.ListenAndServe()
}

func (s *Server) Shutdown(ctx context.Context) error {
	return s.httpServer.Shutdown(ctx)
}

func (s *Server) Close() error {
	return s.httpServer.Close()
}
