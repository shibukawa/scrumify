package id_

import (
	"context"
	"net/url"
	"strconv"

	"scrumify/internal/tracker"
	"scrumify/queries"
)

func LoadProject(ctx context.Context, id int) (View, error) {
	scope, err := tracker.OpenProject(ctx, id)
	if err != nil {
		return View{}, err
	}
	view := View{Id: scope.Project.Id, Name: scope.Project.Name}
	unitID, err := tracker.PrimaryUnit(ctx, id)
	if err != nil {
		return View{}, err
	}
	view.BoardPath = url.URL{Path: "/u/" + strconv.Itoa(unitID) + "/stories"}
	isMember := map[string]bool{}
	for member, err := range queries.ListProjectMembers(ctx, id) {
		if err != nil {
			return View{}, err
		}
		isMember[member.Id] = true
		view.Members = append(view.Members, Person{Id: member.Id, Name: member.DisplayName, Email: member.Email})
	}
	for account, err := range queries.ListAccounts(ctx) {
		if err != nil {
			return View{}, err
		}
		if !isMember[account.Id] {
			view.Candidates = append(view.Candidates, Person{Id: account.Id, Name: account.DisplayName, Email: account.Email})
		}
	}
	view.HasCandidates = len(view.Candidates) > 0
	return view, nil
}
