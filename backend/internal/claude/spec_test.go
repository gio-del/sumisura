package claude

import (
	"strings"
	"testing"

	"github.com/gio-del/sumisura/backend/internal/generation"
)

func TestSelectionUserPrompt_StartingPoint(t *testing.T) {
	req := generation.SelectionRequest{JobDescription: "Go engineer."}
	without, err := SelectionUserPrompt(req)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(without, "Starting point") {
		t.Errorf("no starting point expected:\n%s", without)
	}

	req.Previous = &generation.SelectionResult{Entries: []generation.SelectedEntry{{
		EntryID: "projects/emall",
		Bullets: []generation.SelectedBullet{{SourceIndex: 0, Source: "Built it.", Rewritten: "Built it well."}},
	}}}
	with, err := SelectionUserPrompt(req)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Starting point", "projects/emall", "Built it well.", "Job Description:\nGo engineer."} {
		if !strings.Contains(with, want) {
			t.Errorf("prompt missing %q:\n%s", want, with)
		}
	}
}
