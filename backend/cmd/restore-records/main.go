// Command restore-records writes the records of an export zip (GET
// /api/export, tracking.ExportData) back under a data directory. It is a
// thin wrapper over tracking.RestoreData: flag parsing and printing the
// report, nothing else.
//
// By default it is a dry run that only reports. Pass -write to apply. A file
// that already exists is never touched unless -overwrite is also passed. An
// entry outside jobs/, applications/ and pending-captures/ (or one with ".."
// or an absolute path) fails the command before anything is written.
//
// Exit status: 0 when nothing is pending (or -write applied everything), 1
// on an error, 2 on a bad flag or missing archive argument, and 3 when a dry
// run found records to create or overwrite.
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

//nolint:errcheck // CLI output; if the terminal write fails, the exit status still reports the outcome
func run(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("restore-records", flag.ContinueOnError)
	flags.SetOutput(stderr)
	defaultDataDir := os.Getenv("DATA_DIR")
	if defaultDataDir == "" {
		defaultDataDir = "data"
	}
	dataDir := flags.String("data-dir", defaultDataDir, "data directory to restore into (default $DATA_DIR, else ./data)")
	write := flags.Bool("write", false, "apply the restore; without it, only report what would change")
	overwrite := flags.Bool("overwrite", false, "replace files that already exist (default: leave them untouched)")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if flags.NArg() != 1 {
		fmt.Fprintln(stderr, "usage: restore-records [-data-dir DIR] [-write] [-overwrite] EXPORT.zip")
		return 2
	}

	absDataDir, err := filepath.Abs(*dataDir)
	if err != nil {
		absDataDir = *dataDir
	}
	if *write {
		fmt.Fprintf(stdout, "Restoring %s into %s.\n", flags.Arg(0), absDataDir)
	} else {
		fmt.Fprintf(stdout, "Dry run restoring %s into %s: nothing will be written (pass -write to apply).\n", flags.Arg(0), absDataDir)
	}

	report, err := tracking.RestoreData(flags.Arg(0), *dataDir, tracking.RestoreOptions{DryRun: !*write, Overwrite: *overwrite})
	if err != nil {
		fmt.Fprintf(stderr, "restore-records: %v\n", err)
		if *write {
			fmt.Fprintln(stderr, "If this failed while writing, some records may already be restored; run the dry run again to see what is left.")
		} else {
			fmt.Fprintln(stderr, "Nothing was written.")
		}
		return 1
	}

	created, overwritten := "created", "overwritten"
	if report.DryRun {
		created, overwritten = "would create", "would overwrite"
	}
	for _, p := range report.Created {
		fmt.Fprintf(stdout, "  %-15s %s\n", created, p)
	}
	for _, p := range report.Overwritten {
		fmt.Fprintf(stdout, "  %-15s %s\n", overwritten, p)
	}
	for _, p := range report.Skipped {
		fmt.Fprintf(stdout, "  %-15s %s\n", "skipped", p)
	}
	fmt.Fprintf(stdout, "\n%d to create, %d to overwrite, %d skipped.\n", len(report.Created), len(report.Overwritten), len(report.Skipped))
	if len(report.Skipped) > 0 && !*overwrite {
		fmt.Fprintln(stdout, "Skipped files already exist and were left untouched; pass -overwrite to replace them.")
	}
	if report.DryRun && report.Pending() {
		fmt.Fprintln(stdout, "Re-run with -write to apply.")
	}
	if report.Pending() {
		fmt.Fprintln(stdout, "If the export came from an earlier version, run migrate-records next (cd backend && go run ./cmd/migrate-records -data-dir ../data).")
	}

	if report.DryRun && report.Pending() {
		return exitPending
	}
	return 0
}
