package automation

// ScheduleCatalogEntry describes one schedule mode supported by the shared
// scheduler. Frontends use this catalog instead of maintaining a second list.
type ScheduleCatalogEntry struct {
	ID                string       `json:"id"`
	Kind              ScheduleKind `json:"kind"`
	Frequency         string       `json:"frequency,omitempty"`
	RequiresRunAt     bool         `json:"requiresRunAt,omitempty"`
	RequiresEvery     bool         `json:"requiresEvery,omitempty"`
	RequiresWeekdays  bool         `json:"requiresWeekdays,omitempty"`
	RequiresMonthDays bool         `json:"requiresMonthDays,omitempty"`
	RequiresMonths    bool         `json:"requiresMonths,omitempty"`
}

var scheduleCatalog = []ScheduleCatalogEntry{
	{ID: "once", Kind: ScheduleKindOnce, RequiresRunAt: true},
	{ID: "interval", Kind: ScheduleKindInterval, RequiresEvery: true, RequiresWeekdays: true},
	{ID: "daily", Kind: ScheduleKindCalendar, Frequency: "daily"},
	{ID: "weekly", Kind: ScheduleKindCalendar, Frequency: "weekly", RequiresWeekdays: true},
	{ID: "biweekly", Kind: ScheduleKindCalendar, Frequency: "biweekly", RequiresWeekdays: true},
	{ID: "monthly", Kind: ScheduleKindCalendar, Frequency: "monthly", RequiresMonthDays: true},
	{ID: "yearly", Kind: ScheduleKindCalendar, Frequency: "yearly", RequiresMonthDays: true, RequiresMonths: true},
}

// ScheduleCatalog returns a defensive copy in stable display order.
func ScheduleCatalog() []ScheduleCatalogEntry {
	return append([]ScheduleCatalogEntry(nil), scheduleCatalog...)
}
