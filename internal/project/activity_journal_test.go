package project

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"kwu/internal/cloud"
)

func TestActivityMessageJournalExactScopeCompletionAndTwentyFourHourRedaction(t *testing.T) {
	scope := gatewayTestScope(t, "usr_activity", 1)
	resolver := &gatewayTestScopeResolver{scope: scope}
	now := time.Date(2026, 8, 7, 8, 0, 0, 0, time.UTC)
	random := bytes.NewReader(bytes.Repeat([]byte{0x2a}, 256))
	journal, err := NewActivityMessageJournal(ActivityMessageJournalOptions{Root: t.TempDir(), Scopes: resolver, Now: func() time.Time { return now }, Random: random})
	if err != nil {
		t.Fatal(err)
	}
	first, err := journal.Prepare(context.Background(), scope, "prj_activity", "private pending text", []string{"usr_mention"}, []string{"asset_one"})
	if err != nil || !strings.HasPrefix(first.ClientMessageID, "msg_") || first.IdempotencyKey == "" {
		t.Fatalf("prepared=%+v error=%v", first, err)
	}
	pending, err := journal.Pending(context.Background(), scope, "prj_activity")
	if err != nil || len(pending) != 1 || pending[0].Text != "private pending text" {
		t.Fatalf("pending=%+v error=%v", pending, err)
	}
	if err := journal.Complete(context.Background(), scope, first.ClientMessageID); err != nil {
		t.Fatal(err)
	}
	pending, err = journal.Pending(context.Background(), scope, "prj_activity")
	if err != nil || len(pending) != 0 {
		t.Fatalf("completed pending=%+v error=%v", pending, err)
	}

	second, err := journal.Prepare(context.Background(), scope, "prj_activity", "must be redacted", []string{"usr_mention"}, []string{"asset_private"})
	if err != nil {
		t.Fatal(err)
	}
	now = now.Add(24*time.Hour + time.Second)
	pending, err = journal.Pending(context.Background(), scope, "prj_activity")
	if err != nil || len(pending) != 0 {
		t.Fatalf("expired pending=%+v error=%v", pending, err)
	}
	payload, err := os.ReadFile(journal.path(scope))
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(payload, []byte("must be redacted")) || bytes.Contains(payload, []byte("usr_mention")) || bytes.Contains(payload, []byte("asset_private")) {
		t.Fatalf("expired private material remains in journal: %s", payload)
	}
	var document activityJournalDocument
	if err := json.Unmarshal(payload, &document); err != nil || len(document.Records) != 1 || document.Records[0].ClientMessageID != second.ClientMessageID || document.Records[0].State != "unresolved_expired" || document.Records[0].Text != "" || len(document.Records[0].MentionUserIDs) != 0 || len(document.Records[0].AssetIDs) != 0 {
		t.Fatalf("expired document=%+v error=%v", document, err)
	}

	resolver.Set(gatewayTestScope(t, "usr_other", 2))
	if _, err := journal.Pending(context.Background(), scope, "prj_activity"); err == nil {
		t.Fatal("stale scope unexpectedly succeeded")
	} else {
		var cloudErr *cloud.Error
		if !errors.As(err, &cloudErr) || cloudErr.Code != cloud.ErrorProfileScopeMismatch {
			t.Fatalf("stale scope error=%v", err)
		}
	}
}
