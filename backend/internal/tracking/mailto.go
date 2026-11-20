package tracking

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/gio-del/sumisura/backend/internal/masterdata"
)

// BuildMailtoURI constructs a mailto: URI prefilled with To/Subject/Body
// for an email-method Application's confirmed Contact (story 9) — a draft
// the user's own email client opens ready to send, never sent
// automatically (per ADR-0006's boundary). senderName signs off the draft
// body; pass "" if unknown.
func BuildMailtoURI(contact Contact, company, senderName string) string {
	greeting := "Hiring Team"
	if strings.TrimSpace(contact.Name) != "" {
		greeting = contact.Name
	}

	subject := fmt.Sprintf("Application: %s", company)
	body := fmt.Sprintf(
		"Dear %s,\n\nI am writing to apply for the %s role. Please find my CV attached.\n\nBest regards,\n%s",
		greeting, company, senderName,
	)

	// The recipient itself is left unescaped (per RFC 6068, an email
	// address's characters are all already valid in a mailto path) — only
	// escaping it would %-encode the '@' most mail clients expect literally.
	return fmt.Sprintf("mailto:%s?subject=%s&body=%s", contact.Email, mailtoEscape(subject), mailtoEscape(body))
}

// GetMailtoURI builds the mailto: draft for the Application identified by
// id, addressed to the Contact contactID, or — when contactID is empty — to
// DefaultContact (first recruiter, else first of the list). Job Listing
// (for the company name) and Master Data's Profile (for the sender's
// sign-off name) are both read to fill it in. An Application with no
// Contact is ErrValidation; an unknown contactID is ErrContactNotFound.
func GetMailtoURI(dataDir, id, contactID string) (string, error) {
	application, err := getApplication(dataDir, id)
	if err != nil {
		return "", err
	}
	var contact Contact
	if contactID != "" {
		i := contactIndex(application.Contacts, contactID)
		if i < 0 {
			return "", ErrContactNotFound
		}
		contact = application.Contacts[i]
	} else {
		c, ok := DefaultContact(application.Contacts)
		if !ok {
			return "", fmt.Errorf("%w: application has no confirmed contact yet", ErrValidation)
		}
		contact = c
	}

	listing, err := getJobListing(dataDir, id)
	if err != nil {
		return "", err
	}
	profile, err := masterdata.GetProfile(dataDir)
	if err != nil {
		return "", err
	}

	return BuildMailtoURI(contact, listing.Company, profile.Name), nil
}

// mailtoEscape percent-encodes s for use in a mailto: URI, using %20 for
// spaces rather than url.QueryEscape's '+' (correct for the mailto scheme,
// and what mail clients expect).
func mailtoEscape(s string) string {
	return strings.ReplaceAll(url.QueryEscape(s), "+", "%20")
}
