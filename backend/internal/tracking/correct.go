package tracking

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/gio-del/sumisura/backend/internal/atomicfile"
	"github.com/gio-del/sumisura/backend/internal/generation"
	"github.com/gio-del/sumisura/backend/internal/recordversion"
)

// JobListingCorrection is the set of fields a user may correct on a Job
// Listing (issue #206, stories 61-70). Each is a pointer so that "leave it
// alone" is distinguishable from "set it to empty": clearing a Job Title
// or a Location is a legitimate correction, and the correction of a field
// nobody touched must not be a silent overwrite.
//
// URL and JobDescription are deliberately absent. The URL is the record's
// identity, which the one-Job-Listing-per-posting rule rests on (ADR-0042),
// and the Job Description is what RAL Range, Application Method and every
// recorded Generation were derived from — editing it would quietly detach
// each of those from the text it came from.
type JobListingCorrection struct {
	Title    *string
	Company  *string
	Location *string
	// RAL is the figure the user entered themselves. Its Source is always
	// set to RALSourceManual here, whatever a caller passes: a client
	// cannot claim a figure was stated by the posting.
	RAL *generation.RALRange
}

// CorrectJobListingIfMatch applies a correction, refusing the write with
// recordversion.ErrMismatch when version no longer matches the file on
// disk (issue #89). An empty version writes unconditionally, as every
// other record-writing path does for non-FE callers.
//
// Correcting Company re-evaluates nothing: the same-company warning simply
// reads the new value the next time it runs, and nothing on the record is
// derived from the name.
func CorrectJobListingIfMatch(dataDir, id string, correction JobListingCorrection, version string) (JobListing, error) {
	listing, err := getJobListing(dataDir, id)
	if err != nil {
		return JobListing{}, err
	}

	if correction.Company != nil && strings.TrimSpace(*correction.Company) == "" {
		return JobListing{}, fmt.Errorf("%w: company cannot be empty", ErrValidation)
	}
	if correction.RAL != nil {
		if err := validateManualRAL(*correction.RAL); err != nil {
			return JobListing{}, err
		}
	}

	path := filepath.Join(dataDir, jobsDir, id+".md")
	if err := recordversion.Check(path, version); err != nil {
		return JobListing{}, err
	}

	if correction.Title != nil {
		listing.Title = strings.TrimSpace(*correction.Title)
	}
	if correction.Company != nil {
		listing.Company = strings.TrimSpace(*correction.Company)
	}
	if correction.Location != nil {
		listing.Location = strings.TrimSpace(*correction.Location)
	}
	if correction.RAL != nil {
		ral := *correction.RAL
		ral.Source = generation.RALSourceManual
		// A manual figure is one range, never a conflict between two
		// sources, so any conflict detail the record carried is gone.
		ral.DescriptionStated = nil
		ral.ListingStated = nil
		listing.RAL = ral
	}

	if err := atomicfile.WriteFile(path, renderJobListing(listing), 0o644); err != nil {
		return JobListing{}, err
	}
	return listing, nil
}

// defaultManualRALCurrency mirrors generation.ParseStatedRAL's own default:
// RAL is an Italian-market term, so a figure entered without a currency is
// EUR rather than left ambiguous.
const defaultManualRALCurrency = "EUR"

func validateManualRAL(ral generation.RALRange) error {
	if ral.Min == nil || ral.Max == nil {
		return fmt.Errorf("%w: a RAL Range needs both a minimum and a maximum", ErrValidation)
	}
	if *ral.Min <= 0 || *ral.Max <= 0 {
		return fmt.Errorf("%w: a RAL Range must be a positive figure", ErrValidation)
	}
	if *ral.Max < *ral.Min {
		return fmt.Errorf("%w: a RAL Range's maximum cannot be below its minimum", ErrValidation)
	}
	return nil
}

// NormalizeManualRAL fills in what a hand-entered figure may leave out.
func NormalizeManualRAL(ral generation.RALRange) generation.RALRange {
	if strings.TrimSpace(ral.Currency) == "" {
		ral.Currency = defaultManualRALCurrency
	}
	ral.Source = generation.RALSourceManual
	return ral
}
