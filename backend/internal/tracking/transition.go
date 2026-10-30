package tracking

import (
	"errors"
	"fmt"
	"time"

	"github.com/gio-del/sumisura/backend/internal/recordversion"
)

// ErrInvalidTransition marks a requested Status change that CanTransition
// rejects — the tracker only moves along CONTEXT.md's Status chain (Saved →
// Tailoring → Sent → Interviewing → Rejected/Offer), with Reopen (Rejected →
// Interviewing) as the sole allowed backward move, so any other manual
// correction or skipped stage is caught here rather than silently
// corrupting the pipeline view.
var ErrInvalidTransition = errors.New("invalid status transition")

// allowedTransitions is the Status state machine (story 4): each Status
// maps to the Statuses it may move to next. A missing/empty entry means
// terminal (Offer). Rejected is reachable from Sent directly (a rejection
// without an interview) as well as from Interviewing, and can itself move
// back to Interviewing via Reopen for a premature rejection — Offer stays
// the only fully terminal Status.
//
// Withdrawn records the user ending the process on their own initiative
// (distinct from Rejected, which means the employer ended it) and is
// reachable from any pre-terminal Status — Saved, Tailoring, Sent,
// Interviewing — additively, alongside each Status's existing moves.
// Withdrawn is itself terminal-but-reopenable, mirroring Rejected's Reopen
// precedent exactly: its only outbound move is back to Interviewing.
var allowedTransitions = map[Status][]Status{
	StatusSaved:        {StatusTailoring, StatusWithdrawn},
	StatusTailoring:    {StatusSent, StatusWithdrawn},
	StatusSent:         {StatusInterviewing, StatusRejected, StatusWithdrawn},
	StatusInterviewing: {StatusRejected, StatusOffer, StatusWithdrawn},
	StatusRejected:     {StatusInterviewing},
	StatusOffer:        {},
	StatusWithdrawn:    {StatusInterviewing},
}

// CanTransition reports whether an Application may move from `from` to
// `to`, per the Status state machine.
func CanTransition(from, to Status) bool {
	for _, next := range allowedTransitions[from] {
		if next == to {
			return true
		}
	}
	return false
}

// Transition validates a Status change as a pure function, independent of
// storage (per the PRD's Testing Decisions), returning to if the move is
// allowed.
func Transition(current, to Status) (Status, error) {
	if _, ok := allowedTransitions[current]; !ok {
		return "", fmt.Errorf("%w: unknown status %q", ErrValidation, current)
	}
	if _, ok := allowedTransitions[to]; !ok {
		return "", fmt.Errorf("%w: unknown status %q", ErrValidation, to)
	}
	if !CanTransition(current, to) {
		return "", fmt.Errorf("%w: cannot move from %q to %q", ErrInvalidTransition, current, to)
	}
	return to, nil
}

// UpdateApplicationStatus validates and applies a Status change to the
// Application identified by id, writing it back to disk if allowed.
func UpdateApplicationStatus(dataDir, id string, to Status) (Application, error) {
	return UpdateApplicationStatusIfMatch(dataDir, id, to, "")
}

// UpdateApplicationStatusIfMatch is UpdateApplicationStatus, refusing the
// write with recordversion.ErrMismatch when version no longer matches the
// Application file on disk — so a stale tab cannot move a record backwards
// through the funnel and take the Status history with it (issue #89, story
// 15). An empty version writes unconditionally.
func UpdateApplicationStatusIfMatch(dataDir, id string, to Status, version string) (Application, error) {
	application, err := getApplication(dataDir, id)
	if err != nil {
		return Application{}, err
	}

	// Unlike the other Application mutations, this one validates against
	// the Status already on disk — so a stale caller would otherwise be
	// told its move is an invalid transition, sending it hunting for a
	// mistake in its own input when the real cause is that the record
	// changed underneath (issue #89, story 20). Checking here answers with
	// the truthful reason; the write below still re-checks immediately
	// before writing, which is what keeps the window small.
	if err := recordversion.Check(applicationPath(dataDir, id), version); err != nil {
		return Application{}, err
	}

	newStatus, err := Transition(application.Status, to)
	if err != nil {
		return Application{}, err
	}
	application.Status = newStatus
	application.StatusHistory = append(application.StatusHistory, StatusChange{Status: newStatus, ChangedAt: time.Now().UTC()})
	application.StatusUpdatedAt = time.Now().UTC().Format(time.RFC3339Nano)
	application.IsStale = IsStaleWithFollowUp(application.Status, application.StatusUpdatedAt, application.FollowUpAt, time.Now(), DefaultStaleThreshold)

	return writeApplicationIfMatch(dataDir, id, application, version)
}
