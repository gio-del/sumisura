package tracking

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// SuggestContact asks client to research a Contact for the Job Listing
// identified by id via web search (story 7). It never writes to disk — the
// caller must explicitly PATCH the Application's contact (via
// UpdateApplicationContact) to actually save it, so a suggestion is never
// persisted without confirmation.
func SuggestContact(ctx context.Context, dataDir string, client Client, id string) (Contact, error) {
	listing, err := getJobListing(dataDir, id)
	if err != nil {
		return Contact{}, err
	}
	contact, err := client.SuggestContact(ctx, listing.Company, listing.JobDescription)
	RecordStandaloneUsage(dataDir, client)
	return contact, err
}

// UpdateApplicationContact validates and saves a Contact to the
// Application identified by id — the explicit confirmation step for both a
// manual entry and an accepted Claude suggestion (story 7).
func UpdateApplicationContact(dataDir, id string, contact Contact) (Application, error) {
	return UpdateApplicationContactIfMatch(dataDir, id, contact, "")
}

// UpdateApplicationContactIfMatch is UpdateApplicationContact, refusing the
// write with recordversion.ErrMismatch when version no longer matches the
// Application file on disk, so a confirmed Contact is not overwritten by a
// stale copy (issue #89, story 17). An empty version writes
// unconditionally.
func UpdateApplicationContactIfMatch(dataDir, id string, contact Contact, version string) (Application, error) {
	if strings.TrimSpace(contact.Email) == "" {
		return Application{}, fmt.Errorf("%w: email is required", ErrValidation)
	}

	application, err := getApplication(dataDir, id)
	if err != nil {
		return Application{}, err
	}
	application.Contact = &contact

	return writeApplicationIfMatch(dataDir, id, application, version)
}

// UpdateApplicationFollowUpIfMatch sets the Application's follow-up day
// (YYYY-MM-DD), or clears it when followUpAt is empty (issue #240),
// refusing the write with recordversion.ErrMismatch when version no longer
// matches the file on disk. An empty version writes unconditionally.
func UpdateApplicationFollowUpIfMatch(dataDir, id, followUpAt, version string) (Application, error) {
	followUpAt = strings.TrimSpace(followUpAt)
	if followUpAt != "" {
		if _, err := time.Parse(FollowUpDateLayout, followUpAt); err != nil {
			return Application{}, fmt.Errorf("%w: followUpAt must be a YYYY-MM-DD date", ErrValidation)
		}
	}

	application, err := getApplication(dataDir, id)
	if err != nil {
		return Application{}, err
	}
	application.FollowUpAt = followUpAt
	application.IsStale = IsStaleWithFollowUp(application.Status, application.StatusUpdatedAt, followUpAt, time.Now(), DefaultStaleThreshold)

	return writeApplicationIfMatch(dataDir, id, application, version)
}
