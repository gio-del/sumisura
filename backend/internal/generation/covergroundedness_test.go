package generation

import (
	"testing"

	"github.com/gio-del/sumisura/backend/internal/masterdata"
)

func coverLetterFixture() ([]masterdata.Entry, []masterdata.Snippet, masterdata.Profile) {
	end := "2023-06"
	entries := []masterdata.Entry{{
		ID: "experience/acme", Type: "experience", Employer: "Acme", Start: "2021-03", End: &end,
		Tags:    []string{"Go", "PostgreSQL"},
		Bullets: []string{"Cut p95 latency by 40% on the Go billing API backed by PostgreSQL."},
	}}
	snippets := []masterdata.Snippet{{ID: "closing", Kind: "closing", Body: "I enjoy mentoring junior engineers."}}
	return entries, snippets, masterdata.Profile{Name: "Ada Rossi", Location: "Milan"}
}

func TestCheckCoverLetterGroundedness_flagsUnsupportedFigureAndTool(t *testing.T) {
	entries, snippets, profile := coverLetterFixture()
	letter := "Dear Hiring Manager,\n\nAt Acme I cut latency by 70% and ran everything on Kubernetes."
	flags := CheckCoverLetterGroundedness(letter, entries, snippets, profile)

	terms := map[string]bool{}
	for _, f := range flags {
		if f.Reason != ReasonUnsupportedTerm {
			t.Errorf("reason = %q, want %q", f.Reason, ReasonUnsupportedTerm)
		}
		terms[f.Term] = true
	}
	if !terms["70"] || !terms["Kubernetes"] || len(terms) != 2 {
		t.Errorf("flagged terms = %v, want exactly 70 and Kubernetes", terms)
	}
}

func TestCheckCoverLetterGroundedness_noFlagsWhenBuiltFromSources(t *testing.T) {
	entries, snippets, profile := coverLetterFixture()
	letter := "Dear Hiring Manager,\n\nSince 2021 at Acme I cut p95 latency by 40% on a Go API backed by PostgreSQL. I enjoy mentoring junior engineers.\n\nKind regards,\nAda Rossi"
	if flags := CheckCoverLetterGroundedness(letter, entries, snippets, profile); len(flags) != 0 {
		t.Errorf("flags = %+v, want none", flags)
	}
}

func TestCheckCoverLetterGroundedness_emptyLetter(t *testing.T) {
	entries, snippets, profile := coverLetterFixture()
	if flags := CheckCoverLetterGroundedness("  ", entries, snippets, profile); flags != nil {
		t.Errorf("flags = %+v, want nil", flags)
	}
}

func TestCheckCoverLetterGroundedness_flagsEachTermOnce(t *testing.T) {
	entries, snippets, profile := coverLetterFixture()
	letter := "I used Terraform daily. Terraform also ran in CI."
	if flags := CheckCoverLetterGroundedness(letter, entries, snippets, profile); len(flags) != 1 || flags[0].Term != "Terraform" {
		t.Errorf("flags = %+v, want one for Terraform", flags)
	}
}
