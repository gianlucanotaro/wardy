import type { RadarLayer, StyleRadar } from './api'

export type LayerKey = 'owned' | 'worn' | 'inspo'

// The biggest gaps between layers, in words: liked but not worn, owned but
// not worn, and what's worn most. Each only when both layers have data.
export function radarInsights(radar: StyleRadar): { kind: LayerKey; style: string; text: string; detail: string }[] {
  const out: { kind: LayerKey; style: string; text: string; detail: string }[] = []
  const has = (l: RadarLayer) => l.count > 0
  const biggest = (a: RadarLayer, b: RadarLayer) =>
    radar.axes.map((axis, i) => ({ axis, i, d: a.shares[i] - b.shares[i] })).sort((x, y) => y.d - x.d)[0]
  const name = (axis: string) => axis.replace('-', ' ')

  if (has(radar.inspo) && has(radar.worn)) {
    const g = biggest(radar.inspo, radar.worn)
    if (g.d >= 5) {
      out.push({
        kind: 'inspo',
        style: name(g.axis),
        text: 'more than you wear it',
        detail: `${radar.inspo.shares[g.i]}% of your inspo, ${radar.worn.shares[g.i]}% of what you wore.`,
      })
    }
  }
  if (has(radar.owned) && has(radar.worn)) {
    const g = biggest(radar.owned, radar.worn)
    if (g.d >= 5) {
      out.push({
        kind: 'owned',
        style: name(g.axis),
        text: 'pieces mostly stay in the wardrobe',
        detail: `${radar.owned.shares[g.i]}% of what you own, ${radar.worn.shares[g.i]}% of what you wore.`,
      })
    }
  }
  if (has(radar.worn)) {
    const top = radar.axes.map((axis, i) => ({ axis, v: radar.worn.shares[i] })).sort((a, b) => b.v - a.v)[0]
    out.push({
      kind: 'worn',
      style: name(top.axis),
      text: 'is what you actually reach for',
      detail: `${top.v}% of your wears, the biggest share.`,
    })
  }
  return out
}
