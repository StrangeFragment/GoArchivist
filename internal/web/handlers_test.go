package web

import (
	"html/template"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"GoArchivist/internal/storage"
)

func TestSettingsTemplatesRenderIMAPForm(t *testing.T) {
	page, err := template.ParseFS(assets, "templates/*.html")
	if err != nil {
		t.Fatalf("parse templates: %v", err)
	}

	response := httptest.NewRecorder()
	renderSettingsData(response, page, settingsPageData{
		Section:          "general",
		IMAPPort:         993,
		IMAPPollInterval: 5,
		Mailboxes: []storage.Mailbox{{
			Name:        "INBOX",
			DisplayName: "Входящие",
			Watched:     true,
		}},
	})

	body := response.Body.String()
	for _, expected := range []string{
		`action="/settings/imap"`,
		`name="imap_server"`,
		`name="imap_port"`,
		`name="imap_username"`,
		`name="imap_password"`,
		"Проверить соединение и сохранить",
		`action="/settings/mailboxes"`,
		`value="INBOX" checked`,
		"Входящие",
	} {
		if !strings.Contains(body, expected) {
			t.Errorf("rendered settings page missing %q", expected)
		}
	}
}

func TestPINFormRequestAcceptsURLFormEncoding(t *testing.T) {
	values := url.Values{"pin": {"1234"}, "client_time": {"1234"}}
	r := httptest.NewRequest("POST", "/settings", strings.NewReader(values.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	if err := parseFormRequest(r); err != nil {
		t.Fatalf("parse URL-encoded form: %v", err)
	}
	if got := r.FormValue("pin"); got != "1234" {
		t.Fatalf("pin = %q, want 1234", got)
	}
}
