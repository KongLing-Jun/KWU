package automation

import "testing"

func TestScheduleCatalogListsAllSupportedModes(t *testing.T) {
	entries := ScheduleCatalog()
	wantIDs := []string{"once", "interval", "daily", "weekly", "biweekly", "monthly", "yearly"}
	if len(entries) != len(wantIDs) {
		t.Fatalf("ScheduleCatalog() returned %d entries, want %d", len(entries), len(wantIDs))
	}
	for i, want := range wantIDs {
		if entries[i].ID != want {
			t.Errorf("entry[%d].ID = %q, want %q", i, entries[i].ID, want)
		}
	}
	entries[0].ID = "mutated"
	if ScheduleCatalog()[0].ID != "once" {
		t.Fatal("ScheduleCatalog must return a defensive copy")
	}
}

func TestScheduleCatalogDescribesRequiredFields(t *testing.T) {
	entries := ScheduleCatalog()
	byID := make(map[string]ScheduleCatalogEntry, len(entries))
	for _, entry := range entries {
		byID[entry.ID] = entry
	}
	if !byID["weekly"].RequiresWeekdays || !byID["biweekly"].RequiresWeekdays {
		t.Fatal("weekly catalog entries must require weekdays")
	}
	if !byID["monthly"].RequiresMonthDays {
		t.Fatal("monthly catalog entry must require month days")
	}
	if !byID["yearly"].RequiresMonths || !byID["yearly"].RequiresMonthDays {
		t.Fatal("yearly catalog entry must require months and month days")
	}
}
