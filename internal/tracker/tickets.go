package tracker

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"scrumify/queries"

	"github.com/shibukawa/popcornweb/pw"
)

// Ticket types of the first milestone.
const (
	TypeStory = "story"
	TypePBI   = "pbi"
	TypeTask  = "task"
)

// Statuses allowed for a type, in the order a ticket normally passes through.
func Statuses(ticketType string) []string {
	switch ticketType {
	case TypeStory:
		return []string{"unsorted", "split", "ready", "in_sprint", "done"}
	case TypePBI:
		return []string{"unsorted", "ready", "doing", "done"}
	case TypeTask:
		return []string{"todo", "doing", "done"}
	}
	return nil
}

func validStatus(ticketType, status string) bool {
	for _, candidate := range Statuses(ticketType) {
		if candidate == status {
			return true
		}
	}
	return false
}

// allowedParent says which type may sit directly above another.
func allowedParent(childType, parentType string) bool {
	switch childType {
	case TypePBI:
		return parentType == TypeStory
	case TypeTask:
		return parentType == TypePBI
	}
	return false
}

func LoadUnitTickets(ctx context.Context, unitID int) ([]queries.Ticket, error) {
	return collect(queries.ListUnitTickets(ctx, unitID))
}

// NewTicket describes a ticket to create.
type NewTicket struct {
	UnitID   int
	Type     string
	Title    string
	ParentID *int
	SprintID *int
}

func CreateTicket(ctx context.Context, input NewTicket) (int, error) {
	scope, err := OpenUnit(ctx, input.UnitID)
	if err != nil {
		return 0, err
	}
	statuses := Statuses(input.Type)
	if statuses == nil {
		return 0, pw.BadRequest("unknown ticket type")
	}
	title := strings.TrimSpace(input.Title)
	if title == "" {
		return 0, pw.BadRequest("title is required")
	}
	var ticketID int
	err = pw.TransactionContext(ctx, func(ctx context.Context) error {
		var parent *queries.Ticket
		if input.ParentID != nil {
			parent, err = queries.GetTicket(ctx, *input.ParentID)
			if err != nil {
				return err
			}
			if parent == nil || parent.Deleted || parent.UnitId != scope.Unit.Id {
				return pw.BadRequest("no such parent ticket")
			}
			if !allowedParent(input.Type, parent.Type) {
				return pw.BadRequest(fmt.Sprintf("a %s cannot be placed under a %s", input.Type, parent.Type))
			}
		} else if input.Type == TypeTask {
			return pw.BadRequest(fmt.Sprintf("a %s needs a parent", input.Type))
		}
		sprintID := input.SprintID
		if input.Type == TypeTask && parent != nil {
			sprintID = parent.SprintId
		}
		if sprintID != nil {
			sprint, err := queries.GetSprint(ctx, *sprintID)
			if err != nil {
				return err
			}
			if sprint == nil || sprint.UnitId != scope.Unit.Id {
				return pw.BadRequest("no such sprint")
			}
		}
		top, err := queries.MaxRank(ctx, scope.Unit.Id)
		if err != nil {
			return err
		}
		created, err := queries.InsertTicket(ctx, scope.Project.Id, scope.Unit.Id, input.Type, title,
			statuses[0], sprintID, top.Value+1, scope.Viewer.Id)
		if err != nil {
			return err
		}
		ticketID = created.Id
		changes := map[string]Change{"type": {To: input.Type}, "title": {To: title}}
		if parent != nil {
			if _, err := queries.InsertParentLink(ctx, parent.Id, ticketID); err != nil {
				return err
			}
			changes["parent"] = Change{To: ticketLabel(*parent)}
		}
		if err := recordEvent(ctx, scope, ticketID, EventCreated, changes); err != nil {
			return err
		}
		// Gaining a first child is a fact rather than a judgment, so an
		// unsorted story moves to split on its own.
		if parent != nil && parent.Status == "unsorted" && parent.Type == TypeStory {
			return applyStatus(ctx, scope, *parent, "split")
		}
		return nil
	})
	return ticketID, err
}

func ticketLabel(ticket queries.Ticket) string {
	return "#" + strconv.Itoa(ticket.Id) + " " + ticket.Title
}

// lockedTicket runs fn with the ticket row locked, so concurrent autosaves of
// one ticket apply and record one after another.
func lockedTicket(ctx context.Context, ticketID int, fn func(ctx context.Context, scope TicketScope) error) error {
	scope, err := OpenTicket(ctx, ticketID)
	if err != nil {
		return err
	}
	if scope.Ticket.Deleted {
		return pw.NotFound("no such ticket")
	}
	return pw.TransactionContext(ctx, func(ctx context.Context) error {
		if _, err := queries.LockTicket(ctx, ticketID); err != nil {
			return err
		}
		current, err := queries.GetTicket(ctx, ticketID)
		if err != nil {
			return err
		}
		if current == nil {
			return pw.NotFound("no such ticket")
		}
		scope.Ticket = *current
		return fn(ctx, scope)
	})
}

