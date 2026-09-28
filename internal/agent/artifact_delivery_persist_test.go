package agent

import (
	"path/filepath"
	"testing"
	"time"

	"kwu/internal/provider"
)

const artifactPersistenceTestTimeout = 5 * time.Second

func TestSessionCommitArtifactDeliveryAndSavePersistsContract(t *testing.T) {
	const turnID = "commit-and-save-turn"
	path := filepath.Join(t.TempDir(), "session.jsonl")
	session := newArtifactPersistenceCandidate(turnID)
	delivery := provider.ArtifactDelivery{
		Version:     provider.ArtifactDeliveryVersion,
		Paths:       []string{"reports/final.md"},
		DeliveredAt: 101,
	}

	committed, ok, err := session.CommitArtifactDeliveryAndSave(path, turnID, delivery)
	if err != nil {
		t.Fatalf("CommitArtifactDeliveryAndSave: %v", err)
	}
	if !ok {
		t.Fatal("CommitArtifactDeliveryAndSave ok = false, want true")
	}
	assertArtifactPersistenceState(t, session.Snapshot(), turnID, provider.MessageKindFinalAnswer, &delivery)
	assertArtifactPersistenceState(t, loadArtifactPersistenceMessages(t, path), turnID, provider.MessageKindFinalAnswer, &delivery)

	loaded, err := LoadArtifactDeliveryContracts(path)
	if err != nil {
		t.Fatalf("LoadArtifactDeliveryContracts: %v", err)
	}
	if loaded.State != ArtifactDeliveryContractValid || len(loaded.Deliveries) != 1 {
		t.Fatalf("persisted contracts = %#v, want one valid v1 contract", loaded)
	}
	if got := loaded.Deliveries[0]; got.Version != delivery.Version || got.DeliveredAt != delivery.DeliveredAt ||
		len(got.Paths) != 1 || got.Paths[0] != delivery.Paths[0] {
		t.Fatalf("persisted delivery = %#v, want %#v", got, delivery)
	}
	if got := committed; got.Version != delivery.Version || got.DeliveredAt != delivery.DeliveredAt ||
		len(got.Paths) != 1 || got.Paths[0] != delivery.Paths[0] {
		t.Fatalf("committed delivery = %#v, want %#v", got, delivery)
	}
}

func TestSessionCommitArtifactDeliveryAndSaveEmptyPathDoesNotPromote(t *testing.T) {
	const turnID = "empty-path-turn"
	session := newArtifactPersistenceCandidate(turnID)
	delivery := provider.ArtifactDelivery{
		Version:     provider.ArtifactDeliveryVersion,
		Paths:       []string{"report.md"},
		DeliveredAt: 202,
	}

	committed, ok, err := session.CommitArtifactDeliveryAndSave("", turnID, delivery)
	if err == nil {
		t.Fatal("CommitArtifactDeliveryAndSave with an empty path succeeded, want error")
	}
	if ok {
		t.Fatalf("CommitArtifactDeliveryAndSave ok = true with committed delivery %#v", committed)
	}
	assertArtifactPersistenceState(t, session.Snapshot(), turnID, provider.MessageKindUncommittedFinalAnswer, nil)
}

func TestSessionDowngradeArtifactDeliveryFinalAndSavePersistsFailClosedState(t *testing.T) {
	const turnID = "downgrade-and-save-turn"
	path := filepath.Join(t.TempDir(), "session.jsonl")
	session := newArtifactPersistenceCandidate(turnID)
	delivery := provider.ArtifactDelivery{
		Version:     provider.ArtifactDeliveryVersion,
		Paths:       []string{"report.md"},
		DeliveredAt: 303,
	}
	if _, ok, err := session.CommitArtifactDeliveryAndSave(path, turnID, delivery); err != nil || !ok {
		t.Fatalf("seed CommitArtifactDeliveryAndSave = ok %v, err %v", ok, err)
	}

	found, err := session.DowngradeArtifactDeliveryFinalAndSave(path, turnID)
	if err != nil {
		t.Fatalf("DowngradeArtifactDeliveryFinalAndSave: %v", err)
	}
	if !found {
		t.Fatal("DowngradeArtifactDeliveryFinalAndSave found = false, want true")
	}
	assertArtifactPersistenceState(t, session.Snapshot(), turnID, provider.MessageKindUncommittedFinalAnswer, nil)
	assertArtifactPersistenceState(t, loadArtifactPersistenceMessages(t, path), turnID, provider.MessageKindUncommittedFinalAnswer, nil)

	loaded, err := LoadArtifactDeliveryContracts(path)
	if err != nil {
		t.Fatalf("LoadArtifactDeliveryContracts: %v", err)
	}
	if loaded.State != ArtifactDeliveryContractAbsent || len(loaded.Deliveries) != 0 {
		t.Fatalf("contracts after downgrade = %#v, want absent", loaded)
	}
}

