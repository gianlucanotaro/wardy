// Cut-outs: the phone cuts a garment out of its photo and sends the mask along
// (docs/backend.md, "Cut-outs"). The model runs in cutoutWorker.ts; this is
// the part the screens use.

const MODEL_URL = '/api/models/isnet_quint8.onnx'
const KEY = 'wardy-cutout'
const TIMEOUT_MS = 120_000

// A phone that cannot run WebAssembly with SIMD, or has no OffscreenCanvas,
// cannot run the model: the app does not offer it there. (The SIMD module is
// the standard feature-detection probe.)
const SIMD = new Uint8Array([0, 97, 115, 109, 1, 0, 0, 0, 1, 5, 1, 96, 0, 1, 123, 3, 2, 1, 0, 10, 10, 1, 8, 0, 65, 0, 253, 15, 253, 98, 11])

export function cutoutSupported(): boolean {
  return (
    typeof Worker !== 'undefined' &&
    typeof OffscreenCanvas !== 'undefined' &&
    typeof createImageBitmap === 'function' &&
    typeof WebAssembly === 'object' &&
    WebAssembly.validate(SIMD)
  )
}

// Whether the server has the model files (they are installed by hand, see
// docs/deployment.md). Asked once per page load.
let installed: Promise<boolean> | null = null
export function cutoutInstalled(): Promise<boolean> {
  installed ??= fetch(MODEL_URL, { method: 'HEAD' })
    .then((res) => res.ok)
    .catch(() => false)
  return installed
}

// Per device: phones differ a lot. On unless turned off in Settings.
export function cutoutWanted(): boolean {
  return localStorage.getItem(KEY) !== 'off'
}

export function setCutoutWanted(on: boolean) {
  if (on) localStorage.removeItem(KEY)
  else localStorage.setItem(KEY, 'off')
}

// Whether a new photo should be cut out on this phone right now.
export async function shouldCutOut(): Promise<boolean> {
  return cutoutWanted() && cutoutSupported() && (await cutoutInstalled())
}

// Makes the mask for a photo: a PNG of the photo's shape, opaque where the
// garment is. Rejects if it fails or takes too long; callers carry on without.
// The worker is thrown away each time, so the model's memory is returned to
// the phone as soon as the photo is done.
export function makeMask(photo: Blob, onProgress?: (text: string) => void): Promise<Blob> {
  return new Promise((resolve, reject) => {
    const worker = new Worker(new URL('./cutoutWorker.ts', import.meta.url), { type: 'module' })
    const timer = setTimeout(() => finish(() => reject(new Error('cut-out took too long'))), TIMEOUT_MS)
    function finish(then: () => void) {
      clearTimeout(timer)
      worker.terminate()
      then()
    }
    worker.onmessage = (e: MessageEvent<{ type: string; text?: string; mask?: Blob; message?: string }>) => {
      const m = e.data
      if (m.type === 'progress') onProgress?.(m.text ?? '')
      else if (m.type === 'done') finish(() => resolve(m.mask!))
      else finish(() => reject(new Error(m.message ?? 'cut-out failed')))
    }
    worker.onerror = () => finish(() => reject(new Error('cut-out failed to start')))
    worker.postMessage({ photo })
  })
}