// SetText saves one free-text field as typed.
func SetText(ctx context.Context, ticketID int, field, value string) error {
	return lockedTicket(ctx, ticketID, func(ctx context.Context, scope TicketScope) error {
		ticket := scope.Ticket
		var previous string
		var err error
		switch field {
		case "title":
			previous = ticket.Title
			_, err = queries.SetTicketTitle(ctx, ticket.Id, value)
		case "description":
			previous = ticket.Description
			_, err = queries.SetTicketDescription(ctx, ticket.Id, value)
		case "acceptance_criteria":
			previous = ticket.AcceptanceCriteria
			_, err = queries.SetTicketAcceptanceCriteria(ctx, ticket.Id, value)
		default:
			return pw.BadRequest("unknown field " + field)
		}
		if err != nil {
			return err
		}
		return recordEdit(ctx, scope.UnitScope, ticket.Id, field, Change{From: previous, To: value})
	})
}

func SetStatus(ctx context.Context, ticketID int, status string) error {
	return lockedTicket(ctx, ticketID, func(ctx context.Context, scope TicketScope) error {
		if !validStatus(scope.Ticket.Type, status) {
			return pw.BadRequest("a " + scope.Ticket.Type + " has no status " + status)
		}
		return applyStatus(ctx, scope.UnitScope, scope.Ticket, status)
	})
}

func applyStatus(ctx context.Context, scope UnitScope, ticket queries.Ticket, status string) error {
	if ticket.Status == status {
		return nil
	}
	if _, err := queries.SetTicketStatus(ctx, ticket.Id, status); err != nil {
		return err
	}
	return recordEvent(ctx, scope, ticket.Id, EventStatus, map[string]Change{"status": {From: ticket.Status, To: status}})
}

// SetSprint moves a backlog item into a sprint, or back out with nil. Its
// tasks follow, because a task only exists inside its item's sprint.
func SetSprint(ctx context.Context, ticketID int, sprintID *int) error {
	return lockedTicket(ctx, ticketID, func(ctx context.Context, scope TicketScope) error {
		ticket := scope.Ticket
		if ticket.Type != TypePBI {
			return pw.BadRequest("only a backlog item is planned into a sprint")
		}
		if equalInt(ticket.SprintId, sprintID) {
			return nil
		}
		from, err := sprintName(ctx, scope.UnitScope, ticket.SprintId)
		if err != nil {
			return err
		}
		to, err := sprintName(ctx, scope.UnitScope, sprintID)
		if err != nil {
			return err
		}
		if _, err := queries.SetTicketSprint(ctx, ticket.Id, sprintID); err != nil {
			return err
		}
		tickets, err := LoadUnitTickets(ctx, scope.Unit.Id)
		if err != nil {
			return err
		}
		for _, child := range tickets {
			if child.Type == TypeTask && child.ParentId != nil && *child.ParentId == ticket.Id {
				if _, err := queries.SetTicketSprint(ctx, child.Id, sprintID); err != nil {
					return err
				}
			}
		}
		return recordEvent(ctx, scope.UnitScope, ticket.Id, EventSprint, map[string]Change{"sprint": {From: from, To: to}})
	})
}

func sprintName(ctx context.Context, scope UnitScope, sprintID *int) (string, error) {
	if sprintID == nil {
		return "", nil
	}
	sprint, err := queries.GetSprint(ctx, *sprintID)
	if err != nil {
		return "", err
	}
	if sprint == nil || sprint.UnitId != scope.Unit.Id {
		return "", pw.BadRequest("no such sprint")
	}
	return sprint.Name, nil
}

func SetAssignee(ctx context.Context, ticketID int, accountID *string) error {
	return lockedTicket(ctx, ticketID, func(ctx context.Context, scope TicketScope) error {
		ticket := scope.Ticket
		if equalString(ticket.AssigneeId, accountID) {
			return nil
		}
		to := ""
		if accountID != nil {
			members, err := collect(queries.ListProjectMembers(ctx, scope.Project.Id))
			if err != nil {
				return err
			}
			found := false
			for _, member := range members {
				if member.Id == *accountID {
					to, found = member.DisplayName, true
				}
			}
			if !found {
				return pw.BadRequest("the assignee must be a project member")
			}
		}
		if _, err := queries.SetTicketAssignee(ctx, ticket.Id, accountID); err != nil {
			return err
		}
		return recordEdit(ctx, scope.UnitScope, ticket.Id, "assignee", Change{From: ticket.AssigneeName, To: to})
	})
}

