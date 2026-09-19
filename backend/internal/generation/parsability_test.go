package generation

import (
	"path/filepath"
	"reflect"
	"testing"
)

func TestCheckParsability(t *testing.T) {
	fields := []expectedField{
		{label: "Name", text: "Jane Doe"},
		{label: "Education section header", text: "Education"},
		{label: "Experience section header", text: "Experience"},
		{label: "Experience employer \"Acme Corp\"", text: "Acme Corp"},
		{label: "Projects section header", text: "Projects"},
		{label: "Project \"Widget Tool\"", text: "Widget Tool"},
	}

	tests := []struct {
		name           string
		extractedText  string
		fields         []expectedField
		wantStatus     ParsabilityStatus
		wantMissing    []string
		wantViolations []string
	}{
		{
			name: "happy path: all fields present in order",
			extractedText: "Jane Doe\n" +
				"Education\nMSc Computer Science\n" +
				"Experience\nAcme Corp\nSenior Engineer\n" +
				"Projects\nWidget Tool\n",
			fields:     fields,
			wantStatus: ParsabilityOK,
		},
		{
			name: "missing field: employer absent from extracted text",
			extractedText: "Jane Doe\n" +
				"Education\nMSc Computer Science\n" +
				"Experience\n" +
				"Projects\nWidget Tool\n",
			fields:      fields,
			wantStatus:  ParsabilityWarning,
			wantMissing: []string{"Experience employer \"Acme Corp\""},
		},
		{
			name: "scrambled order: Experience section renders before Education",
			extractedText: "Jane Doe\n" +
				"Experience\nAcme Corp\n" +
				"Education\nMSc Computer Science\n" +
				"Projects\nWidget Tool\n",
			fields:     fields,
			wantStatus: ParsabilityWarning,
			wantViolations: []string{
				"Experience section header appears before Education section header in the extracted text",
				"Experience employer \"Acme Corp\" appears before Education section header in the extracted text",
			},
		},
		{
			name:          "empty extraction: every field missing",
			extractedText: "",
			fields: []expectedField{
				{label: "Name", text: "Jane Doe"},
				{label: "Education section header", text: "Education"},
			},
			wantStatus:  ParsabilityWarning,
			wantMissing: []string{"Name", "Education section header"},
		},
		{
			name:          "no expected fields: trivially ok",
			extractedText: "anything at all",
			fields:        nil,
			wantStatus:    ParsabilityOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := checkParsability(tt.extractedText, tt.fields)
			if got.Status != tt.wantStatus {
				t.Errorf("Status = %v, want %v", got.Status, tt.wantStatus)
			}
			if !reflect.DeepEqual(got.MissingFields, orNilStrings(tt.wantMissing)) {
				t.Errorf("MissingFields = %v, want %v", got.MissingFields, tt.wantMissing)
			}
			if !reflect.DeepEqual(got.OrderingViolations, orNilStrings(tt.wantViolations)) {
				t.Errorf("OrderingViolations = %v, want %v", got.OrderingViolations, tt.wantViolations)
			}
		})
	}
}

// orNilStrings normalizes an empty-but-non-nil slice to nil, matching
// checkParsability's zero-value var declarations (append on a nil slice
// that never appends stays nil), so table cases can omit the field for
// "expect none" instead of writing []string{}.
func orNilStrings(s []string) []string {
	if len(s) == 0 {
		return nil
	}
	return s
}

func TestCVExpectedFields(t *testing.T) {
	cv := cvData{
		Name: "Jane Doe",
		Experience: []cvExperience{
			{Employer: "Acme Corp"},
			{Employer: "Acme Corp"}, // same employer twice (e.g. two Client Engagements) — expect one field, not two
			{Employer: "Globex"},
		},
		Projects:  []cvProject{{Name: "Widget Tool"}},
		TechStack: []string{"Go", "React"},
	}

	fields := cvExpectedFields(cv)

	var labels []string
	for _, f := range fields {
		labels = append(labels, f.label)
	}
	want := []string{
		"Name",
		"Education section header",
		"Experience section header",
		"Experience employer \"Acme Corp\"",
		"Experience employer \"Globex\"",
		"Projects section header",
		"Project \"Widget Tool\"",
		"Tech Stack section header",
	}
	if !reflect.DeepEqual(labels, want) {
		t.Errorf("labels = %v, want %v", labels, want)
	}
}

func TestCVExpectedFields_NoProjectsOrTechStack_OmitsThoseHeaders(t *testing.T) {
	cv := cvData{Name: "Jane Doe"}
	fields := cvExpectedFields(cv)
	for _, f := range fields {
		if f.label == "Projects section header" || f.label == "Tech Stack section header" {
			t.Errorf("expected %q to be omitted when empty, got fields %v", f.label, fields)
		}
	}
}

