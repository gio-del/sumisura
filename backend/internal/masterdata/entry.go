package masterdata

// Entry is a Master Data Entry (Client Engagement or project), covering the
// union of fields either kind of Entry frontmatter can carry. Which fields
// are meaningful is determined by Type.
type Entry struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Employer string `json:"employer,omitempty"`
	// Kind is the experience Entry's shape (ADR-0045): KindConsultancy,
	// where the employer is a consultancy and this Entry is one Client
	// Engagement, or KindInHouse, where the employer is itself the job.
	// Empty means "not recorded" and is resolved by Shape, never by readers.
	Kind   string `json:"kind,omitempty"`
	Client string `json:"client,omitempty"`
	// ClientLabel is printed instead of Client (e.g. "Global hearing-care
	// retailer"), so the real name stays in Master Data; Title is the
	// project name the CV prints in bold, falling back to Role (issue #232).
	ClientLabel string `json:"clientLabel,omitempty"`
	Title       string `json:"title,omitempty"`
	// Group joins this Client Engagement with the others of its employer that
	// carry the same Group into one project on the CV: Group is its title,
	// and the client labels, tags and bullets are merged (issue #232).
	Group    string   `json:"group,omitempty"`
	Role     string   `json:"role,omitempty"`
	Name     string   `json:"name,omitempty"`
	Location string   `json:"location,omitempty"`
	Start    string   `json:"start"`
	End      *string  `json:"end"`
	Flagship bool     `json:"flagship,omitempty"`
	Tags     []string `json:"tags"`
	Repo     string   `json:"repo,omitempty"`
	Bullets  []string `json:"bullets,omitempty"`

	// Version is the Entry file's version token, populated by the API
	// layer (via EntryVersion) on reads and never persisted — the opaque
	// value the FE sends back in an If-Match header so a save that would
	// overwrite someone else's change is refused instead (issue #89).
	Version string `json:"version,omitempty"`

	// LastModified is populated by the API layer (via EntryLastModified),
	// never by ListEntries/GetEntry themselves — it depends on a
	// projectRoot they don't take. Left nil when there's no git history to
	// report (a fresh, uncommitted Entry, or a lookup with no repository
	// behind it).
	LastModified *LastModified `json:"lastModified,omitempty"`
}

const (
	TypeExperience = "experience"
	TypeProject    = "project"
)
