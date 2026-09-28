package event_test

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"testing"
	"time"

	"kwu/internal/event"
)

var identityFactories = []struct {
	prefix string
	newID  func() string
}{
	{"e-", event.NewEventID},
	{"t-", event.NewTurnID},
	{"c-", event.NewCandidateID},
	{"p-", event.NewPromptID},
	{"o-", event.NewOperationID},
	{"occ-", event.NewOccurrenceID},
}

func TestDeliveryIdentitiesAcrossProcesses(t *testing.T) {
	t.Parallel()
	seen := make(map[string]bool)
	for process := 0; process < 2; process++ {
		ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
		cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestDeliveryIdentityProcessHelper$")
		cmd.Env = append(os.Environ(), "KWU_EVENT_ID_TEST_HELPER=1")
		output, err := cmd.Output()
		cancel()
		if err != nil {
			t.Fatalf("identity process %d: %v", process, err)
		}
		var ids []string
		if err := json.Unmarshal(output, &ids); err != nil || len(ids) != len(identityFactories)*32 {
			t.Fatalf("identity process output %q: %v", output, err)
		}
		for _, id := range ids {
			if seen[id] {
				t.Fatalf("identity reused across process lifetimes: %q", id)
			}
			seen[id] = true
		}
	}
}

func TestDeliveryIdentityProcessHelper(t *testing.T) {
	if os.Getenv("KWU_EVENT_ID_TEST_HELPER") != "1" {
		return
	}
	var ids []string
	for range 32 {
		for _, factory := range identityFactories {
			ids = append(ids, factory.newID())
		}
	}
	if err := json.NewEncoder(os.Stdout).Encode(ids); err != nil {
		t.Fatal(err)
	}
	os.Exit(0)
}

func TestDeliveryIdentitiesConcurrentAndWireCompatible(t *testing.T) {
	t.Parallel()
	validOpaqueID := regexp.MustCompile(`^[a-zA-Z0-9_.:-]{1,128}$`)
	for _, factory := range identityFactories {
		t.Run(factory.prefix, func(t *testing.T) {
			t.Parallel()
			const workers, perWorker = 8, 128
			results := make(chan []string, workers)
			for range workers {
				go func() {
					ids := make([]string, perWorker)
					for i := range ids {
						ids[i] = factory.newID()
					}
					results <- ids
				}()
			}
			seen := make(map[string]bool)
			for range workers {
				for _, id := range <-results {
					if seen[id] || !strings.HasPrefix(id, factory.prefix) || !validOpaqueID.MatchString(id) {
						t.Errorf("duplicate or incompatible identity: %q", id)
					}
					seen[id] = true
				}
			}
		})
	}
}