func TestSessionArtifactDeliveryPersistenceSerializesWithSaveIfChanged(t *testing.T) {
	t.Run("commit", func(t *testing.T) {
		const turnID = "concurrent-commit-turn"
		path := filepath.Join(t.TempDir(), "session.jsonl")
		session := newArtifactPersistenceCandidate(turnID)
		delivery := provider.ArtifactDelivery{
			Version:     provider.ArtifactDeliveryVersion,
			Paths:       []string{"concurrent.md"},
			DeliveredAt: 404,
		}

		session.saveMu.Lock()
		locked := true
		defer func() {
			if locked {
				session.saveMu.Unlock()
			}
		}()
		commitStarted := make(chan struct{})
		commitDone := make(chan error, 1)
		go func() {
			close(commitStarted)
			_, ok, err := session.CommitArtifactDeliveryAndSave(path, turnID, delivery)
			if err == nil && !ok {
				err = errArtifactPersistenceUnexpectedResult("commit returned ok=false")
			}
			commitDone <- err
		}()
		saveStarted := make(chan struct{})
		saveDone := make(chan error, 1)
		go func() {
			close(saveStarted)
			_, _, err := session.SaveIfChanged(path)
			saveDone <- err
		}()
		<-commitStarted
		<-saveStarted
		assertArtifactPersistenceOperationBlocked(t, commitDone, "commit")
		assertArtifactPersistenceOperationBlocked(t, saveDone, "SaveIfChanged")
		assertArtifactPersistenceState(t, session.Snapshot(), turnID, provider.MessageKindUncommittedFinalAnswer, nil)
		session.saveMu.Unlock()
		locked = false

		waitArtifactPersistenceOperation(t, commitDone, "commit")
		waitArtifactPersistenceOperation(t, saveDone, "SaveIfChanged")
		assertArtifactPersistenceState(t, session.Snapshot(), turnID, provider.MessageKindFinalAnswer, &delivery)
		assertArtifactPersistenceState(t, loadArtifactPersistenceMessages(t, path), turnID, provider.MessageKindFinalAnswer, &delivery)
	})

	t.Run("downgrade", func(t *testing.T) {
		const turnID = "concurrent-downgrade-turn"
		path := filepath.Join(t.TempDir(), "session.jsonl")
		session := newArtifactPersistenceCandidate(turnID)
		delivery := provider.ArtifactDelivery{
			Version:     provider.ArtifactDeliveryVersion,
			Paths:       []string{"concurrent.md"},
			DeliveredAt: 505,
		}
		if _, ok, err := session.CommitArtifactDeliveryAndSave(path, turnID, delivery); err != nil || !ok {
			t.Fatalf("seed CommitArtifactDeliveryAndSave = ok %v, err %v", ok, err)
		}
		// Give SaveIfChanged real work so either ordering exercises a write.
		session.Add(provider.Message{Role: provider.RoleUser, Content: "persist concurrently"})

		session.saveMu.Lock()
		locked := true
		defer func() {
			if locked {
				session.saveMu.Unlock()
			}
		}()
		downgradeStarted := make(chan struct{})
		downgradeDone := make(chan error, 1)
		go func() {
			close(downgradeStarted)
			found, err := session.DowngradeArtifactDeliveryFinalAndSave(path, turnID)
			if err == nil && !found {
				err = errArtifactPersistenceUnexpectedResult("downgrade returned found=false")
			}
			downgradeDone <- err
		}()
		saveStarted := make(chan struct{})
		saveDone := make(chan error, 1)
		go func() {
			close(saveStarted)
			_, _, err := session.SaveIfChanged(path)
			saveDone <- err
		}()
		<-downgradeStarted
		<-saveStarted
		assertArtifactPersistenceOperationBlocked(t, downgradeDone, "downgrade")
		assertArtifactPersistenceOperationBlocked(t, saveDone, "SaveIfChanged")
		assertArtifactPersistenceState(t, session.Snapshot(), turnID, provider.MessageKindFinalAnswer, &delivery)
		session.saveMu.Unlock()
		locked = false

		waitArtifactPersistenceOperation(t, downgradeDone, "downgrade")
		waitArtifactPersistenceOperation(t, saveDone, "SaveIfChanged")
		assertArtifactPersistenceState(t, session.Snapshot(), turnID, provider.MessageKindUncommittedFinalAnswer, nil)
		assertArtifactPersistenceState(t, loadArtifactPersistenceMessages(t, path), turnID, provider.MessageKindUncommittedFinalAnswer, nil)
	})
}

