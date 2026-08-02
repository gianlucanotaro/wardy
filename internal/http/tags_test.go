package http

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gianlucanotaro/wardy/internal/store"
)

func patchTags(h http.Handler, cookie *http.Cookie, body map[string]any) *httptest.ResponseRecorder {
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPatch, "/api/items/1/tags", bytes.NewReader(b))
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func validTagsPayload() map[string]any {
	return map[string]any{
		"category":        "top",
		"subcategory":     "t-shirt",
		"color_primary":   "blue",
		"color_secondary": "",
		"pattern":         "solid",
		"material":        "cotton",
		"fit":             "regular",
		"formality":       2,
		"warmth":          3,
		"seasons":         []string{"summer", "spring"},
		"occasions":       []string{"casual"},
	}
}

func TestUpdateItemTagsValid(t *testing.T) {
	s := newTestStore(t)
	h := newTestHandler(t, s)
	cookie := loginAndGetCookie(t, h, s, "me@example.com", "correct horse battery staple")
	uploadItem(t, h, cookie, newFixtureJPEG(t))

	rec := patchTags(h, cookie, validTagsPayload())
	if rec.Code != http.StatusOK {
		t.Fatalf("update tags: got status %d, body %s", rec.Code, rec.Body.String())
	}

	var item store.Item
	if err := json.Unmarshal(rec.Body.Bytes(), &item); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if item.Category != "top" || item.Subcategory != "t-shirt" {
		t.Errorf("category/subcategory not applied: %+v", item)
	}
	if len(item.Seasons) != 2 || item.Seasons[0] != "summer" {
		t.Errorf("seasons not applied: %+v", item.Seasons)
	}
	if !item.UserEditedAt.Valid {
		t.Error("user_edited_at should be set after a manual tag edit")
	}
}

func TestUpdateItemTagsRejectsInvalid(t *testing.T) {
	s := newTestStore(t)
	h := newTestHandler(t, s)
	cookie := loginAndGetCookie(t, h, s, "me@example.com", "correct horse battery staple")
	uploadItem(t, h, cookie, newFixtureJPEG(t))

	payload := validTagsPayload()
	payload["category"] = "sock" // not a real category

	rec := patchTags(h, cookie, payload)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("update tags with bad category: got status %d, want %d", rec.Code, http.StatusBadRequest)
	}

	// Confirm nothing was partially applied — color_primary in the rejected
	// payload ("blue") differs from the default ("black"), so this
	// distinguishes "fully untouched" from "everything but category applied".
	getReq := httptest.NewRequest(http.MethodGet, "/api/items/1", nil)
	getReq.AddCookie(cookie)
	getRec := httptest.NewRecorder()
	h.ServeHTTP(getRec, getReq)
	var item store.Item
	json.Unmarshal(getRec.Body.Bytes(), &item)
	if item.ColorPrimary != "black" {
		t.Errorf("item should be untouched after a rejected update, got color_primary %q", item.ColorPrimary)
	}
}

func TestUpdateItemTagsRejectsSubcategoryMismatch(t *testing.T) {
	s := newTestStore(t)
	h := newTestHandler(t, s)
	cookie := loginAndGetCookie(t, h, s, "me@example.com", "correct horse battery staple")
	uploadItem(t, h, cookie, newFixtureJPEG(t))

	payload := validTagsPayload()
	payload["subcategory"] = "jeans" // valid subcategory, wrong category (top)

	rec := patchTags(h, cookie, payload)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("update tags with mismatched subcategory: got status %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestUpdateItemTagsOwnershipEnforced(t *testing.T) {
	s := newTestStore(t)
	h := newTestHandler(t, s)

	cookieA := loginAndGetCookie(t, h, s, "a@example.com", "correct horse battery staple")
	uploadItem(t, h, cookieA, newFixtureJPEG(t))

	cookieB := loginAndGetCookie(t, h, s, "b@example.com", "correct horse battery staple")
	rec := patchTags(h, cookieB, validTagsPayload())
	if rec.Code != http.StatusNotFound {
		t.Fatalf("update tags as non-owner: got status %d, want %d", rec.Code, http.StatusNotFound)
	}
}
