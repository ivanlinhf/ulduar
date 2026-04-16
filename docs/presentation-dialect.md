# Presentation Dialect v1

## Purpose

This document defines the source-of-truth JSON dialect for presentation planning in Ulduar v1.

It is intentionally small, semantic, and layout-based so that:

- an LLM planner can produce it reliably
- a PPTX compiler can consume it deterministically
- future agents can implement against one stable contract

## Goals

- Keep the dialect versioned from day one
- Represent slide intent instead of freeform coordinates
- Limit v1 to a fixed set of layouts and content block types
- Make normalization/defaulting explicit and deterministic

## Non-goals

The following are explicitly out of scope for v1:

- animations
- transitions
- charts or SmartArt
- arbitrary shapes, connectors, or absolute x/y placement
- embedded uploaded images as slide assets
- frontend-visible changes
- HTTP handlers or planner integration
- PPTX compilation details

## Top-level document shape

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

### Top-level fields

- `version` — required string. Must be `"v1"`.
- `slideSize` — optional string. Defaults to `"16:9"`. No other value is allowed in v1.
- `slides` — required array with at least 1 slide.

## Supported slide layouts

Each slide must have:

- `layout` — one of:
  - `title`
  - `section`
  - `title_bullets`
  - `two_column`
  - `table`
  - `closing`
- `title` — required non-empty string after trimming whitespace

### `title`

Allowed fields:

- `title`
- `subtitle` optional

Not allowed:

- `blocks`
- `columns`

### `section`

Allowed fields:

- `title`
- `subtitle` optional

Not allowed:

- `blocks`
- `columns`

### `title_bullets`

Allowed fields:

- `title`
- `blocks`

Rules:

- `blocks` must contain at least 1 block
- `blocks` may only use `paragraph`, `bullet_list`, `numbered_list`, or `quote`
- `blocks` must contain at least 1 `bullet_list` or `numbered_list`

Not allowed:

- `subtitle`
- `columns`

### `two_column`

Allowed fields:

- `title`
- `columns`

Rules:

- `columns` must contain exactly 2 columns
- each column may have:
  - `heading` optional
  - `blocks` required, at least 1 block
- column blocks may only use `paragraph`, `bullet_list`, `numbered_list`, or `quote`

Not allowed:

- `subtitle`
- top-level slide `blocks`

### `table`

Allowed fields:

- `title`
- `blocks`

Rules:

- `blocks` must contain exactly 1 block
- that block must be a `table` block

Not allowed:

- `subtitle`
- `columns`

### `closing`

Allowed fields:

- `title`
- `subtitle` optional
- `blocks` optional

Rules:

- `blocks`, when present, may only use `paragraph`, `bullet_list`, `numbered_list`, or `quote`

Not allowed:

- `columns`

## Supported content blocks

Every block must contain `type`.

Supported block types:

- `paragraph`
- `bullet_list`
- `numbered_list`
- `table`
- `quote`

### `paragraph`

```json
{
  "type": "paragraph",
  "text": "A short supporting statement."
}
```

Rules:

- `text` is required and must be non-empty after trimming

### `bullet_list`

```json
{
  "type": "bullet_list",
  "items": ["Item one", "Item two"]
}
```

Rules:

- `items` is required
- `items` must contain at least 1 non-empty string

### `numbered_list`

```json
{
  "type": "numbered_list",
  "items": ["First step", "Second step"]
}
```

Rules:

- same rules as `bullet_list`

### `table`

```json
{
  "type": "table",
  "header": ["Metric", "Value"],
  "rows": [
    ["Revenue", "$1.2M"],
    ["Margin", "37%"]
  ]
}
```

Rules:

- `header` is required
- `header` must contain at least 1 non-empty string
- `rows` is required
- `rows` must contain at least 1 row
- each row must have exactly the same number of cells as `header`
- each cell must be non-empty after trimming

### `quote`

```json
{
  "type": "quote",
  "text": "A memorable statement.",
  "attribution": "Customer interview"
}
```

Rules:

- `text` is required and must be non-empty after trimming
- `attribution` is optional

## Normalization and defaulting

Validation and normalization happen together.

The normalization pass must:

- trim leading and trailing whitespace from all strings
- default missing or empty `slideSize` to `"16:9"`
- preserve `version` and require it to be `"v1"`
- return canonical empty Go slices for optional collection fields that are allowed by the selected layout or block type
- leave forbidden fields omitted in the normalized in-memory document so repeated validation/normalization stays valid

JSON serialization is not a separate canonicalization contract in v1.
When marshaled from the Go structs, omitted optional fields may still be omitted from JSON instead of appearing as empty arrays.

Normalization does **not**:

- infer missing slides
- infer a missing title
- rewrite one layout into another
- auto-convert paragraphs into bullet lists
- auto-fill table rows or headers

## Field-by-field validation summary

- Reject unsupported `layout` values
- Reject unsupported `type` values
- Reject `slideSize` values other than `"16:9"`
- Reject `null` for all v1 string and array fields; omit the field instead when it is optional
- Reject layout-specific fields used in the wrong layout, even when they are present as `null`, empty strings, or empty arrays
- Reject empty required strings after trimming
- Reject empty list items and empty table cells
- Reject table rows with inconsistent column counts

## Full example document

```json
{
  "version": "v1",
  "slideSize": "16:9",
  "slides": [
    {
      "layout": "title",
      "title": "Quarterly Business Review",
      "subtitle": "FY2026 Q1"
    },
    {
      "layout": "section",
      "title": "Executive summary",
      "subtitle": "Key messages"
    },
    {
      "layout": "title_bullets",
      "title": "Highlights",
      "blocks": [
        {
          "type": "paragraph",
          "text": "The quarter focused on reliability and launch readiness."
        },
        {
          "type": "bullet_list",
          "items": [
            "Launch readiness improved from 61% to 88%",
            "Median response latency decreased by 23%",
            "Support backlog fell for the third straight month"
          ]
        }
      ]
    },
    {
      "layout": "two_column",
      "title": "Opportunities and risks",
      "columns": [
        {
          "heading": "Opportunities",
          "blocks": [
            {
              "type": "bullet_list",
              "items": [
                "Expand into two adjacent buyer segments",
                "Bundle onboarding services with premium tier"
              ]
            }
          ]
        },
        {
          "heading": "Risks",
          "blocks": [
            {
              "type": "quote",
              "text": "Customers are optimistic, but they expect a smoother rollout.",
              "attribution": "March 2026 customer advisory board"
            }
          ]
        }
      ]
    },
    {
      "layout": "table",
      "title": "KPI snapshot",
      "blocks": [
        {
          "type": "table",
          "header": ["Metric", "Current", "Target"],
          "rows": [
            ["Net revenue retention", "109%", "110%"],
            ["Gross margin", "37%", "35%"],
            ["Critical incidents", "2", "0"]
          ]
        }
      ]
    },
    {
      "layout": "closing",
      "title": "Thank you",
      "subtitle": "Questions and discussion",
      "blocks": [
        {
          "type": "paragraph",
          "text": "Next milestone: board-ready deck compilation after planner approval."
        }
      ]
    }
  ]
}
```

## Implementation guidance

Future planner and compiler implementations should treat this file as the v1 source of truth.

If a future change needs richer layouts or media support, define a new dialect version instead of stretching v1 beyond these constraints.
