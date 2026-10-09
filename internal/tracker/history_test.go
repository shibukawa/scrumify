package tracker

import (
	"testing"
	"time"
)

func TestMergeChangesKeepsFirstFromAndLatestTo(t *testing.T) {
	merged := mergeChanges(map[string]Change{"title": {From: "a", To: "b"}}, "title", Change{From: "b", To: "c"})
	if got := merged["title"]; got != (Change{From: "a", To: "c"}) {
		t.Fatalf("title = %+v", got)
	}
}

func TestMergeChangesDropsFieldThatReturnsToItsStart(t *testing.T) {
	merged := mergeChanges(map[string]Change{
		"title":       {From: "a", To: "b"},
		"description": {From: "", To: "x"},
	}, "title", Change{From: "b", To: "a"})
	if _, present := merged["title"]; present {
		t.Fatalf("title should be gone: %+v", merged)
	}
	if len(merged) != 1 {
		t.Fatalf("description should remain: %+v", merged)
	}
}

func TestMergeChangesAddsNewField(t *testing.T) {
	merged := mergeChanges(map[string]Change{"title": {From: "a", To: "b"}}, "assignee", Change{From: "", To: "Ada"})
	if len(merged) != 2 || merged["assignee"] != (Change{To: "Ada"}) {
		t.Fatalf("merged = %+v", merged)
	}
}

func TestLocalDayFollowsProjectTimezone(t *testing.T) {
	// 2026-10-05 15:30 UTC is already 2026-10-06 in Tokyo.
	instant := time.Date(2026, 10, 5, 15, 30, 0, 0, time.UTC)
	if got := localDay("Asia/Tokyo", instant).Format("2006-01-02"); got != "2026-10-06" {
		t.Fatalf("Tokyo day = %s", got)
	}
	if got := localDay("UTC", instant).Format("2006-01-02"); got != "2026-10-05" {
		t.Fatalf("UTC day = %s", got)
	}
}

func TestAllowedParent(t *testing.T) {
	cases := []struct {
		child, parent string
		want          bool
	}{
		{TypePBI, TypeStory, true},
		{TypeTask, TypePBI, true},
		{TypeTask, TypeStory, false},
		{TypeStory, TypeStory, false},
		{TypePBI, TypePBI, false},
		{TypeStory, TypePBI, false},
	}
	for _, c := range cases {
		if got := allowedParent(c.child, c.parent); got != c.want {
			t.Errorf("allowedParent(%s under %s) = %v, want %v", c.child, c.parent, got, c.want)
		}
	}
}
