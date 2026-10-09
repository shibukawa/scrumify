package handlers

import (
	"net/http"

	"scrumify/internal/tracker"

	"github.com/shibukawa/popcornweb/pw"
)

func init() {
	mux.HandleFunc("POST /api/projects", createProject)
	mux.HandleFunc("POST /api/projects/{id}/members", addMember)
	mux.HandleFunc("POST /api/units/{unit}/tickets", createTicket)
	mux.HandleFunc("POST /api/units/{unit}/sprints", createSprint)
	mux.HandleFunc("POST /api/sprints/{id}/field", setSprintField)
	mux.HandleFunc("POST /api/tickets/{id}/text", setTicketText)
	mux.HandleFunc("POST /api/tickets/{id}/status", setTicketStatus)
	mux.HandleFunc("POST /api/tickets/{id}/sprint", setTicketSprint)
	mux.HandleFunc("POST /api/tickets/{id}/assignee", setTicketAssignee)
	mux.HandleFunc("POST /api/tickets/{id}/move", moveTicket)
	mux.HandleFunc("POST /api/tickets/{id}/parent", setTicketParent)
	mux.HandleFunc("POST /api/tickets/{id}/delete", deleteTicket)
}

// Created names the record a request made.
type Created struct {
	ID int `json:"id"`
}

// Saved acknowledges a change that returns nothing else.
type Saved struct {
	OK bool `json:"ok"`
}

func saved(w http.ResponseWriter, r *http.Request, err error) {
	if err != nil {
		pw.WriteProblem(w, r, err)
		return
	}
	pw.WriteAPI(w, r, Saved{OK: true})
}

func created(w http.ResponseWriter, r *http.Request, id int, err error) {
	if err != nil {
		pw.WriteProblem(w, r, err)
		return
	}
	pw.WriteStatus(w, r, http.StatusCreated, Created{ID: id})
}

// optionalID reads the "0 means none" convention the request structs use,
// because a request field cannot be a pointer.
func optionalID(id int) *int {
	if id == 0 {
		return nil
	}
	return &id
}

type createProjectInput struct {
	Name string `payload:"name" check:"required,maxlen=200"`
}

// Create a project with the caller as its first member.
func createProject(w http.ResponseWriter, r *http.Request) {
	input, err := pw.Parse[createProjectInput](r)
	if err != nil {
		pw.WriteProblem(w, r, err)
		return
	}
	id, err := tracker.CreateProject(pw.Context(r), input.Name)
	created(w, r, id, err)
}

type addMemberInput struct {
	ProjectID int    `path:"id"`
	AccountID string `payload:"accountId" check:"required"`
}

// Add an account to a project's members.
func addMember(w http.ResponseWriter, r *http.Request) {
	input, err := pw.Parse[addMemberInput](r)
	if err != nil {
		pw.WriteProblem(w, r, err)
		return
	}
	saved(w, r, tracker.AddMember(pw.Context(r), input.ProjectID, input.AccountID))
}

type createTicketInput struct {
	UnitID int `path:"unit"`
	// Type is story, pbi, or task.
	Type  string `payload:"type" enum:"story,pbi,task"`
	Title string `payload:"title" check:"required,maxlen=500"`
	// ParentID is the ticket this one sits under, or 0 for none.
	ParentID int `payload:"parentId"`
	// SprintID plans a backlog item into a sprint at creation, or 0 for none.
	SprintID int `payload:"sprintId"`
}

// Create a ticket in a team's backlog.
func createTicket(w http.ResponseWriter, r *http.Request) {
	input, err := pw.Parse[createTicketInput](r)
	if err != nil {
		pw.WriteProblem(w, r, err)
		return
	}
	id, err := tracker.CreateTicket(pw.Context(r), tracker.NewTicket{
		UnitID:   input.UnitID,
		Type:     input.Type,
		Title:    input.Title,
		ParentID: optionalID(input.ParentID),
		SprintID: optionalID(input.SprintID),
	})
	created(w, r, id, err)
}

type createSprintInput struct {
	UnitID int    `path:"unit"`
	Name   string `payload:"name" check:"required,maxlen=200"`
}

// Create a sprint for a team.
func createSprint(w http.ResponseWriter, r *http.Request) {
	input, err := pw.Parse[createSprintInput](r)
	if err != nil {
		pw.WriteProblem(w, r, err)
		return
	}
	id, err := tracker.CreateSprint(pw.Context(r), input.UnitID, input.Name)
	created(w, r, id, err)
}

