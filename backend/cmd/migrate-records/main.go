// Command migrate-records brings every Job Listing and Application record
// under a data directory to the current schema version (issue #100,
// ADR-0034). It is a thin wrapper over tracking.MigrateRecords: flag
// parsing and printing the report, nothing else.
//
// By default it is a dry run that only reports. Pass -write to apply.
//
// It also reports any posting held by more than one Job Listing, a corpus
// written before ADR-0042 made one posting exactly one Job Listing. Those
// are named, never resolved: -write merges, archives and deletes nothing.
//
// Exit status: 0 when nothing is pending (or -write applied everything), 1
// on an error (an unparseable record, a record at a newer schema version,
// an I/O failure — nothing is written in the first two cases), 2 on a bad
// flag, and 3 when a dry run found pending work: records still to migrate,
// or duplicated postings to resolve by hand.
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/gio-del/sumisura/backend/internal/tracking"
)

const exitPending = 3

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("migrate-records", flag.ContinueOnError)
	flags.SetOutput(stderr)
	defaultDataDir := os.Getenv("DATA_DIR")
	if defaultDataDir == "" {
		defaultDataDir = "data"
	}
	dataDir := flags.String("data-dir", defaultDataDir, "data directory holding jobs/ and applications/ (default $DATA_DIR, else ./data)")
	write := flags.Bool("write", false, "apply the migration; without it, only report what would change")
	if err := flags.Parse(args); err != nil {
		return 2
	}

	absDataDir, err := filepath.Abs(*dataDir)
	if err != nil {
		absDataDir = *dataDir
	}
	if *write {
		fmt.Fprintf(stdout, "Migrating records under %s to schema version %d.\n", absDataDir, tracking.CurrentSchemaVersion) //nolint:errcheck // CLI output; if the terminal write fails, the exit status still reports the outcome
	} else {
		fmt.Fprintf(stdout, "Dry run over %s: nothing will be written (pass -write to apply).\n", absDataDir) //nolint:errcheck // CLI output; if the terminal write fails, the exit status still reports the outcome
	}
	if _, statErr := os.Stat(absDataDir); os.IsNotExist(statErr) {
		fmt.Fprintf(stdout, "Note: %s does not exist.\n", absDataDir) //nolint:errcheck // CLI output; if the terminal write fails, the exit status still reports the outcome
	}

	report, err := tracking.MigrateRecords(*dataDir, tracking.MigrationOptions{DryRun: !*write})
	if err != nil {
		fmt.Fprintf(stderr, "migrate-records: %v\n", err) //nolint:errcheck // CLI output; if the terminal write fails, the exit status still reports the outcome
		// Every record is read and checked before the first write, so only
		// an I/O failure while writing can leave the run half-applied — and
		// each record it did write was replaced atomically.
		if *write {
			fmt.Fprintln(stderr, "If this failed while writing, some records may already be migrated; run the dry run again to see what is still pending.") //nolint:errcheck // CLI output; if the terminal write fails, the exit status still reports the outcome
		} else {
			fmt.Fprintln(stderr, "Nothing was written.") //nolint:errcheck // CLI output; if the terminal write fails, the exit status still reports the outcome
		}
		return 1
	}
	printReport(stdout, report)
	printOutcome(stdout, report)

	// A dry run reports pending work with exit 3. Duplicated postings are
	// pending work no -write run resolves — they need a human decision —
	// so they never make a write run report failure.
	if report.DryRun && report.Pending() {
		return exitPending
	}
	return 0
}

//nolint:errcheck // prints a CLI report; if the terminal write fails, the exit status still reports the outcome
func printOutcome(w io.Writer, report tracking.MigrationReport) {
	switch {
	case len(report.Migrated) == 0:
		fmt.Fprintf(w, "\nAll %d records are already at schema version %d.\n", report.Scanned, tracking.CurrentSchemaVersion)
	case report.DryRun:
		fmt.Fprintf(w, "\n%d of %d records would be migrated. Re-run with -write to apply.\n", len(report.Migrated), report.Scanned)
	default:
		fmt.Fprintf(w, "\nMigrated %d of %d records.\n", len(report.Migrated), report.Scanned)
	}
	if len(report.DuplicatePostings) > 0 {
		fmt.Fprintf(w, "%d posting(s) are held by more than one Job Listing (listed above). Resolve these yourself:\n", len(report.DuplicatePostings))
		fmt.Fprintln(w, "keep the record whose history you want and archive or delete the others. This tool never merges two Applications.")
		return
	}
	if !report.Pending() {
		fmt.Fprintln(w, "Nothing to do.")
	}
}

//nolint:errcheck // prints a CLI report; if the terminal write fails, the exit status still reports the outcome
func printReport(w io.Writer, report tracking.MigrationReport) {
	fmt.Fprintf(w, "Scanned %d records.\n", report.Scanned)
	for _, m := range report.Migrated {
		fmt.Fprintf(w, "\n%s (%s): schema version %d -> %d\n", m.Path, m.Kind, m.FromVersion, m.ToVersion)
		for _, f := range m.Backfilled {
			fmt.Fprintf(w, "  backfilled   %s = %s\n", f.Field, f.Value)
		}
		for _, f := range m.Unknowable {
			fmt.Fprintf(w, "  left empty   %s: %s\n", f.Field, f.Reason)
		}
		if len(m.Backfilled) == 0 && len(m.Unknowable) == 0 {
			fmt.Fprintln(w, "  version stamp only")
		}
	}
	if len(report.Inconsistencies) > 0 {
		fmt.Fprintln(w, "\nInconsistencies (reported, not changed):")
		for _, inc := range report.Inconsistencies {
			fmt.Fprintf(w, "  %s: %s\n", inc.Path, inc.Problem)
		}
	}
	if len(report.DuplicatePostings) > 0 {
		fmt.Fprintln(w, "\nPostings held by more than one Job Listing (reported, never resolved):")
		for _, dup := range report.DuplicatePostings {
			fmt.Fprintf(w, "  %s\n", dup.PostingKey)
			for _, l := range dup.Listings {
				archived := ""
				if l.Archived {
					archived = ", archived"
				}
				fmt.Fprintf(w, "    %s  %s  saved %s  status %s%s\n", l.Path, titleOrUntitled(l.Title), l.SavedAt, statusOrUnknown(l.Status), archived)
			}
		}
	}
}

// titleOrUntitled keeps the report's columns readable for a Job Listing
// saved without a Job Title.
func titleOrUntitled(title string) string {
	if title == "" {
		return "(no job title)"
	}
	return title
}

// statusOrUnknown covers a Job Listing whose Application file is missing,
// already reported as an inconsistency above.
func statusOrUnknown(status tracking.Status) string {
	if status == "" {
		return "(no application)"
	}
	return string(status)
}
