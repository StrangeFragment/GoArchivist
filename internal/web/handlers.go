package web

import (
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
)

var pinTimePattern = regexp.MustCompile(`^(?:[01][0-9]|2[0-3])[0-5][0-9]$`)

type settingsPageData struct {
	Section          string
	Logs             string
	ArchiveDirectory string
	Message          string
	MessageType      string

	IMAPServer       string
	IMAPPort         int
	IMAPUsername     string
	IMAPMailbox      string
	IMAPPollInterval int
	IMAPHasPassword  bool
	IMAPConfigured   bool
	IMAPMessage      string
	IMAPMessageType  string
	Mailboxes        []string
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
			renderIMAPResult(w, page, cfg, "Не удалось прочитать параметры подключения.", "error", nil)
			return
		}

		server := strings.TrimSpace(r.FormValue("imap_server"))
		username := strings.TrimSpace(r.FormValue("imap_username"))
		mailbox := strings.TrimSpace(r.FormValue("imap_mailbox"))
		password := r.FormValue("imap_password")
		if password == "" {
			password = cfg.IMAP.Password
		}

		port, portErr := strconv.Atoi(r.FormValue("imap_port"))
		pollInterval, intervalErr := strconv.Atoi(r.FormValue("imap_poll_interval"))
		showResult := func(current config.Config, message, messageType string, mailboxes []string) {
			data := settingsData(current)
			data.IMAPServer = server
			data.IMAPUsername = username
			data.IMAPMailbox = mailbox
			if portErr == nil {
				data.IMAPPort = port
			}
			if intervalErr == nil {
				data.IMAPPollInterval = pollInterval
			}
			data.IMAPMessage = message
			data.IMAPMessageType = messageType
			data.Mailboxes = mailboxes
			renderSettingsData(w, page, data)
		}

		if server == "" || username == "" || password == "" || portErr != nil || port < 1 || port > 65535 {
			showResult(cfg, "Заполните IMAP-сервер, порт, имя пользователя и пароль. Порт должен быть от 1 до 65535.", "error", nil)
			return
		}
		if intervalErr != nil || pollInterval < 1 || pollInterval > 1440 {
			showResult(cfg, "Интервал проверки должен быть от 1 до 1440 минут.", "error", nil)
			return
		}

		mailboxes, err := mailclient.CheckIMAPConnection(server, port, username, password)
		if err != nil {
			message := fmt.Sprintf("Не удалось подключиться: %v. Настройки не сохранены.", err)
			if r.FormValue("imap_password") != "" {
				message += " Новый пароль не сохранён — введите его повторно."
			}
			showResult(cfg, message, "error", nil)
			return
		}
		if len(mailboxes) == 0 {
			message := "Соединение установлено, но сервер не вернул список папок. Настройки не сохранены."
			if r.FormValue("imap_password") != "" {
				message += " Новый пароль не сохранён — введите его повторно."
			}
			showResult(cfg, message, "error", mailboxes)
			return
		}

		selectedMailbox, ok := chooseMailbox(mailbox, mailboxes)
		if !ok {
			message := fmt.Sprintf("Соединение проверено, но папка %q не найдена. Выберите папку из списка; настройки пока не сохранены.", mailbox)
			if r.FormValue("imap_password") != "" {
				message += " Новый пароль не сохранён — введите его повторно."
			}
			showResult(cfg, message, "error", mailboxes)
			return
		}

		previousConfig := cfg
		cfg.IMAP = config.IMAPConfig{
			Server:              server,
			Port:                port,
			Username:            username,
			Password:            password,
			Mailbox:             selectedMailbox,
			PollIntervalMinutes: pollInterval,
		}
		mailbox = selectedMailbox
		if err := config.Save(paths.ConfigFile, cfg); err != nil {
			message := fmt.Sprintf("Соединение проверено, но настройки не удалось сохранить: %v", err)
			if r.FormValue("imap_password") != "" {
				message += " Новый пароль не сохранён — введите его повторно."
			}
			showResult(previousConfig, message, "error", mailboxes)
			return
		}

		showResult(cfg, fmt.Sprintf("Соединение успешно. Настройки сохранены; найдено папок: %d.", len(mailboxes)), "success", mailboxes)
	}
}

func chooseMailbox(requested string, mailboxes []string) (string, bool) {
	if requested == "" {
		requested = "INBOX"
	}
	for _, mailbox := range mailboxes {
		if mailbox == requested || strings.EqualFold(mailbox, requested) {
			return mailbox, true
		}
	}
	return "", false
}

func renderIMAPResult(
	w http.ResponseWriter,
	page *template.Template,
	cfg config.Config,
	message string,
	messageType string,
	mailboxes []string,
) {
	data := settingsData(cfg)
	data.IMAPMessage = message
	data.IMAPMessageType = messageType
	data.Mailboxes = mailboxes
	renderSettingsData(w, page, data)
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
		IMAPMailbox:      cfg.IMAP.Mailbox,
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
	cfg, _, err := config.Load()
	if err != nil {
		http.Error(w, "Не удалось загрузить настройки приложения", http.StatusInternalServerError)
		return
	}

	data := settingsData(cfg)
	data.Message = message
	data.MessageType = messageType
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
