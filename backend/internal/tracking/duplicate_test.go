package tracking

import "testing"

func TestFindLikelyDuplicate(t *testing.T) {
	base := "2026-01-15T10:00:00Z"
	closeAfter := "2026-01-15T10:05:00Z" // 5 minutes later
	farApart := "2027-01-15T10:00:00Z"   // ~1 year later

	tests := []struct {
		name      string
		candidate JobListing
		existing  []JobListing
		wantMatch bool
		wantID    string
	}{
		{
			name:      "identical company+title+close dates matches",
			candidate: JobListing{ID: "new", Title: "Senior Backend Engineer", Company: "Acme Inc.", SavedAt: closeAfter},
			existing:  []JobListing{{ID: "old", Title: "Senior Backend Engineer", Company: "Acme Inc.", SavedAt: base}},
			wantMatch: true,
			wantID:    "old",
		},
		{
			name:      "same role reworded across sources matches",
			candidate: JobListing{ID: "new", Title: "Sr Software Engineer, Backend", Company: "Acme", SavedAt: closeAfter},
			existing:  []JobListing{{ID: "old", Title: "Senior Software Engineer - Backend", Company: "Acme Inc.", SavedAt: base}},
			wantMatch: true,
			wantID:    "old",
		},
		{
			name:      "same company different role does not match",
			candidate: JobListing{ID: "new", Title: "Product Designer", Company: "Acme Inc.", SavedAt: closeAfter},
			existing:  []JobListing{{ID: "old", Title: "Senior Backend Engineer", Company: "Acme Inc.", SavedAt: base}},
			wantMatch: false,
		},
		{
			name:      "same title different company does not match",
			candidate: JobListing{ID: "new", Title: "Senior Backend Engineer", Company: "Globex Corp", SavedAt: closeAfter},
			existing:  []JobListing{{ID: "old", Title: "Senior Backend Engineer", Company: "Acme Inc.", SavedAt: base}},
			wantMatch: false,
		},
		{
			name:      "same company+title far apart in time does not match",
			candidate: JobListing{ID: "new", Title: "Senior Backend Engineer", Company: "Acme Inc.", SavedAt: farApart},
			existing:  []JobListing{{ID: "old", Title: "Senior Backend Engineer", Company: "Acme Inc.", SavedAt: base}},
			wantMatch: false,
		},
		{
			name:      "minor formatting noise in company/title still matches",
			candidate: JobListing{ID: "new", Title: "senior backend engineer!!", Company: "ACME, Inc.", SavedAt: closeAfter},
			existing:  []JobListing{{ID: "old", Title: "Senior Backend Engineer", Company: "Acme Inc", SavedAt: base}},
			wantMatch: true,
			wantID:    "old",
		},
		{
			name:      "no existing listings never matches",
			candidate: JobListing{ID: "new", Title: "Senior Backend Engineer", Company: "Acme Inc.", SavedAt: closeAfter},
			existing:  nil,
			wantMatch: false,
		},
		{
			name:      "candidate's own id is excluded from comparison",
			candidate: JobListing{ID: "same", Title: "Senior Backend Engineer", Company: "Acme Inc.", SavedAt: closeAfter},
			existing:  []JobListing{{ID: "same", Title: "Senior Backend Engineer", Company: "Acme Inc.", SavedAt: closeAfter}},
			wantMatch: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			match, found := FindLikelyDuplicate(tt.candidate, tt.existing)
			if found != tt.wantMatch {
				t.Fatalf("found = %v, want %v (score %v)", found, tt.wantMatch, match.Score)
			}
			if tt.wantMatch && match.JobListingID != tt.wantID {
				t.Fatalf("matched id = %q, want %q", match.JobListingID, tt.wantID)
			}
			if tt.wantMatch && (match.Score < duplicateScoreThreshold || match.Score > 1.0) {
				t.Fatalf("score %v out of expected [%v, 1.0] range", match.Score, duplicateScoreThreshold)
			}
		})
	}
}

func TestFindLikelyDuplicatePicksBestMatch(t *testing.T) {
	candidate := JobListing{ID: "new", Title: "Senior Backend Engineer", Company: "Acme Inc.", SavedAt: "2026-01-15T10:00:00Z"}
	existing := []JobListing{
		{ID: "weak", Title: "Backend Engineer II", Company: "Acme Holdings", SavedAt: "2026-01-01T00:00:00Z"},
		{ID: "strong", Title: "Senior Backend Engineer", Company: "Acme Inc.", SavedAt: "2026-01-15T09:55:00Z"},
	}

	match, found := FindLikelyDuplicate(candidate, existing)
	if !found {
		t.Fatalf("expected a match, got none")
	}
	if match.JobListingID != "strong" {
		t.Fatalf("expected best match to be %q, got %q", "strong", match.JobListingID)
	}
}

// SameCompany is what the same-company warning asks before a save (issue
// #206, story 21): one company written the many ways a board and a human
// write it.
func TestSameCompany(t *testing.T) {
	tests := []struct {
		a, b string
		want bool
	}{
		{"Acme", "Acme", true},
		{"Acme Inc.", "Acme", true},
		{"ACME, Inc", "Acme Inc.", true},
		{"acme inc", "Acme", true},
		{"Acme GmbH", "Acme", true},
		// Italian legal forms, which a board writes dotted as often as not.
		{"Acme S.p.A.", "Acme", true},
		{"Acme S.r.l.", "Acme SRL", true},
		{"Acme S.p.A.", "ACME, Inc", true},
		// Different companies stay different.
		{"Acme", "Acme Holdings", false},
		{"Acme", "Globex", false},
		// A name that normalizes to nothing is never a match, or every
		// unnamed company would collide.
		{"", "", false},
		{".", "-", false},
		{"", "Acme", false},
		// A legal suffix alone is a company name, not a suffix to strip.
		{"SPA", "SRL", false},
	}
	for _, tt := range tests {
		if got := SameCompany(tt.a, tt.b); got != tt.want {
			t.Errorf("SameCompany(%q, %q) = %v, want %v", tt.a, tt.b, got, tt.want)
		}
	}
}
