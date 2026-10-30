package tracking_test

import (
	"testing"
	"time"

	"github.com/gio-del/sumisura/backend/internal/generation"
	"github.com/gio-del/sumisura/backend/internal/tracking"
)

func intp(n int) *int { return &n }

func offerListing(id string, status tracking.Status, ral generation.RALRange, history ...tracking.StatusChange) tracking.ListingWithApplication {
	return tracking.ListingWithApplication{
		JobListing:  tracking.JobListing{ID: id, Company: "Co " + id, Location: "Milan", RAL: ral},
		Application: tracking.Application{ID: id, Status: status, StatusHistory: history},
	}
}

func TestComputeOffers_NoOffers_EmptyNotNil(t *testing.T) {
	got := tracking.ComputeOffers([]tracking.ListingWithApplication{offerListing("a", tracking.StatusSent, generation.RALRange{})})
	if got == nil || len(got) != 0 {
		t.Fatalf("expected empty non-nil slice, got %#v", got)
	}
}

func TestComputeOffers_OnlyOfferStatus_DateFromHistory_NewestFirst(t *testing.T) {
	old := time.Date(2026, 1, 5, 9, 0, 0, 0, time.UTC)
	recent := time.Date(2026, 3, 1, 9, 0, 0, 0, time.UTC)
	listings := []tracking.ListingWithApplication{
		offerListing("old", tracking.StatusOffer, generation.RALRange{}, tracking.StatusChange{Status: tracking.StatusOffer, ChangedAt: old}),
		offerListing("rej", tracking.StatusRejected, generation.RALRange{}),
		offerListing("new", tracking.StatusOffer, generation.RALRange{},
			tracking.StatusChange{Status: tracking.StatusOffer, ChangedAt: old},
			tracking.StatusChange{Status: tracking.StatusInterviewing, ChangedAt: old.Add(time.Hour)},
			tracking.StatusChange{Status: tracking.StatusOffer, ChangedAt: recent}),
		offerListing("undated", tracking.StatusOffer, generation.RALRange{}),
	}
	got := tracking.ComputeOffers(listings)
	ids := []string{}
	for _, o := range got {
		ids = append(ids, o.ID)
	}
	want := []string{"new", "old", "undated"}
	if len(ids) != 3 || ids[0] != want[0] || ids[1] != want[1] || ids[2] != want[2] {
		t.Fatalf("got %v want %v", ids, want)
	}
	if got[0].OfferedAt != recent.Format(time.RFC3339Nano) {
		t.Errorf("offeredAt = %q, want the latest Offer entry", got[0].OfferedAt)
	}
	if got[2].OfferedAt != "" {
		t.Errorf("undated offer should have empty offeredAt, got %q", got[2].OfferedAt)
	}
}

func TestComputeOffers_ConflictAndMixedCurrencies_PassedThroughUntouched(t *testing.T) {
	conflict := generation.RALRange{
		Source:            generation.RALSourceConflict,
		DescriptionStated: &generation.RALFigure{Min: 50000, Max: 60000, Currency: "EUR"},
		ListingStated:     &generation.RALFigure{Min: 55000, Max: 65000, Currency: "EUR"},
	}
	usd := generation.RALRange{Source: generation.RALSourceStated, Min: intp(90000), Max: intp(100000), Currency: "USD"}
	got := tracking.ComputeOffers([]tracking.ListingWithApplication{
		offerListing("c", tracking.StatusOffer, conflict),
		offerListing("u", tracking.StatusOffer, usd),
	})
	byID := map[string]tracking.Offer{}
	for _, o := range got {
		byID[o.ID] = o
	}
	c := byID["c"].RAL
	if c.Source != generation.RALSourceConflict || c.DescriptionStated == nil || c.ListingStated == nil || c.Min != nil {
		t.Errorf("conflict was altered: %#v", c)
	}
	if u := byID["u"].RAL; u.Currency != "USD" || *u.Max != 100000 {
		t.Errorf("USD range was altered: %#v", u)
	}
}

func TestComputeOffers_Notes_NewestThreeOnly(t *testing.T) {
	l := offerListing("a", tracking.StatusOffer, generation.RALRange{})
	l.Application.Notes = []tracking.Note{{ID: "4"}, {ID: "3"}, {ID: "2"}, {ID: "1"}}
	got := tracking.ComputeOffers([]tracking.ListingWithApplication{l})
	if len(got[0].Notes) != 3 || got[0].Notes[0].ID != "4" {
		t.Errorf("notes = %#v", got[0].Notes)
	}
}
