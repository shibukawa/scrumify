package tracker

import (
	"context"
	"strings"
	"time"

	"scrumify/queries"

	"github.com/shibukawa/popcornweb/pw"
)

// SprintScope is a sprint inside a unit the viewer may see.
type SprintScope struct {
	UnitScope
	Sprint queries.Sprint
}

func OpenSprint(ctx context.Context, sprintID int) (SprintScope, error) {
	sprint, err := queries.GetSprint(ctx, sprintID)
	if err != nil {
		return SprintScope{}, err
	}
	if sprint == nil {
		if _, err := CurrentViewer(ctx); err != nil {
			return SprintScope{}, err
		}
		return SprintScope{}, pw.NotFound("no such sprint")
	}
	scope, err := OpenUnit(ctx, sprint.UnitId)
	if err != nil {
		return SprintScope{}, err
	}
	return SprintScope{UnitScope: scope, Sprint: *sprint}, nil
}

func ListSprints(ctx context.Context, unitID int) ([]queries.Sprint, error) {
	return collect(queries.ListSprints(ctx, unitID))
}

func CreateSprint(ctx context.Context, unitID int, name string) (int, error) {
	scope, err := OpenUnit(ctx, unitID)
	if err != nil {
		return 0, err
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return 0, pw.BadRequest("sprint name is required")
	}
	created, err := queries.InsertSprint(ctx, scope.Unit.Id, name)
	return created.Id, err
}

// SetSprintField saves one field of a sprint as typed.
func SetSprintField(ctx context.Context, sprintID int, field, value string) error {
	scope, err := OpenSprint(ctx, sprintID)
	if err != nil {
		return err
	}
	sprint := scope.Sprint
	switch field {
	case "name":
		if strings.TrimSpace(value) == "" {
			return pw.BadRequest("sprint name is required")
		}
		sprint.Name = value
	case "goal":
		sprint.Goal = value
	case "starts_on":
		sprint.StartsOn, err = parseDate(value)
	case "ends_on":
		sprint.EndsOn, err = parseDate(value)
	case "state":
		if value != "planned" && value != "active" && value != "closed" {
			return pw.BadRequest("unknown sprint state " + value)
		}
		sprint.State = value
	default:
		return pw.BadRequest("unknown field " + field)
	}
	if err != nil {
		return err
	}
	_, err = queries.UpdateSprint(ctx, sprint.Id, sprint.Name, sprint.Goal, sprint.StartsOn, sprint.EndsOn, sprint.State)
	return err
}

func parseDate(value string) (*time.Time, error) {
	if value == "" {
		return nil, nil
	}
	parsed, err := time.Parse("2006-01-02", value)
	if err != nil {
		return nil, pw.BadRequest("a date is written YYYY-MM-DD")
	}
	return &parsed, nil
}
