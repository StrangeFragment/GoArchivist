package storage

import (
	"context"
	"testing"
)

func TestMailboxCatalogAndSelection(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatalf("open in-memory database: %v", err)
	}
	defer db.Close()

	ctx := context.Background()
	initialNames := []string{"Archive", "&BB4EQgQ,BEAEMAQyBDsENQQ9BD0ESwQ1-", "INBOX"}
	if err := RefreshMailboxes(ctx, db, initialNames); err != nil {
		t.Fatalf("refresh mailbox catalog: %v", err)
	}
	mailboxes, err := ListMailboxes(ctx, db)
	if err != nil {
		t.Fatalf("list mailboxes: %v", err)
	}
	if len(mailboxes) != 3 {
		t.Fatalf("got %d mailboxes, want 3", len(mailboxes))
	}
	if mailboxes[0].Name != "INBOX" || mailboxes[0].DisplayName != "Входящие" || !mailboxes[0].Watched {
		t.Fatalf("INBOX default selection incorrect: %+v", mailboxes[0])
	}

	var sentFolder Mailbox
	for _, mailbox := range mailboxes {
		if mailbox.Name == "&BB4EQgQ,BEAEMAQyBDsENQQ9BD0ESwQ1-" {
			sentFolder = mailbox
			break
		}
	}
	if sentFolder.Name == "" || sentFolder.DisplayName != "Отправленные" || sentFolder.Watched {
		t.Fatalf("modified UTF-7 folder handling incorrect: %+v", sentFolder)
	}

	if err := SaveWatchedMailboxes(ctx, db, []string{"INBOX", "Archive"}); err != nil {
		t.Fatalf("save selection: %v", err)
	}
	if err := RefreshMailboxes(ctx, db, []string{"INBOX", "Archive", "Drafts"}); err != nil {
		t.Fatalf("refresh catalog again: %v", err)
	}
	mailboxes, err = ListMailboxes(ctx, db)
	if err != nil {
		t.Fatalf("list refreshed mailboxes: %v", err)
	}
	watched := map[string]bool{}
	for _, mailbox := range mailboxes {
		watched[mailbox.Name] = mailbox.Watched
	}
	if !watched["INBOX"] || !watched["Archive"] || watched["Drafts"] {
		t.Fatalf("unexpected watch state after refresh: %#v", watched)
	}
	if err := SaveWatchedMailboxes(ctx, db, []string{"unknown"}); err == nil {
		t.Fatal("saving an unknown mailbox should fail")
	}
}
