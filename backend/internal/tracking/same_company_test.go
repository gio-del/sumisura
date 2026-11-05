package tracking

import "testing"

func TestOtherListingsAtCompany(t *testing.T) {
	mk := func(id, company, title string, archived bool, status Status) ListingWithApplication {
		return ListingWithApplication{
			JobListing:  JobListing{ID: id, Company: company, Title: title, Archived: archived},
			Application: Application{Status: status, StatusUpdatedAt: "2026-10-01T00:00:00Z"},
		}
	}
	rejected := mk("d", "Acme", "Old role", true, StatusRejected)
	rejected.Application.CloseReason = CloseReason("no-response")
	all := []ListingWithApplication{
		mk("a", "Acme S.p.A.", "Self", false, StatusSaved),
		mk("b", "ACME, Inc", "Other", false, StatusInterviewing),
		mk("c", "Acmeron", "Similar name", false, StatusSaved),
		rejected,
	}
	got := OtherListingsAtCompany(all[0].JobListing, all)
	if len(got) != 2 || got[0].JobListingID != "b" || got[1].JobListingID != "d" {
		t.Fatalf("got %+v", got)
	}
	if got[0].Status != StatusInterviewing || got[0].Archived || !got[1].Archived || got[1].CloseReason != "no-response" || got[1].StatusUpdatedAt == "" {
		t.Fatalf("fields wrong: %+v", got)
	}
	if none := OtherListingsAtCompany(all[2].JobListing, all); none == nil || len(none) != 0 {
		t.Fatalf("want empty non-nil, got %#v", none)
	}
}
