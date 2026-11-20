package generation

import (
	"reflect"
	"testing"

	"github.com/gio-del/sumisura/backend/internal/masterdata"
)

func TestCheckJobTermCoverage(t *testing.T) {
	entries := []masterdata.Entry{
		{ID: "kept", Employer: "Acme", Tags: []string{"Go"}, Bullets: []string{"Built Go services"}},
		{ID: "left-out", Employer: "Beta", Tags: []string{"Kubernetes"}, Bullets: []string{"Ran kubernetes clusters"}},
	}
	jd := "We use Kubernetes daily. Kubernetes experience matters. Go is used here. Terraform and terraform modules."

	tests := []struct {
		name     string
		jd, text string
		selected []string
		want     []MissingJobTerm
	}{
		{"all present", jd, "kubernetes terraform", []string{"kept"}, nil},
		{"empty job description", "", "anything", nil, nil},
		{
			"absent term names unselected entry",
			jd, "Go terraform", []string{"kept"},
			[]MissingJobTerm{{Term: "kubernetes", Count: 2, EntryIDs: []string{"left-out"}}},
		},
		{
			"absent term no entry has",
			jd, "kubernetes", []string{"kept"},
			[]MissingJobTerm{{Term: "terraform", Count: 2}},
		},
		{
			"selected entry is not named",
			jd, "terraform", []string{"kept", "left-out"},
			[]MissingJobTerm{{Term: "kubernetes", Count: 2}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CheckJobTermCoverage(tt.jd, tt.text, entries, tt.selected).Missing
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("got %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestCheckJobTermCoverageDropsWordsOfAListedPair(t *testing.T) {
	jd := "Machine learning here. Machine learning there."
	got := CheckJobTermCoverage(jd, "nothing relevant", nil, nil).Missing
	want := []MissingJobTerm{{Term: "machine learning", Count: 2}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}
}
