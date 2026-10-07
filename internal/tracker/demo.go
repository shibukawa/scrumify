package tracker

import (
	"context"

	"scrumify/queries"
)

// devIssuer is where pw dev runs the development identity provider; account
// IDs are issuer + "|" + subject, so demo rows must use the same shape.
const devIssuer = "http://127.0.0.1:18080"

// DemoAccounts are the roster of devidp.toml, as the accounts they resolve to.
var DemoAccounts = []queries.Account{
	{Id: devIssuer + "|admin", Kind: "human", DisplayName: "Administrator", Email: "admin@example.com"},
	{Id: devIssuer + "|member", Kind: "human", DisplayName: "Member", Email: "member@example.com"},
}

// EnsureDemo fills an empty development database with a sample project so a
// local run has something to look at. A database that already holds a project
// is left alone.
func EnsureDemo(ctx context.Context) error {
	var any bool
	for _, err := range queries.ListProjectsForAccount(ctx, DemoAccounts[0].Id) {
		if err != nil {
			return err
		}
		any = true
		break
	}
	if any {
		return nil
	}
	for _, account := range DemoAccounts {
		if _, err := queries.UpsertAccount(ctx, account.Id, account.DisplayName, account.Email); err != nil {
			return err
		}
	}
	ctx = WithViewer(ctx, DemoAccounts[0])
	projectID, err := CreateProject(ctx, "デモプロジェクト")
	if err != nil {
		return err
	}
	for _, account := range DemoAccounts[1:] {
		if err := AddMember(ctx, projectID, account.Id); err != nil {
			return err
		}
	}
	units, err := collect(queries.ListUnits(ctx, projectID))
	if err != nil {
		return err
	}
	unitID := units[0].Id

	story := func(title, description string) int {
		id, _ := CreateTicket(ctx, NewTicket{UnitID: unitID, Type: TypeStory, Title: title})
		if description != "" {
			_ = SetText(ctx, id, "description", description)
		}
		return id
	}
	item := func(parent int, title string) int {
		id, _ := CreateTicket(ctx, NewTicket{UnitID: unitID, Type: TypePBI, Title: title, ParentID: &parent})
		return id
	}
	task := func(parent int, title, status string) {
		id, _ := CreateTicket(ctx, NewTicket{UnitID: unitID, Type: TypeTask, Title: title, ParentID: &parent})
		if status != "todo" {
			_ = SetStatus(ctx, id, status)
		}
	}

	ordering := story("チケットを優先順位で並べ替えたい", "PO がバックログの順番をドラッグで入れ替えられるようにする。")
	_ = SetText(ctx, ordering, "acceptance_criteria", "- 並び順がリロード後も保たれる\n- 変更履歴に誰が並べ替えたか残る")
	dragRow := item(ordering, "バックログの行をドラッグできる")
	saveOrder := item(ordering, "並び順を保存する")
	item(ordering, "優先順位の番号を表示する")

	board := story("スプリントボードで進捗を見たい", "デイリースクラムで PBI とタスクの状態を一目で確認する。")
	columns := item(board, "To Do / 作業中 / 完了の列を表示する")
	item(board, "PBI の中にタスクを並べる")

	history := story("変更履歴を確認したい", "")
	_ = story("レトロスペクティブの改善アクションを追跡したい", "")
	item(history, "チケット詳細に履歴を表示する")

	sprintID, err := CreateSprint(ctx, unitID, "Sprint 1")
	if err != nil {
		return err
	}
	_ = SetSprintField(ctx, sprintID, "state", "active")
	_ = SetSprintField(ctx, sprintID, "goal", "バックログの並べ替えをデモできる状態にする")
	for _, id := range []int{dragRow, saveOrder, columns} {
		_ = SetSprint(ctx, id, &sprintID)
	}
	_ = SetStatus(ctx, dragRow, "done")
	_ = SetStatus(ctx, saveOrder, "doing")
	_ = SetStatus(ctx, columns, "ready")
	member := DemoAccounts[1].Id
	_ = SetAssignee(ctx, saveOrder, &member)
	task(saveOrder, "rank 列を更新する API", "done")
	task(saveOrder, "ドラッグ後に一覧を再描画する", "doing")
	task(saveOrder, "同時編集の挙動を確認する", "todo")
	return nil
}
