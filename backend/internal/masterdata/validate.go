package masterdata

import (
	"fmt"
	"regexp"
)

var dateRe = regexp.MustCompile(`^\d{4}(-\d{2})?$`)

// Validate checks that an Entry's frontmatter fields are well-formed and
// complete enough to write to disk, per story 9: reject invalid edits with a
// clear error before anything touches disk.
func Validate(entryType string, e Entry) error {
	if !dateRe.MatchString(e.Start) {
		return fmt.Errorf("start must be a year (YYYY) or year-month (YYYY-MM), got %q", e.Start)
	}
	if e.End != nil && *e.End != "" && !dateRe.MatchString(*e.End) {
		return fmt.Errorf("end must be a year (YYYY) or year-month (YYYY-MM), got %q", *e.End)
	}

	switch entryType {
	case TypeExperience:
		if e.Employer == "" {
			return fmt.Errorf("employer is required")
		}
		if e.Role == "" {
			return fmt.Errorf("role is required")
		}
		switch e.Kind {
		case "", KindConsultancy, KindInHouse:
		default:
			return fmt.Errorf("kind must be %q or %q when set, got %q", KindConsultancy, KindInHouse, e.Kind)
		}
	case TypeProject:
		if e.Name == "" {
			return fmt.Errorf("name is required")
		}
		if e.Kind != "" {
			return fmt.Errorf("kind describes an employer relationship and applies to experience entries only")
		}
	default:
		return fmt.Errorf("unknown entry type %q", entryType)
	}
	return nil
}
