package generation

import (
	"errors"
	"slices"
	"testing"

	"github.com/gio-del/sumisura/backend/internal/masterdata"
)

// Issue #232: the About me, the tag rows, the employer display and the
// grouping of engagements into one project.

func layoutEntries() []masterdata.Entry {
	return []masterdata.Entry{
		{ID: "experience/a", Type: masterdata.TypeExperience, Employer: "Acme S.p.A.", Kind: masterdata.KindConsultancy, Client: "Client A", ClientLabel: "A retailer", Title: "AI Platform", Role: "Engineer", Start: "2025-01", Tags: []string{"Go", "MCP", "A2A"}, Bullets: []string{"Built a platform."}},
		{ID: "experience/b", Type: masterdata.TypeExperience, Employer: "Acme S.p.A.", Kind: masterdata.KindConsultancy, Client: "Client B", Group: "Data", Role: "Engineer", Start: "2024-01", End: strPtr("2024-06"), Tags: []string{"Spark"}, Bullets: []string{"Moved rows."}},
		{ID: "experience/c", Type: masterdata.TypeExperience, Employer: "Acme S.p.A.", Kind: masterdata.KindConsultancy, Client: "Client C", Group: "Data", Role: "Engineer", Start: "2024-07", End: strPtr("2024-12"), Tags: []string{"Kafka"}, Bullets: []string{"Streamed events."}},
	}
}

func TestKeepOwnTags_DropsForeignTagsDuplicatesAndOverflow(t *testing.T) {
	many := make([]string, 0, MaxEntryTags+3)
	for i := 0; i < MaxEntryTags+3; i++ {
		many = append(many, string(rune('a'+i)))
	}
	entries := []masterdata.Entry{{ID: "experience/a", Tags: many}}
	sel := SelectionResult{Entries: []SelectedEntry{{EntryID: "experience/a", Tags: append([]string{"invented", "a", "a"}, many...)}}}

	got := keepOwnTags(sel, entries).Entries[0].Tags

	if slices.Contains(got, "invented") {
		t.Errorf("a tag the Entry does not have must be dropped: %v", got)
	}
	if len(got) != MaxEntryTags || got[0] != "a" || got[1] != "b" {
		t.Errorf("want the first %d own tags, deduplicated, got %v", MaxEntryTags, got)
	}
}

func TestValidateSelection_RefusesATagTheEntryDoesNotHave(t *testing.T) {
	sel := SelectionResult{Entries: []SelectedEntry{{EntryID: "experience/a", Tags: []string{"Rust"}, Bullets: []SelectedBullet{{SourceIndex: 0, Source: "Built a platform.", Rewritten: "Built it."}}}}}

	if err := validateSelection(sel, layoutEntries()); !errors.Is(err, ErrInvalidSelection) {
		t.Fatalf("want ErrInvalidSelection, got %v", err)
	}
}

func TestResolveSummary_StaysTraceable(t *testing.T) {
	if got := resolveSummary("", "Invented About me."); got != "" {
		t.Errorf("no source must print no About me, got %q", got)
	}
	if got := resolveSummary("Mine.", "  "); got != "Mine." {
		t.Errorf("a skipped rewrite falls back to the user's text, got %q", got)
	}
	if got := resolveSummary("Mine.", "Adapted."); got != "Adapted." {
		t.Errorf("want the adapted summary, got %q", got)
	}
}

func TestSummaryGroundedness_FlagsAnInventedNumber(t *testing.T) {
	sel := SelectionResult{Summary: "Data engineer building AI platforms for 4 million users."}
	if flags := summaryGroundedness("Data engineer building AI platforms.", sel); len(flags) == 0 {
		t.Fatal("want the invented figure flagged")
	}
	sel.Entries = []SelectedEntry{{Bullets: []SelectedBullet{{Source: "The platform served 4 million users.", Rewritten: "The platform served 4 million users."}}}}
	if flags := summaryGroundedness("Data engineer building AI platforms.", sel); len(flags) != 0 {
		t.Errorf("a figure a kept bullet states is grounded, got %v", flags)
	}
	if summaryGroundedness("Same.", SelectionResult{Summary: "Same."}) != nil {
		t.Error("an unchanged summary has nothing to flag")
	}
}

