package generation

import "testing"

func TestFindClientNames_FlagsAHiddenClientNamedInABullet(t *testing.T) {
	cv := cvData{Experience: []cvExperience{{
		Employer: "Acme", Client: "Amplifon", ClientLabel: "Global retailer",
		Bullets: []string{"Built Amplifon's AI Platform."},
	}, {
		Employer: "Acme", Client: "Autostrade per l'Italia", ClientLabel: "Motorway operator",
		Bullets: []string{"Migrated autostrade tables."},
	}}}

	warnings := findClientNames(cv)

	if len(warnings) != 2 {
		t.Fatalf("want both names flagged (full name and distinctive first word), got %#v", warnings)
	}
}

func TestFindClientNames_SilentWithoutALabel(t *testing.T) {
	cv := cvData{Experience: []cvExperience{{Employer: "Acme", Client: "Amplifon", Bullets: []string{"Built Amplifon's AI Platform."}}}}

	if warnings := findClientNames(cv); len(warnings) != 0 {
		t.Fatalf("a client shown by name is no leak, got %#v", warnings)
	}
}
