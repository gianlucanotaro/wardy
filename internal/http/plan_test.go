package http

import (
	"encoding/json"
	"net/http"
	"testing"
)

func TestPlanEndpointRoundTrip(t *testing.T) {
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

	rec = doJSON(h, cookie, http.MethodPut, "/api/plan/2026-09-14", map[string]any{"look_id": lookID})
	if rec.Code != http.StatusOK {
		t.Fatalf("set plan day: status %d, body %s", rec.Code, rec.Body.String())
	}
	var day map[string]any
	json.Unmarshal(rec.Body.Bytes(), &day)
	if day["date"] != "2026-09-14" {
		t.Errorf("date = %v, want 2026-09-14", day["date"])
	}
	got, _ := day["look"].(map[string]any)
	if got == nil || int64(got["id"].(float64)) != lookID {
		t.Errorf("look = %v, want look %d", day["look"], lookID)
	}

	rec = doJSON(h, cookie, http.MethodGet, "/api/plan?start=2026-09-14&end=2026-09-20", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("get plan: status %d, body %s", rec.Code, rec.Body.String())
	}
	var days []map[string]any
	json.Unmarshal(rec.Body.Bytes(), &days)
	if len(days) != 7 {
		t.Fatalf("got %d days, want 7", len(days))
	}
	if days[0]["date"] != "2026-09-14" {
		t.Errorf("days[0].date = %v, want 2026-09-14", days[0]["date"])
	}
	got, _ = days[0]["look"].(map[string]any)
	if got == nil || int64(got["id"].(float64)) != lookID {
		t.Errorf("days[0].look = %v, want look %d", days[0]["look"], lookID)
	}
	if days[1]["look"] != nil {
		t.Errorf("days[1].look = %v, want null (unplanned)", days[1]["look"])
	}

	// Clearing a day (look_id: null) should stick.
	rec = doJSON(h, cookie, http.MethodPut, "/api/plan/2026-09-14", map[string]any{"look_id": nil})
	if rec.Code != http.StatusOK {
		t.Fatalf("clear plan day: status %d, body %s", rec.Code, rec.Body.String())
	}
	json.Unmarshal(rec.Body.Bytes(), &day)
	if day["look"] != nil {
		t.Errorf("look = %v, want null after clearing", day["look"])
	}
}

func TestPlanEndpointRejectsLookFromAnotherUser(t *testing.T) {
	s := newTestStore(t)
	h := newTestHandler(t, s)

	cookieA := loginAndGetCookie(t, h, s, "a@example.com", "correct horse battery staple")
	itemA := createTestItem(t, s, 1)
	rec := doJSON(h, cookieA, http.MethodPost, "/api/looks", map[string]any{
		"name": "Look", "item_ids": []int64{itemA},
	})
	var look map[string]any
	json.Unmarshal(rec.Body.Bytes(), &look)
	lookID := int64(look["id"].(float64))

	cookieB := loginAndGetCookie(t, h, s, "b@example.com", "correct horse battery staple")
	rec = doJSON(h, cookieB, http.MethodPut, "/api/plan/2026-09-14", map[string]any{"look_id": lookID})
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestPlanEndpointRejectsBadDates(t *testing.T) {
	s := newTestStore(t)
	h := newTestHandler(t, s)
	cookie := loginAndGetCookie(t, h, s, "me@example.com", "correct horse battery staple")

	if rec := doJSON(h, cookie, http.MethodPut, "/api/plan/not-a-date", map[string]any{"look_id": nil}); rec.Code != http.StatusBadRequest {
		t.Errorf("put bad date: status %d, want %d", rec.Code, http.StatusBadRequest)
	}
	if rec := doJSON(h, cookie, http.MethodGet, "/api/plan?start=bogus&end=2026-09-20", nil); rec.Code != http.StatusBadRequest {
		t.Errorf("get bad start: status %d, want %d", rec.Code, http.StatusBadRequest)
	}
	if rec := doJSON(h, cookie, http.MethodGet, "/api/plan?start=2026-09-20&end=2026-09-14", nil); rec.Code != http.StatusBadRequest {
		t.Errorf("get end before start: status %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestPlanEndpointOwnershipScoped(t *testing.T) {
	s := newTestStore(t)
	h := newTestHandler(t, s)

	cookieA := loginAndGetCookie(t, h, s, "a@example.com", "correct horse battery staple")
	itemA := createTestItem(t, s, 1)
	rec := doJSON(h, cookieA, http.MethodPost, "/api/looks", map[string]any{
		"name": "Look", "item_ids": []int64{itemA},
	})
	var look map[string]any
	json.Unmarshal(rec.Body.Bytes(), &look)
	lookID := int64(look["id"].(float64))
	doJSON(h, cookieA, http.MethodPut, "/api/plan/2026-09-14", map[string]any{"look_id": lookID})

	cookieB := loginAndGetCookie(t, h, s, "b@example.com", "correct horse battery staple")
	rec = doJSON(h, cookieB, http.MethodGet, "/api/plan?start=2026-09-14&end=2026-09-14", nil)
	var days []map[string]any
	json.Unmarshal(rec.Body.Bytes(), &days)
	if len(days) != 1 || days[0]["look"] != nil {
		t.Errorf("user B should not see user A's plan, got: %v", days)
	}
}

// #78: the plan carries each day's wears, and only the caller's own.
func TestPlanEndpointWearsOwnershipScoped(t *testing.T) {
	s := newTestStore(t)
	h := newTestHandler(t, s)

	cookieA := loginAndGetCookie(t, h, s, "a@example.com", "correct horse battery staple")
	itemA := createTestItem(t, s, 1)
	rec := doJSON(h, cookieA, http.MethodPost, "/api/wears", map[string]any{
		"item_ids": []int64{itemA}, "worn_on": "2026-09-14",
	})
	if rec.Code != http.StatusCreated && rec.Code != http.StatusOK {
		t.Fatalf("log wear: status %d, body %s", rec.Code, rec.Body.String())
	}

	cookieB := loginAndGetCookie(t, h, s, "b@example.com", "correct horse battery staple")

	var days []map[string]any
	rec = doJSON(h, cookieA, http.MethodGet, "/api/plan?start=2026-09-14&end=2026-09-15", nil)
	json.Unmarshal(rec.Body.Bytes(), &days)
	if wears, _ := days[0]["wears"].([]any); len(wears) != 1 {
		t.Errorf("user A's plan: days[0].wears = %v, want one wear", days[0]["wears"])
	}
	if wears, ok := days[1]["wears"].([]any); !ok || len(wears) != 0 {
		t.Errorf("user A's plan: days[1].wears = %v, want []", days[1]["wears"])
	}

	rec = doJSON(h, cookieB, http.MethodGet, "/api/plan?start=2026-09-14&end=2026-09-15", nil)
	json.Unmarshal(rec.Body.Bytes(), &days)
	for _, d := range days {
		if wears, _ := d["wears"].([]any); len(wears) != 0 {
			t.Errorf("user B's plan shows user A's wear on %v: %v", d["date"], d["wears"])
		}
	}
}
