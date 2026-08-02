package http

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gianlucanotaro/wardy/internal/jobs"
)

func getTaggingPending(t *testing.T, h http.Handler, cookie *http.Cookie, itemID string) bool {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/api/items/"+itemID, nil)
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("get item: status %d, body %s", rec.Code, rec.Body.String())
	}

	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	pending, ok := resp["tagging_pending"].(bool)
	if !ok {
		t.Fatalf("response missing tagging_pending field: %s", rec.Body.String())
	}
	return pending
}

func TestUploadEnqueuesTagJobWhenAIEnabled(t *testing.T) {
	s := newTestStore(t)
	h := newTestHandlerAI(t, s, true)
	cookie := loginAndGetCookie(t, h, s, "me@example.com", "correct horse battery staple")

	rec := uploadItem(t, h, cookie, newFixtureJPEG(t))
	if rec.Code != http.StatusCreated {
		t.Fatalf("upload: status %d", rec.Code)
	}

	var upload map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &upload); err != nil {
		t.Fatalf("decode upload response: %v", err)
	}
	if pending, _ := upload["tagging_pending"].(bool); !pending {
		t.Errorf("upload response tagging_pending = %v, want true", upload["tagging_pending"])
	}

	if !getTaggingPending(t, h, cookie, "1") {
		t.Error("GET item tagging_pending = false, want true right after upload")
	}
}

func TestUploadSkipsTagJobWhenAIDisabled(t *testing.T) {
	s := newTestStore(t)
	h := newTestHandlerAI(t, s, false)
	cookie := loginAndGetCookie(t, h, s, "me@example.com", "correct horse battery staple")

	uploadItem(t, h, cookie, newFixtureJPEG(t))

	if getTaggingPending(t, h, cookie, "1") {
		t.Error("tagging_pending = true, want false when AI is disabled (no job should be queued)")
	}
}

func TestTaggingPendingClearsAfterJobCompletes(t *testing.T) {
	s := newTestStore(t)
	h := newTestHandlerAI(t, s, true)
	cookie := loginAndGetCookie(t, h, s, "me@example.com", "correct horse battery staple")

	uploadItem(t, h, cookie, newFixtureJPEG(t))

	if !getTaggingPending(t, h, cookie, "1") {
		t.Fatal("expected tagging_pending = true right after upload")
	}

	ctx := context.Background()
	job, err := s.ClaimNextJob(ctx, jobs.TagItemType)
	if err != nil {
		t.Fatalf("ClaimNextJob: %v", err)
	}
	if err := s.CompleteJob(ctx, job.ID); err != nil {
		t.Fatalf("CompleteJob: %v", err)
	}

	if getTaggingPending(t, h, cookie, "1") {
		t.Error("tagging_pending = true, want false once the job is done")
	}
}
