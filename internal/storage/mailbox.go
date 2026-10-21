package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"GoArchivist/internal/mail"
)

// Mailbox preserves the original server-side name separately from the label
// displayed to the user. Name is the value to use in future IMAP commands.
type Mailbox struct {
	Name        string
	DisplayName string
	Watched     bool
	Available   bool
}

// RefreshMailboxes atomically updates the server's folder catalog. Existing
// watch choices are retained, and newly discovered folders start disabled
// except INBOX, which defaults to enabled.
func RefreshMailboxes(ctx context.Context, db *sql.DB, names []string) error {
	if db == nil {
		return errors.New("база данных не открыта")
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("не удалось начать обновление списка папок: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `UPDATE mailboxes SET is_available = 0`); err != nil {
		return fmt.Errorf("не удалось отметить старые папки: %w", err)
	}

	now := time.Now().UTC().Format(time.RFC3339Nano)
	for _, name := range names {
		if name == "" {
			continue
		}
		defaultWatched := 0
		if strings.EqualFold(name, "INBOX") {
			defaultWatched = 1
		}
		_, err := tx.ExecContext(ctx, `
			INSERT INTO mailboxes (
				imap_name, display_name, is_watched, is_available, discovered_at, last_seen_at
			) VALUES (?, ?, ?, 1, ?, ?)
			ON CONFLICT(imap_name) DO UPDATE SET
				display_name = excluded.display_name,
				is_available = 1,
				last_seen_at = excluded.last_seen_at
		`, name, mail.DisplayMailboxName(name), defaultWatched, now, now)
		if err != nil {
			return fmt.Errorf("не удалось сохранить папку IMAP %q: %w", name, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("не удалось завершить обновление папок: %w", err)
	}
	return nil
}

// ListMailboxes returns available folders with INBOX at the top.
func ListMailboxes(ctx context.Context, db *sql.DB) ([]Mailbox, error) {
	if db == nil {
		return nil, errors.New("база данных не открыта")
	}
	rows, err := db.QueryContext(ctx, `
		SELECT imap_name, display_name, is_watched, is_available
		FROM mailboxes
		WHERE is_available = 1
		ORDER BY CASE WHEN upper(imap_name) = 'INBOX' THEN 0 ELSE 1 END,
		         display_name COLLATE NOCASE,
		         imap_name
	`)
	if err != nil {
		return nil, fmt.Errorf("не удалось загрузить список папок: %w", err)
	}
	defer rows.Close()

	var mailboxes []Mailbox
	for rows.Next() {
		var mailbox Mailbox
		var watched, available int
		if err := rows.Scan(&mailbox.Name, &mailbox.DisplayName, &watched, &available); err != nil {
			return nil, fmt.Errorf("не удалось прочитать папку из базы данных: %w", err)
		}
		mailbox.Watched = watched != 0
		mailbox.Available = available != 0
		mailboxes = append(mailboxes, mailbox)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("ошибка чтения списка папок: %w", err)
	}
	return mailboxes, nil
}

// SaveWatchedMailboxes atomically stores the user's selection and rejects
// names that are unknown or are no longer available on the IMAP server.
func SaveWatchedMailboxes(ctx context.Context, db *sql.DB, names []string) error {
	if db == nil {
		return errors.New("база данных не открыта")
	}
	unique := make(map[string]struct{}, len(names))
	for _, name := range names {
		if name == "" {
			return errors.New("получено пустое имя папки")
		}
		unique[name] = struct{}{}
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("не удалось начать сохранение выбора папок: %w", err)
	}
	defer tx.Rollback()

	for name := range unique {
		var available int
		if err := tx.QueryRowContext(ctx,
			`SELECT is_available FROM mailboxes WHERE imap_name = ?`, name,
		).Scan(&available); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return fmt.Errorf("неизвестная папка IMAP: %q", name)
			}
			return fmt.Errorf("не удалось проверить папку %q: %w", name, err)
		}
		if available == 0 {
			return fmt.Errorf("папка %q больше не доступна на сервере", name)
		}
	}

	if _, err := tx.ExecContext(ctx, `UPDATE mailboxes SET is_watched = 0 WHERE is_available = 1`); err != nil {
		return fmt.Errorf("не удалось сбросить выбор папок: %w", err)
	}
	for name := range unique {
		if _, err := tx.ExecContext(ctx,
			`UPDATE mailboxes SET is_watched = 1 WHERE imap_name = ? AND is_available = 1`, name,
		); err != nil {
			return fmt.Errorf("не удалось включить отслеживание папки %q: %w", name, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("не удалось сохранить выбор папок: %w", err)
	}
	return nil
}
