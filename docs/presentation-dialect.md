# Presentation Dialect v1 and v2

## Purpose

This document defines the source-of-truth JSON dialect for presentation planning in Ulduar.

The contract is versioned so:

- existing `v1` documents remain valid and supported
- new work can target `v2` without stretching `v1`
- planners, backend normalization, compiler entrypoints, and future frontend work can share one stable contract

## Shared constraints

Both versions:

- use semantic layouts instead of arbitrary coordinates
- require `version`
- allow only `slideSize: "16:9"`
- require at least 1 slide
- reject unknown fields and explicit `null` values

Out of scope for both versions:

- animations
- transitions
- charts or SmartArt
- arbitrary shapes, connectors, or x/y authoring
- remote image fetching
- PPTX renderer implementation details beyond what the dialect requires

## Theme preset registry and fallback

`v2` introduces stable preset IDs and metadata.

Canonical built-in preset IDs reserved from the start:

- `general_clean`
- `travel_editorial`

Stable preset metadata shape:

```json
{
  "id": "general_clean",
  "label": "General Clean",
  "description": "Default balanced preset for general-purpose decks.",
  "isDefault": true
}
```

Rules:

- `id` is the stable backend/compiler/frontend identifier.
- `label` is short human-readable copy.
- `description` is optional.
- `isDefault` is optional and only `true` for the default preset.
- The default preset is always `general_clean`.
- If a requested or planner-emitted preset is unknown or unavailable, normalization resolves it to `general_clean`.
- Capabilities exposed to the frontend should return preset metadata in this exact shape.

## Current shipped implementation notes

- The built-in preset catalog currently contains `general_clean` (default) and `travel_editorial`.
- The public create API does not require a user-supplied preset field today; the backend planner chooses within the built-in catalog and normalization still falls back to `general_clean`.
- `theme:hero-image` resolves to a backend-managed generated PNG owned by the resolved preset. No external template pack, compose mount, or Docker `COPY` step is required for that asset in the current rollout.
- The compiler writes preset font family names into the PPTX theme metadata, but it does not embed or package font binaries. Exact typography can therefore vary with the PowerPoint/viewer font fallback behavior.

### Preset boundary vs slide JSON

Theme presets own:

- color system
- fonts, including default Latin/CJK-capable font choices
- background treatments
- card/image/table visual styling
- decorative assets bundled with the app

Per-slide JSON owns:

- slide intent and layout choice
- titles and semantic content blocks
- symbolic asset references
- optional emphasis/tone choices that stay within the preset system

Per-slide JSON must **not** contain:

- raw font family names
- absolute positions
- arbitrary dimensions
- freeform CSS-like style objects

## Version coexistence

- `v1` stays valid and compilable.
- `v2` is additive and is the target for new planner output.
- There is no silent auto-conversion from `v1` to `v2`.
- Normalized JSON is the source of truth for compilation.
- Older stored `v1` generations remain valid without migration.

## v1 summary

`v1` remains unchanged in spirit and remains the compatibility contract for existing documents.

### Top-level shape

```json
{
  "version": "v1",
  "slideSize": "16:9",
  "slides": [
    {
      "layout": "title",
      "title": "Quarterly Business Review",
      "subtitle": "FY2026 Q1"
    }
  ]
}
```

### v1 layouts

- `title`
- `section`
- `title_bullets`
- `two_column`
- `table`
- `closing`

### v1 block types

- `paragraph`
- `bullet_list`
- `numbered_list`
- `table`
- `quote`

For exact `v1` validation behavior, use the backend normalization rules and tests as the source of truth.

## v2 top-level shape

```json
{
  "version": "v2",
  "slideSize": "16:9",
  "themePresetId": "general_clean",
  "slides": [
    {
      "layout": "cover_hero",
      "title": "Kyoto in Four Days",
      "subtitle": "Autumn editorial itinerary",
      "blocks": [
        {
          "type": "image",
          "assetRef": "attachment:cover-photo",
          "caption": "Arashiyama at golden hour"
        }
      ]
    }
  ]
}
```

### v2 top-level fields

- `version` — required, must be `"v2"`
- `slideSize` — optional, defaults to `"16:9"`
- `themePresetId` — optional; if missing, empty, unknown, or unavailable, normalization resolves it to `"general_clean"`
- `slides` — required array with at least 1 slide

## v2 symbolic asset references

