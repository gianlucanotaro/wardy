package http

import (
	"bytes"
	"context"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/gianlucanotaro/wardy/internal/inspo"
	"github.com/gianlucanotaro/wardy/internal/jobs"
	"github.com/gianlucanotaro/wardy/internal/store"
)

func uploadInspo(t *testing.T, h http.Handler, cookie *http.Cookie, photo []byte) *httptest.ResponseRecorder {
	t.Helper()
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	part, err := mw.CreateFormFile("photo", "fit.jpg")
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	part.Write(photo)
	mw.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/inspo", &body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func getBoard(t *testing.T, h http.Handler, cookie *http.Cookie) inspoBoardResponse {
	t.Helper()
	rec := doJSON(h, cookie, http.MethodGet, "/api/inspo", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/inspo: status %d, body %s", rec.Code, rec.Body.String())
	}
	var board inspoBoardResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &board); err != nil {
		t.Fatalf("decode board: %v", err)
	}
	return board
}

func validInspoTagsBody() map[string]any {
	return map[string]any{
		"styles": []string{"minimal"}, "colors": []string{"black", "white"},
		"pieces": []string{"jeans", "t-shirt"}, "fit": "relaxed",
		"formality_min": 2, "formality_max": 3,
		"seasons": []string{"summer"}, "occasions": []string{"casual"},
		"custom_tags": []string{"want this"},
	}
}

func TestInspoBoardRoundTrip(t *testing.T) {
	s := newTestStore(t)
	h := newTestHandler(t, s)
	cookie := loginAndGetCookie(t, h, s, "me@example.com", "correct horse battery staple")

	if board := getBoard(t, h, cookie); len(board.Fits) != 0 || !board.UseInSuggestions || board.MinFits == 0 {
		t.Fatalf("empty board = %+v, want no fits, use_in_suggestions on, min_fits set", board)
	}

	rec := uploadInspo(t, h, cookie, newFixtureJPEG(t))
	if rec.Code != http.StatusCreated {
		t.Fatalf("upload: status %d, body %s", rec.Code, rec.Body.String())
	}
	var fit inspoFitResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &fit); err != nil {
		t.Fatalf("decode fit: %v", err)
	}
	if !fit.TaggingPending {
		t.Error("new fit with AI on: tagging_pending = false, want true")
	}
	if bytes.Contains(rec.Body.Bytes(), []byte("photo_path")) {
		t.Error("fit response leaks its photo path")
	}
	id := strconv.FormatInt(fit.ID, 10)

	for _, path := range []string{"/api/inspo/" + id + "/photo", "/api/inspo/" + id + "/thumb"} {
		if rec := doJSON(h, cookie, http.MethodGet, path, nil); rec.Code != http.StatusOK {
			t.Errorf("GET %s: status %d, want 200", path, rec.Code)
		}
	}

	rec = doJSON(h, cookie, http.MethodPatch, "/api/inspo/"+id+"/tags", validInspoTagsBody())
	if rec.Code != http.StatusOK {
		t.Fatalf("PATCH tags: status %d, body %s", rec.Code, rec.Body.String())
	}

	board := getBoard(t, h, cookie)
	if len(board.Fits) != 1 || board.Profile.Tagged != 1 || board.Profile.Styles[0].Value != "minimal" {
		t.Fatalf("board after tagging = %+v", board)
	}
	if got := board.Fits[0].CustomTags; len(got) != 1 || got[0] != "want this" {
		t.Errorf("custom tags = %v, want [want this]", got)
	}

	if rec := doJSON(h, cookie, http.MethodDelete, "/api/inspo/"+id, nil); rec.Code != http.StatusNoContent {
		t.Fatalf("DELETE: status %d", rec.Code)
	}
	if board := getBoard(t, h, cookie); len(board.Fits) != 0 {
		t.Errorf("board after delete has %d fits, want 0", len(board.Fits))
	}
	if rec := doJSON(h, cookie, http.MethodGet, "/api/inspo/"+id+"/thumb", nil); rec.Code != http.StatusNotFound {
		t.Errorf("thumb of a deleted fit: status %d, want 404", rec.Code)
	}
}

