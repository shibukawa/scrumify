package tracker

// TypeLabel is the name of a ticket type as the screens show it.
func TypeLabel(ticketType string) string {
	switch ticketType {
	case TypeStory:
		return "ストーリー"
	case TypePBI:
		return "PBI"
	case TypeTask:
		return "タスク"
	}
	return ticketType
}

// StatusLabel is the name of a status as the screens show it.
func StatusLabel(status string) string {
	switch status {
	case "unsorted":
		return "未整理"
	case "split":
		return "分割済み"
	case "ready":
		return "Ready"
	case "in_sprint":
		return "スプリント中"
	case "todo":
		return "未着手"
	case "doing":
		return "作業中"
	case "done":
		return "完了"
	}
	return status
}

// SprintStateLabel is the name of a sprint state as the screens show it.
func SprintStateLabel(state string) string {
	switch state {
	case "planned":
		return "計画中"
	case "active":
		return "実施中"
	case "closed":
		return "終了"
	}
	return state
}
