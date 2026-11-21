package api_test

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gio-del/sumisura/backend/internal/api"
)

// Notes (issue #96) are exercised through the HTTP API over a temporary
// data directory, like every other Application action: the status code, the
// returned Application and what is on disk afterwards.

func newNotesServer(t *testing.T) (dataDir string, server *httptest.Server) {
	t.Helper()
	dataDir = seedDataDir(t)
	server = httptest.NewServer(api.NewRouter(api.RouterConfig{DataDir: dataDir, GenerationClient: &fakeGenerationClient{}}))
	t.Cleanup(server.Close)
	return dataDir, server
}

func decodeApplication(t *testing.T, resp *http.Response) map[string]any {
	t.Helper()
	var application map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&application); err != nil {
		t.Fatal(err)
	}
	return application
}

func notesOf(t *testing.T, application map[string]any) []map[string]any {
	t.Helper()
	raw, ok := application["notes"]
	if !ok || raw == nil {
		return nil
	}
	list, ok := raw.([]any)
	if !ok {
		t.Fatalf("expected notes to be an array, got %T: %v", raw, raw)
	}
	notes := make([]map[string]any, len(list))
	for i, n := range list {
		notes[i] = n.(map[string]any)
	}
	return notes
}

func noteBodies(notes []map[string]any) []string {
	bodies := make([]string, len(notes))
	for i, n := range notes {
		bodies[i], _ = n["body"].(string)
	}
	return bodies
}

func addNote(t *testing.T, serverURL, id, body string) *http.Response {
	t.Helper()
	return postJSON(t, serverURL+"/api/applications/"+id+"/notes", map[string]any{"body": body})
}

// addNoteOK adds a Note and returns the Application the API answered with.
func addNoteOK(t *testing.T, serverURL, id, body string) map[string]any {
	t.Helper()
	resp := addNote(t, serverURL, id, body)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201 adding a Note, got %d", resp.StatusCode)
	}
	return decodeApplication(t, resp)
}

// storedApplication is the Application as a fresh read from disk returns
// it, via the Job Listing detail endpoint.
func storedApplication(t *testing.T, serverURL, id string) map[string]any {
	t.Helper()
	return getJobListingDetail(t, serverURL, id)["application"].(map[string]any)
}

func TestAddNote_ValidBody_Returns201WithTheNoteOnTheApplication(t *testing.T) {
	_, server := newNotesServer(t)
	id := saveJobListing(t, server.URL, "Acme Corp")

	before := time.Now().UTC()
	application := addNoteOK(t, server.URL, id, "Recruiter said the budget tops out at **€48k**.")

	if application["id"] != id {
		t.Errorf("expected the updated Application %q back, got id %v", id, application["id"])
	}
	notes := notesOf(t, application)
	if len(notes) != 1 {
		t.Fatalf("expected one Note, got %d: %v", len(notes), notes)
	}
	note := notes[0]
	if note["body"] != "Recruiter said the budget tops out at **€48k**." {
		t.Errorf("expected the Markdown body verbatim, got %v", note["body"])
	}
	if id, _ := note["id"].(string); id == "" {
		t.Errorf("expected a server-assigned Note id, got %v", note["id"])
	}
	createdAt, err := time.Parse(time.RFC3339Nano, note["createdAt"].(string))
	if err != nil {
		t.Fatalf("expected an RFC3339 createdAt, got %v: %v", note["createdAt"], err)
	}
	if createdAt.Before(before.Add(-time.Second)) || createdAt.After(time.Now().Add(time.Second)) {
		t.Errorf("expected createdAt to be the moment the Note was written, got %v", createdAt)
	}
	if _, edited := note["editedAt"]; edited {
		t.Errorf("did not expect a fresh Note to carry editedAt, got %v", note["editedAt"])
	}
}

func TestAddNote_TwoInSequence_BothPersistNewestFirst(t *testing.T) {
	_, server := newNotesServer(t)
	id := saveJobListing(t, server.URL, "Acme Corp")

	addNoteOK(t, server.URL, id, "Screening call booked.")
	application := addNoteOK(t, server.URL, id, "Take-home due Friday.")

	want := []string{"Take-home due Friday.", "Screening call booked."}
	if got := noteBodies(notesOf(t, application)); !equalStrings(got, want) {
		t.Errorf("expected response Notes newest-first %v, got %v", want, got)
	}
	notes := notesOf(t, application)
	if notes[0]["id"] == notes[1]["id"] {
		t.Errorf("expected distinct Note ids, both were %v", notes[0]["id"])
	}

	if got := noteBodies(notesOf(t, storedApplication(t, server.URL, id))); !equalStrings(got, want) {
		t.Errorf("expected re-read Notes newest-first %v, got %v", want, got)
	}
}

