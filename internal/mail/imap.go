package mail

import (
	"bufio"
	"crypto/tls"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"
)

const imapConnectionTimeout = 15 * time.Second

// CheckIMAPConnection устанавливает защищённое TLS-соединение,
// проверяет учётные данные и возвращает список папок почтового ящика.
func CheckIMAPConnection(server string, port int, username, password string) ([]string, error) {
	server = strings.TrimSpace(server)
	username = strings.TrimSpace(username)

	if server == "" {
		return nil, fmt.Errorf("не указан IMAP-сервер")
	}
	if port < 1 || port > 65535 {
		return nil, fmt.Errorf("некорректный порт IMAP: %d", port)
	}
	if username == "" {
		return nil, fmt.Errorf("не указано имя пользователя")
	}
	if password == "" {
		return nil, fmt.Errorf("не указан пароль")
	}

	if err := validateIMAPArgument(username); err != nil {
		return nil, fmt.Errorf("некорректное имя пользователя: %w", err)
	}
	if err := validateIMAPArgument(password); err != nil {
		return nil, fmt.Errorf("некорректный пароль: %w", err)
	}

	address := net.JoinHostPort(server, strconv.Itoa(port))
	dialer := &net.Dialer{Timeout: imapConnectionTimeout}

	conn, err := tls.DialWithDialer(dialer, "tcp", address, &tls.Config{
		ServerName: server,
		MinVersion: tls.VersionTLS12,
	})
	if err != nil {
		return nil, fmt.Errorf("не удалось подключиться к IMAP-серверу: %w", err)
	}
	defer conn.Close()

	if err := conn.SetDeadline(time.Now().Add(imapConnectionTimeout)); err != nil {
		return nil, fmt.Errorf("не удалось установить тайм-аут IMAP-соединения: %w", err)
	}

	reader := bufio.NewReader(conn)

	greeting, err := readIMAPLine(reader)
	if err != nil {
		return nil, fmt.Errorf("не удалось прочитать приветствие IMAP-сервера: %w", err)
	}
	if !strings.HasPrefix(strings.ToUpper(greeting), "* OK") &&
		!strings.HasPrefix(strings.ToUpper(greeting), "* PREAUTH") {
		return nil, fmt.Errorf("IMAP-сервер отклонил соединение: %s", greeting)
	}

	if strings.HasPrefix(strings.ToUpper(greeting), "* PREAUTH") {
		return listIMAPMailboxes(conn, reader)
	}

	if err := writeIMAPCommand(conn, `A001 LOGIN `+quoteIMAPString(username)+` `+quoteIMAPString(password)); err != nil {
		return nil, fmt.Errorf("не удалось отправить команду авторизации IMAP: %w", err)
	}

	if err := readIMAPCommandResult(reader, "A001"); err != nil {
		return nil, fmt.Errorf("ошибка авторизации IMAP: %w", err)
	}

	return listIMAPMailboxes(conn, reader)
}

func listIMAPMailboxes(conn net.Conn, reader *bufio.Reader) ([]string, error) {
	if err := writeIMAPCommand(conn, `A002 LIST "" "*"`); err != nil {
		return nil, fmt.Errorf("не удалось запросить список папок IMAP: %w", err)
	}

	var mailboxes []string

	for {
		line, err := readIMAPLine(reader)
		if err != nil {
			return nil, fmt.Errorf("ошибка при получении списка папок IMAP: %w", err)
		}

		if strings.HasPrefix(line, "* LIST ") {
			mailbox, ok := parseIMAPMailbox(line)
			if ok {
				mailboxes = append(mailboxes, mailbox)
			}
			continue
		}

		if strings.HasPrefix(strings.ToUpper(line), "A002 ") {
			if !strings.HasPrefix(strings.ToUpper(line), "A002 OK") {
				return nil, fmt.Errorf("IMAP-сервер не смог вернуть список папок: %s", line)
			}
			return mailboxes, nil
		}
	}
}

func readIMAPCommandResult(reader *bufio.Reader, tag string) error {
	for {
		line, err := readIMAPLine(reader)
		if err != nil {
			return err
		}

		if !strings.HasPrefix(strings.ToUpper(line), strings.ToUpper(tag)+" ") {
			continue
		}

		if strings.HasPrefix(strings.ToUpper(line), strings.ToUpper(tag)+" OK") {
			return nil
		}

		return fmt.Errorf("%s", line)
	}
}

func readIMAPLine(reader *bufio.Reader) (string, error) {
	line, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}

	return strings.TrimRight(line, "\r\n"), nil
}

func writeIMAPCommand(conn net.Conn, command string) error {
	_, err := fmt.Fprintf(conn, "%s\r\n", command)
	return err
}

func validateIMAPArgument(value string) error {
	if strings.ContainsAny(value, "\r\n") {
		return fmt.Errorf("значение содержит недопустимый перевод строки")
	}
	return nil
}

func quoteIMAPString(value string) string {
	value = strings.ReplaceAll(value, `\`, `\\`)
	value = strings.ReplaceAll(value, `"`, `\"`)
	return `"` + value + `"`
}

func parseIMAPMailbox(line string) (string, bool) {
	// У LIST-ответа после списка флагов идут разделитель и имя папки.
	flagsEnd := strings.IndexByte(line, ')')
	if flagsEnd < 0 || flagsEnd+1 >= len(line) {
		return "", false
	}

	rest := strings.TrimSpace(line[flagsEnd+1:])
	_, rest, ok := readIMAPToken(rest) // delimiter
	if !ok {
		return "", false
	}

	mailboxToken, _, ok := readIMAPToken(rest)
	if !ok {
		return "", false
	}

	if len(mailboxToken) >= 2 && mailboxToken[0] == '"' && mailboxToken[len(mailboxToken)-1] == '"' {
		value := mailboxToken[1 : len(mailboxToken)-1]
		value = strings.ReplaceAll(value, `\"`, `"`)
		value = strings.ReplaceAll(value, `\\`, `\`)
		return value, true
	}

	return mailboxToken, mailboxToken != ""
}

func readIMAPToken(value string) (token, rest string, ok bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", "", false
	}

	if value[0] != '"' {
		end := strings.IndexByte(value, ' ')
		if end < 0 {
			return value, "", true
		}
		return value[:end], strings.TrimSpace(value[end+1:]), true
	}

	var escaped bool
	for i := 1; i < len(value); i++ {
		switch {
		case escaped:
			escaped = false
		case value[i] == '\\':
			escaped = true
		case value[i] == '"':
			return value[:i+1], strings.TrimSpace(value[i+1:]), true
		}
	}

	return "", "", false
}
