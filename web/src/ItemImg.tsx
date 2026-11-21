import type { CSSProperties } from 'react'
import { fitGarment, type GarmentBox } from './normalize'

// An item's photo. With a cut-out, the photo is shown through its mask: the
// garment sits on whatever is behind it (a colour block) in its own colours,
// where a plain photo is blended into the block (index.css, .wy-block).
//
// In a tile, a cut-out garment is also given the same size as every other
// (normalize.ts): moved to the middle and scaled to fill the tile, less a
// margin. `tile` is the tile's width/height; the tile must clip what spills
// (.wy-block does). `full` is the photo on its own screen, left as it was
// taken. The mask has the photo's shape, so it is sized the way the photo is:
// `cover` or `contain` the same as object-fit.
//
// A photo enlarged much would show its pixels, so one that needs it comes
// from the full-size file, not the thumbnail.
const SHARP_ABOVE = 1.3

export default function ItemImg({
  item,
  full,
  fit = 'cover',
  tile = 0.75,
  alt,
  className = '',
  loading,
}: {
  item: { id: number; mask_path: string; mask_box?: GarmentBox | null; name?: string; category?: string }
  full?: boolean
  fit?: 'cover' | 'contain'
  tile?: number
  alt?: string
  className?: string
  loading?: 'lazy'
}) {
  const masked = item.mask_path !== ''
  const norm = masked && !full && item.mask_box ? fitGarment(item.mask_box, tile) : null
  // the normalizing maths assumes the photo covers the tile
  const sizing = norm ? 'cover' : fit
  const mask = `url(/api/items/${item.id}/mask?v=${encodeURIComponent(item.mask_path)})`
  const style: CSSProperties | undefined = masked
    ? {
        mixBlendMode: 'normal',
        WebkitMaskImage: mask,
        maskImage: mask,
        WebkitMaskSize: sizing,
        maskSize: sizing,
        WebkitMaskPosition: 'center',
        maskPosition: 'center',
        WebkitMaskRepeat: 'no-repeat',
        maskRepeat: 'no-repeat',
        transform: norm ? `scale(${norm.scale}) translate(${norm.dx * 100}%, ${norm.dy * 100}%)` : undefined,
      }
    : undefined
  const big = full || (norm !== null && norm.scale > SHARP_ABOVE)
  return (
    <img
      src={`/api/items/${item.id}/${big ? 'photo' : 'thumb'}`}
      alt={alt ?? (item.name || item.category || '')}
      loading={loading}
      className={`w-full h-full ${sizing === 'cover' ? 'object-cover' : 'object-contain'} ${className}`.trim()}
      style={style}
    />
  )
}
