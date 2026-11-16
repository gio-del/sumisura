package tracking

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// SuggestContact asks client to research a Contact for the Job Listing
// identified by id via web search (story 7). It never writes to disk — the
// caller must explicitly POST the Application's contacts (via
// AddContactIfMatch) to actually save it, so a suggestion is never
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

// ErrContactNotFound marks an edit or delete addressing a Contact id the
// Application does not carry.
var ErrContactNotFound = errors.New("contact not found")

// AddContactIfMatch appends a Contact to the Application id and returns the
// updated Application — the explicit confirmation step for both a manual
// entry and an accepted Claude suggestion (story 7). It refuses with
// recordversion.ErrMismatch when version no longer matches the file on
// disk (issue #89); an empty version writes unconditionally. Any ID in
// contact is ignored: the server assigns one.
func AddContactIfMatch(dataDir, id string, contact Contact, version string) (Application, error) {
	contact, err := validContact(contact)
	if err != nil {
		return Application{}, err
	}
	application, err := getApplicationIfMatch(dataDir, id, version)
	if err != nil {
		return Application{}, err
	}
	contact.ID = uniqueContactID(application.Contacts, time.Now().UTC())
	application.Contacts = append(append([]Contact{}, application.Contacts...), contact)
	return writeApplicationIfMatch(dataDir, id, application, version)
}

// UpdateContactIfMatch replaces the fields of the Contact contactID on the
// Application id. An unknown contact id is ErrContactNotFound; the version
// is checked first so a Contact deleted elsewhere reads as the conflict it
// is.
func UpdateContactIfMatch(dataDir, id, contactID string, contact Contact, version string) (Application, error) {
	contact, err := validContact(contact)
	if err != nil {
		return Application{}, err
	}
	application, err := getApplicationIfMatch(dataDir, id, version)
	if err != nil {
		return Application{}, err
	}
	i := contactIndex(application.Contacts, contactID)
	if i < 0 {
		return Application{}, ErrContactNotFound
	}
	contact.ID = contactID
	contacts := append([]Contact{}, application.Contacts...)
	contacts[i] = contact
	application.Contacts = contacts
	return writeApplicationIfMatch(dataDir, id, application, version)
}

// DeleteContactIfMatch removes the Contact contactID from the Application
// id. Deleting the last one leaves no contacts key at all.
func DeleteContactIfMatch(dataDir, id, contactID, version string) (Application, error) {
	application, err := getApplicationIfMatch(dataDir, id, version)
	if err != nil {
		return Application{}, err
	}
	i := contactIndex(application.Contacts, contactID)
	if i < 0 {
		return Application{}, ErrContactNotFound
	}
	application.Contacts = append(application.Contacts[:i:i], application.Contacts[i+1:]...)
	if len(application.Contacts) == 0 {
		application.Contacts = nil
	}
	return writeApplicationIfMatch(dataDir, id, application, version)
}

// DefaultContact picks the Contact a mailto draft is addressed to when the
// user chose none: the first recruiter, otherwise the first of the list.
func DefaultContact(contacts []Contact) (Contact, bool) {
	for _, c := range contacts {
		if c.Role == ContactRecruiter {
			return c, true
		}
	}
	if len(contacts) > 0 {
		return contacts[0], true
	}
	return Contact{}, false
}

func validContact(c Contact) (Contact, error) {
	c.Name = strings.TrimSpace(c.Name)
	c.Email = strings.TrimSpace(c.Email)
	c.Detail = strings.TrimSpace(c.Detail)
	if c.Email == "" {
		return Contact{}, fmt.Errorf("%w: email is required", ErrValidation)
	}
	if c.Role == "" {
		c.Role = ContactOther
	}
	if !c.Role.Valid() {
		return Contact{}, fmt.Errorf("%w: role must be recruiter, manager, referral or other", ErrValidation)
	}
	return c, nil
}

func contactIndex(contacts []Contact, contactID string) int {
	for i, c := range contacts {
		if c.ID == contactID {
			return i
		}
	}
	return -1
}

func uniqueContactID(existing []Contact, at time.Time) string {
	taken := make(map[string]bool, len(existing))
	for _, c := range existing {
		taken[c.ID] = true
	}
	nanos := at.UnixNano()
	for {
		id := "c" + strconv.FormatInt(nanos, 36)
		if !taken[id] {
			return id
		}
		nanos++
	}
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
