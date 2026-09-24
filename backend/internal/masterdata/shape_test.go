package masterdata

import "testing"

func ptr(s string) *string { return &s }

func TestShape_RecordedKindWinsOverInference(t *testing.T) {
	// The case inference gets wrong: a Client Engagement whose client
	// cannot be named would otherwise be published as a standalone job.
	e := Entry{Type: TypeExperience, Employer: "Acme Consulting", Kind: KindConsultancy}
	if got := e.Shape(); got != KindConsultancy {
		t.Fatalf("Shape() = %q, want %q", got, KindConsultancy)
	}
}

func TestShape_InferredFromClientWhenNotRecorded(t *testing.T) {
	withClient := Entry{Type: TypeExperience, Employer: "Acme Consulting", Client: "Globex"}
	withoutClient := Entry{Type: TypeExperience, Employer: "Initech"}

	if got := withClient.Shape(); got != KindConsultancy {
		t.Errorf("with client: Shape() = %q, want %q", got, KindConsultancy)
	}
	if got := withoutClient.Shape(); got != KindInHouse {
		t.Errorf("without client: Shape() = %q, want %q", got, KindInHouse)
	}
}

// The regression this whole model change exists for: a Tenure must span
// every Entry of the employer, so selecting only some of them cannot shrink
// the employment the CV claims.
func TestEmployerTenures_SpansAllEngagementsAndStaysOpenWhenOngoing(t *testing.T) {
	entries := quantycaLike()

	tenures := EmployerTenures(entries, "Quantyca")
	if len(tenures) != 1 {
		t.Fatalf("got %d tenures, want 1: %+v", len(tenures), tenures)
	}
	if tenures[0].Start != "2024-10" {
		t.Errorf("Start = %q, want the earliest engagement's start 2024-10", tenures[0].Start)
	}
	if !tenures[0].Ongoing() {
		t.Errorf("End = %v, want ongoing — one engagement has no end", tenures[0].End)
	}
}

func TestTenureOf_IsTheSameForEveryEngagementOfOneEmployer(t *testing.T) {
	entries := quantycaLike()

	for _, e := range entries {
		if e.Employer != "Quantyca" {
			continue
		}
		tenure, ok := TenureOf(entries, e)
		if !ok {
			t.Fatalf("no tenure for %s", e.ID)
		}
		if tenure.Start != "2024-10" || !tenure.Ongoing() {
			t.Errorf("%s: tenure = %s..%v, want 2024-10..ongoing", e.ID, tenure.Start, tenure.End)
		}
	}
}

// A CV must not claim continuous employment across a gap, so two stints at
// one employer stay two Tenures rather than one span swallowing the break.
func TestEmployerTenures_GapProducesTwoTenures(t *testing.T) {
	entries := []Entry{
		{ID: "experience/a", Type: TypeExperience, Employer: "Initech", Start: "2018-01", End: ptr("2019-06")},
		{ID: "experience/b", Type: TypeExperience, Employer: "Initech", Start: "2022-03", End: ptr("2023-01")},
	}

	tenures := EmployerTenures(entries, "Initech")
	if len(tenures) != 2 {
		t.Fatalf("got %d tenures, want 2: %+v", len(tenures), tenures)
	}
	if tenures[0].Start != "2018-01" || *tenures[0].End != "2019-06" {
		t.Errorf("first stint = %s..%v, want 2018-01..2019-06", tenures[0].Start, *tenures[0].End)
	}
	if tenures[1].Start != "2022-03" || *tenures[1].End != "2023-01" {
		t.Errorf("second stint = %s..%v, want 2022-03..2023-01", tenures[1].Start, *tenures[1].End)
	}
}

// Ending one engagement and starting the next in the same or the next month
// is ordinary handover, not a break in employment.
func TestEmployerTenures_AdjacentEngagementsAreOneTenure(t *testing.T) {
	entries := []Entry{
		{ID: "experience/a", Type: TypeExperience, Employer: "Acme", Start: "2024-10", End: ptr("2025-07")},
		{ID: "experience/b", Type: TypeExperience, Employer: "Acme", Start: "2025-07", End: ptr("2025-11")},
	}

	if tenures := EmployerTenures(entries, "Acme"); len(tenures) != 1 {
		t.Fatalf("got %d tenures, want 1: %+v", len(tenures), tenures)
	}
}

func quantycaLike() []Entry {
	return []Entry{
		{ID: "experience/autostrade", Type: TypeExperience, Employer: "Quantyca", Client: "Autostrade", Start: "2024-10", End: ptr("2025-07")},
		{ID: "experience/arcese", Type: TypeExperience, Employer: "Quantyca", Client: "Arcese", Start: "2025-07", End: ptr("2025-11")},
		{ID: "experience/amplifon", Type: TypeExperience, Employer: "Quantyca", Client: "Amplifon", Start: "2025-11", End: nil},
		{ID: "experience/chiesi", Type: TypeExperience, Employer: "Quantyca", Client: "Chiesi", Start: "2026-06", End: nil},
		{ID: "experience/necst", Type: TypeExperience, Employer: "NECST", Kind: KindInHouse, Start: "2024-02", End: ptr("2025-04")},
	}
}
