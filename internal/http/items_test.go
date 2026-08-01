package http

import (
	"bytes"
	"encoding/json"
	"image"
	"image/jpeg"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gianlucanotaro/wardy/internal/store"
)

// loginAndGetCookie creates a user, logs in through the handler, and
// returns the session cookie for use in subsequent requests.
func loginAndGetCookie(t *testing.T, h http.Handler, s *store.Store, email, password string) *http.Cookie {
	t.Helper()
	createTestUser(t, s, email, password)
	rec := doLogin(h, email, password)
	if rec.Code != http.StatusOK {
		t.Fatalf("login: got status %d, body %s", rec.Code, rec.Body.String())
	}
	cookies := rec.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatal("login: expected a session cookie")
	}
	return cookies[0]
}

func newFixtureJPEG(t *testing.T) []byte {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, 200, 150))
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 90}); err != nil {
		t.Fatalf("encode fixture jpeg: %v", err)
	}
	return buf.Bytes()
}

func uploadItem(t *testing.T, h http.Handler, cookie *http.Cookie, photo []byte) *httptest.ResponseRecorder {
	t.Helper()
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	part, err := mw.CreateFormFile("photo", "test.jpg")
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	if _, err := part.Write(photo); err != nil {
		t.Fatalf("write photo: %v", err)
	}
	if err := mw.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/items", &body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestUploadAndListItem(t *testing.T) {
	s := newTestStore(t)
	h := newTestHandler(t, s)
	cookie := loginAndGetCookie(t, h, s, "me@example.com", "correct horse battery staple")

	rec := uploadItem(t, h, cookie, newFixtureJPEG(t))
	if rec.Code != http.StatusCreated {
		t.Fatalf("upload: got status %d, body %s", rec.Code, rec.Body.String())
	}

	var item store.Item
	if err := json.Unmarshal(rec.Body.Bytes(), &item); err != nil {
		t.Fatalf("decode upload response: %v", err)
	}
	if item.PhotoPath == "" || item.ThumbPath == "" {
		t.Fatalf("item paths not set: %+v", item)
	}

	listReq := httptest.NewRequest(http.MethodGet, "/api/items", nil)
	listReq.AddCookie(cookie)
	listRec := httptest.NewRecorder()
	h.ServeHTTP(listRec, listReq)
	if listRec.Code != http.StatusOK {
		t.Fatalf("list: got status %d", listRec.Code)
	}
	var items []store.Item
	if err := json.Unmarshal(listRec.Body.Bytes(), &items); err != nil {
		t.Fatalf("decode list response: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("list: got %d items, want 1", len(items))
	}

	for _, path := range []string{"photo", "thumb"} {
		req := httptest.NewRequest(http.MethodGet, "/api/items/1/"+path, nil)
		req.AddCookie(cookie)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("%s: got status %d", path, rec.Code)
		}
	}
}

func TestUploadRejectsHEIC(t *testing.T) {
	s := newTestStore(t)
	h := newTestHandler(t, s)
	cookie := loginAndGetCookie(t, h, s, "me@example.com", "correct horse battery staple")

	heic := append([]byte{0, 0, 0, 0x20}, []byte("ftypheic")...)
	heic = append(heic, make([]byte, 16)...)

	rec := uploadItem(t, h, cookie, heic)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("upload HEIC: got status %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestItemOwnershipHTTP(t *testing.T) {
	s := newTestStore(t)
	h := newTestHandler(t, s)

	cookieA := loginAndGetCookie(t, h, s, "a@example.com", "correct horse battery staple")
	rec := uploadItem(t, h, cookieA, newFixtureJPEG(t))
	if rec.Code != http.StatusCreated {
		t.Fatalf("upload as A: got status %d", rec.Code)
	}

	cookieB := loginAndGetCookie(t, h, s, "b@example.com", "correct horse battery staple")

	getReq := httptest.NewRequest(http.MethodGet, "/api/items/1", nil)
	getReq.AddCookie(cookieB)
	getRec := httptest.NewRecorder()
	h.ServeHTTP(getRec, getReq)
	if getRec.Code != http.StatusNotFound {
		t.Errorf("GET item as non-owner: got status %d, want %d", getRec.Code, http.StatusNotFound)
	}

	thumbReq := httptest.NewRequest(http.MethodGet, "/api/items/1/thumb", nil)
	thumbReq.AddCookie(cookieB)
	thumbRec := httptest.NewRecorder()
	h.ServeHTTP(thumbRec, thumbReq)
	if thumbRec.Code != http.StatusNotFound {
		t.Errorf("GET thumb as non-owner: got status %d, want %d", thumbRec.Code, http.StatusNotFound)
	}

	listReq := httptest.NewRequest(http.MethodGet, "/api/items", nil)
	listReq.AddCookie(cookieB)
	listRec := httptest.NewRecorder()
	h.ServeHTTP(listRec, listReq)
	var items []store.Item
	json.Unmarshal(listRec.Body.Bytes(), &items)
	if len(items) != 0 {
		t.Errorf("list as non-owner: got %d items, want 0", len(items))
	}
}

func TestDeleteItem(t *testing.T) {
	s := newTestStore(t)
	h := newTestHandler(t, s)
	cookie := loginAndGetCookie(t, h, s, "me@example.com", "correct horse battery staple")

	uploadItem(t, h, cookie, newFixtureJPEG(t))

	delReq := httptest.NewRequest(http.MethodDelete, "/api/items/1", nil)
	delReq.AddCookie(cookie)
	delRec := httptest.NewRecorder()
	h.ServeHTTP(delRec, delReq)
	if delRec.Code != http.StatusNoContent {
		t.Fatalf("delete: got status %d", delRec.Code)
	}

	getReq := httptest.NewRequest(http.MethodGet, "/api/items/1", nil)
	getReq.AddCookie(cookie)
	getRec := httptest.NewRecorder()
	h.ServeHTTP(getRec, getReq)
	if getRec.Code != http.StatusNotFound {
		t.Errorf("get after delete: got status %d, want %d", getRec.Code, http.StatusNotFound)
	}
}
