package tracking

// SameCompanyListing is another Job Listing at the same company as the one
// being looked at (issue #287): enough to see where that other application
// stands before generating a new CV.
type SameCompanyListing struct {
	JobListingID    string      `json:"jobListingId"`
	Title           string      `json:"title,omitempty"`
	Status          Status      `json:"status"`
	CloseReason     CloseReason `json:"closeReason,omitempty"`
	StatusUpdatedAt string      `json:"statusUpdatedAt,omitempty"`
	Archived        bool        `json:"archived"`
}

// OtherListingsAtCompany returns the entries of all that are at the same
// company as listing (SameCompany's normalization: case, punctuation and
// legal suffixes ignored), excluding listing itself and keeping archived
// ones, marked as such. The result is never nil.
func OtherListingsAtCompany(listing JobListing, all []ListingWithApplication) []SameCompanyListing {
	out := []SameCompanyListing{}
	for _, l := range all {
		if l.JobListing.ID == listing.ID || !SameCompany(l.JobListing.Company, listing.Company) {
			continue
		}
		out = append(out, SameCompanyListing{
			JobListingID:    l.JobListing.ID,
			Title:           l.JobListing.Title,
			Status:          l.Application.Status,
			CloseReason:     l.Application.CloseReason,
			StatusUpdatedAt: l.Application.StatusUpdatedAt,
			Archived:        l.JobListing.Archived,
		})
	}
	return out
}
