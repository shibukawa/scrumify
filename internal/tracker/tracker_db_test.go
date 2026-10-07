package tracker_test

import (
	"context"
	"os"
	"testing"
	"time"

	"scrumify/handlers"
	"scrumify/internal/tracker"
	"scrumify/queries"

	_ "scrumify"
	_ "scrumify/templates"

	_ "github.com/shibukawa/popcornweb/database/postgres"
	"github.com/shibukawa/popcornweb/pw"
	"github.com/shibukawa/popcornweb/testutil"
)

// These tests run against a real PostgreSQL database, inside one transaction
// per test that is rolled back at the end. Start the development database
// (devbox services up) and they use scrumify_test; set SCRUMIFY_TEST_DSN to
// point elsewhere.
func testDSN() string {
	if dsn := os.Getenv("SCRUMIFY_TEST_DSN"); dsn != "" {
		return dsn
	}
	return "postgres://scrumify:scrumify@127.0.0.1:5432/scrumify_test?sslmode=disable"
}

type fixture struct {
	ctx     context.Context
	admin   queries.Account
	unitID  int
	project int
}

func setup(t *testing.T) fixture {
	t.Helper()
	server := testutil.TestRun(t, handlers.Handlers(), func(config *testutil.Config) {
		config.Update(func(middleware *pw.MiddlewareConfig) {
			middleware.RDB = pw.RDBConfig{
				Enabled: true,
				Connections: []pw.RDBConnectionConfig{{
					DSN:            testDSN(),
					ConnectTimeout: 3 * time.Second,
					MaxOpenConns:   2,
					MaxIdleConns:   1,
				}},
			}
		})
	}, testutil.WithMigrations("../../migrations"), testutil.WithTransaction(true))

	ctx := server.Context()
	admin := queries.Account{Id: "test|admin", Kind: "human", DisplayName: "Admin", Email: "admin@example.com"}
	if _, err := queries.UpsertAccount(ctx, admin.Id, admin.DisplayName, admin.Email); err != nil {
		t.Fatal(err)
	}
	ctx = tracker.WithViewer(ctx, admin)
	projectID, err := tracker.CreateProject(ctx, "Test")
	if err != nil {
		t.Fatal(err)
	}
	units, err := collectUnits(ctx, projectID)
	if err != nil || len(units) != 1 {
		t.Fatalf("a new project should have one team: %v %v", units, err)
	}
	return fixture{ctx: ctx, admin: admin, unitID: units[0].Id, project: projectID}
}

func collectUnits(ctx context.Context, projectID int) ([]queries.Unit, error) {
	var units []queries.Unit
	for unit, err := range queries.ListUnits(ctx, projectID) {
		if err != nil {
			return nil, err
		}
		units = append(units, unit)
	}
	return units, nil
}

func (f fixture) create(t *testing.T, ticketType, title string, parent *int) int {
	t.Helper()
	id, err := tracker.CreateTicket(f.ctx, tracker.NewTicket{UnitID: f.unitID, Type: ticketType, Title: title, ParentID: parent})
	if err != nil {
		t.Fatalf("create %s %q: %v", ticketType, title, err)
	}
	return id
}

func (f fixture) ticket(t *testing.T, id int) queries.Ticket {
	t.Helper()
	ticket, err := queries.GetTicket(f.ctx, id)
	if err != nil || ticket == nil {
		t.Fatalf("ticket %d: %v", id, err)
	}
	return *ticket
}