type sprintFieldInput struct {
	ID    int    `path:"id"`
	Field string `payload:"field" enum:"name,goal,starts_on,ends_on,state"`
	Value string `payload:"value"`
}

// Save one field of a sprint.
func setSprintField(w http.ResponseWriter, r *http.Request) {
	input, err := pw.Parse[sprintFieldInput](r)
	if err != nil {
		pw.WriteProblem(w, r, err)
		return
	}
	saved(w, r, tracker.SetSprintField(pw.Context(r), input.ID, input.Field, input.Value))
}

type ticketTextInput struct {
	ID    int    `path:"id"`
	Field string `payload:"field" enum:"title,description,acceptance_criteria"`
	Value string `payload:"value"`
}

// Save one text field of a ticket as it is typed.
func setTicketText(w http.ResponseWriter, r *http.Request) {
	input, err := pw.Parse[ticketTextInput](r)
	if err != nil {
		pw.WriteProblem(w, r, err)
		return
	}
	saved(w, r, tracker.SetText(pw.Context(r), input.ID, input.Field, input.Value))
}

type ticketStatusInput struct {
	ID     int    `path:"id"`
	Status string `payload:"status" check:"required"`
}

// Move a ticket to another status.
func setTicketStatus(w http.ResponseWriter, r *http.Request) {
	input, err := pw.Parse[ticketStatusInput](r)
	if err != nil {
		pw.WriteProblem(w, r, err)
		return
	}
	saved(w, r, tracker.SetStatus(pw.Context(r), input.ID, input.Status))
}

type ticketSprintInput struct {
	ID int `path:"id"`
	// SprintID is the sprint to plan the item into, or 0 to return it to the backlog.
	SprintID int `payload:"sprintId"`
}

// Plan a backlog item into a sprint, or take it back out.
func setTicketSprint(w http.ResponseWriter, r *http.Request) {
	input, err := pw.Parse[ticketSprintInput](r)
	if err != nil {
		pw.WriteProblem(w, r, err)
		return
	}
	saved(w, r, tracker.SetSprint(pw.Context(r), input.ID, optionalID(input.SprintID)))
}

type ticketAssigneeInput struct {
	ID int `path:"id"`
	// AccountID is the member who takes the ticket, or empty for nobody.
	AccountID string `payload:"accountId"`
}

// Assign a ticket to a project member.
func setTicketAssignee(w http.ResponseWriter, r *http.Request) {
	input, err := pw.Parse[ticketAssigneeInput](r)
	if err != nil {
		pw.WriteProblem(w, r, err)
		return
	}
	var accountID *string
	if input.AccountID != "" {
		accountID = &input.AccountID
	}
	saved(w, r, tracker.SetAssignee(pw.Context(r), input.ID, accountID))
}

type moveTicketInput struct {
	ID int `path:"id"`
	// BeforeID is the ticket this one is placed before, or 0 for the end.
	BeforeID int `payload:"beforeId"`
}

// Reorder a ticket among those of its own type.
func moveTicket(w http.ResponseWriter, r *http.Request) {
	input, err := pw.Parse[moveTicketInput](r)
	if err != nil {
		pw.WriteProblem(w, r, err)
		return
	}
	saved(w, r, tracker.MoveBefore(pw.Context(r), input.ID, optionalID(input.BeforeID)))
}

type ticketParentInput struct {
	ID int `path:"id"`
	// ParentID is the new parent, or 0 to detach.
	ParentID int `payload:"parentId"`
}

// Move a ticket under another parent.
func setTicketParent(w http.ResponseWriter, r *http.Request) {
	input, err := pw.Parse[ticketParentInput](r)
	if err != nil {
		pw.WriteProblem(w, r, err)
		return
	}
	saved(w, r, tracker.SetParent(pw.Context(r), input.ID, optionalID(input.ParentID)))
}

type ticketInput struct {
	ID int `path:"id"`
}

// Delete a ticket and everything below it. The rows are kept and hidden.
func deleteTicket(w http.ResponseWriter, r *http.Request) {
	input, err := pw.Parse[ticketInput](r)
	if err != nil {
		pw.WriteProblem(w, r, err)
		return
	}
	saved(w, r, tracker.DeleteTicket(pw.Context(r), input.ID))
}
