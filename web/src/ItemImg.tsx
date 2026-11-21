import type { CSSProperties } from 'react'

// An item's photo. With a cut-out, the photo is shown through its mask: the
// garment sits on whatever is behind it (a colour block) in its own colours,
// where a plain photo is blended into the block (index.css, .wy-block). The
// mask has the photo's shape, so it is sized the way the photo is: `cover`
// or `contain` the same as object-fit.
export default function ItemImg({
  item,
  full,
  fit = 'cover',
  alt,
  className = '',
  loading,
}: {
  item: { id: number; mask_path: string; name?: string; category?: string }
  full?: boolean
  fit?: 'cover' | 'contain'
  alt?: string
  className?: string
  loading?: 'lazy'
}) {
  const mask = `url(/api/items/${item.id}/mask?v=${encodeURIComponent(item.mask_path)})`
  const style: CSSProperties | undefined = item.mask_path
    ? {
        mixBlendMode: 'normal',
        WebkitMaskImage: mask,
        maskImage: mask,
        WebkitMaskSize: fit,
        maskSize: fit,
        WebkitMaskPosition: 'center',
        maskPosition: 'center',
        WebkitMaskRepeat: 'no-repeat',
        maskRepeat: 'no-repeat',
      }
    : undefined
  return (
    <img
      src={`/api/items/${item.id}/${full ? 'photo' : 'thumb'}`}
      alt={alt ?? (item.name || item.category || '')}
      loading={loading}
      className={`w-full h-full ${fit === 'cover' ? 'object-cover' : 'object-contain'} ${className}`.trim()}
      style={style}
    />
  )
}