func TestAddNote_MultiLineMarkdown_SurvivesARereadFromDisk(t *testing.T) {
	dataDir, server := newNotesServer(t)
	id := saveJobListing(t, server.URL, "Acme Corp")

	body := "Second interview panel:\n\n- Platform team lead\n- Staff engineer\n\nHybrid, three days a week."
	added := notesOf(t, addNoteOK(t, server.URL, id, body))[0]

	// A brand-new server over the same directory stands in for a restart.
	restarted := httptest.NewServer(api.NewRouter(api.RouterConfig{DataDir: dataDir, GenerationClient: &fakeGenerationClient{}}))
	defer restarted.Close()

	notes := notesOf(t, storedApplication(t, restarted.URL, id))
	if len(notes) != 1 {
		t.Fatalf("expected the Note to survive a restart, got %v", notes)
	}
	if notes[0]["body"] != body || notes[0]["id"] != added["id"] || notes[0]["createdAt"] != added["createdAt"] {
		t.Errorf("expected the stored Note to match what was added (%v), got %v", added, notes[0])
	}
}

func TestAddNote_EmptyOrWhitespaceBody_Returns400AndWritesNothing(t *testing.T) {
	for name, body := range map[string]string{"empty": "", "whitespace": "  \n\t "} {
		t.Run(name, func(t *testing.T) {
			dataDir, server := newNotesServer(t)
			id := saveJobListing(t, server.URL, "Acme Corp")
			path := filepath.Join(dataDir, "applications", id+".md")
			before, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}

			resp := addNote(t, server.URL, id, body)
			resp.Body.Close()

			if resp.StatusCode != http.StatusBadRequest {
				t.Fatalf("expected 400, got %d", resp.StatusCode)
			}
			after, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(before, after) {
				t.Errorf("expected the Application file untouched, before:\n%s\nafter:\n%s", before, after)
			}
		})
	}
}

