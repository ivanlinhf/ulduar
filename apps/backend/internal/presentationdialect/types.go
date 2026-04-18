package presentationdialect

import "strings"

const (
	VersionV1      = "v1"
	VersionV2      = "v2"
	SlideSize16By9 = "16:9"
)

type SlideLayout string

const (
	LayoutTitle             SlideLayout = "title"
	LayoutSection           SlideLayout = "section"
	LayoutTitleBullets      SlideLayout = "title_bullets"
	LayoutTwoColumn         SlideLayout = "two_column"
	LayoutTable             SlideLayout = "table"
	LayoutClosing           SlideLayout = "closing"
	LayoutCoverHero         SlideLayout = "cover_hero"
	LayoutChapterDivider    SlideLayout = "chapter_divider"
	LayoutTOCGrid           SlideLayout = "toc_grid"
	LayoutCardGrid          SlideLayout = "card_grid"
	LayoutComparisonCards   SlideLayout = "comparison_cards"
	LayoutTimelineItinerary SlideLayout = "timeline_itinerary"
	LayoutSummaryMatrix     SlideLayout = "summary_matrix"
	LayoutRecommendation    SlideLayout = "recommendation_split"
)

type BlockType string

const (
	BlockTypeParagraph    BlockType = "paragraph"
	BlockTypeBulletList   BlockType = "bullet_list"
	BlockTypeNumberedList BlockType = "numbered_list"
	BlockTypeTable        BlockType = "table"
	BlockTypeQuote        BlockType = "quote"
	BlockTypeImage        BlockType = "image"
	BlockTypeCard         BlockType = "card"
	BlockTypeStat         BlockType = "stat"
	BlockTypeBadge        BlockType = "badge"
	BlockTypeCallout      BlockType = "callout"
	BlockTypeRichText     BlockType = "rich_text"
)

type Document struct {
	Version       string  `json:"version"`
	SlideSize     string  `json:"slideSize,omitempty"`
	ThemePresetID string  `json:"themePresetId,omitempty"`
	Slides        []Slide `json:"slides"`
}

type Slide struct {
	Layout   SlideLayout `json:"layout"`
	Title    string      `json:"title"`
	Subtitle *string     `json:"subtitle,omitempty"`
	Blocks   []Block     `json:"blocks,omitempty"`
	Columns  []Column    `json:"columns,omitempty"`
}

type Column struct {
	Heading string  `json:"heading,omitempty"`
	Blocks  []Block `json:"blocks"`
}

type Block struct {
	Type        BlockType  `json:"type"`
	Title       *string    `json:"title,omitempty"`
	Text        *string    `json:"text,omitempty"`
	Items       []string   `json:"items,omitempty"`
	Header      []string   `json:"header,omitempty"`
	Rows        [][]string `json:"rows,omitempty"`
	Attribution *string    `json:"attribution,omitempty"`
	AssetRef    *string    `json:"assetRef,omitempty"`
	AltText     *string    `json:"altText,omitempty"`
	Caption     *string    `json:"caption,omitempty"`
	Body        *string    `json:"body,omitempty"`
	Label       *string    `json:"label,omitempty"`
	Value       *string    `json:"value,omitempty"`
	Tone        string     `json:"tone,omitempty"`
	Spans       []TextSpan `json:"spans,omitempty"`
}

type TextSpan struct {
	Text     string `json:"text"`
	Emphasis string `json:"emphasis,omitempty"`
	Lang     string `json:"lang,omitempty"`
}

type ThemePresetMetadata struct {
	ID          string `json:"id"`
	Label       string `json:"label"`
	Description string `json:"description,omitempty"`
	IsDefault   bool   `json:"isDefault,omitempty"`
}

const (
	ThemePresetGeneralClean    = "general_clean"
	ThemePresetTravelEditorial = "travel_editorial"
)

var builtInThemePresets = []ThemePresetMetadata{
	{
		ID:          ThemePresetGeneralClean,
		Label:       "General Clean",
		Description: "Default balanced preset for general-purpose decks.",
		IsDefault:   true,
	},
	{
		ID:          ThemePresetTravelEditorial,
		Label:       "Travel Editorial",
		Description: "Editorial preset for image-led travel and itinerary narratives.",
	},
}

func BuiltInThemePresets() []ThemePresetMetadata {
	presets := make([]ThemePresetMetadata, len(builtInThemePresets))
	copy(presets, builtInThemePresets)
	return presets
}

func ResolveThemePresetID(requested string) string {
	requested = strings.TrimSpace(requested)
	for _, preset := range builtInThemePresets {
		if preset.ID == requested {
			return preset.ID
		}
	}
	return ThemePresetGeneralClean
}
