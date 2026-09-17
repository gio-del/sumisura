package tracking

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/gio-del/sumisura/backend/internal/atomicfile"
	"github.com/gio-del/sumisura/backend/internal/postingkey"
)

// pendingCapturesDir holds one JSON file per Pending Capture (issue #182).
const pendingCapturesDir = "pending-captures"

// PendingCaptureSchemaVersion is the record format of a Pending Capture.
// The record type was born versioned, so there is no legacy form (ADR-0034).
const PendingCaptureSchemaVersion = 1

// sharedTextLimit caps what is kept of a share sheet's text: enough for a
// title and company hint, not a whole pasted Job Description.
const sharedTextLimit = 500

// PendingCapture is a link to a job posting saved from another device (a
// phone's share sheet) to turn into a Job Listing later, once its Job
// Description is known. It is deliberately not a Job Listing: it has no
// Application, no Status and no Job Description, and it never counts toward
// the funnel, duplicate detection or Generation (ADR-0041).
type PendingCapture struct {
	SchemaVersion int    `json:"schemaVersion"`
	ID            string `json:"id"`
	URL           string `json:"url"`
	PostingKey    string `json:"postingKey"`
	Provider      string `json:"provider"`
	// Title and Company are best-effort hints from what was shared — the
	// share's own title, or an "at <Company>" in its text — for the user to
	// confirm when completing. Never trusted as-is.
	Title      string `json:"title,omitempty"`
	Company    string `json:"company,omitempty"`
	SharedText string `json:"sharedText,omitempty"`
	SavedAt    string `json:"savedAt"`
}

// PendingCaptureInput is what a share sheet hands over: a URL, some text
// with a URL in it, or both, plus an optional title.
type PendingCaptureInput struct {
	URL   string
	Text  string
	Title string
}

// PendingCaptureOutcome says what adding a shared link did.
type PendingCaptureOutcome string

const (
	// OutcomePending: a new Pending Capture was saved.
	OutcomePending PendingCaptureOutcome = "pending"
	// OutcomeAlreadyPending: the posting was already waiting in the inbox;
	// nothing was written.
	OutcomeAlreadyPending PendingCaptureOutcome = "already-pending"
	// OutcomeAlreadyTracked: a Job Listing for the posting already exists;
	// nothing was written.
	OutcomeAlreadyTracked PendingCaptureOutcome = "already-tracked"
)

// AddPendingCaptureResult is AddPendingCapture's result: the outcome, plus
// the Pending Capture (pending, already-pending) or the id of the Job
// Listing that already tracks the posting (already-tracked).
type AddPendingCaptureResult struct {
	Outcome        PendingCaptureOutcome
	PendingCapture *PendingCapture
	JobListingID   string
}

var (
	pendingCaptureID = regexp.MustCompile(`^[a-z]+-[0-9a-f]{12}$`)
	// companyHint matches LinkedIn's share text, "… at Acme: https://…", and
	// the "… at Acme - https://…" variants some apps produce.
	companyHint = regexp.MustCompile(`(?i)\bat\s+([^:\n]+?)\s*[:\-–|]?\s*https?://`)
)

// AddPendingCapture saves a shared link as a Pending Capture, unless the
// posting it points at is already pending or already a Job Listing — the
// same posting shared twice leaves one record, and a posting already
// tracked is reported rather than duplicated.
func AddPendingCapture(dataDir string, in PendingCaptureInput) (AddPendingCaptureResult, error) {
	link := postingkey.ExtractURL(in.URL)
	if link == "" {
		link = postingkey.ExtractURL(in.Text)
	}
	key, ok := postingkey.Of(link)
	if !ok {
		return AddPendingCaptureResult{}, fmt.Errorf("%w: no http(s) link found in what was shared", ErrValidation)
	}

	listings, err := List(dataDir)
	if err != nil {
		return AddPendingCaptureResult{}, err
	}
	for _, l := range listings {
		if other, ok := postingkey.Of(l.JobListing.URL); ok && other == key {
			return AddPendingCaptureResult{Outcome: OutcomeAlreadyTracked, JobListingID: l.JobListing.ID}, nil
		}
	}

	id := pendingCaptureIDFor(key)
	if existing, err := GetPendingCapture(dataDir, id); err == nil {
		return AddPendingCaptureResult{Outcome: OutcomeAlreadyPending, PendingCapture: &existing}, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return AddPendingCaptureResult{}, err
	}

	capture := PendingCapture{
		SchemaVersion: PendingCaptureSchemaVersion,
		ID:            id,
		URL:           postingkey.CanonicalURL(link),
		PostingKey:    key.String(),
		Provider:      string(key.Provider),
		Title:         strings.TrimSpace(in.Title),
		Company:       companyHintFrom(in.Text),
		SharedText:    sharedTextFrom(in.Text, link),
		SavedAt:       time.Now().UTC().Format(time.RFC3339Nano),
	}
	if err := writePendingCapture(dataDir, capture); err != nil {
		return AddPendingCaptureResult{}, err
	}
	return AddPendingCaptureResult{Outcome: OutcomePending, PendingCapture: &capture}, nil
}

