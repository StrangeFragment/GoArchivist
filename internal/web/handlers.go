package web

import (
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"

	"GoArchivist/internal/archive"
	"GoArchivist/internal/config"
	mailclient "GoArchivist/internal/mail"
	"GoArchivist/internal/storage"
)

var pinTimePattern = regexp.MustCompile(`^(?:[01][0-9]|2[0-3])[0-5][0-9]$`)

type settingsPageData struct {
	Section          string
	Logs             string
	ArchiveDirectory string
	Message          string
	MessageType      string

	IMAPServer        string
	IMAPPort          int
	IMAPUsername      string
	IMAPPollInterval  int
	IMAPHasPassword   bool
	IMAPConfigured    bool
	IMAPMessage       string
	IMAPMessageType   string
	Mailboxes         []storage.Mailbox
	FolderMessage     string
	FolderMessageType string
}

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
			if err := parseFormRequest(r); err != nil {
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

// parseFormRequest accepts both the browser's default URL-encoded forms and
// multipart forms submitted by the PIN dialog's JavaScript FormData request.
func parseFormRequest(r *http.Request) error {
	contentType := strings.ToLower(r.Header.Get("Content-Type"))
	if strings.HasPrefix(contentType, "multipart/form-data") {
		return r.ParseMultipartForm(1 << 20)
	}
	return r.ParseForm()
}

func imapSettingsHandler(page *template.Template) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", "POST")
			renderErrorPage(w, page, "405.html", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Cache-Control", "no-store")

		cfg, paths, err := config.Load()
		if err != nil {
			http.Error(w, "Не удалось загрузить настройки приложения", http.StatusInternalServerError)
			return
		}
		if err := r.ParseForm(); err != nil {
			renderSettingsPage(w, page, "Не удалось прочитать параметры подключения.", "error")
			return
		}

		server := strings.TrimSpace(r.FormValue("imap_server"))
		username := strings.TrimSpace(r.FormValue("imap_username"))
		password := r.FormValue("imap_password")
		if password == "" {
			password = cfg.IMAP.Password
		}
		port, portErr := strconv.Atoi(r.FormValue("imap_port"))
		pollInterval, intervalErr := strconv.Atoi(r.FormValue("imap_poll_interval"))

		showResult := func(current config.Config, message, messageType string) {
			data := settingsData(current)
			data.IMAPServer = server
			data.IMAPUsername = username
			if portErr == nil {
				data.IMAPPort = port
			}
			if intervalErr == nil {
				data.IMAPPollInterval = pollInterval
			}
			data.IMAPMessage = message
			data.IMAPMessageType = messageType
			data.Mailboxes, err = loadMailboxCatalog(paths.Database)
			if err != nil {
				data.FolderMessage = "Не удалось загрузить сохранённый список папок: " + err.Error()
				data.FolderMessageType = "error"
			}
			renderSettingsData(w, page, data)
		}

		if server == "" || username == "" || password == "" || portErr != nil || port < 1 || port > 65535 {
			showResult(cfg, "Заполните IMAP-сервер, порт, имя пользователя и пароль. Порт должен быть от 1 до 65535.", "error")
			return
		}
		if intervalErr != nil || pollInterval < 1 || pollInterval > 1440 {
			showResult(cfg, "Интервал проверки должен быть от 1 до 1440 минут.", "error")
			return
		}

		mailboxes, err := mailclient.CheckIMAPConnection(server, port, username, password)
		if err != nil {
			message := fmt.Sprintf("Не удалось подключиться: %v. Настройки не сохранены.", err)
			if r.FormValue("imap_password") != "" {
				message += " Новый пароль не сохранён — введите его повторно."
			}
			showResult(cfg, message, "error")
			return
		}
		if len(mailboxes) == 0 {
			message := "Соединение установлено, но сервер не вернул папки. Настройки не сохранены."
			if r.FormValue("imap_password") != "" {
				message += " Новый пароль не сохранён — введите его повторно."
			}
			showResult(cfg, message, "error")
			return
		}

		previousConfig := cfg
		cfg.IMAP.Server = server
		cfg.IMAP.Port = port
		cfg.IMAP.Username = username
		cfg.IMAP.Password = password
		cfg.IMAP.PollIntervalMinutes = pollInterval
		if err := config.Save(paths.ConfigFile, cfg); err != nil {
			message := fmt.Sprintf("Соединение проверено, но настройки не удалось сохранить: %v", err)
			if r.FormValue("imap_password") != "" {
				message += " Новый пароль не сохранён — введите его повторно."
			}
			showResult(previousConfig, message, "error")
			return
		}

		db, err := storage.Open(paths.Database)
		if err != nil {
			showResult(cfg, "Соединение успешно, параметры сохранены, но не удалось обновить список папок: "+err.Error(), "error")
			return
		}
		refreshErr := storage.RefreshMailboxes(r.Context(), db, mailboxes)
		closeErr := db.Close()
		if refreshErr != nil {
			showResult(cfg, "Соединение успешно, параметры сохранены, но список папок не обновлён: "+refreshErr.Error(), "error")
			return
		}
		if closeErr != nil {
			showResult(cfg, "Соединение и сохранение прошли успешно, но базу данных не удалось закрыть: "+closeErr.Error(), "error")
			return
		}

		showResult(cfg, "Соединение успешно. Настройки сохранены. Выберите папки для отслеживания ниже.", "success")
	}
}

