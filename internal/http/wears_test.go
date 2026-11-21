package http

import (
	"encoding/json"
	"net/http"
	"strconv"
	"testing"
)

func TestLogWearFromLookEndpoint(t *testing.T) {
	s := newTestStore(t)
	h := newTestHandler(t, s)
	cookie := loginAndGetCookie(t, h, s, "me@example.com", "correct horse battery staple")
	itemID := createTestItem(t, s, 1)

	rec := doJSON(h, cookie, http.MethodPost, "/api/looks", map[string]any{
		"name": "Look", "item_ids": []int64{itemID},
	})
	var look map[string]any
	json.Unmarshal(rec.Body.Bytes(), &look)
	lookID := int64(look["id"].(float64))

	rec = doJSON(h, cookie, http.MethodPost, "/api/wears", map[string]any{
		"look_id": lookID, "worn_on": "2026-07-15",
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("create wear: status %d, body %s", rec.Code, rec.Body.String())
	}

	var wear map[string]any
	json.Unmarshal(rec.Body.Bytes(), &wear)
	if wear["worn_on"] != "2026-07-15" {
		t.Errorf("worn_on = %v, want 2026-07-15", wear["worn_on"])
	}
	items, _ := wear["items"].([]any)
	if len(items) != 1 {
		t.Errorf("items = %v, want 1 item (derived from the look)", wear["items"])
	}
}

func TestLogWearAdHocEndpoint(t *testing.T) {
	s := newTestStore(t)
	h := newTestHandler(t, s)
	cookie := loginAndGetCookie(t, h, s, "me@example.com", "correct horse battery staple")
	itemID := createTestItem(t, s, 1)

	rec := doJSON(h, cookie, http.MethodPost, "/api/wears", map[string]any{
		"item_ids": []int64{itemID}, "worn_on": "2026-07-15", "occasion": "casual",
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("create wear: status %d, body %s", rec.Code, rec.Body.String())
	}
}

func TestCreateWearEndpointRequiresLookOrItems(t *testing.T) {
	s := newTestStore(t)
	h := newTestHandler(t, s)
	cookie := loginAndGetCookie(t, h, s, "me@example.com", "correct horse battery staple")

	rec := doJSON(h, cookie, http.MethodPost, "/api/wears", map[string]any{"worn_on": "2026-07-15"})
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestCreateWearEndpointRejectsBadDate(t *testing.T) {
	s := newTestStore(t)
	h := newTestHandler(t, s)
	cookie := loginAndGetCookie(t, h, s, "me@example.com", "correct horse battery staple")
	itemID := createTestItem(t, s, 1)

	rec := doJSON(h, cookie, http.MethodPost, "/api/wears", map[string]any{
		"item_ids": []int64{itemID}, "worn_on": "not-a-date",
	})
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestCreateWearEndpointRejectsBadOccasion(t *testing.T) {
	s := newTestStore(t)
	h := newTestHandler(t, s)
	cookie := loginAndGetCookie(t, h, s, "me@example.com", "correct horse battery staple")
	itemID := createTestItem(t, s, 1)

	rec := doJSON(h, cookie, http.MethodPost, "/api/wears", map[string]any{
		"item_ids": []int64{itemID}, "worn_on": "2026-07-15", "occasion": "bogus",
	})
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestWearsEndpointCRUDRoundTrip(t *testing.T) {
	s := newTestStore(t)
	h := newTestHandler(t, s)
	cookie := loginAndGetCookie(t, h, s, "me@example.com", "correct horse battery staple")
	itemID := createTestItem(t, s, 1)

	rec := doJSON(h, cookie, http.MethodPost, "/api/wears", map[string]any{
		"item_ids": []int64{itemID}, "worn_on": "2026-07-15",
	})
	var created map[string]any
	json.Unmarshal(rec.Body.Bytes(), &created)
	wearID := int64(created["id"].(float64))

	rec = doJSON(h, cookie, http.MethodGet, "/api/wears", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("list: status %d", rec.Code)
	}
	var list []map[string]any
	json.Unmarshal(rec.Body.Bytes(), &list)
	if len(list) != 1 {
		t.Fatalf("list: got %d wears, want 1", len(list))
	}

	rec = doJSON(h, cookie, http.MethodGet, "/api/wears/"+strconv.FormatInt(wearID, 10), nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("get: status %d", rec.Code)
	}

	rec = doJSON(h, cookie, http.MethodDelete, "/api/wears/"+strconv.FormatInt(wearID, 10), nil)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("delete: status %d", rec.Code)
	}

	rec = doJSON(h, cookie, http.MethodGet, "/api/wears/"+strconv.FormatInt(wearID, 10), nil)
	if rec.Code != http.StatusNotFound {
		t.Errorf("get after delete: status %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestWearsEndpointOwnershipScoped(t *testing.T) {
	s := newTestStore(t)
	h := newTestHandler(t, s)

	cookieA := loginAndGetCookie(t, h, s, "a@example.com", "correct horse battery staple")
	itemA := createTestItem(t, s, 1)
	rec := doJSON(h, cookieA, http.MethodPost, "/api/wears", map[string]any{
		"item_ids": []int64{itemA}, "worn_on": "2026-07-15",
	})
	var created map[string]any
	json.Unmarshal(rec.Body.Bytes(), &created)
	wearID := int64(created["id"].(float64))

	cookieB := loginAndGetCookie(t, h, s, "b@example.com", "correct horse battery staple")

	rec = doJSON(h, cookieB, http.MethodGet, "/api/wears", nil)
	var list []map[string]any
	json.Unmarshal(rec.Body.Bytes(), &list)
	if len(list) != 0 {
		t.Errorf("user B's wear list should be empty, got %d", len(list))
	}

	if rec := doJSON(h, cookieB, http.MethodGet, "/api/wears/"+strconv.FormatInt(wearID, 10), nil); rec.Code != http.StatusNotFound {
		t.Errorf("get as user B: status %d, want %d", rec.Code, http.StatusNotFound)
	}
	if rec := doJSON(h, cookieB, http.MethodDelete, "/api/wears/"+strconv.FormatInt(wearID, 10), nil); rec.Code != http.StatusNotFound {
		t.Errorf("delete as user B: status %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestHistoryEndpoint(t *testing.T) {
	s := newTestStore(t)
	h := newTestHandler(t, s)
	cookie := loginAndGetCookie(t, h, s, "me@example.com", "correct horse battery staple")
	wornItem := createTestItem(t, s, 1)
	neverWornItem := createTestItem(t, s, 1)

	doJSON(h, cookie, http.MethodPost, "/api/wears", map[string]any{
		"item_ids": []int64{wornItem}, "worn_on": "2026-07-15",
	})

	rec := doJSON(h, cookie, http.MethodGet, "/api/history", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("history: status %d, body %s", rec.Code, rec.Body.String())
	}

	var resp struct {
		Wears []map[string]any `json:"wears"`
		Items []map[string]any `json:"items"`
	}
	json.Unmarshal(rec.Body.Bytes(), &resp)
	if len(resp.Wears) != 1 {
		t.Errorf("wears = %d, want 1", len(resp.Wears))
	}
	if len(resp.Items) != 2 {
		t.Fatalf("items = %d, want 2", len(resp.Items))
	}
	// Never-worn item sorts first (staleness order).
	if int64(resp.Items[0]["id"].(float64)) != neverWornItem {
		t.Errorf("items[0] id = %v, want the never-worn item %d", resp.Items[0]["id"], neverWornItem)
	}
	if resp.Items[0]["last_worn_on"] != nil {
		t.Errorf("items[0].last_worn_on = %v, want null", resp.Items[0]["last_worn_on"])
	}
	if resp.Items[1]["last_worn_on"] != "2026-07-15" {
		t.Errorf("items[1].last_worn_on = %v, want 2026-07-15", resp.Items[1]["last_worn_on"])
	}
}

func TestHistoryEndpointOwnershipScoped(t *testing.T) {
	s := newTestStore(t)
	h := newTestHandler(t, s)

	cookieA := loginAndGetCookie(t, h, s, "a@example.com", "correct horse battery staple")
	itemA := createTestItem(t, s, 1)
	doJSON(h, cookieA, http.MethodPost, "/api/wears", map[string]any{
		"item_ids": []int64{itemA}, "worn_on": "2026-07-15",
	})

	cookieB := loginAndGetCookie(t, h, s, "b@example.com", "correct horse battery staple")
	rec := doJSON(h, cookieB, http.MethodGet, "/api/history", nil)
	var resp struct {
		Wears []map[string]any `json:"wears"`
		Items []map[string]any `json:"items"`
	}
	json.Unmarshal(rec.Body.Bytes(), &resp)
	if len(resp.Wears) != 0 || len(resp.Items) != 0 {
		t.Errorf("user B's history should be empty, got wears=%d items=%d", len(resp.Wears), len(resp.Items))
	}
}

// Today's "Wear this" on a planned look can be tapped twice (or retried on a
// flaky connection); the same look on the same day must log one wear, not two.
func TestLogWearFromLookTwiceSameDayLogsOnce(t *testing.T) {
	s := newTestStore(t)
	h := newTestHandler(t, s)
	cookie := loginAndGetCookie(t, h, s, "me@example.com", "correct horse battery staple")
	itemID := createTestItem(t, s, 1)

	rec := doJSON(h, cookie, http.MethodPost, "/api/looks", map[string]any{
		"name": "Look", "item_ids": []int64{itemID},
	})
	var look map[string]any
	json.Unmarshal(rec.Body.Bytes(), &look)
	lookID := int64(look["id"].(float64))

	var ids []float64
	for _, wornOn := range []string{"2026-07-15", "2026-07-15", "2026-07-16"} {
		rec = doJSON(h, cookie, http.MethodPost, "/api/wears", map[string]any{
			"look_id": lookID, "worn_on": wornOn,
		})
		if rec.Code != http.StatusCreated {
			t.Fatalf("create wear: status %d, body %s", rec.Code, rec.Body.String())
		}
		var wear map[string]any
		json.Unmarshal(rec.Body.Bytes(), &wear)
		ids = append(ids, wear["id"].(float64))
	}
	if ids[0] != ids[1] {
		t.Errorf("second tap on the same day logged wear %v, want the first wear %v back", ids[1], ids[0])
	}
	if ids[2] == ids[0] {
		t.Errorf("the same look on the next day reused wear %v, want a new wear", ids[0])
	}

	rec = doJSON(h, cookie, http.MethodGet, "/api/wears", nil)
	var wears []map[string]any
	json.Unmarshal(rec.Body.Bytes(), &wears)
	if len(wears) != 2 {
		t.Errorf("got %d wears, want 2 (one per day)", len(wears))
	}
}
