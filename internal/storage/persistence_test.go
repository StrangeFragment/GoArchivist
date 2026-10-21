package storage

import (
	"context"
	"path/filepath"
	"testing"
)

func TestWatchedSelectionPersistsAcrossDatabaseReopen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "goarchivist.db")
	ctx := context.Background()

	db, err := Open(path)
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	if err := RefreshMailboxes(ctx, db, []string{"INBOX", "Archive"}); err != nil {
		_ = db.Close()
		t.Fatalf("refresh mailboxes: %v", err)
	}
	if err := SaveWatchedMailboxes(ctx, db, []string{"Archive"}); err != nil {
		_ = db.Close()
		t.Fatalf("save selection: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close database: %v", err)
	}

	db, err = Open(path)
	if err != nil {
		t.Fatalf("reopen database: %v", err)
	}
	defer db.Close()
	mailboxes, err := ListMailboxes(ctx, db)
	if err != nil {
		t.Fatalf("list mailboxes: %v", err)
	}
	watched := map[string]bool{}
	for _, mailbox := range mailboxes {
		watched[mailbox.Name] = mailbox.Watched
	}
	if watched["INBOX"] || !watched["Archive"] {
		t.Fatalf("selection was not persisted: %#v", watched)
	}
}