func (f fixture) history(t *testing.T, id int) []tracker.HistoryEntry {
	t.Helper()
	entries, err := tracker.History(f.ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	return entries
}

func TestBreakdownLinksChildrenAndMarksTheStorySplit(t *testing.T) {
	f := setup(t)
	story := f.create(t, tracker.TypeStory, "story", nil)
	if got := f.ticket(t, story).Status; got != "unsorted" {
		t.Fatalf("new story status = %s", got)
	}
	item := f.create(t, tracker.TypePBI, "item", &story)

	if got := f.ticket(t, story).Status; got != "split" {
		t.Fatalf("story with a child should be split, got %s", got)
	}
	if parent := f.ticket(t, item).ParentId; parent == nil || *parent != story {
		t.Fatalf("item parent = %v, want %d", parent, story)
	}
}

func TestHierarchyRulesAreEnforced(t *testing.T) {
	f := setup(t)
	story := f.create(t, tracker.TypeStory, "story", nil)
	if _, err := tracker.CreateTicket(f.ctx, tracker.NewTicket{UnitID: f.unitID, Type: tracker.TypeTask, Title: "task", ParentID: &story}); err == nil {
		t.Fatal("a task directly under a story should be refused")
	}
	item := f.create(t, tracker.TypePBI, "item", &story)
	if _, err := tracker.CreateTicket(f.ctx, tracker.NewTicket{UnitID: f.unitID, Type: tracker.TypePBI, Title: "nested", ParentID: &item}); err == nil {
		t.Fatal("a backlog item under another backlog item should be refused")
	}
	if _, err := tracker.CreateTicket(f.ctx, tracker.NewTicket{UnitID: f.unitID, Type: tracker.TypeTask, Title: "orphan"}); err == nil {
		t.Fatal("a task without a parent should be refused")
	}
	if id := f.create(t, tracker.TypePBI, "standalone", nil); id == 0 {
		t.Fatal("a backlog item without a parent should be allowed")
	}
}

func TestSameDayEditsShareOneHistoryEntry(t *testing.T) {
	f := setup(t)
	story := f.create(t, tracker.TypeStory, "first", nil)
	for _, title := range []string{"second", "third"} {
		if err := tracker.SetText(f.ctx, story, "title", title); err != nil {
			t.Fatal(err)
		}
	}
	if err := tracker.SetText(f.ctx, story, "description", "why"); err != nil {
		t.Fatal(err)
	}

	var edits []tracker.HistoryEntry
	for _, entry := range f.history(t, story) {
		if entry.Kind == tracker.EventEdited {
			edits = append(edits, entry)
		}
	}
	if len(edits) != 1 {
		t.Fatalf("want one merged edit entry, got %d: %+v", len(edits), edits)
	}
	changes := map[string]tracker.HistoryChange{}
	for _, change := range edits[0].Changes {
		changes[change.Field] = change
	}
	if got := changes["title"]; got.From != "first" || got.To != "third" {
		t.Fatalf("title change = %+v, want first -> third", got)
	}
	if got := changes["description"]; got.From != "" || got.To != "why" {
		t.Fatalf("description change = %+v", got)
	}

	// Putting everything back leaves nothing to report for the day.
	if err := tracker.SetText(f.ctx, story, "title", "first"); err != nil {
		t.Fatal(err)
	}
	if err := tracker.SetText(f.ctx, story, "description", ""); err != nil {
		t.Fatal(err)
	}
	for _, entry := range f.history(t, story) {
		if entry.Kind == tracker.EventEdited {
			t.Fatalf("edit entry should be gone once nothing differs: %+v", entry)
		}
	}
}

func TestStatusChangesKeepOneEntryEach(t *testing.T) {
	f := setup(t)
	item := f.create(t, tracker.TypePBI, "item", nil)
	for _, status := range []string{"ready", "doing", "done"} {
		if err := tracker.SetStatus(f.ctx, item, status); err != nil {
			t.Fatal(err)
		}
	}
	count := 0
	for _, entry := range f.history(t, item) {
		if entry.Kind == tracker.EventStatus {
			count++
		}
	}
	if count != 3 {
		t.Fatalf("want 3 status entries, got %d", count)
	}
	if err := tracker.SetStatus(f.ctx, item, "split"); err == nil {
		t.Fatal("a backlog item has no split status")
	}
}

func TestMoveBeforeReordersWithinType(t *testing.T) {
	f := setup(t)
	a := f.create(t, tracker.TypePBI, "a", nil)
	b := f.create(t, tracker.TypePBI, "b", nil)
	c := f.create(t, tracker.TypePBI, "c", nil)

	order := func() []int {
		tickets, err := tracker.LoadUnitTickets(f.ctx, f.unitID)
		if err != nil {
			t.Fatal(err)
		}
		var ids []int
		for _, ticket := range tickets {
			if ticket.Type == tracker.TypePBI {
				ids = append(ids, ticket.Id)
			}
		}
		return ids
	}
	equal := func(got, want []int) bool {
		if len(got) != len(want) {
			return false
		}
		for i := range got {
			if got[i] != want[i] {
				return false
			}
		}
		return true
	}

	if err := tracker.MoveBefore(f.ctx, c, &a); err != nil {
		t.Fatal(err)
	}
	if got := order(); !equal(got, []int{c, a, b}) {
		t.Fatalf("after moving c first: %v", got)
	}
	if err := tracker.MoveBefore(f.ctx, c, nil); err != nil {
		t.Fatal(err)
	}
	if got := order(); !equal(got, []int{a, b, c}) {
		t.Fatalf("after moving c last: %v", got)
	}
	// Repeatedly inserting into the same gap must keep working.
	for range 80 {
		if err := tracker.MoveBefore(f.ctx, c, &b); err != nil {
			t.Fatal(err)
		}
		if err := tracker.MoveBefore(f.ctx, a, &b); err != nil {
			t.Fatal(err)
		}
	}
	if got := order(); !equal(got, []int{c, a, b}) {
		t.Fatalf("after many moves: %v", got)
	}
}

func TestSprintPlanningCarriesTasksAlong(t *testing.T) {
	f := setup(t)
	item := f.create(t, tracker.TypePBI, "item", nil)
	task := f.create(t, tracker.TypeTask, "task", &item)
	sprint, err := tracker.CreateSprint(f.ctx, f.unitID, "Sprint 1")
	if err != nil {
		t.Fatal(err)
	}
	if err := tracker.SetSprint(f.ctx, item, &sprint); err != nil {
		t.Fatal(err)
	}
	if got := f.ticket(t, task).SprintId; got == nil || *got != sprint {
		t.Fatalf("task sprint = %v, want %d", got, sprint)
	}
	later := f.create(t, tracker.TypeTask, "later", &item)
	if got := f.ticket(t, later).SprintId; got == nil || *got != sprint {
		t.Fatalf("a task added to a planned item should join its sprint, got %v", got)
	}
	story := f.create(t, tracker.TypeStory, "story", nil)
	if err := tracker.SetSprint(f.ctx, story, &sprint); err == nil {
		t.Fatal("only a backlog item is planned into a sprint")
	}
}

func TestDeleteHidesTheWholeSubtree(t *testing.T) {
	f := setup(t)
	story := f.create(t, tracker.TypeStory, "story", nil)
	item := f.create(t, tracker.TypePBI, "item", &story)
	task := f.create(t, tracker.TypeTask, "task", &item)
	other := f.create(t, tracker.TypePBI, "other", nil)

	if err := tracker.DeleteTicket(f.ctx, story); err != nil {
		t.Fatal(err)
	}
	tickets, err := tracker.LoadUnitTickets(f.ctx, f.unitID)
	if err != nil {
		t.Fatal(err)
	}
	if len(tickets) != 1 || tickets[0].Id != other {
		t.Fatalf("only the unrelated item should remain: %+v", tickets)
	}
	if !f.ticket(t, item).Deleted || !f.ticket(t, task).Deleted {
		t.Fatal("the rows are kept and marked deleted")
	}
}

func TestOnlyMembersSeeAProject(t *testing.T) {
	f := setup(t)
	story := f.create(t, tracker.TypeStory, "story", nil)
	outsider := queries.Account{Id: "test|outsider", Kind: "human", DisplayName: "Outsider"}
	if _, err := queries.UpsertAccount(f.ctx, outsider.Id, outsider.DisplayName, ""); err != nil {
		t.Fatal(err)
	}
	asOutsider := tracker.WithViewer(f.ctx, outsider)
	if _, err := tracker.OpenTicket(asOutsider, story); err == nil {
		t.Fatal("a non-member should not open a ticket")
	}
	if err := tracker.SetText(asOutsider, story, "title", "hijacked"); err == nil {
		t.Fatal("a non-member should not edit a ticket")
	}
	if err := tracker.AddMember(f.ctx, f.project, outsider.Id); err != nil {
		t.Fatal(err)
	}
	if _, err := tracker.OpenTicket(asOutsider, story); err != nil {
		t.Fatalf("a member should open a ticket: %v", err)
	}
	if err := tracker.SetAssignee(f.ctx, story, &outsider.Id); err != nil {
		t.Fatalf("a member can be assigned: %v", err)
	}
	if got := f.ticket(t, story).AssigneeName; got != "Outsider" {
		t.Fatalf("assignee = %q", got)
	}
}
