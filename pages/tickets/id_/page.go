package id_

import (
	"context"
	"net/url"
	"strconv"
	"strings"

	"scrumify/internal/tracker"
	"scrumify/queries"
)

func ticketPath(id int) url.URL {
	return url.URL{Path: "/tickets/" + strconv.Itoa(id)}
}

func LoadTicket(ctx context.Context, id int) (View, error) {
	scope, err := tracker.OpenTicket(ctx, id)
	if err != nil {
		return View{}, err
	}
	ticket := scope.Ticket
	unitBase := "/u/" + strconv.Itoa(scope.Unit.Id)
	view := View{
		Id:                 ticket.Id,
		KindLabel:          tracker.TypeLabel(ticket.Type),
		Title:              ticket.Title,
		Description:        ticket.Description,
		AcceptanceCriteria: ticket.AcceptanceCriteria,
		Planned:            ticket.Type == tracker.TypePBI,
		BackPath:           unitBase + "/backlog",
		Crumbs: []Crumb{
			{Label: scope.Project.Name, Path: url.URL{Path: unitBase + "/stories"}},
		},
	}
	if ticket.Type == tracker.TypeStory {
		view.BackPath = unitBase + "/stories"
	}

	tickets, err := tracker.LoadUnitTickets(ctx, scope.Unit.Id)
	if err != nil {
		return View{}, err
	}
	byID := map[int]queries.Ticket{}
	for _, other := range tickets {
		byID[other.Id] = other
		if other.ParentId != nil && *other.ParentId == ticket.Id {
			view.Children = append(view.Children, ChildItem{
				Id:          other.Id,
				KindLabel:   tracker.TypeLabel(other.Type),
				Title:       other.Title,
				StatusLabel: tracker.StatusLabel(other.Status),
				Path:        ticketPath(other.Id),
			})
		}
	}
	view.HasChildren = len(view.Children) > 0
	ancestors := []Crumb{}
	for cursor := ticket.ParentId; cursor != nil; {
		parent, ok := byID[*cursor]
		if !ok {
			break
		}
		ancestors = append([]Crumb{{Label: parent.Title, Path: ticketPath(parent.Id)}}, ancestors...)
		cursor = parent.ParentId
	}
	view.Crumbs = append(view.Crumbs, ancestors...)

	for _, status := range tracker.Statuses(ticket.Type) {
		view.Statuses = append(view.Statuses, Option{Value: status, Label: tracker.StatusLabel(status), Selected: status == ticket.Status})
	}
	view.Assignees = []Option{{Value: "", Label: "未割り当て", Selected: ticket.AssigneeId == nil}}
	for member, err := range queries.ListProjectMembers(ctx, scope.Project.Id) {
		if err != nil {
			return View{}, err
		}
		view.Assignees = append(view.Assignees, Option{
			Value:    member.Id,
			Label:    member.DisplayName,
			Selected: ticket.AssigneeId != nil && *ticket.AssigneeId == member.Id,
		})
	}
	if view.Planned {
		sprints, err := tracker.ListSprints(ctx, scope.Unit.Id)
		if err != nil {
			return View{}, err
		}
		view.Sprints = []Option{{Value: "0", Label: "バックログ", Selected: ticket.SprintId == nil}}
		for _, sprint := range sprints {
			view.Sprints = append(view.Sprints, Option{
				Value:    strconv.Itoa(sprint.Id),
				Label:    sprint.Name,
				Selected: ticket.SprintId != nil && *ticket.SprintId == sprint.Id,
			})
		}
	}

	history, err := tracker.History(ctx, ticket.Id)
	if err != nil {
		return View{}, err
	}
	for _, entry := range history {
		item := HistoryItem{
			ActorName: entry.ActorName,
			At:        entry.At.Format("2006-01-02 15:04"),
			Summary:   eventSummary(entry.Kind),
		}
		for _, change := range entry.Changes {
			item.Changes = append(item.Changes, ChangeLine{
				Label: fieldLabel(change.Field),
				From:  displayValue(change.Field, change.From),
				To:    displayValue(change.Field, change.To),
				Added: entry.Kind == tracker.EventCreated,
			})
		}
		view.History = append(view.History, item)
	}
	return view, nil
}

func eventSummary(kind string) string {
	switch kind {
	case tracker.EventCreated:
		return "が作成"
	case tracker.EventEdited:
		return "が編集"
	case tracker.EventStatus:
		return "がステータスを変更"
	case tracker.EventSprint:
		return "がスプリントを変更"
	case tracker.EventDeleted:
		return "が削除"
	case tracker.EventRestored:
		return "が復元"
	}
	return kind
}

func fieldLabel(field string) string {
	switch field {
	case "title":
		return "タイトル"
	case "description":
		return "説明"
	case "acceptance_criteria":
		return "受入条件"
	case "status":
		return "ステータス"
	case "sprint":
		return "スプリント"
	case "assignee":
		return "担当"
	case "parent":
		return "親"
	case "rank":
		return "優先順位"
	case "type":
		return "種別"
	}
	return field
}

func displayValue(field, value string) string {
	switch field {
	case "status":
		return tracker.StatusLabel(value)
	case "type":
		return tracker.TypeLabel(value)
	}
	if strings.TrimSpace(value) == "" {
		return "（なし）"
	}
	return value
}