type artifactPersistenceUnexpectedResult string

func (err artifactPersistenceUnexpectedResult) Error() string { return string(err) }

func errArtifactPersistenceUnexpectedResult(message string) error {
	return artifactPersistenceUnexpectedResult(message)
}

func newArtifactPersistenceCandidate(turnID string) *Session {
	session := NewSession("system")
	session.BeginArtifactDeliveryTurn(turnID)
	session.Add(provider.Message{Role: provider.RoleUser, Content: "create a report"})
	session.Add(provider.Message{
		Role:        provider.RoleAssistant,
		Content:     "report ready",
		MessageKind: provider.MessageKindFinalAnswer,
	})
	session.EndArtifactDeliveryTurn(turnID)
	return session
}

func loadArtifactPersistenceMessages(t *testing.T, path string) []provider.Message {
	t.Helper()
	loaded, err := LoadSession(path)
	if err != nil {
		t.Fatalf("LoadSession(%q): %v", path, err)
	}
	return loaded.Snapshot()
}

func assertArtifactPersistenceState(
	t *testing.T,
	messages []provider.Message,
	turnID string,
	wantKind provider.MessageKind,
	wantDelivery *provider.ArtifactDelivery,
) {
	t.Helper()
	for i := len(messages) - 1; i >= 0; i-- {
		message := messages[i]
		if message.Role != provider.RoleAssistant || message.ArtifactTurnID != turnID {
			continue
		}
		if message.MessageKind != wantKind {
			t.Fatalf("artifact final kind = %q, want %q; message = %#v", message.MessageKind, wantKind, message)
		}
		if wantDelivery == nil {
			if message.ArtifactDelivery != nil {
				t.Fatalf("artifact final delivery = %#v, want nil", message.ArtifactDelivery)
			}
			return
		}
		if message.ArtifactDelivery == nil {
			t.Fatalf("artifact final delivery = nil, want %#v", wantDelivery)
		}
		got := message.ArtifactDelivery
		if got.Version != provider.ArtifactDeliveryVersion || got.Version != wantDelivery.Version ||
			got.DeliveredAt != wantDelivery.DeliveredAt || len(got.Paths) != len(wantDelivery.Paths) {
			t.Fatalf("artifact final delivery = %#v, want %#v", got, wantDelivery)
		}
		for pathIndex := range wantDelivery.Paths {
			if got.Paths[pathIndex] != wantDelivery.Paths[pathIndex] {
				t.Fatalf("artifact final delivery = %#v, want %#v", got, wantDelivery)
			}
		}
		return
	}
	t.Fatalf("no assistant final found for artifact turn %q in %#v", turnID, messages)
}

func assertArtifactPersistenceOperationBlocked(t *testing.T, done <-chan error, name string) {
	t.Helper()
	select {
	case err := <-done:
		t.Fatalf("%s completed while saveMu was held: %v", name, err)
	case <-time.After(50 * time.Millisecond):
	}
}

func waitArtifactPersistenceOperation(t *testing.T, done <-chan error, name string) {
	t.Helper()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
	case <-time.After(artifactPersistenceTestTimeout):
		t.Fatalf("timed out waiting for %s", name)
	}
}
