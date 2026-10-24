package generation

import (
	"strings"
	"testing"

	"github.com/gio-del/sumisura/backend/internal/masterdata"
)

func strPtr(s string) *string { return &s }

// consultancyMasterData is the shape that exposed the bug: one employer,
// several Client Engagements, the earliest not the one Selection led with.
func consultancyMasterData() []masterdata.Entry {
	return []masterdata.Entry{
		{ID: "experience/autostrade", Type: masterdata.TypeExperience, Employer: "Quantyca", Kind: masterdata.KindConsultancy, Client: "Autostrade", Role: "Junior Data Engineer", Start: "2024-10", End: strPtr("2025-07")},
		{ID: "experience/arcese", Type: masterdata.TypeExperience, Employer: "Quantyca", Kind: masterdata.KindConsultancy, Client: "Arcese", Role: "Junior Data Engineer", Start: "2025-07", End: strPtr("2025-11")},
		// Ongoing, and the engagement that makes the tenure continuous: without
		// it Nov 2025 -> Jun 2026 is a real gap and two Tenures is correct.
		{ID: "experience/amplifon", Type: masterdata.TypeExperience, Employer: "Quantyca", Kind: masterdata.KindConsultancy, Client: "Amplifon", Role: "Data Engineer", Start: "2025-11", End: nil},
		{ID: "experience/chiesi", Type: masterdata.TypeExperience, Employer: "Quantyca", Kind: masterdata.KindConsultancy, Client: "Chiesi", Role: "Data Engineer", Start: "2026-06", End: nil},
	}
}

func renderedText(spans ...string) string {
	return "Experience\nQuantyca S.p.A.\n" + strings.Join(spans, "\n")
}

// The original defect: the employer block took its dates from whichever
// engagement rendered first, so a two-year tenure was published as five
// months. The check must name it and say by how much.
func TestCheckTimeline_UnderstatedEmployerTenureIsFlagged(t *testing.T) {
	cv := cvData{Experience: []cvExperience{{
		Employer: "Quantyca", Kind: masterdata.KindConsultancy, Client: "Arcese",
		Start: "2025-07", End: strPtr("2025-11"),
		EmployerStart: "2025-07", EmployerEnd: strPtr("2025-11"),
	}}}

	warnings := checkTimeline(cv, consultancyMasterData(), renderedText("Jul 2025 – Nov 2025"))

	if !containsSubstring(warnings, "understates the tenure by") {
		t.Fatalf("no understatement warning in %#v", warnings)
	}
	if !containsSubstring(warnings, "Oct 2024 – present") {
		t.Errorf("warning should name the real tenure; got %#v", warnings)
	}
}

// Dropping a Client Engagement from Selection narrows what the CV shows and
// must not narrow the tenure it claims, so the full tenure stays clean.
func TestCheckTimeline_FullTenureWithOneEngagementSelectedIsClean(t *testing.T) {
	cv := cvData{Experience: []cvExperience{{
		Employer: "Quantyca", Kind: masterdata.KindConsultancy, Client: "Chiesi",
		Start: "2026-06", End: nil,
		EmployerStart: "2024-10", EmployerEnd: nil,
	}}}

	warnings := checkTimeline(cv, consultancyMasterData(), renderedText("Oct 2024 – present", "Jun 2026 – present"))

	if len(warnings) != 0 {
		t.Fatalf("want no warnings, got %#v", warnings)
	}
}

// The level that would actually have caught the bug: the assembled data was
// right and the template printed none of it.
func TestCheckTimeline_SpanMissingFromRenderedTextIsFlagged(t *testing.T) {
	cv := cvData{Experience: []cvExperience{{
		Employer: "Quantyca", Kind: masterdata.KindConsultancy, Client: "Autostrade",
		Start: "2024-10", End: strPtr("2025-07"),
		EmployerStart: "2024-10", EmployerEnd: nil,
	}}}

	// The employer tenure renders; the engagement's own span does not.
	warnings := checkTimeline(cv, consultancyMasterData(), renderedText("Oct 2024 – present"))

	if !containsSubstring(warnings, "renders no span of its own in the PDF text") {
		t.Fatalf("want the dropped-span warning, got %#v", warnings)
	}
}

func TestCheckTimeline_OngoingRenderedAsEndedIsFlagged(t *testing.T) {
	cv := cvData{Experience: []cvExperience{{
		Employer: "Quantyca", Kind: masterdata.KindConsultancy, Client: "Chiesi",
		Start: "2026-06", End: strPtr("2026-09"),
		EmployerStart: "2024-10", EmployerEnd: nil,
	}}}

	warnings := checkTimeline(cv, consultancyMasterData(), renderedText("Oct 2024 – present", "Jun 2026 – Sep 2026"))

	if !containsSubstring(warnings, "ongoing in Master Data but renders as ended") {
		t.Fatalf("want the ongoing warning, got %#v", warnings)
	}
}

// With no Master Data in hand the comparisons say nothing about the CV, so
// they must stay silent rather than report every span as unbacked.
func TestCheckTimeline_NoMasterDataSkipsComparisons(t *testing.T) {
	cv := cvData{Experience: []cvExperience{{
		Employer: "Quantyca", Kind: masterdata.KindConsultancy, Client: "Arcese",
		Start: "2025-07", End: strPtr("2025-11"), EmployerStart: "2024-10", EmployerEnd: nil,
	}}}

	warnings := checkTimeline(cv, nil, renderedText("Oct 2024 – present", "Jul 2025 – Nov 2025"))

	if len(warnings) != 0 {
		t.Fatalf("want no warnings without Master Data, got %#v", warnings)
	}
}

func TestFindSuspiciousUnicode_ZeroWidthSpaceInsideATechnologyName(t *testing.T) {
	// The failure mode: it looks identical and an ATS keyword match misses it.
	cv := cvData{TechStack: []string{"Postgre" + string(rune(0x200B)) + "SQL"}}

	warnings := findSuspiciousUnicode(cv)

	if !containsSubstring(warnings, "zero-width space") {
		t.Fatalf("want a zero-width space warning, got %#v", warnings)
	}
}

func TestFindSuspiciousUnicode_CleanTextIsSilent(t *testing.T) {
	cv := cvData{
		TechStack:  []string{"PostgreSQL", "Apache Spark"},
		Experience: []cvExperience{{Employer: "Quantyca", Bullets: []string{"Migrated several billion rows into AWS."}}},
	}

	if warnings := findSuspiciousUnicode(cv); len(warnings) != 0 {
		t.Fatalf("want no warnings, got %#v", warnings)
	}
}

func containsSubstring(haystack []string, needle string) bool {
	for _, h := range haystack {
		if strings.Contains(h, needle) {
			return true
		}
	}
	return false
}