// ListPendingCaptures reads every Pending Capture, newest first. A missing
// directory is an ordinary empty inbox.
func ListPendingCaptures(dataDir string) ([]PendingCapture, error) {
	files, err := os.ReadDir(filepath.Join(dataDir, pendingCapturesDir))
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var result []PendingCapture
	for _, f := range files {
		id, isRecord := strings.CutSuffix(f.Name(), ".json")
		if f.IsDir() || !isRecord || !pendingCaptureID.MatchString(id) {
			continue
		}
		capture, err := GetPendingCapture(dataDir, id)
		if err != nil {
			return nil, fmt.Errorf("reading pending capture %s: %w", f.Name(), err)
		}
		result = append(result, capture)
	}
	sort.SliceStable(result, func(i, j int) bool { return result[i].SavedAt > result[j].SavedAt })
	return result, nil
}

// GetPendingCapture reads one Pending Capture; os.ErrNotExist when there is
// no such record (including an id that could never be one).
func GetPendingCapture(dataDir, id string) (PendingCapture, error) {
	if !pendingCaptureID.MatchString(id) {
		return PendingCapture{}, os.ErrNotExist
	}
	content, err := os.ReadFile(pendingCapturePath(dataDir, id))
	if err != nil {
		return PendingCapture{}, err
	}
	var capture PendingCapture
	if err := json.Unmarshal(content, &capture); err != nil {
		return PendingCapture{}, err
	}
	return capture, nil
}

// DeletePendingCapture removes a Pending Capture — the user dismissing it,
// or its completion into a Job Listing. os.ErrNotExist when there is none.
func DeletePendingCapture(dataDir, id string) error {
	if !pendingCaptureID.MatchString(id) {
		return os.ErrNotExist
	}
	return os.Remove(pendingCapturePath(dataDir, id))
}

// CompletePendingCaptureRequest is what the user confirms to turn a Pending
// Capture into a Job Listing. Title falls back to the capture's own hint.
type CompletePendingCaptureRequest struct {
	Company        string
	Title          string
	JobDescription string
}

// CompletePendingCapture saves a Job Listing for the Pending Capture's link
// through the ordinary Save — same validation, RAL Range, Application
// Method and Application — and only then removes the Pending Capture. A
// crash between the two leaves a leftover Pending Capture at worst (the
// next share of that posting reports it as already tracked), never a lost
// Job Listing.
func CompletePendingCapture(ctx context.Context, dataDir string, client Client, doer HTTPDoer, id string, req CompletePendingCaptureRequest) (JobListing, Application, error) {
	capture, err := GetPendingCapture(dataDir, id)
	if err != nil {
		return JobListing{}, Application{}, err
	}
	title := strings.TrimSpace(req.Title)
	if title == "" {
		title = capture.Title
	}
	listing, application, err := Save(ctx, dataDir, client, doer, SaveRequest{
		Title:          title,
		Company:        strings.TrimSpace(req.Company),
		URL:            capture.URL,
		JobDescription: strings.TrimSpace(req.JobDescription),
	})
	if err != nil {
		return JobListing{}, Application{}, err
	}
	if err := DeletePendingCapture(dataDir, id); err != nil && !errors.Is(err, os.ErrNotExist) {
		return listing, application, fmt.Errorf("job listing saved, but removing pending capture %s failed: %w", id, err)
	}
	return listing, application, nil
}

// RemovePendingCaptureFor removes the Pending Capture for the posting
// rawURL points at, if there is one, and returns its id ("" when none was
// pending). It is how saving a Job Listing some other way — a desktop
// extension capture, a manual paste — completes the link shared earlier
// from a phone (issue #183).
func RemovePendingCaptureFor(dataDir, rawURL string) (string, error) {
	key, ok := postingkey.Of(rawURL)
	if !ok {
		return "", nil
	}
	id := pendingCaptureIDFor(key)
	if err := DeletePendingCapture(dataDir, id); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", nil
		}
		return "", err
	}
	return id, nil
}

func pendingCaptureIDFor(key postingkey.Key) string {
	sum := sha256.Sum256([]byte(key.String()))
	return string(key.Provider) + "-" + hex.EncodeToString(sum[:])[:12]
}

func pendingCapturePath(dataDir, id string) string {
	return filepath.Join(dataDir, pendingCapturesDir, id+".json")
}

func writePendingCapture(dataDir string, capture PendingCapture) error {
	if err := os.MkdirAll(filepath.Join(dataDir, pendingCapturesDir), 0o755); err != nil {
		return err
	}
	content, err := json.MarshalIndent(capture, "", "  ")
	if err != nil {
		return err
	}
	return atomicfile.WriteFile(pendingCapturePath(dataDir, capture.ID), append(content, '\n'), 0o644)
}

func companyHintFrom(text string) string {
	m := companyHint.FindStringSubmatch(text)
	if m == nil {
		return ""
	}
	return strings.TrimSpace(m[1])
}

func sharedTextFrom(text, link string) string {
	t := strings.TrimSpace(strings.Replace(text, link, "", 1))
	if len([]rune(t)) > sharedTextLimit {
		t = string([]rune(t)[:sharedTextLimit])
	}
	return t
}
