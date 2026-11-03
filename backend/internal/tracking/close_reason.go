package tracking

import (
	"fmt"
	"strings"
)

// CloseReason is why an Application ended at Rejected or Withdrawn
// (issue #278): a fixed vocabulary, so the Stats page can count it. It is
// optional and carries no deadline or reminder.
type CloseReason string

const (
	CloseReasonNoResponse             CloseReason = "no-response"
	CloseReasonRejectedAfterScreening CloseReason = "rejected-after-screening"
	CloseReasonRejectedAfterInterview CloseReason = "rejected-after-interview"
	CloseReasonSalaryMismatch         CloseReason = "salary-mismatch"
	CloseReasonPositionClosed         CloseReason = "position-closed"
	CloseReasonOther                  CloseReason = "other"
)

// AllCloseReasons lists every CloseReason in display order.
var AllCloseReasons = []CloseReason{
	CloseReasonNoResponse,
	CloseReasonRejectedAfterScreening,
	CloseReasonRejectedAfterInterview,
	CloseReasonSalaryMismatch,
	CloseReasonPositionClosed,
	CloseReasonOther,
}

// AcceptsCloseReason reports whether a CloseReason may be set while the
// Application is at s.
func AcceptsCloseReason(s Status) bool {
	return s == StatusRejected || s == StatusWithdrawn
}

// validateCloseReason checks reason against the vocabulary and against the
// Status it would be stored with. An empty reason is always valid.
func validateCloseReason(reason CloseReason, status Status) error {
	if reason == "" {
		return nil
	}
	known := false
	for _, r := range AllCloseReasons {
		if r == reason {
			known = true
			break
		}
	}
	if !known {
		return fmt.Errorf("%w: unknown closeReason %q", ErrValidation, reason)
	}
	if !AcceptsCloseReason(status) {
		return fmt.Errorf("%w: closeReason applies only to Rejected or Withdrawn, not %q", ErrValidation, status)
	}
	return nil
}

// UpdateApplicationCloseReasonIfMatch sets (or clears, when reason is
// empty) the CloseReason of an Application already at Rejected or
// Withdrawn, refusing the write with recordversion.ErrMismatch when version
// no longer matches the file on disk. An empty version writes
// unconditionally.
func UpdateApplicationCloseReasonIfMatch(dataDir, id string, reason CloseReason, version string) (Application, error) {
	reason = CloseReason(strings.TrimSpace(string(reason)))
	application, err := getApplication(dataDir, id)
	if err != nil {
		return Application{}, err
	}
	if err := validateCloseReason(reason, application.Status); err != nil {
		return Application{}, err
	}
	application.CloseReason = reason
	return writeApplicationIfMatch(dataDir, id, application, version)
}