// MoveBefore reorders a ticket among those of its own type: before the named
// ticket, or to the end with nil.
func MoveBefore(ctx context.Context, ticketID int, beforeID *int) error {
	return lockedTicket(ctx, ticketID, func(ctx context.Context, scope TicketScope) error {
		ticket := scope.Ticket
		all, err := LoadUnitTickets(ctx, scope.Unit.Id)
		if err != nil {
			return err
		}
		peers := make([]queries.Ticket, 0, len(all))
		oldPosition := 0
		for _, candidate := range all {
			if candidate.Type != ticket.Type {
				continue
			}
			if candidate.Id == ticket.Id {
				oldPosition = len(peers) + 1
				continue
			}
			peers = append(peers, candidate)
		}
		index := len(peers)
		if beforeID != nil {
			index = -1
			for position, peer := range peers {
				if peer.Id == *beforeID {
					index = position
				}
			}
			if index < 0 {
				return pw.BadRequest("no such ticket to move before")
			}
		}
		if index+1 == oldPosition {
			return nil
		}
		var rank float64
		switch {
		case len(peers) == 0:
			rank = ticket.Rank
		case index == 0:
			rank = peers[0].Rank - 1
		case index == len(peers):
			rank = peers[len(peers)-1].Rank + 1
		default:
			rank = (peers[index-1].Rank + peers[index].Rank) / 2
			if rank <= peers[index-1].Rank || rank >= peers[index].Rank {
				// The gap between two neighbours ran out of precision.
				return renumber(ctx, scope, all, ticket, peers[index].Id, oldPosition, index+1)
			}
		}
		if _, err := queries.SetTicketRank(ctx, ticket.Id, rank); err != nil {
			return err
		}
		return recordEdit(ctx, scope.UnitScope, ticket.Id, "rank",
			Change{From: strconv.Itoa(oldPosition), To: strconv.Itoa(index + 1)})
	})
}

func renumber(ctx context.Context, scope TicketScope, all []queries.Ticket, moved queries.Ticket, beforeID, oldPosition, newPosition int) error {
	ordered := make([]queries.Ticket, 0, len(all))
	for _, candidate := range all {
		if candidate.Id == moved.Id {
			continue
		}
		if candidate.Id == beforeID {
			ordered = append(ordered, moved)
		}
		ordered = append(ordered, candidate)
	}
	for position, ticket := range ordered {
		if _, err := queries.SetTicketRank(ctx, ticket.Id, float64(position+1)); err != nil {
			return err
		}
	}
	return recordEdit(ctx, scope.UnitScope, moved.Id, "rank",
		Change{From: strconv.Itoa(oldPosition), To: strconv.Itoa(newPosition)})
}

// SetParent moves a ticket under another parent, or detaches it with nil.
func SetParent(ctx context.Context, ticketID int, parentID *int) error {
	return lockedTicket(ctx, ticketID, func(ctx context.Context, scope TicketScope) error {
		ticket := scope.Ticket
		if equalInt(ticket.ParentId, parentID) {
			return nil
		}
		from := ""
		if ticket.ParentId != nil {
			previous, err := queries.GetTicket(ctx, *ticket.ParentId)
			if err != nil {
				return err
			}
			if previous != nil {
				from = ticketLabel(*previous)
			}
		}
		to := ""
		if parentID != nil {
			parent, err := queries.GetTicket(ctx, *parentID)
			if err != nil {
				return err
			}
			if parent == nil || parent.Deleted || parent.UnitId != scope.Unit.Id {
				return pw.BadRequest("no such parent ticket")
			}
			if !allowedParent(ticket.Type, parent.Type) {
				return pw.BadRequest(fmt.Sprintf("a %s cannot be placed under a %s", ticket.Type, parent.Type))
			}
			to = ticketLabel(*parent)
		} else if ticket.Type == TypeTask {
			return pw.BadRequest(fmt.Sprintf("a %s needs a parent", ticket.Type))
		}
		if _, err := queries.CloseParentLink(ctx, ticket.Id); err != nil {
			return err
		}
		if parentID != nil {
			if _, err := queries.InsertParentLink(ctx, *parentID, ticket.Id); err != nil {
				return err
			}
		}
		return recordEdit(ctx, scope.UnitScope, ticket.Id, "parent", Change{From: from, To: to})
	})
}

// DeleteTicket hides a ticket and everything below it. Nothing is removed.
func DeleteTicket(ctx context.Context, ticketID int) error {
	return lockedTicket(ctx, ticketID, func(ctx context.Context, scope TicketScope) error {
		all, err := LoadUnitTickets(ctx, scope.Unit.Id)
		if err != nil {
			return err
		}
		doomed := []int{scope.Ticket.Id}
		for cursor := 0; cursor < len(doomed); cursor++ {
			for _, candidate := range all {
				if candidate.ParentId != nil && *candidate.ParentId == doomed[cursor] {
					doomed = append(doomed, candidate.Id)
				}
			}
		}
		for _, id := range doomed {
			if _, err := queries.SetTicketDeleted(ctx, id, true); err != nil {
				return err
			}
			if err := recordEvent(ctx, scope.UnitScope, id, EventDeleted, nil); err != nil {
				return err
			}
		}
		return nil
	})
}

func equalInt(a, b *int) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}

func equalString(a, b *string) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}
