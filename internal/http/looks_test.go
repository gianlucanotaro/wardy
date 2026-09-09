package http

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/gianlucanotaro/wardy/internal/store"
)

func createTestItem(t *testing.T, s *store.Store, userID int64) int64 {
	t.Helper()
	id, err := s.CreateItem(context.Background(), userID, "photo.jpg", "thumb.jpg")
	if err != nil {
		t.Fatalf("CreateItem: %v", err)
	}
	return id
}

func doJSON(h http.Handler, cookie *http.Cookie, method, path string, body any) *httptest.ResponseRecorder {
	var buf bytes.Buffer
	if body != nil {
		json.NewEncoder(&buf).Encode(body)
	}
	req := httptest.NewRequest(method, path, &buf)
	req.AddCookie(cookie)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestCreateLookEndpoint(t *testing.T) {
	s := newTestStore(t)
	h := newTestHandler(t, s)
	cookie := loginAndGetCookie(t, h, s, "me@example.com", "correct horse battery staple")
	itemID := createTestItem(t, s, 1)

	rec := doJSON(h, cookie, http.MethodPost, "/api/looks", map[string]any{
		"name": "Casual look", "item_ids": []int64{itemID},
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("create look: status %d, body %s", rec.Code, rec.Body.String())
	}

	var look map[string]any
	json.Unmarshal(rec.Body.Bytes(), &look)
	if look["name"] != "Casual look" {
		t.Errorf("name = %v, want %q", look["name"], "Casual look")
	}
	items, _ := look["items"].([]any)
	if len(items) != 1 {
		t.Errorf("items = %v, want 1 item", look["items"])
	}
}

func TestCreateLookEndpointRejectsEmptyItemIDs(t *testing.T) {
	s := newTestStore(t)
	h := newTestHandler(t, s)
	cookie := loginAndGetCookie(t, h, s, "me@example.com", "correct horse battery staple")

	rec := doJSON(h, cookie, http.MethodPost, "/api/looks", map[string]any{
		"name": "Empty", "item_ids": []int64{},
	})
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestCreateLookEndpointRejectsBadItemID(t *testing.T) {
	s := newTestStore(t)
	h := newTestHandler(t, s)
	cookie := loginAndGetCookie(t, h, s, "me@example.com", "correct horse battery staple")

	rec := doJSON(h, cookie, http.MethodPost, "/api/looks", map[string]any{
		"name": "Bogus", "item_ids": []int64{999999},
	})
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status %d, want %d, body %s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
}

func TestLooksEndpointCRUDRoundTrip(t *testing.T) {
	s := newTestStore(t)
	h := newTestHandler(t, s)
	cookie := loginAndGetCookie(t, h, s, "me@example.com", "correct horse battery staple")
	itemA := createTestItem(t, s, 1)
	itemB := createTestItem(t, s, 1)

	// Create.
	rec := doJSON(h, cookie, http.MethodPost, "/api/looks", map[string]any{
		"name": "Original", "item_ids": []int64{itemA},
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("create: status %d", rec.Code)
	}
	var created map[string]any
	json.Unmarshal(rec.Body.Bytes(), &created)
	lookID := int64(created["id"].(float64))

	// List.
	rec = doJSON(h, cookie, http.MethodGet, "/api/looks", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("list: status %d", rec.Code)
	}
	var list []map[string]any
	json.Unmarshal(rec.Body.Bytes(), &list)
	if len(list) != 1 {
		t.Fatalf("list: got %d looks, want 1", len(list))
	}

	// Get.
	rec = doJSON(h, cookie, http.MethodGet, "/api/looks/"+strconv.FormatInt(lookID, 10), nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("get: status %d", rec.Code)
	}

	// Update: rename + replace items.
	rec = doJSON(h, cookie, http.MethodPatch, "/api/looks/"+strconv.FormatInt(lookID, 10), map[string]any{
		"name": "Renamed", "item_ids": []int64{itemB},
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("update: status %d, body %s", rec.Code, rec.Body.String())
	}
	var updated map[string]any
	json.Unmarshal(rec.Body.Bytes(), &updated)
	if updated["name"] != "Renamed" {
		t.Errorf("name after update = %v, want %q", updated["name"], "Renamed")
	}
	items, _ := updated["items"].([]any)
	if len(items) != 1 || int64(items[0].(map[string]any)["id"].(float64)) != itemB {
		t.Errorf("items after update = %v, want just item %d", updated["items"], itemB)
	}

	// Delete.
	rec = doJSON(h, cookie, http.MethodDelete, "/api/looks/"+strconv.FormatInt(lookID, 10), nil)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("delete: status %d", rec.Code)
	}

	rec = doJSON(h, cookie, http.MethodGet, "/api/looks/"+strconv.FormatInt(lookID, 10), nil)
	if rec.Code != http.StatusNotFound {
		t.Errorf("get after delete: status %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestUpdateLookEndpointRejectsEmptyItemIDs(t *testing.T) {
	s := newTestStore(t)
	h := newTestHandler(t, s)
	cookie := loginAndGetCookie(t, h, s, "me@example.com", "correct horse battery staple")
	itemID := createTestItem(t, s, 1)

	rec := doJSON(h, cookie, http.MethodPost, "/api/looks", map[string]any{
		"name": "Look", "item_ids": []int64{itemID},
	})
	var created map[string]any
	json.Unmarshal(rec.Body.Bytes(), &created)
	lookID := int64(created["id"].(float64))

	rec = doJSON(h, cookie, http.MethodPatch, "/api/looks/"+strconv.FormatInt(lookID, 10), map[string]any{
		"item_ids": []int64{},
	})
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestLooksEndpointOwnershipScoped(t *testing.T) {
	s := newTestStore(t)
	h := newTestHandler(t, s)

	cookieA := loginAndGetCookie(t, h, s, "a@example.com", "correct horse battery staple")
	itemA := createTestItem(t, s, 1)
	rec := doJSON(h, cookieA, http.MethodPost, "/api/looks", map[string]any{
		"name": "A's look", "item_ids": []int64{itemA},
	})
	var created map[string]any
	json.Unmarshal(rec.Body.Bytes(), &created)
	lookID := int64(created["id"].(float64))

	cookieB := loginAndGetCookie(t, h, s, "b@example.com", "correct horse battery staple")

	// User B can't see it in the list.
	rec = doJSON(h, cookieB, http.MethodGet, "/api/looks", nil)
	var list []map[string]any
	json.Unmarshal(rec.Body.Bytes(), &list)
	if len(list) != 0 {
		t.Errorf("user B's look list should be empty, got %d", len(list))
	}

	// User B can't fetch, update, or delete it directly.
	if rec := doJSON(h, cookieB, http.MethodGet, "/api/looks/"+strconv.FormatInt(lookID, 10), nil); rec.Code != http.StatusNotFound {
		t.Errorf("get as user B: status %d, want %d", rec.Code, http.StatusNotFound)
	}
	if rec := doJSON(h, cookieB, http.MethodPatch, "/api/looks/"+strconv.FormatInt(lookID, 10), map[string]any{"name": "Stolen"}); rec.Code != http.StatusNotFound {
		t.Errorf("update as user B: status %d, want %d", rec.Code, http.StatusNotFound)
	}
	if rec := doJSON(h, cookieB, http.MethodDelete, "/api/looks/"+strconv.FormatInt(lookID, 10), nil); rec.Code != http.StatusNotFound {
		t.Errorf("delete as user B: status %d, want %d", rec.Code, http.StatusNotFound)
	}
}
