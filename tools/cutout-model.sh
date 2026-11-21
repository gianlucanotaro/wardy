#!/usr/bin/env bash
# Makes the files a phone needs to cut a garment out of its photo (about 56 MB):
# the ISNet model, shrunk to 8 bits, and the ONNX runtime that runs it in the
# browser. Both are MIT-licensed (imgly/isnet-general-onnx, onnxruntime-web).
#
#   tools/cutout-model.sh <dir>      e.g. tools/cutout-model.sh data/models
#
# Needs curl, python3 and internet. The shrinking step wants about 1.5 GB of
# RAM for a few seconds: run it on a laptop, then copy <dir> to the server's
# <data>/models/ (docs/deployment.md). Nothing here is part of the binary.
set -euo pipefail

OUT="${1:?usage: $0 <dir>   (for example data/models)}"
ORT_VERSION=1.31.1
RUNTIME="https://cdn.jsdelivr.net/npm/onnxruntime-web@${ORT_VERSION}/dist"
MODEL="https://huggingface.co/imgly/isnet-general-onnx/resolve/main/onnx/model.onnx"

mkdir -p "$OUT"
work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT

echo "runtime (onnxruntime-web ${ORT_VERSION})"
for f in ort.wasm.min.mjs ort-wasm-simd-threaded.mjs ort-wasm-simd-threaded.wasm; do
  curl -fL --progress-bar -o "${OUT}/${f}" "${RUNTIME}/${f}"
done

echo "model (176 MB to download, 44 MB once shrunk)"
curl -fL --progress-bar -o "${work}/isnet.onnx" "$MODEL"
python3 -m venv "${work}/venv"
"${work}/venv/bin/pip" install -q onnxruntime onnx
"${work}/venv/bin/python" - "${work}/isnet.onnx" "${OUT}/isnet_quint8.onnx" <<'PY'
import sys
from onnxruntime.quantization import QuantType, quantize_dynamic

quantize_dynamic(sys.argv[1], sys.argv[2], weight_type=QuantType.QUInt8)
PY

ls -l "$OUT"