func TestAddNote_UnknownApplication_Returns404(t *testing.T) {
	_, server := newNotesServer(t)

	resp := addNote(t, server.URL, "does-not-exist", "Hello")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

func TestAddNote_InvalidJSON_Returns400(t *testing.T) {
	_, server := newNotesServer(t)
	id := saveJobListing(t, server.URL, "Acme Corp")

	resp, err := http.Post(server.URL+"/api/applications/"+id+"/notes", "application/json", bytes.NewReader([]byte("{")))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

// writeStaleSentApplication stores a Job Listing and an Application that has
// sat in Sent for a month, in the pre-Notes file shape.
func writeStaleSentApplication(t *testing.T, dataDir, id string) (statusUpdatedAt string) {
	t.Helper()
	statusUpdatedAt = time.Now().UTC().Add(-30 * 24 * time.Hour).Format(time.RFC3339Nano)
	writeFile(t, filepath.Join(dataDir, "jobs", id+".md"), `---
company: Globex
source: manual
savedAt: "2026-01-05T10:00:00Z"
ral:
  source: n/a
---

A role.
`)
	writeFile(t, filepath.Join(dataDir, "applications", id+".md"), `jobListingId: `+id+`
status: sent
statusUpdatedAt: "`+statusUpdatedAt+`"
method:
  kind: email
  value: jobs@globex.example
statusHistory:
  - status: saved
    changedAt: 2026-01-05T10:00:00Z
  - status: sent
    changedAt: `+statusUpdatedAt+`
`)
	return statusUpdatedAt
}

func TestAddNote_LeavesStatusHistoryAndStalenessUntouched(t *testing.T) {
	dataDir, server := newNotesServer(t)
	statusUpdatedAt := writeStaleSentApplication(t, dataDir, "globex")
	before := storedApplication(t, server.URL, "globex")
	if before["isStale"] != true {
		t.Fatalf("precondition: expected the seeded Application to be stale, got %v", before["isStale"])
	}

	application := addNoteOK(t, server.URL, "globex", "Chased them by email, no reply yet.")

	for label, got := range map[string]map[string]any{"response": application, "re-read": storedApplication(t, server.URL, "globex")} {
		if got["status"] != "sent" {
			t.Errorf("%s: expected status sent, got %v", label, got["status"])
		}
		if got["statusUpdatedAt"] != statusUpdatedAt {
			t.Errorf("%s: expected statusUpdatedAt %q unchanged, got %v", label, statusUpdatedAt, got["statusUpdatedAt"])
		}
		if got["isStale"] != true {
			t.Errorf("%s: expected a Note not to make a stale Application fresh, got isStale %v", label, got["isStale"])
		}
		if history, _ := got["statusHistory"].([]any); len(history) != 2 {
			t.Errorf("%s: expected the two statusHistory entries unchanged, got %v", label, got["statusHistory"])
		}
	}
}

func TestApplicationWithoutNotes_LoadsUnchangedAndWritesNoNotesKey(t *testing.T) {
	dataDir, server := newNotesServer(t)
	writeStaleSentApplication(t, dataDir, "globex")

	application := storedApplication(t, server.URL, "globex")
	if _, ok := application["notes"]; ok {
		t.Errorf("expected an Application with no Notes to carry no notes key, got %v", application["notes"])
	}
	if application["status"] != "sent" {
		t.Errorf("expected the pre-Notes Application to load as before, got status %v", application["status"])
	}

	// Any unrelated write re-renders the file; it must not grow a stray key.
	resp := postJSON(t, server.URL+"/api/applications/globex/contacts", map[string]any{"name": "Jane", "email": "jane@globex.example"})
	resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201 saving a contact, got %d", resp.StatusCode)
	}
	content, err := os.ReadFile(filepath.Join(dataDir, "applications", "globex.md"))
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(content, []byte("notes")) {
		t.Errorf("expected no notes key in an Application file without Notes, got:\n%s", content)
	}
}

func editNote(t *testing.T, serverURL, id, noteID, body string) *http.Response {
	t.Helper()
	return patchJSON(t, serverURL+"/api/applications/"+id+"/notes/"+noteID, map[string]any{"body": body})
}

func TestEditNote_ChangesTheBodyKeepsCreatedAtAndMarksItEdited(t *testing.T) {
	_, server := newNotesServer(t)
	id := saveJobListing(t, server.URL, "Acme Corp")
	original := notesOf(t, addNoteOK(t, server.URL, id, "Budget tops out at 84k."))[0]

	resp := editNote(t, server.URL, id, original["id"].(string), "Budget tops out at 48k.")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	for label, application := range map[string]map[string]any{"response": decodeApplication(t, resp), "re-read": storedApplication(t, server.URL, id)} {
		notes := notesOf(t, application)
		if len(notes) != 1 {
			t.Fatalf("%s: expected the one Note, got %v", label, notes)
		}
		edited := notes[0]
		if edited["body"] != "Budget tops out at 48k." {
			t.Errorf("%s: expected the corrected body, got %v", label, edited["body"])
		}
		if edited["id"] != original["id"] {
			t.Errorf("%s: expected the Note to keep its id %v, got %v", label, original["id"], edited["id"])
		}
		if edited["createdAt"] != original["createdAt"] {
			t.Errorf("%s: expected createdAt %v unchanged by an edit, got %v", label, original["createdAt"], edited["createdAt"])
		}
		editedAt, _ := edited["editedAt"].(string)
		if _, err := time.Parse(time.RFC3339Nano, editedAt); err != nil {
			t.Errorf("%s: expected an editedAt timestamp after an edit, got %v", label, edited["editedAt"])
		}
	}
}

func TestEditNote_OlderNote_KeepsItsPlaceInTheLog(t *testing.T) {
	_, server := newNotesServer(t)
	id := saveJobListing(t, server.URL, "Acme Corp")
	older := notesOf(t, addNoteOK(t, server.URL, id, "Screening call bookd."))[0]
	addNoteOK(t, server.URL, id, "Take-home due Friday.")

	resp := editNote(t, server.URL, id, older["id"].(string), "Screening call booked.")
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	want := []string{"Take-home due Friday.", "Screening call booked."}
	if got := noteBodies(notesOf(t, storedApplication(t, server.URL, id))); !equalStrings(got, want) {
		t.Errorf("expected an edit not to reorder the log, want %v, got %v", want, got)
	}
}

func TestEditNote_EmptyOrWhitespaceBody_Returns400AndWritesNothing(t *testing.T) {
	dataDir, server := newNotesServer(t)
	id := saveJobListing(t, server.URL, "Acme Corp")
	note := notesOf(t, addNoteOK(t, server.URL, id, "Keep me."))[0]
	path := filepath.Join(dataDir, "applications", id+".md")
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	resp := editNote(t, server.URL, id, note["id"].(string), " \n ")
	resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Errorf("expected the Application file untouched, before:\n%s\nafter:\n%s", before, after)
	}
}