func TestDefaultModeSelection_PrintsTheSummaryAsWritten(t *testing.T) {
	sel := defaultModeSelection(layoutEntries(), "My own words.")
	if sel.Summary != "My own words." {
		t.Errorf("summary = %q", sel.Summary)
	}
}

func TestAssembleCVData_EmployerDisplayTagsAndFallback(t *testing.T) {
	entries := layoutEntries()
	byID := map[string]masterdata.Entry{}
	for _, e := range entries {
		byID[e.ID] = e
	}
	profile := masterdata.Profile{
		Name: "Jane", LocationNote: "Open to relocate", Headline: "Engineer",
		Employers: masterdata.Employers{"Acme S.p.A.": {Display: "Acme, consulting", Role: "AI Engineer"}},
	}
	sel := SelectionResult{Summary: "About me.", Entries: []SelectedEntry{
		{EntryID: "experience/a", Tags: []string{"MCP"}, Bullets: []SelectedBullet{{Rewritten: "Built a platform."}}},
		{EntryID: "experience/b", Bullets: []SelectedBullet{{Rewritten: "Moved rows."}}},
	}}

	cv, err := assembleCVData(profile, byID, sel)
	if err != nil {
		t.Fatal(err)
	}
	a := cv.Experience[0]
	if a.EmployerDisplay != "Acme, consulting" || a.EmployerRole != "AI Engineer" || a.ClientLabel != "A retailer" || a.Title != "AI Platform" {
		t.Errorf("display fields not carried: %+v", a)
	}
	if !slices.Equal(a.Tags, []string{"MCP"}) {
		t.Errorf("chosen tags = %v", a.Tags)
	}
	if b := cv.Experience[1]; !slices.Equal(b.Tags, []string{"Spark"}) || b.Group != "Data" {
		t.Errorf("unchosen tags fall back to the Entry's own, group carried: %+v", b)
	}
	if cv.Summary != "About me." || cv.Headline != "Engineer" || cv.LocationNote != "Open to relocate" {
		t.Errorf("header fields = %q %q %q", cv.Summary, cv.Headline, cv.LocationNote)
	}
}

func TestOrderExperience_KeepsAGroupTogether(t *testing.T) {
	items := []cvExperience{
		{Employer: "Acme", EmployerStart: "2024-01", Client: "B", Group: "Data", Start: "2024-01"},
		{Employer: "Acme", EmployerStart: "2024-01", Client: "A", Start: "2025-01"},
		{Employer: "Acme", EmployerStart: "2024-01", Client: "C", Group: "Data", Start: "2024-07"},
		{Employer: "Acme", EmployerStart: "2024-01", Client: "D", Start: "2024-03"},
	}

	var clients []string
	for _, it := range orderExperience(items) {
		clients = append(clients, it.Client)
	}

	// Selection's order (ADR-0048: no engagement dates on the CV), except
	// that C joins B, the first of its group.
	if want := []string{"B", "C", "A", "D"}; !slices.Equal(clients, want) {
		t.Errorf("order = %v, want %v", clients, want)
	}
}

// Issue #238: the headline and the employer role are chosen from the lists
// in profile.yaml and never invented.

func titleProfile() masterdata.Profile {
	return masterdata.Profile{
		Name: "Jane", Headline: "AI Engineer", HeadlineOptions: []string{"AI Engineer", "Data Engineer"},
		Employers: masterdata.Employers{
			"Acme S.p.A.": {Role: "AI & Data Engineer", RoleOptions: []string{"Data Engineer"}},
			"Plain Ltd":   {Display: "Plain"},
		},
	}
}

