package view_

import (
	"context"
	"fmt"
	"net/url"
	"strconv"

	"scrumify/internal/tracker"
	"scrumify/queries"

	"github.com/shibukawa/popcornweb/pw"
)

const (
	modeStories    = "stories"
	modeBacklog    = "backlog"
	modeSprint     = "sprint"
	modeRefinement = "refinement"
)

func unitPath(unitID int, mode string, query url.Values) url.URL {
	return url.URL{Path: "/u/" + strconv.Itoa(unitID) + "/" + mode, RawQuery: query.Encode()}
}

func ticketPath(id int) url.URL {
	return url.URL{Path: "/tickets/" + strconv.Itoa(id)}
}

func statusOptions(ticket queries.Ticket) []Option {
	options := []Option{}
	for _, status := range tracker.Statuses(ticket.Type) {
		options = append(options, Option{Value: status, Label: tracker.StatusLabel(status), Selected: status == ticket.Status})
	}
	return options
}

func assigneeOptions(members []queries.Account, ticket queries.Ticket) []Option {
	options := []Option{{Value: "", Label: "未割り当て", Selected: ticket.AssigneeId == nil}}
	for _, member := range members {
		options = append(options, Option{
			Value:    member.Id,
			Label:    member.DisplayName,
			Selected: ticket.AssigneeId != nil && *ticket.AssigneeId == member.Id,
		})
	}
	return options
}

// board holds everything one request loaded, indexed the ways the three views
// read it.
type board struct {
	scope    tracker.UnitScope
	tickets  []queries.Ticket
	byID     map[int]queries.Ticket
	children map[int][]queries.Ticket
	sprints  []queries.Sprint
	members  []queries.Account
}

func (b board) parentPath(ticket queries.Ticket) url.URL {
	if ticket.ParentId == nil {
		return url.URL{}
	}
	return ticketPath(*ticket.ParentId)
}

func (b board) parentLabel(ticket queries.Ticket) string {
	if ticket.ParentId == nil {
		return ""
	}
	parent, ok := b.byID[*ticket.ParentId]
	if !ok {
		return ""
	}
	if above := b.parentLabel(parent); above != "" {
		return above + " › " + parent.Title
	}
	return parent.Title
}

func LoadView(ctx context.Context, unit int, view string, story *int, sprint *int, layout *string) (View, error) {
	scope, err := tracker.OpenUnit(ctx, unit)
	if err != nil {
		return View{}, err
	}
	if view != modeStories && view != modeBacklog && view != modeSprint && view != modeRefinement {
		return View{}, pw.NotFound("no such view")
	}
	data := board{scope: scope, byID: map[int]queries.Ticket{}, children: map[int][]queries.Ticket{}}
	if data.tickets, err = tracker.LoadUnitTickets(ctx, unit); err != nil {
		return View{}, err
	}
	for _, ticket := range data.tickets {
		data.byID[ticket.Id] = ticket
		if ticket.ParentId != nil {
			data.children[*ticket.ParentId] = append(data.children[*ticket.ParentId], ticket)
		}
	}
	if data.sprints, err = tracker.ListSprints(ctx, unit); err != nil {
		return View{}, err
	}
	for member, err := range queries.ListProjectMembers(ctx, scope.Project.Id) {
		if err != nil {
			return View{}, err
		}
		data.members = append(data.members, member)
	}

	result := View{
		UnitId: unit,
		Mode:   view,
		Nav: Nav{
			ProjectName: scope.Project.Name,
			ProjectPath: url.URL{Path: "/projects/" + strconv.Itoa(scope.Project.Id)},
			UnitName:    scope.Unit.Name,
			Tabs: []Tab{
				{Label: "ストーリー", Path: unitPath(unit, modeStories, nil), Current: view == modeStories},
				{Label: "プロダクトバックログ", Path: unitPath(unit, modeBacklog, nil), Current: view == modeBacklog},
				{Label: "スプリントバックログ", Path: unitPath(unit, modeSprint, nil), Current: view == modeSprint},
				{Label: "リファインメント", Path: unitPath(unit, modeRefinement, nil), Current: view == modeRefinement},
			},
		},
	}
	switch view {
	case modeStories:
		result.Stories = data.stories(layout)
	case modeRefinement:
		result.Refinement = data.refinement(story)
	case modeBacklog:
		result.Backlog = data.backlog(layout)
	case modeSprint:
		result.Sprint = data.sprint(sprint, layout)
	}
	return result, nil
}