func TestInspoUploadQueuesTaggingOnlyWithAI(t *testing.T) {
	s := newTestStore(t)
	h := newTestHandlerAI(t, s, false)
	cookie := loginAndGetCookie(t, h, s, "me@example.com", "correct horse battery staple")

	rec := uploadInspo(t, h, cookie, newFixtureJPEG(t))
	if rec.Code != http.StatusCreated {
		t.Fatalf("upload: status %d", rec.Code)
	}
	if _, err := s.ClaimNextJob(context.Background(), jobs.TagInspoType, jobs.SummarizeStyleType); err != store.ErrNotFound {
		t.Errorf("job queued with AI off: ClaimNextJob = %v, want ErrNotFound", err)
	}
}

func TestInspoTagsRejectsInvalid(t *testing.T) {
	s := newTestStore(t)
	h := newTestHandler(t, s)
	cookie := loginAndGetCookie(t, h, s, "me@example.com", "correct horse battery staple")

	rec := uploadInspo(t, h, cookie, newFixtureJPEG(t))
	var fit inspoFitResponse
	json.Unmarshal(rec.Body.Bytes(), &fit)
	path := "/api/inspo/" + strconv.FormatInt(fit.ID, 10) + "/tags"

	for field, value := range map[string]any{
		"styles":        []string{"cottagecore"},
		"colors":        []string{"sage mist"},
		"pieces":        []string{"outerwear"},
		"fit":           "baggy",
		"formality_max": 9,
	} {
		body := validInspoTagsBody()
		body[field] = value
		if rec := doJSON(h, cookie, http.MethodPatch, path, body); rec.Code != http.StatusBadRequest {
			t.Errorf("%s=%v: status %d, want 400", field, value, rec.Code)
		}
	}
}

// TestInspoOwnership: another user's fits are invisible — not listed, not
// served, not editable, not deletable.
func TestInspoOwnership(t *testing.T) {
	s := newTestStore(t)
	h := newTestHandler(t, s)
	owner := loginAndGetCookie(t, h, s, "a@example.com", "correct horse battery staple")
	other := loginAndGetCookie(t, h, s, "b@example.com", "correct horse battery staple")

	rec := uploadInspo(t, h, owner, newFixtureJPEG(t))
	var fit inspoFitResponse
	json.Unmarshal(rec.Body.Bytes(), &fit)
	id := strconv.FormatInt(fit.ID, 10)

	if board := getBoard(t, h, other); len(board.Fits) != 0 {
		t.Errorf("other user's board lists %d fits, want 0", len(board.Fits))
	}
	for _, tc := range []struct{ method, path string }{
		{http.MethodGet, "/api/inspo/" + id + "/photo"},
		{http.MethodGet, "/api/inspo/" + id + "/thumb"},
		{http.MethodPatch, "/api/inspo/" + id + "/tags"},
		{http.MethodDelete, "/api/inspo/" + id},
	} {
		var body any
		if tc.method == http.MethodPatch {
			body = validInspoTagsBody()
		}
		if rec := doJSON(h, other, tc.method, tc.path, body); rec.Code != http.StatusNotFound {
			t.Errorf("%s %s as another user: status %d, want 404", tc.method, tc.path, rec.Code)
		}
	}
	if board := getBoard(t, h, owner); len(board.Fits) != 1 || len(board.Fits[0].Styles) != 0 {
		t.Errorf("owner's fit changed by another user: %+v", board.Fits)
	}
}

func TestInspoSettings(t *testing.T) {
	s := newTestStore(t)
	h := newTestHandler(t, s)
	cookie := loginAndGetCookie(t, h, s, "me@example.com", "correct horse battery staple")

	if rec := doJSON(h, cookie, http.MethodPatch, "/api/inspo/settings", map[string]any{}); rec.Code != http.StatusBadRequest {
		t.Errorf("missing use_in_suggestions: status %d, want 400", rec.Code)
	}
	if rec := doJSON(h, cookie, http.MethodPatch, "/api/inspo/settings", map[string]bool{"use_in_suggestions": false}); rec.Code != http.StatusOK {
		t.Fatalf("turn off: status %d", rec.Code)
	}
	if getBoard(t, h, cookie).UseInSuggestions {
		t.Error("use_in_suggestions still on after turning it off")
	}
}