func TestCoverLetterExpectedFields(t *testing.T) {
	cl := coverLetterData{
		Name: "Jane Doe",
		Body: "\n  Dear Hiring Manager,\nI'm excited to apply.\n",
	}
	fields := coverLetterExpectedFields(cl)
	want := []expectedField{
		{label: "Name", group: ATSGroupIdentity, text: "Jane Doe"},
		{label: "Body opening", group: ATSGroupBody, text: "Dear Hiring Manager,"},
	}
	if !reflect.DeepEqual(fields, want) {
		t.Errorf("fields = %+v, want %+v", fields, want)
	}
}

func TestBuildATSReport_ExtractionFails_ReportsUnavailable(t *testing.T) {
	requireBinary(t, "pdftotext")

	missing := filepath.Join(t.TempDir(), "does-not-exist.pdf")
	result := buildATSReport(missing, []expectedField{{label: "Name", text: "Jane Doe"}}, "", nil)

	if result.Status != ParsabilityUnavailable {
		t.Errorf("Status = %v, want %v", result.Status, ParsabilityUnavailable)
	}
	if result.Reason == "" {
		t.Error("expected a non-empty Reason explaining the Unavailable status")
	}
	if result.MissingFields != nil || result.OrderingViolations != nil {
		t.Errorf("expected no MissingFields/OrderingViolations on an unavailable result, got %+v", result)
	}
}

func TestFirstNonEmptyLine(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "leading blank lines skipped", in: "\n\n  first  \nsecond", want: "first"},
		{name: "single line", in: "only", want: "only"},
		{name: "all blank", in: "\n  \n\t\n", want: ""},
		{name: "empty string", in: "", want: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := firstNonEmptyLine(tt.in); got != tt.want {
				t.Errorf("firstNonEmptyLine(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestCheckParsability_ReportsEachFieldAndKeepsTheText(t *testing.T) {
	fields := []expectedField{
		{label: "Name", group: ATSGroupIdentity, text: "Jane Doe"},
		{label: "Email", group: ATSGroupContact, text: "jane@example.com"},
		{label: "Education section header", group: ATSGroupSection, text: "Education"},
		{label: "Experience section header", group: ATSGroupSection, text: "Experience"},
	}
	text := "Jane Doe\nExperience\nEducation\n"

	got := checkParsability(text, fields)

	want := []ATSField{
		{Label: "Name", Group: ATSGroupIdentity, Found: true, InOrder: true},
		{Label: "Email", Group: ATSGroupContact},
		{Label: "Education section header", Group: ATSGroupSection, Found: true, InOrder: true},
		{Label: "Experience section header", Group: ATSGroupSection, Found: true},
	}
	if !reflect.DeepEqual(got.Fields, want) {
		t.Errorf("Fields = %+v, want %+v", got.Fields, want)
	}
	if got.ExtractedText != text {
		t.Errorf("ExtractedText = %q, want the text checked", got.ExtractedText)
	}
	if got.Status != ParsabilityWarning || !reflect.DeepEqual(got.MissingFields, []string{"Email"}) {
		t.Errorf("got %+v, want a warning naming Email", got)
	}
}

func TestCVExpectedFields_ContactLineAfterName(t *testing.T) {
	cv := cvData{Name: "Jane Doe", Email: "jane@example.com", Phone: "+39 333 000 0000", LinkedIn: "janedoe"}

	var got []expectedField
	for _, f := range cvExpectedFields(cv) {
		if f.group == ATSGroupIdentity || f.group == ATSGroupContact {
			got = append(got, f)
		}
	}

	want := []expectedField{
		{label: "Name", group: ATSGroupIdentity, text: "Jane Doe"},
		{label: "Email", group: ATSGroupContact, text: "jane@example.com"},
		{label: "Phone", group: ATSGroupContact, text: "+39 333 000 0000"},
		{label: "LinkedIn", group: ATSGroupContact, text: "linkedin.com/in/janedoe"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("fields = %+v, want %+v (no GitHub: the profile has none)", got, want)
	}
}

func TestTermCoverage(t *testing.T) {
	tags := []string{"Go", "Kubernetes", "kubernetes", "dbt", "Python", "Node.js", "R"}
	jd := "We use Go, dbt and Kubernetes. Experience with Node.js is a plus. You will go far."
	cvText := "Jane Doe\nBuilt services in Go on kubernetes.\n"

	got := termCoverage(cvText, jd, tags)

	want := &TermCoverage{Present: []string{"Go", "Kubernetes"}, Missing: []string{"Node.js", "dbt"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("termCoverage() = %+v, want %+v", got, want)
	}
}

func TestContainsTerm(t *testing.T) {
	tests := []struct {
		text, term string
		want       bool
	}{
		{"we go far", "Go", false},
		{"written in Go.", "Go", true},
		{"Gopher", "Go", false},
		{"C++ and Rust", "C++", true},
		{"PYTHON developer", "Python", true},
		{"pythonic", "Python", false},
	}
	for _, tt := range tests {
		if got := containsTerm(tt.text, tt.term); got != tt.want {
			t.Errorf("containsTerm(%q, %q) = %v, want %v", tt.text, tt.term, got, tt.want)
		}
	}
}
