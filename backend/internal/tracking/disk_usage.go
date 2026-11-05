package tracking

import (
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// DefaultSupersededAfterDays is how long a closed (Rejected/Withdrawn)
// Application's Generations are kept before they are proposed as superseded.
const DefaultSupersededAfterDays = 90

// GenerationDiskUsage is one output/<slug>/ directory's footprint.
type GenerationDiskUsage struct {
	Slug string `json:"slug"`
	// Bytes is the sum of the sizes of every file under the directory.
	Bytes     int64  `json:"bytes"`
	CreatedAt string `json:"createdAt"`
	// ApplicationID is empty for a directory no Application record claims.
	ApplicationID string `json:"applicationId,omitempty"`
	Company       string `json:"company,omitempty"`
	JobTitle      string `json:"jobTitle,omitempty"`
	// Superseded is only a proposal for the user to review: the Generation
	// is not the latest of its Application, or the Application has been
	// closed for longer than the threshold. The latest Generation of an
	// open Application, and any unrecorded directory, is never superseded.
	Superseded bool `json:"superseded"`
}

// DiskUsage is the footprint of output/ (issue #285).
type DiskUsage struct {
	TotalBytes  int64                 `json:"totalBytes"`
	Generations []GenerationDiskUsage `json:"generations"`
}

// GetDiskUsage sizes every output/<slug>/ directory and marks the ones that
// are superseded as of now, with closed Applications counting as old after
// olderThanDays days. Records whose directory is gone are not listed: there
// is nothing left to free. Nothing is ever deleted here.
func GetDiskUsage(dataDir, projectRoot string, olderThanDays int, now time.Time) (DiskUsage, error) {
	listings, err := List(dataDir)
	if err != nil {
		return DiskUsage{}, err
	}

	type owner struct {
		appID, company, title string
		createdAt             string
		superseded            bool
	}
	owners := map[string]owner{}
	cutoff := now.AddDate(0, 0, -olderThanDays)
	for _, l := range listings {
		app := l.Application
		closedLongAgo := false
		if AcceptsCloseReason(app.Status) {
			if t, err := time.Parse(time.RFC3339Nano, app.StatusUpdatedAt); err == nil && t.Before(cutoff) {
				closedLongAgo = true
			}
		}
		latest := -1
		for i, g := range app.Generations {
			if latest < 0 || g.CreatedAt >= app.Generations[latest].CreatedAt {
				latest = i
			}
		}
		for i, g := range app.Generations {
			owners[g.Slug] = owner{
				appID: app.ID, company: l.JobListing.Company, title: l.JobListing.Title,
				createdAt:  g.CreatedAt,
				superseded: i != latest || closedLongAgo,
			}
		}
	}

	entries, err := os.ReadDir(filepath.Join(projectRoot, "output"))
	if err != nil && !os.IsNotExist(err) {
		return DiskUsage{}, err
	}
	usage := DiskUsage{Generations: []GenerationDiskUsage{}}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		size, err := dirSize(filepath.Join(projectRoot, "output", e.Name()))
		if err != nil {
			return DiskUsage{}, err
		}
		row := GenerationDiskUsage{Slug: e.Name(), Bytes: size, CreatedAt: createdAtFromSlug(e.Name())}
		if o, ok := owners[e.Name()]; ok {
			row.CreatedAt, row.ApplicationID, row.Company, row.JobTitle, row.Superseded = o.createdAt, o.appID, o.company, o.title, o.superseded
		}
		usage.TotalBytes += size
		usage.Generations = append(usage.Generations, row)
	}
	sort.SliceStable(usage.Generations, func(i, j int) bool {
		a, b := usage.Generations[i], usage.Generations[j]
		if a.CreatedAt != b.CreatedAt {
			return a.CreatedAt > b.CreatedAt
		}
		return a.Slug > b.Slug
	})
	return usage, nil
}

func dirSize(dir string) (int64, error) {
	var total int64
	err := filepath.WalkDir(dir, func(_ string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		total += info.Size()
		return nil
	})
	return total, err
}