func TestEditNote_UnknownApplication_Returns404(t *testing.T) {
	_, server := newNotesServer(t)

	resp := editNote(t, server.URL, "does-not-exist", "n1", "Hello")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

func TestEditNote_UnknownNoteID_Returns404AndLeavesNotesAlone(t *testing.T) {
	_, server := newNotesServer(t)
	id := saveJobListing(t, server.URL, "Acme Corp")
	addNoteOK(t, server.URL, id, "Real Note.")

	resp := editNote(t, server.URL, id, "no-such-note", "Hello")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
	if got := noteBodies(notesOf(t, storedApplication(t, server.URL, id))); !equalStrings(got, []string{"Real Note."}) {
		t.Errorf("expected the existing Note untouched, got %v", got)
	}
}

func deleteNote(t *testing.T, serverURL, id, noteID string) *http.Response {
	t.Helper()
	return deleteRequest(t, serverURL+"/api/applications/"+id+"/notes/"+noteID)
}

func TestDeleteNote_RemovesOnlyTheAddressedNote(t *testing.T) {
	_, server := newNotesServer(t)
	id := saveJobListing(t, server.URL, "Acme Corp")
	addNoteOK(t, server.URL, id, "First.")
	middle := notesOf(t, addNoteOK(t, server.URL, id, "Typed into the wrong Application."))[0]
	addNoteOK(t, server.URL, id, "Third.")

	resp := deleteNote(t, server.URL, id, middle["id"].(string))
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	want := []string{"Third.", "First."}
	for label, application := range map[string]map[string]any{"response": decodeApplication(t, resp), "re-read": storedApplication(t, server.URL, id)} {
		if got := noteBodies(notesOf(t, application)); !equalStrings(got, want) {
			t.Errorf("%s: expected the siblings %v intact, got %v", label, want, got)
		}
	}
}

func TestDeleteNote_LastNote_LeavesNoNotesKeyBehind(t *testing.T) {
	dataDir, server := newNotesServer(t)
	id := saveJobListing(t, server.URL, "Acme Corp")
	note := notesOf(t, addNoteOK(t, server.URL, id, "Only Note."))[0]

	resp := deleteNote(t, server.URL, id, note["id"].(string))
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if _, ok := decodeApplication(t, resp)["notes"]; ok {
		t.Errorf("expected an Application with no Notes left to carry no notes key")
	}
	content, err := os.ReadFile(filepath.Join(dataDir, "applications", id+".md"))
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(content, []byte("notes")) {
		t.Errorf("expected no stray notes key once the last Note is deleted, got:\n%s", content)
	}
}

func TestDeleteNote_UnknownApplication_Returns404(t *testing.T) {
	_, server := newNotesServer(t)

	resp := deleteNote(t, server.URL, "does-not-exist", "n1")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

func TestDeleteNote_UnknownNoteID_Returns404AndLeavesNotesAlone(t *testing.T) {
	_, server := newNotesServer(t)
	id := saveJobListing(t, server.URL, "Acme Corp")
	addNoteOK(t, server.URL, id, "Real Note.")

	resp := deleteNote(t, server.URL, id, "no-such-note")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
	if got := noteBodies(notesOf(t, storedApplication(t, server.URL, id))); !equalStrings(got, []string{"Real Note."}) {
		t.Errorf("expected the existing Note untouched, got %v", got)
	}
}

func TestExportData_IncludesTheApplicationsNotes(t *testing.T) {
	_, server := newNotesServer(t)
	id := saveJobListing(t, server.URL, "Acme Corp")
	addNoteOK(t, server.URL, id, "Offer call went well; they will send numbers Monday.")

	resp, err := http.Get(server.URL + "/api/export")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	archive, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	zr, err := zip.NewReader(bytes.NewReader(archive), int64(len(archive)))
	if err != nil {
		t.Fatalf("expected a zip archive: %v", err)
	}
	f, err := zr.Open("applications/" + id + ".md")
	if err != nil {
		t.Fatalf("expected the Application file in the export: %v", err)
	}
	defer f.Close()
	content, err := io.ReadAll(f)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(content, []byte("they will send numbers Monday")) {
		t.Errorf("expected the exported Application to carry its Note, got:\n%s", content)
	}
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
