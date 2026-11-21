package http

import (
	"bytes"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"testing/fstest"

	"github.com/gianlucanotaro/wardy/internal/ai"
	"github.com/gianlucanotaro/wardy/internal/store"
	"github.com/gianlucanotaro/wardy/internal/weather"
)

// newTestHandlerDirs is newTestHandler, but hands back the photo and model
// directories so a test can look at the files.
func newTestHandlerDirs(t *testing.T, s *store.Store) (h http.Handler, photosDir, modelsDir string) {
	t.Helper()
	frontend := fstest.MapFS{"index.html": &fstest.MapFile{Data: []byte("<html></html>")}}
	provider, err := ai.New("mock", "")
	if err != nil {
		t.Fatalf("ai.New: %v", err)
	}
	photosDir, modelsDir = t.TempDir(), t.TempDir()
	return New(s, frontend, false, photosDir, modelsDir, true, weather.NewClient(), provider, 5.00, OIDCConfig{}), photosDir, modelsDir
}

// a mask the same shape as newFixtureJPEG's 200x150 photo (for example
// 100x75): opaque where the garment is, transparent elsewhere
func fixtureMaskPNG(t *testing.T, w, h int) []byte {
	t.Helper()
	m := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := h / 4; y < h*3/4; y++ {
		for x := w / 4; x < w*3/4; x++ {
			m.SetNRGBA(x, y, color.NRGBA{A: 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, m); err != nil {
		t.Fatalf("encode mask: %v", err)
	}
	return buf.Bytes()
}

func multipartWith(t *testing.T, files map[string][]byte) (*bytes.Buffer, string) {
	t.Helper()
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	for field, data := range files {
		part, err := mw.CreateFormFile(field, field+".bin")
		if err != nil {
			t.Fatalf("create form file: %v", err)
		}
		part.Write(data)
	}
	mw.Close()
	return &body, mw.FormDataContentType()
}

func uploadItemWithMask(t *testing.T, h http.Handler, cookie *http.Cookie, photo, mask []byte) *httptest.ResponseRecorder {
	t.Helper()
	files := map[string][]byte{"photo": photo}
	if mask != nil {
		files["mask"] = mask
	}
	body, contentType := multipartWith(t, files)
	req := httptest.NewRequest(http.MethodPost, "/api/items", body)
	req.Header.Set("Content-Type", contentType)
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func putMask(t *testing.T, h http.Handler, cookie *http.Cookie, id int64, mask []byte) *httptest.ResponseRecorder {
	t.Helper()
	body, contentType := multipartWith(t, map[string][]byte{"mask": mask})
	req := httptest.NewRequest(http.MethodPut, "/api/items/"+strconv.FormatInt(id, 10)+"/mask", body)
	req.Header.Set("Content-Type", contentType)
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func request(h http.Handler, cookie *http.Cookie, method, path string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, nil)
	if cookie != nil {
		req.AddCookie(cookie)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func decodeItem(t *testing.T, rec *httptest.ResponseRecorder) store.Item {
	t.Helper()
	var item store.Item
	if err := json.Unmarshal(rec.Body.Bytes(), &item); err != nil {
		t.Fatalf("decode item: %v (body %s)", err, rec.Body.String())
	}
	return item
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func TestUploadWithMaskKeepsTheCutOut(t *testing.T) {
	s := newTestStore(t)
	h, photosDir, _ := newTestHandlerDirs(t, s)
	cookie := loginAndGetCookie(t, h, s, "me@example.com", "correct horse battery staple")

	rec := uploadItemWithMask(t, h, cookie, newFixtureJPEG(t), fixtureMaskPNG(t, 100, 75))
	if rec.Code != http.StatusCreated {
		t.Fatalf("upload: status %d, body %s", rec.Code, rec.Body.String())
	}
	item := decodeItem(t, rec)
	if item.MaskPath == "" || !fileExists(filepath.Join(photosDir, item.MaskPath)) {
		t.Fatalf("mask not stored: mask_path %q", item.MaskPath)
	}
	// fixtureMaskPNG keeps the middle half; the box says so, with the photo's shape
	if b := item.MaskBox.Box; !item.MaskBox.Valid || b.X < 0.24 || b.X > 0.26 || b.W < 0.49 || b.W > 0.51 || b.Ratio < 1.33 || b.Ratio > 1.34 {
		t.Errorf("mask_box = %+v, want x .25 w .5 ratio 4/3", item.MaskBox)
	}

	got := request(h, cookie, http.MethodGet, "/api/items/"+strconv.FormatInt(item.ID, 10)+"/mask")
	if got.Code != http.StatusOK || got.Header().Get("Content-Type") != "image/png" {
		t.Fatalf("serve mask: status %d, type %q", got.Code, got.Header().Get("Content-Type"))
	}
	if _, err := png.Decode(got.Body); err != nil {
		t.Errorf("served mask is not a PNG: %v", err)
	}

	// the list says so too, so the app knows which photos to cut out
	list := request(h, cookie, http.MethodGet, "/api/items")
	var items []store.Item
	json.Unmarshal(list.Body.Bytes(), &items)
	if len(items) != 1 || items[0].MaskPath != item.MaskPath {
		t.Errorf("list does not carry mask_path: %+v", items)
	}
}

// A cut-out is optional: no bad mask may cost someone their upload.
func TestUploadWithUnusableMaskStillCreatesTheItem(t *testing.T) {
	s := newTestStore(t)
	h, _, _ := newTestHandlerDirs(t, s)
	cookie := loginAndGetCookie(t, h, s, "me@example.com", "correct horse battery staple")

	for name, mask := range map[string][]byte{
		"garbage":     []byte("not an image"),
		"a JPEG":      newFixtureJPEG(t),
		"wrong shape": fixtureMaskPNG(t, 100, 100),
	} {
		rec := uploadItemWithMask(t, h, cookie, newFixtureJPEG(t), mask)
		if rec.Code != http.StatusCreated {
			t.Fatalf("%s: upload status %d, body %s", name, rec.Code, rec.Body.String())
		}
		if item := decodeItem(t, rec); item.MaskPath != "" {
			t.Errorf("%s: mask_path %q, want none", name, item.MaskPath)
		}
	}
}

func TestPutMaskReplacesAndDeleteRemoves(t *testing.T) {
	s := newTestStore(t)
	h, photosDir, _ := newTestHandlerDirs(t, s)
	cookie := loginAndGetCookie(t, h, s, "me@example.com", "correct horse battery staple")

	item := decodeItem(t, uploadItem(t, h, cookie, newFixtureJPEG(t)))
	id := strconv.FormatInt(item.ID, 10)
	if item.MaskPath != "" {
		t.Fatalf("a plain upload has no mask, got %q", item.MaskPath)
	}

	first := decodeItem(t, putMask(t, h, cookie, item.ID, fixtureMaskPNG(t, 100, 75)))
	if first.MaskPath == "" || !fileExists(filepath.Join(photosDir, first.MaskPath)) {
		t.Fatalf("PUT did not store a mask: %q", first.MaskPath)
	}

	second := decodeItem(t, putMask(t, h, cookie, item.ID, fixtureMaskPNG(t, 200, 150)))
	if second.MaskPath == first.MaskPath {
		t.Errorf("a new mask reused the old name %q, so browsers would keep showing the old one", first.MaskPath)
	}
	if fileExists(filepath.Join(photosDir, first.MaskPath)) {
		t.Errorf("the replaced mask file %q is still on disk", first.MaskPath)
	}

	// a mask of the wrong shape is refused and the old one stays
	if rec := putMask(t, h, cookie, item.ID, fixtureMaskPNG(t, 100, 100)); rec.Code != http.StatusBadRequest {
		t.Errorf("wrong shape: status %d, want 400", rec.Code)
	}
	if rec := putMask(t, h, cookie, item.ID, []byte("nope")); rec.Code != http.StatusBadRequest {
		t.Errorf("garbage: status %d, want 400", rec.Code)
	}
	if !fileExists(filepath.Join(photosDir, second.MaskPath)) {
		t.Errorf("a refused mask removed the current one")
	}

	if rec := request(h, cookie, http.MethodDelete, "/api/items/"+id+"/mask"); rec.Code != http.StatusNoContent {
		t.Fatalf("DELETE mask: status %d", rec.Code)
	}
	if fileExists(filepath.Join(photosDir, second.MaskPath)) {
		t.Errorf("the mask file is still on disk after DELETE")
	}
	if rec := request(h, cookie, http.MethodGet, "/api/items/"+id+"/mask"); rec.Code != http.StatusNotFound {
		t.Errorf("GET after DELETE: status %d, want 404", rec.Code)
	}
	// the photo itself is untouched
	if rec := request(h, cookie, http.MethodGet, "/api/items/"+id+"/photo"); rec.Code != http.StatusOK {
		t.Errorf("photo after DELETE mask: status %d, want 200", rec.Code)
	}
}

// User A's cut-out is not user B's to read, replace or remove.
func TestMaskOwnership(t *testing.T) {
	s := newTestStore(t)
	h, photosDir, _ := newTestHandlerDirs(t, s)
	cookieA := loginAndGetCookie(t, h, s, "a@example.com", "correct horse battery staple")
	cookieB := loginAndGetCookie(t, h, s, "b@example.com", "correct horse battery staple")

	item := decodeItem(t, uploadItemWithMask(t, h, cookieA, newFixtureJPEG(t), fixtureMaskPNG(t, 100, 75)))
	id := strconv.FormatInt(item.ID, 10)

	if rec := request(h, cookieB, http.MethodGet, "/api/items/"+id+"/mask"); rec.Code != http.StatusNotFound {
		t.Errorf("B reads A's mask: status %d, want 404", rec.Code)
	}
	if rec := putMask(t, h, cookieB, item.ID, fixtureMaskPNG(t, 100, 75)); rec.Code != http.StatusNotFound {
		t.Errorf("B replaces A's mask: status %d, want 404", rec.Code)
	}
	if rec := request(h, cookieB, http.MethodDelete, "/api/items/"+id+"/mask"); rec.Code != http.StatusNotFound {
		t.Errorf("B deletes A's mask: status %d, want 404", rec.Code)
	}
	if !fileExists(filepath.Join(photosDir, item.MaskPath)) {
		t.Errorf("A's mask file is gone")
	}
	if rec := request(h, nil, http.MethodGet, "/api/items/"+id+"/mask"); rec.Code != http.StatusUnauthorized {
		t.Errorf("no login: status %d, want 401", rec.Code)
	}
}

func TestDeletingAnItemRemovesItsMaskFile(t *testing.T) {
	s := newTestStore(t)
	h, photosDir, _ := newTestHandlerDirs(t, s)
	cookie := loginAndGetCookie(t, h, s, "me@example.com", "correct horse battery staple")

	item := decodeItem(t, uploadItemWithMask(t, h, cookie, newFixtureJPEG(t), fixtureMaskPNG(t, 100, 75)))
	if rec := request(h, cookie, http.MethodDelete, "/api/items/"+strconv.FormatInt(item.ID, 10)); rec.Code != http.StatusNoContent {
		t.Fatalf("delete item: status %d", rec.Code)
	}
	for _, rel := range []string{item.PhotoPath, item.ThumbPath, item.MaskPath} {
		if fileExists(filepath.Join(photosDir, rel)) {
			t.Errorf("%s is still on disk after the item was deleted", rel)
		}
	}
}

func TestModelRoute(t *testing.T) {
	s := newTestStore(t)
	h, _, modelsDir := newTestHandlerDirs(t, s)
	cookie := loginAndGetCookie(t, h, s, "me@example.com", "correct horse battery staple")

	if rec := request(h, nil, http.MethodGet, "/api/models/isnet_quint8.onnx"); rec.Code != http.StatusUnauthorized {
		t.Errorf("no login: status %d, want 401", rec.Code)
	}
	// not installed: the app just does not offer cut-outs
	if rec := request(h, cookie, http.MethodGet, "/api/models/isnet_quint8.onnx"); rec.Code != http.StatusNotFound {
		t.Errorf("not installed: status %d, want 404", rec.Code)
	}

	os.WriteFile(filepath.Join(modelsDir, "isnet_quint8.onnx"), []byte("model"), 0o644)
	os.WriteFile(filepath.Join(modelsDir, "ort-wasm-simd-threaded.wasm"), []byte("wasm"), 0o644)
	os.WriteFile(filepath.Join(modelsDir, "secret.txt"), []byte("no"), 0o644)
	rec := request(h, cookie, http.MethodGet, "/api/models/isnet_quint8.onnx")
	if rec.Code != http.StatusOK || rec.Body.String() != "model" {
		t.Errorf("installed model: status %d, body %q", rec.Code, rec.Body.String())
	}
	if ct := request(h, cookie, http.MethodGet, "/api/models/ort-wasm-simd-threaded.wasm").Header().Get("Content-Type"); ct != "application/wasm" {
		t.Errorf("wasm content type %q, want application/wasm (the browser refuses it otherwise)", ct)
	}

	// only the listed names are served, whatever else is in the folder
	for _, name := range []string{"secret.txt", "..%2Fphotos", "..", "isnet_quint8.onnx%00.txt"} {
		if rec := request(h, cookie, http.MethodGet, "/api/models/"+name); rec.Code == http.StatusOK {
			t.Errorf("/api/models/%s served a file", name)
		}
	}
}

func TestMaskBoxFollowsTheMask(t *testing.T) {
	s := newTestStore(t)
	h, _, _ := newTestHandlerDirs(t, s)
	cookie := loginAndGetCookie(t, h, s, "me@example.com", "correct horse battery staple")

	item := decodeItem(t, uploadItem(t, h, cookie, newFixtureJPEG(t)))
	if item.MaskBox.Valid {
		t.Fatalf("a plain upload has a box: %+v", item.MaskBox)
	}

	// a mask that keeps only the left tenth: the box moves with it
	m := image.NewNRGBA(image.Rect(0, 0, 100, 75))
	for y := 0; y < 75; y++ {
		for x := 0; x < 10; x++ {
			m.SetNRGBA(x, y, color.NRGBA{A: 255})
		}
	}
	var buf bytes.Buffer
	png.Encode(&buf, m)
	put := decodeItem(t, putMask(t, h, cookie, item.ID, buf.Bytes()))
	if b := put.MaskBox.Box; !put.MaskBox.Valid || b.X != 0 || b.W < 0.09 || b.W > 0.11 || b.H != 1 {
		t.Errorf("after PUT: mask_box = %+v, want x 0, w .1, h 1", put.MaskBox)
	}
	got := decodeItem(t, request(h, cookie, http.MethodGet, "/api/items/"+strconv.FormatInt(item.ID, 10)))
	if got.MaskBox != put.MaskBox {
		t.Errorf("GET says %+v, PUT said %+v", got.MaskBox, put.MaskBox)
	}

	request(h, cookie, http.MethodDelete, "/api/items/"+strconv.FormatInt(item.ID, 10)+"/mask")
	after := request(h, cookie, http.MethodGet, "/api/items/"+strconv.FormatInt(item.ID, 10))
	if !bytes.Contains(after.Body.Bytes(), []byte(`"mask_box":null`)) {
		t.Errorf("after DELETE the JSON should say \"mask_box\":null, got %s", after.Body.String())
	}
}
