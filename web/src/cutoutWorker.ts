// Cuts a garment out of its photo, on the phone: ISNet through onnxruntime-web
// (both served from our own server, see models.go). Runs in a worker so the
// screen stays alive for the few seconds it takes. lib/cutout.ts is the only
// thing that talks to it.
//
// In:  { photo: Blob }
// Out: { type: 'progress', text } while it works, then { type: 'done', mask }
//      or { type: 'error', message }.
// The mask is a PNG with the photo's shape, opaque where the garment is.

const MODELS = '/api/models/'
const SIZE = 1024 // what the model sees
const MASK_MAX = 1000 // longest edge of the mask we send (the server's cap)
const MEAN = [0.485, 0.456, 0.406]

type Session = {
  inputNames: string[]
  outputNames: string[]
  run(feeds: Record<string, unknown>): Promise<Record<string, { data: Float32Array }>>
}
type Ort = {
  InferenceSession: { create(bytes: Uint8Array, options: object): Promise<Session> }
  Tensor: new (type: 'float32', data: Float32Array, dims: number[]) => unknown
  env: { wasm: { wasmPaths: string; numThreads: number } }
}

const post = (message: object) => self.postMessage(message)

async function download(url: string, label: string): Promise<Uint8Array> {
  const res = await fetch(url)
  if (!res.ok) throw new Error(`${url}: ${res.status}`)
  const total = Number(res.headers.get('content-length')) || 0
  const reader = res.body!.getReader()
  const parts: Uint8Array[] = []
  let got = 0
  for (;;) {
    const { done, value } = await reader.read()
    if (done) break
    parts.push(value)
    got += value.length
    if (total) post({ type: 'progress', text: `${label} ${Math.round((got / total) * 100)}%` })
  }
  const bytes = new Uint8Array(got)
  let at = 0
  for (const part of parts) {
    bytes.set(part, at)
    at += part.length
  }
  return bytes
}

// The garment is the biggest blob: everything else the model dotted about is
// dropped. Holes inside it (a pale garment on a pale wall) are part of it:
// what the border cannot reach without crossing the blob is filled in. The
// edge is pulled in a pixel, to leave out the pixels that are half background,
// and then softened a pixel, so it does not look cut with scissors.
function cleanMask(a: Float32Array): Float32Array {
  const n = SIZE * SIZE
  const labels = new Int32Array(n)
  const stack = new Int32Array(n)
  let best = 0
  let bestSize = 0
  let next = 0
  for (let start = 0; start < n; start++) {
    if (a[start] <= 0.5 || labels[start]) continue
    next++
    let size = 0
    let sp = 0
    stack[sp++] = start
    labels[start] = next
    while (sp) {
      const p = stack[--sp]
      size++
      const x = p % SIZE
      const y = (p / SIZE) | 0
      if (x > 0 && a[p - 1] > 0.5 && !labels[p - 1]) { labels[p - 1] = next; stack[sp++] = p - 1 }
      if (x < SIZE - 1 && a[p + 1] > 0.5 && !labels[p + 1]) { labels[p + 1] = next; stack[sp++] = p + 1 }
      if (y > 0 && a[p - SIZE] > 0.5 && !labels[p - SIZE]) { labels[p - SIZE] = next; stack[sp++] = p - SIZE }
      if (y < SIZE - 1 && a[p + SIZE] > 0.5 && !labels[p + SIZE]) { labels[p + SIZE] = next; stack[sp++] = p + SIZE }
    }
    if (size > bestSize) { bestSize = size; best = next }
  }
  if (bestSize < n * 0.02) throw new Error('no garment found')

  const keep = new Uint8Array(n)
  for (let i = 0; i < n; i++) keep[i] = labels[i] === best ? 1 : 0

  // what the border can reach without crossing the garment is background
  const outside = new Uint8Array(n)
  let sp = 0
  const push = (p: number) => { if (!keep[p] && !outside[p]) { outside[p] = 1; stack[sp++] = p } }
  for (let x = 0; x < SIZE; x++) { push(x); push((SIZE - 1) * SIZE + x) }
  for (let y = 0; y < SIZE; y++) { push(y * SIZE); push(y * SIZE + SIZE - 1) }
  while (sp) {
    const p = stack[--sp]
    const x = p % SIZE
    const y = (p / SIZE) | 0
    if (x > 0) push(p - 1)
    if (x < SIZE - 1) push(p + 1)
    if (y > 0) push(p - SIZE)
    if (y < SIZE - 1) push(p + SIZE)
  }

  // pulled in a pixel (the smallest of the 3x3 around it), then softened (their mean)
  const solid = new Float32Array(n)
  for (let i = 0; i < n; i++) solid[i] = outside[i] ? 0 : 1
  const pulled = new Float32Array(n)
  const out = new Float32Array(n)
  for (let y = 1; y < SIZE - 1; y++) {
    for (let x = 1; x < SIZE - 1; x++) {
      const p = y * SIZE + x
      pulled[p] = Math.min(
        solid[p - SIZE - 1], solid[p - SIZE], solid[p - SIZE + 1],
        solid[p - 1], solid[p], solid[p + 1],
        solid[p + SIZE - 1], solid[p + SIZE], solid[p + SIZE + 1],
      )
    }
  }
  for (let y = 1; y < SIZE - 1; y++) {
    for (let x = 1; x < SIZE - 1; x++) {
      const p = y * SIZE + x
      out[p] =
        (pulled[p - SIZE - 1] + pulled[p - SIZE] + pulled[p - SIZE + 1] +
          pulled[p - 1] + pulled[p] + pulled[p + 1] +
          pulled[p + SIZE - 1] + pulled[p + SIZE] + pulled[p + SIZE + 1]) / 9
    }
  }
  return out
}

