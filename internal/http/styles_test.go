package http

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/gianlucanotaro/wardy/internal/store"
)

func TestUpdateItemTagsStyles(t *testing.T) {
	s := newTestStore(t)
	h := newTestHandler(t, s)
	cookie := loginAndGetCookie(t, h, s, "me@example.com", "correct horse battery staple")
	uploadItem(t, h, cookie, newFixtureJPEG(t))

	body := validTagsPayload()
	body["styles"] = []string{"minimal", "workwear"}
	rec := patchTags(h, cookie, body)
	if rec.Code != http.StatusOK {
		t.Fatalf("set styles: status %d, body %s", rec.Code, rec.Body.String())
	}
	var item store.Item
	json.Unmarshal(rec.Body.Bytes(), &item)
	if !reflect.DeepEqual([]string(item.Styles), []string{"minimal", "workwear"}) {
		t.Fatalf("Styles = %v", item.Styles)
	}

	// A client from before styles leaves the field out: the styles stay.
	rec = patchTags(h, cookie, validTagsPayload())
	if rec.Code != http.StatusOK {
		t.Fatalf("edit without styles: status %d", rec.Code)
	}
	json.Unmarshal(rec.Body.Bytes(), &item)
	if !reflect.DeepEqual([]string(item.Styles), []string{"minimal", "workwear"}) {
		t.Errorf("Styles after an edit without the field = %v, want them kept", item.Styles)
	}

	// An explicit empty list clears them.
	body["styles"] = []string{}
	rec = patchTags(h, cookie, body)
	json.Unmarshal(rec.Body.Bytes(), &item)
	if len(item.Styles) != 0 {
		t.Errorf("Styles after clearing = %v, want none", item.Styles)
	}

	body["styles"] = []string{"cottagecore"}
	if rec := patchTags(h, cookie, body); rec.Code != http.StatusBadRequest {
		t.Errorf("out-of-vocabulary style: status %d, want 400", rec.Code)
	}
}

func TestInspoBoardHasRadar(t *testing.T) {
	s := newTestStore(t)
	h := newTestHandler(t, s)
	cookie := loginAndGetCookie(t, h, s, "me@example.com", "correct horse battery staple")
	uploadItem(t, h, cookie, newFixtureJPEG(t))
	body := validTagsPayload()
	body["styles"] = []string{"sporty"}
	if rec := patchTags(h, cookie, body); rec.Code != http.StatusOK {
		t.Fatalf("set styles: status %d", rec.Code)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/inspo", nil)
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("inspo board: status %d", rec.Code)
	}
	var board struct {
		Radar struct {
			Axes  []string `json:"axes"`
			Owned struct {
				Shares []float64 `json:"shares"`
				Count  int       `json:"count"`
			} `json:"owned"`
		} `json:"radar"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &board); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if board.Radar.Owned.Count != 1 || board.Radar.Axes[4] != "sporty" || board.Radar.Owned.Shares[4] != 100 {
		t.Errorf("radar = %+v, want one owned item, all sporty", board.Radar)
	}
}
