package masterdata

import (
	"reflect"
	"testing"
)

func TestComputeGaps_UncoveredFirstAndCoveredByTagOrBody(t *testing.T) {
	listings := []GapListing{
		{ID: "a", JobDescription: "We use Kubernetes and  Terraform. Machine learning is a plus; Go services."},
		{ID: "b", JobDescription: "KUBERNETES, terraform, machine   learning, Go."},
		{ID: "c", JobDescription: "Only Terraform here."},
	}
	entries := []Entry{{
		Tags:    []string{"Go", "Machine Learning"},
		Bullets: []string{"Ran Kubernetes clusters."},
	}}

	gaps := ComputeGaps(listings, entries)

	got := map[string]Gap{}
	for _, g := range gaps {
		got[g.Term] = g
	}
	if g := got["terraform"]; g.Covered || !reflect.DeepEqual(g.ListingIDs, []string{"a", "b", "c"}) {
		t.Errorf("terraform = %+v, want uncovered in a,b,c", g)
	}
	for _, term := range []string{"kubernetes", "go", "machine learning"} {
		if !got[term].Covered {
			t.Errorf("%s should be covered: %+v", term, got[term])
		}
	}
	if gaps[0].Term != "terraform" {
		t.Errorf("first = %q, want the uncovered term first", gaps[0].Term)
	}
	for i := 1; i < len(gaps); i++ {
		if gaps[i-1].Covered && !gaps[i].Covered {
			t.Fatalf("uncovered term %q after covered %q", gaps[i].Term, gaps[i-1].Term)
		}
	}
}

func TestComputeGaps_BelowThresholdAndEmpty(t *testing.T) {
	single := []GapListing{{ID: "a", JobDescription: "Terraform"}}
	if gaps := ComputeGaps(single, nil); len(gaps) != 0 || gaps == nil {
		t.Errorf("one listing = %#v, want empty non-nil", gaps)
	}
	if gaps := ComputeGaps(nil, nil); len(gaps) != 0 || gaps == nil {
		t.Errorf("no listings = %#v, want empty non-nil", gaps)
	}
}

func TestComputeGaps_TermCountedOncePerListing(t *testing.T) {
	listings := []GapListing{
		{ID: "a", JobDescription: "Rust Rust Rust"},
		{ID: "b", JobDescription: "nothing"},
	}
	for _, g := range ComputeGaps(listings, nil) {
		if g.Term == "rust" {
			t.Errorf("rust cited by one listing must not pass the threshold: %+v", g)
		}
	}
}