`assetRef` values are symbolic only. Examples:

- `attachment:cover-photo`
- `attachment:map-scan`
- `theme:hero-image`

Rules:

- do not use URLs
- do not inline binary data
- do not fetch remote assets during planning
- downstream orchestration resolves symbolic refs into concrete assets before compilation

Current backend resolution support:

- `attachment:*` resolves to uploaded presentation input assets using backend-provided alias guidance
- `theme:*` resolves only to backend-managed bundled theme assets from the minimal built-in registry
- remote/fetched asset sources are not supported in v1/v2 orchestration today

## v2 layouts

`v2` continues to allow the simple legacy layouts:

- `title`
- `section`
- `title_bullets`
- `two_column`

New `v2`-first layouts:

- `cover_hero`
- `chapter_divider`
- `toc_grid`
- `card_grid`
- `comparison_cards`
- `timeline_itinerary`
- `summary_matrix`
- `recommendation_split`
- `table`
- `closing`

Every slide still requires non-empty `title`.

### `cover_hero`

Allowed fields:

- `title`
- `subtitle` optional
- `blocks`

Rules:

- `blocks` must contain 1 to 3 blocks
- `blocks` may only contain `image`, `badge`, `rich_text`, or `callout`
- `blocks` must contain exactly 1 `image`
- `columns` is not allowed

### `chapter_divider`

Allowed fields:

- `title`
- `subtitle` optional
- `blocks` optional

Rules:

- `blocks` may contain at most 2 blocks
- `blocks` may only contain `image`, `badge`, or `rich_text`
- `blocks` may contain at most 1 `image`
- `columns` is not allowed

### `toc_grid`

Allowed fields:

- `title`
- `subtitle` optional
- `blocks`

Rules:

- `blocks` must contain 2 to 8 blocks
- every block must be `card`
- `columns` is not allowed

### `card_grid`

Rules:

- `blocks` must contain 2 to 6 blocks
- every block must be `card`

### `comparison_cards`

Rules:

- `blocks` must contain exactly 2 blocks
- every block must be `card`

### `timeline_itinerary`

Rules:

- `blocks` must contain 2 to 6 blocks
- every block must be `card`

### `summary_matrix`

Rules:

- `blocks` must contain 2 to 6 blocks
- block types allowed: `stat`, `table`, `callout`
- must contain exactly 1 `table`
- must contain at least 1 `stat`

### `recommendation_split`

Rules:

- `blocks` must contain 2 to 3 blocks
- block types allowed: `image`, `callout`, `badge`
- must contain exactly 1 `image`
- must contain exactly 1 `callout`

### `table`

In `v2`, `table` remains semantic and must compile as a real table in downstream renderer work.

Rules:

- `blocks` must contain 1 to 2 blocks
- block types allowed: `table`, `callout`
- must contain exactly 1 `table`

### `closing`

Rules:

- `subtitle` optional
- `blocks` optional
- `blocks` may contain up to 3 blocks
- block types allowed: `paragraph`, `rich_text`, `callout`, `badge`, `image`

## v2 block types

### Legacy blocks still valid where allowed

- `paragraph`
- `bullet_list`
- `numbered_list`
- `table`
- `quote`

### `image`

```json
{
  "type": "image",
  "assetRef": "attachment:cover-photo",
  "altText": "Torii gate at sunset",
  "caption": "Fushimi Inari at dusk"
}
```

Rules:

- `assetRef` required
- `altText` optional
- `caption` optional

### `card`

```json
{
  "type": "card",
  "title": "Arashiyama",
  "label": "Day 1",
  "body": "Bamboo grove, river walk, and evening lantern streets.",
  "assetRef": "attachment:arashiyama-photo"
}
```

Rules:

- `title` required
- `label` optional
- `body` optional
- `assetRef` optional

### `stat`

```json
{
  "type": "stat",
  "value": "4",
  "label": "Days",
  "body": "Ideal first-time visit length"
}
```

Rules:

- `value` required
- `label` required
- `body` optional

### `badge`

```json
{
  "type": "badge",
  "text": "Best in late November",
  "tone": "accent"
}
```

Rules:

- `text` required
- `tone` optional
- allowed `tone` values:
  - `neutral`
  - `accent`
  - `success`
  - `warning`

### `callout`

```json
{
  "type": "callout",
  "title": "Recommendation",
  "body": "Stay in Gion for the first two nights, then move west only if the itinerary is nature-heavy."
}
```