// TestSuggestUsesInspoStyle: with enough tagged fits the style goes to the
// AI and the response says which suggested pieces look like the fits;
// switched off, it doesn't.
func TestSuggestUsesInspoStyle(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	h := newTestHandler(t, s)
	cookie := loginAndGetCookie(t, h, s, "me@example.com", "correct horse battery staple")
	user, err := s.UserByEmail(ctx, "me@example.com")
	if err != nil {
		t.Fatalf("UserByEmail: %v", err)
	}

	season := currentSeason()
	topID := tagItemForSuggest(t, s, user.ID, "top", 2, 3, []string{season}, []string{"casual"})
	tagItemForSuggest(t, s, user.ID, "bottom", 2, 3, []string{season}, []string{"casual"})
	tagItemForSuggest(t, s, user.ID, "shoes", 2, 3, []string{season}, []string{"casual"})

	suggestOnce := func() suggestResponse {
		t.Helper()
		rec := getSuggest(h, cookie, "casual")
		if rec.Code != http.StatusOK {
			t.Fatalf("suggest: status %d, body %s", rec.Code, rec.Body.String())
		}
		var resp suggestResponse
		json.Unmarshal(rec.Body.Bytes(), &resp)
		return resp
	}

	// Too few fits: no style yet.
	if resp := suggestOnce(); resp.StyleUsed {
		t.Error("style_used with an empty board, want false")
	}

	for i := range 5 {
		fitID, err := s.CreateInspoFit(ctx, user.ID, "p.jpg", "t.jpg")
		if err != nil {
			t.Fatalf("CreateInspoFit: %v", err)
		}
		// tagItemForSuggest's items are all "generic" and blue; the
		// first three fits show one.
		pieces := []string{"jeans"}
		if i < 3 {
			pieces = []string{"generic"}
		}
		if err := s.UpdateInspoTags(ctx, user.ID, fitID, store.InspoTags{
			Styles: []string{"minimal"}, Colors: []string{"blue"}, Pieces: pieces,
		}); err != nil {
			t.Fatalf("UpdateInspoTags: %v", err)
		}
	}

	resp := suggestOnce()
	if !resp.StyleUsed {
		t.Fatal("style_used with 5 tagged fits = false, want true")
	}
	if len(resp.Outfits) == 0 || resp.Outfits[0].InspoMatches[topID] != 3 {
		t.Errorf("inspo_matches = %v, want top %d matched by 3 fits", resp.Outfits[0].InspoMatches, topID)
	}

	if err := s.SetUseInspo(ctx, user.ID, false); err != nil {
		t.Fatalf("SetUseInspo: %v", err)
	}
	if resp := suggestOnce(); resp.StyleUsed || resp.Outfits[0].InspoMatches != nil {
		t.Errorf("switched off: style_used %v, inspo_matches %v, want neither", resp.StyleUsed, resp.Outfits[0].InspoMatches)
	}
}

// TestInspoBoardHidesStaleSummary: a summary written for a different
// profile isn't shown next to the current counts.
func TestInspoBoardHidesStaleSummary(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	h := newTestHandler(t, s)
	cookie := loginAndGetCookie(t, h, s, "me@example.com", "correct horse battery staple")
	user, _ := s.UserByEmail(ctx, "me@example.com")

	fitID, _ := s.CreateInspoFit(ctx, user.ID, "p.jpg", "t.jpg")
	s.UpdateInspoTags(ctx, user.ID, fitID, store.InspoTags{Styles: []string{"minimal"}})
	fits, _ := s.InspoFitsByUser(ctx, user.ID)
	s.SetStyleSummary(ctx, user.ID, "Clean and quiet.", inspo.Build(fits).Key())

	if got := getBoard(t, h, cookie).Summary; got != "Clean and quiet." {
		t.Fatalf("summary = %q, want the current one", got)
	}
	s.UpdateInspoTags(ctx, user.ID, fitID, store.InspoTags{Styles: []string{"edgy"}})
	if got := getBoard(t, h, cookie).Summary; got != "" {
		t.Errorf("summary after the profile changed = %q, want none until rewritten", got)
	}
}
