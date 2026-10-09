// Package tracker holds the domain rules of the ticket service: who may see
// what, how tickets relate, and how changes are recorded.
package tracker

import (
	"context"
	"strings"

	"scrumify/queries"

	"github.com/shibukawa/popcornweb/plugin/auth"
	"github.com/shibukawa/popcornweb/pw"
)

const loginPath = "/auth/login"

type viewerKey struct{}

// WithViewer makes account the viewer of ctx without a browser session. It is
// for callers that have already established who is acting, such as tests; the
// account row must exist.
func WithViewer(ctx context.Context, account queries.Account) context.Context {
	return context.WithValue(ctx, viewerKey{}, account)
}

// CurrentViewer returns the signed-in account, creating its row on first use.
func CurrentViewer(ctx context.Context) (queries.Account, error) {
	if account, ok := ctx.Value(viewerKey{}).(queries.Account); ok {
		return account, nil
	}
	user, ok := auth.User(ctx)
	if !ok {
		return queries.Account{}, pw.SeeOther(loginPath)
	}
	name := user.DisplayName
	if name == "" {
		name = user.Email
	}
	if name == "" {
		name = user.AccountID
	}
	account, err := queries.GetAccount(ctx, user.AccountID)
	if err != nil {
		return queries.Account{}, err
	}
	if account == nil || account.DisplayName != name || account.Email != user.Email {
		if _, err := queries.UpsertAccount(ctx, user.AccountID, name, user.Email); err != nil {
			return queries.Account{}, err
		}
		return queries.Account{Id: user.AccountID, Kind: "human", DisplayName: name, Email: user.Email}, nil
	}
	return *account, nil
}

// ProjectScope is a project the viewer is a member of.
type ProjectScope struct {
	Viewer  queries.Account
	Project queries.Project
}

// UnitScope is a unit inside such a project.
type UnitScope struct {
	ProjectScope
	Unit queries.Unit
}

// TicketScope is a ticket inside such a unit.
type TicketScope struct {
	UnitScope
	Ticket queries.Ticket
}

func OpenProject(ctx context.Context, projectID int) (ProjectScope, error) {
	viewer, err := CurrentViewer(ctx)
	if err != nil {
		return ProjectScope{}, err
	}
	project, err := queries.GetProjectForMember(ctx, projectID, viewer.Id)
	if err != nil {
		return ProjectScope{}, err
	}
	if project == nil {
		return ProjectScope{}, pw.NotFound("no such project")
	}
	return ProjectScope{Viewer: viewer, Project: *project}, nil
}

func OpenUnit(ctx context.Context, unitID int) (UnitScope, error) {
	unit, err := queries.GetUnit(ctx, unitID)
	if err != nil {
		return UnitScope{}, err
	}
	if unit == nil {
		if _, err := CurrentViewer(ctx); err != nil {
			return UnitScope{}, err
		}
		return UnitScope{}, pw.NotFound("no such unit")
	}
	scope, err := OpenProject(ctx, unit.ProjectId)
	if err != nil {
		return UnitScope{}, err
	}
	return UnitScope{ProjectScope: scope, Unit: *unit}, nil
}

func OpenTicket(ctx context.Context, ticketID int) (TicketScope, error) {
	ticket, err := queries.GetTicket(ctx, ticketID)
	if err != nil {
		return TicketScope{}, err
	}
	if ticket == nil {
		if _, err := CurrentViewer(ctx); err != nil {
			return TicketScope{}, err
		}
		return TicketScope{}, pw.NotFound("no such ticket")
	}
	scope, err := OpenUnit(ctx, ticket.UnitId)
	if err != nil {
		return TicketScope{}, err
	}
	return TicketScope{UnitScope: scope, Ticket: *ticket}, nil
}

func ListProjects(ctx context.Context) ([]queries.Project, error) {
	viewer, err := CurrentViewer(ctx)
	if err != nil {
		return nil, err
	}
	return collect(queries.ListProjectsForAccount(ctx, viewer.Id))
}

// CreateProject makes the viewer its first member and gives it one team, so a
// new project is usable without visiting any settings.
func CreateProject(ctx context.Context, name string) (int, error) {
	viewer, err := CurrentViewer(ctx)
	if err != nil {
		return 0, err
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return 0, pw.BadRequest("project name is required")
	}
	var projectID int
	err = pw.TransactionContext(ctx, func(ctx context.Context) error {
		created, err := queries.InsertProject(ctx, name, DefaultTimezone)
		if err != nil {
			return err
		}
		projectID = created.Id
		if _, err := queries.AddProjectMember(ctx, projectID, viewer.Id); err != nil {
			return err
		}
		_, err = queries.InsertUnit(ctx, projectID, nil, "team", name)
		return err
	})
	return projectID, err
}

func AddMember(ctx context.Context, projectID int, accountID string) error {
	if _, err := OpenProject(ctx, projectID); err != nil {
		return err
	}
	account, err := queries.GetAccount(ctx, accountID)
	if err != nil {
		return err
	}
	if account == nil {
		return pw.BadRequest("no such account")
	}
	_, err = queries.AddProjectMember(ctx, projectID, accountID)
	return err
}

// PrimaryUnit is the project's one team. Until Scrum@Scale levels are
// designed, every project has exactly one unit and the screens never ask
// which.
func PrimaryUnit(ctx context.Context, projectID int) (int, error) {
	units, err := collect(queries.ListUnits(ctx, projectID))
	if err != nil {
		return 0, err
	}
	if len(units) == 0 {
		return 0, pw.NotFound("the project has no team")
	}
	return units[0].Id, nil
}

func collect[T any](rows func(func(T, error) bool)) ([]T, error) {
	result := make([]T, 0)
	for row, err := range rows {
		if err != nil {
			return nil, err
		}
		result = append(result, row)
	}
	return result, nil
}