async function cutOut(photo: Blob): Promise<Blob> {
  post({ type: 'progress', text: 'loading' })
  const ort = (await import(/* @vite-ignore */ MODELS + 'ort.wasm.min.mjs')) as Ort
  ort.env.wasm.wasmPaths = MODELS
  ort.env.wasm.numThreads = 1 // more needs cross-origin isolation, which we do not ask for
  const model = await download(MODELS + 'isnet_quint8.onnx', 'downloading the model')
  post({ type: 'progress', text: 'preparing' })
  const session = await ort.InferenceSession.create(model, { executionProviders: ['wasm'], graphOptimizationLevel: 'all' })

  // the photo as the server will see it: upright
  const bitmap = await createImageBitmap(photo, { imageOrientation: 'from-image' })
  const shape = { w: bitmap.width, h: bitmap.height }
  const small = new OffscreenCanvas(SIZE, SIZE)
  const sctx = small.getContext('2d', { willReadFrequently: true })!
  sctx.drawImage(bitmap, 0, 0, SIZE, SIZE)
  bitmap.close()
  const px = sctx.getImageData(0, 0, SIZE, SIZE).data
  let max = 1e-6
  for (let i = 0; i < px.length; i += 4) max = Math.max(max, px[i], px[i + 1], px[i + 2])
  const data = new Float32Array(3 * SIZE * SIZE)
  for (let i = 0; i < SIZE * SIZE; i++) {
    for (let c = 0; c < 3; c++) data[c * SIZE * SIZE + i] = px[i * 4 + c] / max - MEAN[c]
  }

  post({ type: 'progress', text: 'cutting out' })
  const input = new ort.Tensor('float32', data, [1, 3, SIZE, SIZE])
  const result = await session.run({ [session.inputNames[0]]: input })
  const raw = result[session.outputNames[0]].data
  let lo = Infinity
  let hi = -Infinity
  for (let i = 0; i < raw.length; i++) {
    if (raw[i] < lo) lo = raw[i]
    if (raw[i] > hi) hi = raw[i]
  }
  const probability = new Float32Array(SIZE * SIZE)
  for (let i = 0; i < probability.length; i++) probability[i] = (raw[i] - lo) / (hi - lo)
  const mask = cleanMask(probability)

  // black, with the mask as its alpha, at the photo's shape
  const square = new OffscreenCanvas(SIZE, SIZE)
  const qctx = square.getContext('2d')!
  const image = qctx.createImageData(SIZE, SIZE)
  for (let i = 0; i < SIZE * SIZE; i++) image.data[i * 4 + 3] = Math.round(mask[i] * 255)
  qctx.putImageData(image, 0, 0)
  const scale = Math.min(1, MASK_MAX / Math.max(shape.w, shape.h))
  const out = new OffscreenCanvas(Math.round(shape.w * scale), Math.round(shape.h * scale))
  out.getContext('2d')!.drawImage(square, 0, 0, out.width, out.height)
  return out.convertToBlob({ type: 'image/png' })
}

self.onmessage = async (e: MessageEvent<{ photo: Blob }>) => {
  try {
    post({ type: 'done', mask: await cutOut(e.data.photo) })
  } catch (err) {
    post({ type: 'error', message: err instanceof Error ? err.message : String(err) })
  }
}