Rules:

- `title` required
- `body` required

### `rich_text`

```json
{
  "type": "rich_text",
  "spans": [
    { "text": "Slow travel through " },
    { "text": "京都", "lang": "ja", "emphasis": "accent" }
  ]
}
```

Rules:

- `spans` required with at least 1 span
- each span requires non-empty `text`
- optional `emphasis` values:
  - `strong`
  - `emphasis`
  - `accent`
- optional `lang` supports language tagging for bilingual typography

## Planner guidance for v2

When targeting `v2`, the planner should:

- default to `general_clean`
- switch to `travel_editorial` only when the prompt is clearly travel/editorial/image-led
- choose semantic layouts instead of stuffing content into generic bullet slides
- keep asset references symbolic and deterministic
- use `table` blocks for real tabular data
- use `rich_text` only when emphasis or bilingual text is needed
- avoid freeform style invention

## Example: travel deck (`v2`)

```json
{
  "version": "v2",
  "themePresetId": "travel_editorial",
  "slides": [
    {
      "layout": "cover_hero",
      "title": "Kyoto In Four Days",
      "subtitle": "Autumn editorial itinerary",
      "blocks": [
        {
          "type": "image",
          "assetRef": "attachment:cover-photo",
          "caption": "Arashiyama at golden hour"
        },
        {
          "type": "badge",
          "text": "Late November",
          "tone": "accent"
        },
        {
          "type": "rich_text",
          "spans": [
            { "text": "A calm city break with " },
            { "text": "京都", "lang": "ja", "emphasis": "accent" }
          ]
        }
      ]
    },
    {
      "layout": "timeline_itinerary",
      "title": "Four-day flow",
      "blocks": [
        {
          "type": "card",
          "label": "Day 1",
          "title": "Arashiyama west side",
          "body": "Bamboo grove, river walk, and Tenryu-ji before sunset.",
          "assetRef": "attachment:day1-photo"
        },
        {
          "type": "card",
          "label": "Day 2",
          "title": "Higashiyama and Gion",
          "body": "Early temple route, afternoon tea, and evening alleys."
        }
      ]
    },
    {
      "layout": "summary_matrix",
      "title": "Trip snapshot",
      "blocks": [
        {
          "type": "stat",
          "value": "4",
          "label": "Days"
        },
        {
          "type": "stat",
          "value": "2",
          "label": "Hotel moves avoided"
        },
        {
          "type": "table",
          "header": ["Area", "Best for"],
          "rows": [
            ["Gion", "Night walks and dining"],
            ["Arashiyama", "Scenery and slower pacing"]
          ]
        }
      ]
    },
    {
      "layout": "closing",
      "title": "Book the trip",
      "blocks": [
        {
          "type": "callout",
          "title": "Recommendation",
          "body": "Use Gion as the main base and reserve one sunrise slot for Arashiyama."
        }
      ]
    }
  ]
}
```

## Example: non-travel deck (`v2`)

```json
{
  "version": "v2",
  "themePresetId": "general_clean",
  "slides": [
    {
      "layout": "cover_hero",
      "title": "FY2026 Hiring Plan",
      "subtitle": "Operating review and recommendations",
      "blocks": [
        {
          "type": "image",
          "assetRef": "theme:hero-image",
          "caption": "Preset-owned abstract cover art"
        },
        {
          "type": "badge",
          "text": "Board draft",
          "tone": "neutral"
        }
      ]
    },
    {
      "layout": "comparison_cards",
      "title": "Two staffing paths",
      "blocks": [
        {
          "type": "card",
          "title": "Lean plan",
          "body": "Hire only critical customer-facing roles and delay platform expansion."
        },
        {
          "type": "card",
          "title": "Balanced plan",
          "body": "Hire customer-facing roles plus a limited platform pod for reliability."
        }
      ]
    },
    {
      "layout": "table",
      "title": "Role summary",
      "blocks": [
        {
          "type": "callout",
          "title": "Constraint",
          "body": "Headcount must stay within the approved operating plan."
        },
        {
          "type": "table",
          "header": ["Role", "H1", "H2"],
          "rows": [
            ["Account executives", "2", "1"],
            ["Support engineers", "1", "1"],
            ["Platform engineers", "0", "2"]
          ]
        }
      ]
    }
  ]
}
```
