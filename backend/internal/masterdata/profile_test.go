package masterdata

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestGetProfile_MissingFileIsActionable pins ADR-0037's user-facing half:
// profile.yaml is untracked on purpose, so a fresh clone legitimately has
// none, and the error the user then sees must name the fix rather than
// surfacing a raw "no such file or directory".
func TestGetProfile_MissingFileIsActionable(t *testing.T) {
	_, err := GetProfile(t.TempDir())
	if !errors.Is(err, ErrNoProfile) {
		t.Fatalf("expected ErrNoProfile, got %v", err)
	}
	for _, want := range []string{"data/examples/", "data/profile.yaml"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error message should tell the user about %q, got %q", want, err.Error())
		}
	}
}

// Certifications are a Static Section like any other (issue #167): they
// round-trip through a read/write cycle with their optional fields intact,
// and an entry without a verification link survives — a link is usually
// added long after the exam is passed.
func TestProfile_CertificationsRoundTrip(t *testing.T) {
	dir := t.TempDir()
	profile := Profile{
		Name:  "Jane Doe",
		Email: "jane@example.com",
		Certifications: []Certification{
			{Title: "AWS Certified Cloud Practitioner", Issuer: "Amazon Web Services", Date: "2025-03", Link: "https://example.com/verify"},
			{Title: "SnowPro Core", Issuer: "Snowflake", Date: "2025-10"},
		},
	}
	if _, err := UpdateProfile(dir, profile); err != nil {
		t.Fatalf("UpdateProfile: %v", err)
	}

	got, err := GetProfile(dir)
	if err != nil {
		t.Fatalf("GetProfile: %v", err)
	}
	if len(got.Certifications) != 2 {
		t.Fatalf("expected 2 certifications, got %d", len(got.Certifications))
	}
	if got.Certifications[0] != profile.Certifications[0] {
		t.Errorf("first certification changed: %+v", got.Certifications[0])
	}
	if got.Certifications[1].Link != "" {
		t.Errorf("expected no link on the second certification, got %q", got.Certifications[1].Link)
	}

	content, err := os.ReadFile(filepath.Join(dir, "profile.yaml"))
	if err != nil {
		t.Fatalf("reading profile.yaml: %v", err)
	}
	if strings.Contains(string(content), "link: \"\"") {
		t.Errorf("an absent link should be omitted from the file, got:\n%s", content)
	}
}

// A certification with no title has nothing to print, so it is refused
// before it reaches disk rather than rendering as an empty bullet.
func TestValidateProfile_CertificationNeedsTitle(t *testing.T) {
	err := ValidateProfile(Profile{
		Name:           "Jane Doe",
		Email:          "jane@example.com",
		Certifications: []Certification{{Issuer: "Snowflake", Date: "2025-10"}},
	})
	if err == nil || !strings.Contains(err.Error(), "certifications[0]") {
		t.Fatalf("expected a certifications[0] validation error, got %v", err)
	}
}
