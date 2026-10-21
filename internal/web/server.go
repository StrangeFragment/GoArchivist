package web

import (
	"context"
	"embed"
	"fmt"
	"html/template"
	"io/fs"
	"net/http"
	"time"
)

//go:embed templates/*.html static/css/* static/js/*
var assets embed.FS

type Server struct {
	httpServer *http.Server
}

func NewServer(address string, logFilePath string) (*Server, error) {
	page, err := template.ParseFS(assets, "templates/*.html")
	if err != nil {
		return nil, fmt.Errorf("загрузка HTML-шаблонов: %w", err)
	}

	staticFiles, err := fs.Sub(assets, "static")
	if err != nil {
		return nil, fmt.Errorf("подготовка статических файлов: %w", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/settings/clear-logs", clearLogsHandler(logFilePath))
	mux.HandleFunc("/settings/archive-directory", archiveDirectoryHandler(page))
	mux.HandleFunc("/settings/imap", imapSettingsHandler(page))
	mux.HandleFunc("/settings/mailboxes", watchedMailboxesHandler(page))
	mux.HandleFunc("/settings", settingsHandler(page, logFilePath))
	mux.HandleFunc("/", homeHandler(page))
	mux.Handle(
		"/static/",
		http.StripPrefix("/static/", http.FileServer(http.FS(staticFiles))),
	)

	return &Server{
		httpServer: &http.Server{
			Addr:              address,
			Handler:           mux,
			ReadHeaderTimeout: 5 * time.Second,
			ReadTimeout:       15 * time.Second,
			WriteTimeout:      30 * time.Second,
			IdleTimeout:       60 * time.Second,
			MaxHeaderBytes:    1 << 20, // 1 MiB
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
