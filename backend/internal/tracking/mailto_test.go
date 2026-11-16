package tracking_test

import (
	"net/url"
	"strings"
	"testing"

	"github.com/gio-del/sumisura/backend/internal/tracking"
)

func TestBuildMailtoURI_WellFormedWithExpectedFields(t *testing.T) {
	contact := tracking.Contact{Name: "Jane Recruiter", Email: "jane@acme.example"}
	uri := tracking.BuildMailtoURI(contact, "Acme Corp", "Test User")

	if !strings.HasPrefix(uri, "mailto:jane@acme.example?") {
		t.Fatalf("expected uri to start with mailto:jane@acme.example?, got %q", uri)
	}

	parsed, err := url.Parse(uri)
	if err != nil {
		t.Fatalf("expected a parseable URI, got error: %v", err)
	}
	if parsed.Scheme != "mailto" {
		t.Errorf("expected scheme mailto, got %q", parsed.Scheme)
	}
	if parsed.Opaque != "jane@acme.example" {
		t.Errorf("expected recipient jane@acme.example, got %q", parsed.Opaque)
	}

	query := parsed.Query()
	if subject := query.Get("subject"); subject != "Application: Acme Corp" {
		t.Errorf("expected subject %q, got %q", "Application: Acme Corp", subject)
	}
	body := query.Get("body")
	if !strings.Contains(body, "Dear Jane Recruiter,") {
		t.Errorf("expected body to greet the contact by name, got %q", body)
	}
	if !strings.Contains(body, "Acme Corp") {
		t.Errorf("expected body to mention the company, got %q", body)
	}
	if !strings.Contains(body, "Test User") {
		t.Errorf("expected body to sign off with the sender's name, got %q", body)
	}
}

func TestBuildMailtoURI_NoContactName_GreetsHiringTeam(t *testing.T) {
	contact := tracking.Contact{Email: "jobs@acme.example"}
	uri := tracking.BuildMailtoURI(contact, "Acme Corp", "Test User")

	parsed, err := url.Parse(uri)
	if err != nil {
		t.Fatalf("expected a parseable URI, got error: %v", err)
	}
	body := parsed.Query().Get("body")
	if !strings.Contains(body, "Dear Hiring Team,") {
		t.Errorf("expected a generic greeting when no contact name is set, got %q", body)
	}
}

func TestBuildMailtoURI_EncodesSpacesAsPercent20NotPlus(t *testing.T) {
	contact := tracking.Contact{Email: "jane@acme.example"}
	uri := tracking.BuildMailtoURI(contact, "Acme Corp", "Test User")

	if strings.Contains(uri, "+") {
		t.Errorf("expected no '+' encoding in a mailto URI, got %q", uri)
	}
	if !strings.Contains(uri, "%20") {
		t.Errorf("expected spaces encoded as %%20, got %q", uri)
	}
}

func TestDefaultContact_FirstRecruiterElseFirst(t *testing.T) {
	boss := tracking.Contact{ID: "a", Email: "boss@acme.example", Role: tracking.ContactManager}
	rita := tracking.Contact{ID: "b", Email: "rita@acme.example", Role: tracking.ContactRecruiter}
	if got, ok := tracking.DefaultContact([]tracking.Contact{boss, rita}); !ok || got.ID != "b" {
		t.Errorf("expected the recruiter, got %+v", got)
	}
	if got, ok := tracking.DefaultContact([]tracking.Contact{boss}); !ok || got.ID != "a" {
		t.Errorf("expected the first, got %+v", got)
	}
	if _, ok := tracking.DefaultContact(nil); ok {
		t.Error("expected none for an empty list")
	}
}