func watchedMailboxesHandler(page *template.Template) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", "POST")
			renderErrorPage(w, page, "405.html", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		if err := r.ParseForm(); err != nil {
			renderMailboxSelectionPage(w, page, "Не удалось прочитать выбранные папки.", "error")
			return
		}

		_, paths, err := config.Load()
		if err != nil {
			http.Error(w, "Не удалось загрузить настройки приложения", http.StatusInternalServerError)
			return
		}
		db, err := storage.Open(paths.Database)
		if err != nil {
			renderMailboxSelectionPage(w, page, "Не удалось открыть базу данных папок: "+err.Error(), "error")
			return
		}
		selected := r.Form["watched_mailbox"]
		saveErr := storage.SaveWatchedMailboxes(r.Context(), db, selected)
		closeErr := db.Close()
		if saveErr != nil {
			renderMailboxSelectionPage(w, page, saveErr.Error(), "error")
			return
		}
		if closeErr != nil {
			renderMailboxSelectionPage(w, page, "Выбор сохранён, но базу данных не удалось закрыть: "+closeErr.Error(), "error")
			return
		}
		if len(selected) == 0 {
			renderMailboxSelectionPage(w, page, "Выбор сохранён. Сейчас ни одна папка не отслеживается.", "success")
			return
		}
		unique := make(map[string]struct{}, len(selected))
		for _, name := range selected {
			unique[name] = struct{}{}
		}
		renderMailboxSelectionPage(w, page, fmt.Sprintf("Выбор сохранён. Отслеживается папок: %d.", len(unique)), "success")
	}
}

func loadMailboxCatalog(databasePath string) ([]storage.Mailbox, error) {
	db, err := storage.Open(databasePath)
	if err != nil {
		return nil, err
	}
	defer db.Close()
	return storage.ListMailboxes(context.Background(), db)
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

func settingsData(cfg config.Config) settingsPageData {
	return settingsPageData{
		Section:          "general",
		ArchiveDirectory: cfg.Archive.Directory,
		IMAPServer:       cfg.IMAP.Server,
		IMAPPort:         cfg.IMAP.Port,
		IMAPUsername:     cfg.IMAP.Username,
		IMAPPollInterval: cfg.IMAP.PollIntervalMinutes,
		IMAPHasPassword:  cfg.IMAP.Password != "",
		IMAPConfigured:   cfg.IMAP.Server != "" && cfg.IMAP.Username != "" && cfg.IMAP.Password != "",
	}
}

func renderSettingsPage(
	w http.ResponseWriter,
	page *template.Template,
	message string,
	messageType string,
) {
	cfg, paths, err := config.Load()
	if err != nil {
		http.Error(w, "Не удалось загрузить настройки приложения", http.StatusInternalServerError)
		return
	}

	data := settingsData(cfg)
	data.Message = message
	data.MessageType = messageType
	data.Mailboxes, err = loadMailboxCatalog(paths.Database)
	if err != nil {
		data.FolderMessage = "Не удалось загрузить список папок: " + err.Error()
		data.FolderMessageType = "error"
	}
	renderSettingsData(w, page, data)
}

func renderMailboxSelectionPage(
	w http.ResponseWriter,
	page *template.Template,
	message string,
	messageType string,
) {
	cfg, paths, err := config.Load()
	if err != nil {
		http.Error(w, "Не удалось загрузить настройки приложения", http.StatusInternalServerError)
		return
	}
	data := settingsData(cfg)
	data.FolderMessage = message
	data.FolderMessageType = messageType
	data.Mailboxes, err = loadMailboxCatalog(paths.Database)
	if err != nil {
		data.FolderMessage = "Не удалось загрузить список папок: " + err.Error()
		data.FolderMessageType = "error"
	}
	renderSettingsData(w, page, data)
}

func renderSettingsData(w http.ResponseWriter, page *template.Template, data settingsPageData) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
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
