package http

import (
	"net/http"
	"path/filepath"
)

// The files the phone needs to cut a garment out of its photo: the model and
// the ONNX runtime that runs it. They are large (about 56 MB together), so
// they are not in git or the binary: they live in <data>/models/ and are put
// there once (see docs/deployment.md). Only these names are ever served.
var modelFiles = map[string]string{
	"isnet_quint8.onnx":           "application/octet-stream",
	"ort.wasm.min.mjs":            "text/javascript",
	"ort-wasm-simd-threaded.mjs":  "text/javascript",
	"ort-wasm-simd-threaded.wasm": "application/wasm",
}

// serveModelHandler serves one of modelFiles from modelsDir, or 404 if the
// name is not on the list or the file is not installed (the app then simply
// does not offer cut-outs). They are the same for everyone and never change
// under one name, so browsers keep them a week.
func serveModelHandler(modelsDir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		contentType, ok := modelFiles[r.PathValue("name")]
		if !ok {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", contentType)
		w.Header().Set("Cache-Control", "private, max-age=604800")
		http.ServeFile(w, r, filepath.Join(modelsDir, r.PathValue("name")))
	}
}
