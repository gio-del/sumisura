package tracking

import (
	"sort"
	"time"

	"github.com/gio-del/sumisura/backend/internal/generation"
)

// OfferNotesLimit is how many of an Application's newest Notes an Offer
// carries: enough to recall where the conversation stands, not the whole log.
const OfferNotesLimit = 3

// Offer is one Application in Status Offer, reduced to what the side-by-side
// comparison shows. Every field is read from the records as they are: the
// RAL Range is passed through untouched (a conflict keeps both declared
// figures, no currency is converted or picked).
type Offer struct {
	ID       string              `json:"id"`
	Company  string              `json:"company"`
	Title    string              `json:"title,omitempty"`
	Location string              `json:"location,omitempty"`
	RAL      generation.RALRange `json:"ral"`
	// OfferedAt (RFC3339Nano) is when the Application last moved to Offer,
	// per StatusHistory; StatusUpdatedAt stands in when the history holds no
	// Offer entry, and it is empty when neither exists (a migrated legacy
	// record whose date is unknowable).
	OfferedAt string `json:"offeredAt,omitempty"`
	// Notes are the newest Notes, newest first, at most OfferNotesLimit.
	Notes []Note `json:"notes"`
}

// ComputeOffers keeps the Applications currently in Status Offer, newest
// offer first (ties and undated offers by id, undated last). Archived Job
// Listings are not special-cased: the caller filters the set it passes in.
func ComputeOffers(listings []ListingWithApplication) []Offer {
	offers := []Offer{}
	for _, l := range listings {
		if l.Application.Status != StatusOffer {
			continue
		}
		notes := l.Application.Notes
		if len(notes) > OfferNotesLimit {
			notes = notes[:OfferNotesLimit]
		}
		offers = append(offers, Offer{
			ID:        l.Application.ID,
			Company:   l.JobListing.Company,
			Title:     l.JobListing.Title,
			Location:  l.JobListing.Location,
			RAL:       l.JobListing.RAL,
			OfferedAt: offeredAt(l.Application),
			Notes:     append([]Note{}, notes...),
		})
	}
	sort.SliceStable(offers, func(i, j int) bool {
		a, b := offers[i], offers[j]
		if (a.OfferedAt == "") != (b.OfferedAt == "") {
			return a.OfferedAt != ""
		}
		ta, tb := parseOfferTime(a.OfferedAt), parseOfferTime(b.OfferedAt)
		if !ta.Equal(tb) {
			return ta.After(tb)
		}
		return a.ID < b.ID
	})
	return offers
}

func parseOfferTime(s string) time.Time {
	t, _ := time.Parse(time.RFC3339Nano, s)
	return t
}

func offeredAt(a Application) string {
	for i := len(a.StatusHistory) - 1; i >= 0; i-- {
		if a.StatusHistory[i].Status == StatusOffer {
			return a.StatusHistory[i].ChangedAt.UTC().Format(time.RFC3339Nano)
		}
	}
	return a.StatusUpdatedAt
}