func (b board) refinement(selected *int) Refinement {
	unit := b.scope.Unit.Id
	view := Refinement{}
	for _, ticket := range b.tickets {
		if ticket.Type != tracker.TypeStory {
			continue
		}
		items := 0
		for _, child := range b.children[ticket.Id] {
			if child.Type == tracker.TypePBI {
				items++
			}
		}
		isSelected := selected != nil && *selected == ticket.Id
		view.Stories = append(view.Stories, StoryItem{
			Id:          ticket.Id,
			Title:       ticket.Title,
			StatusLabel: tracker.StatusLabel(ticket.Status),
			Summary:     fmt.Sprintf("PBI %d", items),
			Path:        unitPath(unit, modeRefinement, url.Values{"story": {strconv.Itoa(ticket.Id)}}),
			Selected:    isSelected,
		})
		if isSelected {
			view.HasSelection = true
			view.Selected = b.storyDetail(ticket)
		}
	}
	view.HasStories = len(view.Stories) > 0
	return view
}

func (b board) storyDetail(story queries.Ticket) StoryDetail {
	detail := StoryDetail{
		Id:                 story.Id,
		Title:              story.Title,
		Description:        story.Description,
		AcceptanceCriteria: story.AcceptanceCriteria,
		Statuses:           statusOptions(story),
		Path:               ticketPath(story.Id),
	}
	for _, child := range b.children[story.Id] {
		if child.Type != tracker.TypePBI {
			continue
		}
		detail.Rows = append(detail.Rows, TreeRow{
			Id:        child.Id,
			Kind:      child.Type,
			KindLabel: tracker.TypeLabel(child.Type),
			Title:     child.Title,
			Statuses:  statusOptions(child),
			Path:      ticketPath(child.Id),
		})
	}
	detail.HasRows = len(detail.Rows) > 0
	return detail
}

func (b board) sprintItems(current int) []SprintItem {
	counts := map[int]int{}
	for _, ticket := range b.tickets {
		if ticket.Type == tracker.TypePBI && ticket.SprintId != nil {
			counts[*ticket.SprintId]++
		}
	}
	items := []SprintItem{}
	for _, sprint := range b.sprints {
		items = append(items, SprintItem{
			Id:         sprint.Id,
			Name:       sprint.Name,
			StateLabel: tracker.SprintStateLabel(sprint.State),
			Count:      counts[sprint.Id],
			Path:       unitPath(b.scope.Unit.Id, modeSprint, url.Values{"sprint": {strconv.Itoa(sprint.Id)}}),
			Current:    sprint.Id == current,
		})
	}
	return items
}

// layoutSwitch describes the list/card toggle of a view and which one is on.
func (b board) layoutSwitch(mode string, layout *string) LayoutSwitch {
	cards := layout != nil && *layout == "card"
	return LayoutSwitch{
		Cards:    cards,
		ListPath: unitPath(b.scope.Unit.Id, mode, url.Values{"layout": {"list"}}),
		CardPath: unitPath(b.scope.Unit.Id, mode, url.Values{"layout": {"card"}}),
	}
}

func (b board) stories(layout *string) Stories {
	view := Stories{Layout: b.layoutSwitch(modeStories, layout)}
	for _, ticket := range b.tickets {
		if ticket.Type != tracker.TypeStory {
			continue
		}
		entry := StoryEntry{
			Id:          ticket.Id,
			Position:    len(view.Stories) + 1,
			Title:       ticket.Title,
			Description: ticket.Description,
			Statuses:    statusOptions(ticket),
			Path:        ticketPath(ticket.Id),
			RefinePath:  unitPath(b.scope.Unit.Id, modeRefinement, url.Values{"story": {strconv.Itoa(ticket.Id)}}),
		}
		for _, child := range b.children[ticket.Id] {
			if child.Type != tracker.TypePBI {
				continue
			}
			entry.Items = append(entry.Items, ItemLink{
				Id:          child.Id,
				Title:       child.Title,
				StatusLabel: tracker.StatusLabel(child.Status),
				Done:        child.Status == "done",
				Path:        ticketPath(child.Id),
			})
		}
		entry.HasItems = len(entry.Items) > 0
		entry.Summary = fmt.Sprintf("PBI %d", len(entry.Items))
		view.Stories = append(view.Stories, entry)
	}
	view.HasStories = len(view.Stories) > 0
	return view
}

func (b board) backlog(layout *string) Backlog {
	view := Backlog{Sprints: b.sprintItems(0), Layout: b.layoutSwitch(modeBacklog, layout)}
	view.HasSprints = len(view.Sprints) > 0
	for _, ticket := range b.tickets {
		if ticket.Type != tracker.TypePBI {
			continue
		}
		sprints := []Option{{Value: "0", Label: "バックログ", Selected: ticket.SprintId == nil}}
		for _, sprint := range b.sprints {
			if sprint.State == "closed" && (ticket.SprintId == nil || *ticket.SprintId != sprint.Id) {
				continue
			}
			sprints = append(sprints, Option{
				Value:    strconv.Itoa(sprint.Id),
				Label:    sprint.Name,
				Selected: ticket.SprintId != nil && *ticket.SprintId == sprint.Id,
			})
		}
		sprintLabel := ""
		for _, sprint := range b.sprints {
			if ticket.SprintId != nil && *ticket.SprintId == sprint.Id {
				sprintLabel = sprint.Name
			}
		}
		view.Rows = append(view.Rows, BacklogRow{
			Id:          ticket.Id,
			Position:    len(view.Rows) + 1,
			Title:       ticket.Title,
			ParentLabel: b.parentLabel(ticket),
			ParentPath:  b.parentPath(ticket),
			StatusLabel: tracker.StatusLabel(ticket.Status),
			SprintLabel: sprintLabel,
			Statuses:    statusOptions(ticket),
			Sprints:     sprints,
			Path:        ticketPath(ticket.Id),
		})
	}
	view.HasRows = len(view.Rows) > 0
	return view
}