func TestResolveTitles_KeepsListedChoicesAndFallsBackOtherwise(t *testing.T) {
	profile := titleProfile()

	chosen := resolveTitles(profile, SelectionResult{Headline: "Data Engineer", EmployerRoles: map[string]string{"Acme S.p.A.": "Data Engineer"}})
	if chosen.Headline != "Data Engineer" || chosen.EmployerRoles["Acme S.p.A."] != "Data Engineer" {
		t.Errorf("listed choice lost: %+v", chosen)
	}

	invented := resolveTitles(profile, SelectionResult{Headline: "Data Wizard", EmployerRoles: map[string]string{"Acme S.p.A.": "data engineer", "Plain Ltd": "CTO"}})
	if invented.Headline != "AI Engineer" || invented.EmployerRoles["Acme S.p.A."] != "AI & Data Engineer" {
		t.Errorf("an unlisted title must fall back to the profile's own: %+v", invented)
	}
	if _, ok := invented.EmployerRoles["Plain Ltd"]; ok {
		t.Errorf("an employer without a role gets none: %+v", invented.EmployerRoles)
	}

	none := resolveTitles(masterdata.Profile{}, SelectionResult{Headline: "CEO"})
	if none.Headline != "" || none.EmployerRoles != nil {
		t.Errorf("a profile without titles prints none: %+v", none)
	}
}

func TestTitleOptions_OnlyWhereThereIsAChoice(t *testing.T) {
	headlines, roles := titleOptions(titleProfile())
	if !slices.Equal(headlines, []string{"AI Engineer", "Data Engineer"}) {
		t.Errorf("headlines = %v", headlines)
	}
	if len(roles) != 1 || !slices.Equal(roles["Acme S.p.A."], []string{"AI & Data Engineer", "Data Engineer"}) {
		t.Errorf("roles = %v", roles)
	}
	if h, r := titleOptions(masterdata.Profile{Headline: "X"}); h != nil || r != nil {
		t.Errorf("no options, no choice: %v %v", h, r)
	}
}

func TestValidateTitles_RefusesAValueNotInTheLists(t *testing.T) {
	profile := titleProfile()
	for name, sel := range map[string]SelectionResult{
		"headline":         {Headline: "Data Wizard"},
		"role":             {EmployerRoles: map[string]string{"Acme S.p.A.": "CTO"}},
		"unknown employer": {EmployerRoles: map[string]string{"Nobody": "Data Engineer"}},
	} {
		if err := validateTitles(profile, sel); !errors.Is(err, ErrInvalidSelection) {
			t.Errorf("%s: err = %v, want ErrInvalidSelection", name, err)
		}
	}
	ok := SelectionResult{Headline: "Data Engineer", EmployerRoles: map[string]string{"Acme S.p.A.": "Data Engineer"}}
	if err := validateTitles(profile, ok); err != nil {
		t.Errorf("listed titles refused: %v", err)
	}
}

func TestAssembleCVData_PrintsTheChosenTitles(t *testing.T) {
	byID := map[string]masterdata.Entry{}
	for _, e := range layoutEntries() {
		byID[e.ID] = e
	}
	sel := SelectionResult{
		Headline: "Data Engineer", EmployerRoles: map[string]string{"Acme S.p.A.": "Data Engineer"},
		Entries: []SelectedEntry{{EntryID: "experience/a", Bullets: []SelectedBullet{{Rewritten: "Built a platform."}}}},
	}
	cv, err := assembleCVData(titleProfile(), byID, sel)
	if err != nil {
		t.Fatal(err)
	}
	if cv.Headline != "Data Engineer" || cv.Experience[0].EmployerRole != "Data Engineer" {
		t.Errorf("chosen titles not printed: %q %q", cv.Headline, cv.Experience[0].EmployerRole)
	}

	sel.Headline, sel.EmployerRoles = "", nil
	cv, _ = assembleCVData(titleProfile(), byID, sel)
	if cv.Headline != "AI Engineer" || cv.Experience[0].EmployerRole != "AI & Data Engineer" {
		t.Errorf("no choice prints the profile's own: %q %q", cv.Headline, cv.Experience[0].EmployerRole)
	}

	sel.Headline = "Data Wizard"
	if _, err := assembleCVData(titleProfile(), byID, sel); !errors.Is(err, ErrInvalidRenderRequest) {
		t.Errorf("an unlisted headline must be refused, got %v", err)
	}
}
