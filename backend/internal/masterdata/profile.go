package masterdata

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/gio-del/sumisura/backend/internal/atomicfile"
	"github.com/gio-del/sumisura/backend/internal/recordversion"
	"gopkg.in/yaml.v3"
)

// Education, Publication, Award, Activity, and Language are the Static
// Sections carried by profile.yaml alongside contact info (see CONTEXT.md).
type Education struct {
	Degree      string   `yaml:"degree" json:"degree"`
	Institution string   `yaml:"institution" json:"institution"`
	Program     string   `yaml:"program" json:"program"`
	Start       string   `yaml:"start" json:"start"`
	End         string   `yaml:"end" json:"end"`
	Grade       string   `yaml:"grade" json:"grade"`
	Courses     []string `yaml:"courses,omitempty" json:"courses,omitempty"`
}

type Publication struct {
	Title   string `yaml:"title" json:"title"`
	Authors string `yaml:"authors" json:"authors"`
	Venue   string `yaml:"venue" json:"venue"`
	Link    string `yaml:"link,omitempty" json:"link,omitempty"`
	Note    string `yaml:"note,omitempty" json:"note,omitempty"`
}

type Award struct {
	Title       string `yaml:"title" json:"title"`
	Description string `yaml:"description,omitempty" json:"description,omitempty"`
}

type Activity struct {
	Title       string `yaml:"title" json:"title"`
	Description string `yaml:"description,omitempty" json:"description,omitempty"`
}

type Language struct {
	Name  string `yaml:"name" json:"name"`
	Level string `yaml:"level" json:"level"`
}

// Certification is a professional certification (issue #167). Link is
// optional because a verification URL is often added long after the exam is
// passed, and an entry without one is still worth printing.
type Certification struct {
	Title  string `yaml:"title" json:"title"`
	Issuer string `yaml:"issuer,omitempty" json:"issuer,omitempty"`
	Date   string `yaml:"date,omitempty" json:"date,omitempty"`
	Link   string `yaml:"link,omitempty" json:"link,omitempty"`
}

// Profile is the contents of profile.yaml: contact info plus the Static
// Sections, always included in full and never subject to Selection or
// Rewrite (see CONTEXT.md).
type Profile struct {
	Name     string `yaml:"name" json:"name"`
	Location string `yaml:"location" json:"location"`
	Email    string `yaml:"email" json:"email"`
	Phone    string `yaml:"phone" json:"phone"`
	LinkedIn string `yaml:"linkedin" json:"linkedin"`
	GitHub   string `yaml:"github" json:"github"`

	Education      []Education     `yaml:"education,omitempty" json:"education"`
	Publications   []Publication   `yaml:"publications,omitempty" json:"publications"`
	Certifications []Certification `yaml:"certifications,omitempty" json:"certifications"`
	Awards         []Award         `yaml:"awards,omitempty" json:"awards"`
	Activities     []Activity      `yaml:"activities,omitempty" json:"activities"`
	Languages      []Language      `yaml:"languages,omitempty" json:"languages"`

	// Version is profile.yaml's version token, populated by the API layer
	// (via ProfileVersion) on reads. yaml:"-" keeps it out of the file
	// itself — UpdateProfile marshals this struct straight to disk, and a
	// read-time field must never be persisted (issue #89).
	Version string `yaml:"-" json:"version,omitempty"`
}

// ErrNoProfile is returned when profile.yaml does not exist. It is the
// expected state of a fresh clone, not a corrupt install: profile.yaml holds
// the user's real contact details and is deliberately untracked (ADR-0037,
// ADR-0038), so the repo ships stubs under data/examples/ and the user copies
// them once.
var ErrNoProfile = errors.New("no data/profile.yaml yet — run `cp -r data/examples/. data/` and fill in your own details")

// GetProfile reads and parses dataDir/profile.yaml.
func GetProfile(dataDir string) (Profile, error) {
	content, err := os.ReadFile(filepath.Join(dataDir, "profile.yaml"))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Profile{}, ErrNoProfile
		}
		return Profile{}, err
	}
	var profile Profile
	if err := yaml.Unmarshal(content, &profile); err != nil {
		return Profile{}, err
	}
	return profile, nil
}

// UpdateProfile validates profile and, if valid, writes it to
// dataDir/profile.yaml. On validation failure the file is left untouched.
func UpdateProfile(dataDir string, profile Profile) (Profile, error) {
	return UpdateProfileIfMatch(dataDir, profile, "")
}

// UpdateProfileIfMatch is UpdateProfile, refusing the write with
// recordversion.ErrMismatch when version no longer matches profile.yaml on
// disk (issue #89, story 14). An empty version writes unconditionally.
func UpdateProfileIfMatch(dataDir string, profile Profile, version string) (Profile, error) {
	if err := ValidateProfile(profile); err != nil {
		return Profile{}, fmt.Errorf("%w: %v", ErrValidation, err)
	}

	content, err := yaml.Marshal(profile)
	if err != nil {
		return Profile{}, err
	}

	path := filepath.Join(dataDir, "profile.yaml")
	if err := recordversion.Check(path, version); err != nil {
		return Profile{}, err
	}

	if err := atomicfile.WriteFile(path, content, 0o644); err != nil {
		return Profile{}, err
	}
	return profile, nil
}

// ProfileVersion returns the version token of the profile.yaml GetProfile
// reads.
func ProfileVersion(dataDir string) (string, error) {
	return recordversion.Of(filepath.Join(dataDir, "profile.yaml"))
}

// ValidateProfile checks that a Profile's required fields are present and
// well-formed before it is written to disk (story 9).
func ValidateProfile(p Profile) error {
	if p.Name == "" {
		return fmt.Errorf("name is required")
	}
	if p.Email == "" {
		return fmt.Errorf("email is required")
	}
	if !strings.Contains(p.Email, "@") {
		return fmt.Errorf("email must be a valid email address, got %q", p.Email)
	}
	for i, e := range p.Education {
		if e.Degree == "" {
			return fmt.Errorf("education[%d]: degree is required", i)
		}
		if e.Institution == "" {
			return fmt.Errorf("education[%d]: institution is required", i)
		}
	}
	for i, c := range p.Certifications {
		if c.Title == "" {
			return fmt.Errorf("certifications[%d]: title is required", i)
		}
	}
	for i, l := range p.Languages {
		if l.Name == "" {
			return fmt.Errorf("languages[%d]: name is required", i)
		}
	}
	return nil
}