// pickSprint chooses the sprint to show when the URL names none: the one in
// progress, else the newest.
func (b board) pickSprint(requested *int) (queries.Sprint, bool) {
	if requested != nil {
		for _, sprint := range b.sprints {
			if sprint.Id == *requested {
				return sprint, true
			}
		}
		return queries.Sprint{}, false
	}
	for _, sprint := range b.sprints {
		if sprint.State == "active" {
			return sprint, true
		}
	}
	if len(b.sprints) > 0 {
		return b.sprints[len(b.sprints)-1], true
	}
	return queries.Sprint{}, false
}

func dateValue(sprintDate interface{ Format(string) string }) string {
	return sprintDate.Format("2006-01-02")
}

func (b board) sprint(requested *int, layout *string) SprintView {
	unit := b.scope.Unit.Id
	sprint, ok := b.pickSprint(requested)
	if !ok {
		return SprintView{BacklogPath: unitPath(unit, modeBacklog, nil)}
	}
	isBoard := layout == nil || *layout != "list"
	query := func(layoutName string) url.Values {
		return url.Values{"sprint": {strconv.Itoa(sprint.Id)}, "layout": {layoutName}}
	}
	view := SprintView{
		Exists:      true,
		Id:          sprint.Id,
		Name:        sprint.Name,
		Goal:        sprint.Goal,
		Board:       isBoard,
		BoardPath:   unitPath(unit, modeSprint, query("board")),
		ListPath:    unitPath(unit, modeSprint, query("list")),
		BacklogPath: unitPath(unit, modeBacklog, nil),
		Sprints:     b.sprintItems(sprint.Id),
	}
	if sprint.StartsOn != nil {
		view.StartsOn = dateValue(*sprint.StartsOn)
	}
	if sprint.EndsOn != nil {
		view.EndsOn = dateValue(*sprint.EndsOn)
	}
	for _, state := range []string{"planned", "active", "closed"} {
		view.States = append(view.States, Option{Value: state, Label: tracker.SprintStateLabel(state), Selected: state == sprint.State})
	}

	columns := []Column{
		{Key: "todo", Label: "To Do", DropStatus: "ready"},
		{Key: "doing", Label: "作業中", DropStatus: "doing"},
		{Key: "done", Label: "完了", DropStatus: "done"},
	}
	for _, ticket := range b.tickets {
		if ticket.Type != tracker.TypePBI || ticket.SprintId == nil || *ticket.SprintId != sprint.Id {
			continue
		}
		card := Card{
			Id:          ticket.Id,
			Title:       ticket.Title,
			ParentLabel: b.parentLabel(ticket),
			Path:        ticketPath(ticket.Id),
			Statuses:    statusOptions(ticket),
			Assignees:   assigneeOptions(b.members, ticket),
			Doing:       ticket.Status == "doing",
		}
		for _, child := range b.children[ticket.Id] {
			if child.Type == tracker.TypeTask {
				card.Tasks = append(card.Tasks, taskItem(child))
			}
		}
		column := 0
		switch ticket.Status {
		case "doing":
			column = 1
		case "done":
			column = 2
		}
		columns[column].Cards = append(columns[column].Cards, card)
		view.Cards = append(view.Cards, card)
	}
	for index := range columns {
		columns[index].Count = len(columns[index].Cards)
	}
	view.Columns = columns
	view.HasCards = len(view.Cards) > 0
	return view
}

// taskItem carries how a task looks and what one click on its marker does:
// the marker walks todo, doing, done and back.
func taskItem(task queries.Ticket) TaskItem {
	item := TaskItem{Id: task.Id, Title: task.Title, StatusLabel: tracker.StatusLabel(task.Status), Statuses: statusOptions(task)}
	switch task.Status {
	case "doing":
		item.Mark, item.Tone, item.NextStatus = "◐", "text-amber-500", "done"
	case "done":
		item.Mark, item.Tone, item.NextStatus = "●", "text-emerald-600", "todo"
		item.Done = true
	default:
		item.Mark, item.Tone, item.NextStatus = "○", "text-slate-400", "doing"
	}
	return item
}
