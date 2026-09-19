package tracking

import (
	"context"
	"fmt"

	"github.com/gio-del/sumisura/backend/internal/generation"
)

// Client is the seam tracking needs an LLM for: generation.Client's RAL
// Range lookup (a Job Listing's RAL Range, reused from Generation),
// Application Method inference from a Job Description (story 5), and
// researching a Contact suggestion via web search (story 7). The real
// implementation is claude.Client, which already satisfies
// generation.Client.
type Client interface {
	generation.Client
	InferApplicationMethod(ctx context.Context, jobDescription string) (ApplicationMethod, error)
	SuggestContact(ctx context.Context, company, jobDescription string) (Contact, error)
	SuggestCaptureHints(ctx context.Context, jobDescription string) (CaptureHints, error)
}

// knownMethodKinds validates a user correction (story 6) without
// restricting which Kind it moves to or from — unlike Status, there's no
// meaningful progression to enforce here.
var knownMethodKinds = map[ApplicationMethodKind]bool{
	MethodPortal:    true,
	MethodEmail:     true,
	MethodEasyApply: true,
	MethodOther:     true,
}

// UpdateApplicationMethod validates and applies a user correction to the
// Application identified by id (story 6), writing it back to disk.
func UpdateApplicationMethod(dataDir, id string, method ApplicationMethod) (Application, error) {
	return UpdateApplicationMethodIfMatch(dataDir, id, method, "")
}

// UpdateApplicationMethodIfMatch is UpdateApplicationMethod, refusing the
// write with recordversion.ErrMismatch when version no longer matches the
// Application file on disk — so a correction cannot silently revert an
// inferred value that was already corrected elsewhere (issue #89, story
// 16). An empty version writes unconditionally.
func UpdateApplicationMethodIfMatch(dataDir, id string, method ApplicationMethod, version string) (Application, error) {
	if !knownMethodKinds[method.Kind] {
		return Application{}, fmt.Errorf("%w: unknown application method kind %q", ErrValidation, method.Kind)
	}

	application, err := getApplication(dataDir, id)
	if err != nil {
		return Application{}, err
	}
	application.Method = method

	return writeApplicationIfMatch(dataDir, id, application, version)
}
