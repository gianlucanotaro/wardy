# Tagging taxonomy — closed vocabularies

Read before touching tagging, filtering, or the item schema.

Free-text AI tags make filtering useless. Every attribute is a fixed enum,
defined once in Go and mirrored in TypeScript. Any model response containing an
out-of-vocabulary value is rejected and retried once, then failed.

```
category     top | bottom | dress | outerwear | shoes | bag | accessory
subcategory  constrained per category (t-shirt, knit, jeans, blazer, boots…)
pattern      solid | striped | checked | floral | graphic | textured | other
material     cotton | linen | wool | cashmere | denim | leather | silk |
             synthetic | blend | unknown
fit          slim | regular | relaxed | oversized
color        fixed ~20-value palette
seasons      [spring | summer | autumn | winter]
occasions    [work | casual | formal | date | sport | outdoor | home]
styles       [up to 3 of the inspo style list below]
formality    range min–max on 1 (loungewear) … 5 (black tie)
warmth       range min–max on 1 (hot weather) … 5 (deep winter)
```

## Why the numeric scales matter

`warmth` and `formality` do the heavy lifting. Weather matching is arithmetic on
`warmth`; occasion matching is arithmetic on `formality`. Keep it that way —
cheap, instant, debuggable, and it works with zero AI.

Both are **ranges per item**, not single values: a plain shirt can be formality
2–4 and warmth 1–5 (worn under a knit in winter), a wool coat warmth 4–5. They
are judged per **outfit** (`internal/suggest`):

- **Warmth layers.** A piece fits the day when the forecast's target falls in
  its warmth range — the range already counts layering. A piece is about one
  step warmer than the hottest weather it suits, and the outfit adds a warmer
  top and/or outerwear until its warmest upper layer is within one step of the
  target.
- **Formality ranges meet.** Each occasion accepts a range too (casual 1–4,
  work 3–5, …); a piece qualifies when the two overlap. Pieces in one outfit
  stay within 2 steps of each other — blazer with jeans, not with joggers —
  and a piece with a wider range bridges more.
- **Occasion and season tags are preferences.** Tagged items are suggested
  first; untagged ones still qualify.

## Inspiration fits

Photos of other people's outfits (#49) reuse these vocabularies and add one:

```
style        minimal | classic | preppy | workwear | streetwear | sporty |
             smart-casual | bohemian | edgy | romantic | avant-garde | vintage
```

A fit has up to 3 styles, 5 colours and 8 pieces (pieces are item
subcategories, so they can be matched against the wardrobe). Every field may
be empty; formality 0–0 means unset.

Items carry **up to 3 styles** from the same list, so the wardrobe, what's
worn and the inspo board compare on one scale (the style radar,
`docs/ai.md`). Unlike the other item fields, styles may be empty: items
uploaded before they existed start without and get them from a backfill.

## Colors

Map to the nearest palette entry. Never let the model invent "sage mist" or
"dusty olive" — two names for one color kills filtering.

## Material is unreliable from photos

A vision model cannot distinguish wool from acrylic. Expect low accuracy here
and make it easy for the user to correct. `unknown` is a valid, common answer —
prefer it over a confident guess.
