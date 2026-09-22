package tracking

import (
	"errors"
	"fmt"
)

// Schema versions of the persisted Job Listing and Application records
// (issue #100, ADR-0034). Each record carries its own schemaVersion key —
// in a Job Listing's frontmatter, at the top level of an Application — and
// each GenerationRecord inside an Application carries one too, because an
// Application written long ago keeps accumulating Generations afterwards.
//
// The version says what an absent field means:
//
//   - On a record at CurrentSchemaVersion, absent means genuinely none: a
//     Generation with no sourceSnippetIds used no Cover Letter Snippet, one
//     with no entryIds recorded no Selection, a Job Listing always carries
//     freshnessStatus.
//   - On a LegacySchemaVersion record, absent means unknowable: the record
//     may simply predate the field.
//
// Adding the next version is: bump CurrentSchemaVersion, stamp the new
// field's guarantee in this comment, teach MigrateRecords the step from the
// previous version (what it can backfill and what it must report as
// unknowable), and record the change in ADR-0034. Never add a new "absent
// for records written before this field existed" tolerance to a reader
// instead.
const (
	// LegacySchemaVersion is every record written before schemaVersion
	// existed. It is the zero value, so a file with no schemaVersion key
	// reads as legacy and renders without the key.
	LegacySchemaVersion = 0

	// CurrentSchemaVersion is what Save stamps on new Job Listings and
	// Applications, what RecordGeneration stamps on new Generations, and
	// what MigrateRecords brings legacy Job Listings and Applications to.
	//
	// Version 1 guarantees: a Job Listing carries freshnessStatus; an
	// Application created at v1 carries statusUpdatedAt and a statusHistory
	// starting at Saved (a migrated one carries them only if it was still
	// at Saved when migrated — see MigrateRecords); archived and notes are
	// optional keys whose absence means false and no Notes respectively.
	//
	// Version 2 adds Job Listing location (issue #206). On a v2 record an
	// absent location means the source had none, or the user has not
	// entered one; on a v1 or legacy record it is unknowable, since
	// nothing on disk records where the role was. MigrateRecords stamps
	// the version and reports location as unknowable — there is nothing to
	// backfill it from, and re-deriving it from the Job Description would
	// be a guess written down as a fact.
	CurrentSchemaVersion = 2
)

// ErrUnsupportedSchemaVersion marks a record stamped at a version this
// build does not know — typically a corpus written by newer code. It is an
// error rather than being read as legacy, so older code fails loudly
// instead of misreading (or rewriting) a record it does not understand.
var ErrUnsupportedSchemaVersion = errors.New("unsupported schema version")

// checkSchemaVersion rejects any version outside [Legacy, Current].
func checkSchemaVersion(version int) error {
	if version < LegacySchemaVersion || version > CurrentSchemaVersion {
		return fmt.Errorf("%w: schemaVersion %d (this build reads up to %d)", ErrUnsupportedSchemaVersion, version, CurrentSchemaVersion)
	}
	return nil
}

// checkApplicationSchemaVersions checks an Application's own version and
// that of every Generation it holds.
func checkApplicationSchemaVersions(raw rawApplication) error {
	if err := checkSchemaVersion(raw.SchemaVersion); err != nil {
		return err
	}
	for i, g := range raw.Generations {
		if err := checkSchemaVersion(g.SchemaVersion); err != nil {
			return fmt.Errorf("generations[%d]: %w", i, err)
		}
	}
	return nil
}

// IsLegacy reports whether g was recorded before GenerationRecords carried
// a schema version, in which case an absent SourceSnippetIDs, EntryIDs,
// Usage or Language is unknowable rather than genuinely empty.
func (g GenerationRecord) IsLegacy() bool {
	return g.SchemaVersion == LegacySchemaVersion
}
