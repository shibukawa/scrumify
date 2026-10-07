package pages

import (
	"context"
	"net/url"
	"strconv"

	"scrumify/internal/tracker"

	"github.com/shibukawa/popcornweb/plugin/auth"
)

// LoadChrome backs the layout. It never fails: an anonymous reader simply has
// no name to show.
func LoadChrome(ctx context.Context) Chrome {
	user, ok := auth.User(ctx)
	if !ok {
		return Chrome{}
	}
	name := user.DisplayName
	if name == "" {
		name = user.Email
	}
	return Chrome{SignedIn: true, Name: name}
}

func LoadProjects(ctx context.Context) ([]ProjectItem, error) {
	projects, err := tracker.ListProjects(ctx)
	if err != nil {
		return nil, err
	}
	items := make([]ProjectItem, 0, len(projects))
	for _, project := range projects {
		unitID, err := tracker.PrimaryUnit(ctx, project.Id)
		if err != nil {
			return nil, err
		}
		items = append(items, ProjectItem{
			Id:   project.Id,
			Name: project.Name,
			Path: url.URL{Path: "/u/" + strconv.Itoa(unitID) + "/stories"},
		})
	}
	return items, nil
}

func HasProjects(projects []ProjectItem) bool { return len(projects) > 0 }
