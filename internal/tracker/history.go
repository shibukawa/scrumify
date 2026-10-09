package tracker

import (
	"context"
	"encoding/json"
	"sort"
	"time"

	"scrumify/queries"
)

// DefaultTimezone decides where a project's day ends until it is configurable.
const DefaultTimezone = "Asia/Tokyo"

// Change is one field's movement inside a history entry.
type Change struct {
	From string `json:"from"`
	To   string `json:"to"`
}

// Event kinds. Edits are merged per actor and local day; the others keep one
// row each because their timing is what later analysis reads.
const (
	EventCreated  = "created"
	EventEdited   = "edited"
	EventStatus   = "status"
	EventSprint   = "sprint"
	EventDeleted  = "deleted"
	EventRestored = "restored"
)

func localDay(timezone string, now time.Time) time.Time {
	location, err := time.LoadLocation(timezone)
	if err != nil {
		location = time.UTC
	}
	local := now.In(location)
	return time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, time.UTC)
}

func encodeChanges(changes map[string]Change) string {
	encoded, err := json.Marshal(changes)
	if err != nil {
		return "{}"
	}
	return string(encoded)
}

func decodeChanges(raw string) map[string]Change {
	changes := map[string]Change{}
	_ = json.Unmarshal([]byte(raw), &changes)
	return changes
}

// mergeChanges folds a new edit into the day's entry: the first value of the
// day stays as "from", the newest becomes "to", and a field that ends where it
// started is no longer a change.
func mergeChanges(existing map[string]Change, field string, change Change) map[string]Change {
	merged := make(map[string]Change, len(existing)+1)
	for name, value := range existing {
		merged[name] = value
	}
	if previous, ok := merged[field]; ok {
		change.From = previous.From
	}
	if change.From == change.To {
		delete(merged, field)
	} else {
		merged[field] = change
	}
	return merged
}

func recordEvent(ctx context.Context, scope UnitScope, ticketID int, kind string, changes map[string]Change) error {
	day := localDay(scope.Project.Timezone, time.Now())
	_, err := queries.InsertTicketEvent(ctx, ticketID, scope.Viewer.Id, kind, day, encodeChanges(changes))
	return err
}

// recordEdit must run inside the transaction that locked the ticket, which is
// what keeps two autosaves from both creating the day's entry.
func recordEdit(ctx context.Context, scope UnitScope, ticketID int, field string, change Change) error {
	if change.From == change.To {
		return nil
	}
	day := localDay(scope.Project.Timezone, time.Now())
	existing, err := queries.GetDailyEdit(ctx, ticketID, scope.Viewer.Id, day)
	if err != nil {
		return err
	}
	if existing == nil {
		_, err := queries.InsertTicketEvent(ctx, ticketID, scope.Viewer.Id, EventEdited, day,
			encodeChanges(map[string]Change{field: change}))
		return err
	}
	merged := mergeChanges(decodeChanges(existing.Changes), field, change)
	if len(merged) == 0 {
		_, err := queries.DeleteTicketEvent(ctx, existing.Id)
		return err
	}
	_, err = queries.UpdateTicketEventChanges(ctx, existing.Id, encodeChanges(merged))
	return err
}

// HistoryChange is one field of an entry, in display order.
type HistoryChange struct {
	Field string
	From  string
	To    string
}

// HistoryEntry is one row of a ticket's history.
type HistoryEntry struct {
	Kind      string
	ActorName string
	At        time.Time
	Changes   []HistoryChange
}

func History(ctx context.Context, ticketID int) ([]HistoryEntry, error) {
	scope, err := OpenTicket(ctx, ticketID)
	if err != nil {
		return nil, err
	}
	events, err := collect(queries.ListTicketEvents(ctx, scope.Ticket.Id))
	if err != nil {
		return nil, err
	}
	location, err := time.LoadLocation(scope.Project.Timezone)
	if err != nil {
		location = time.UTC
	}
	entries := make([]HistoryEntry, 0, len(events))
	for _, event := range events {
		entry := HistoryEntry{Kind: event.Kind, ActorName: event.ActorName, At: event.UpdatedAt.In(location)}
		for field, change := range decodeChanges(event.Changes) {
			entry.Changes = append(entry.Changes, HistoryChange{Field: field, From: change.From, To: change.To})
		}
		sort.Slice(entry.Changes, func(i, j int) bool { return entry.Changes[i].Field < entry.Changes[j].Field })
		entries = append(entries, entry)
	}
	return entries, nil
}
