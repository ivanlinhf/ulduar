package presentationdialect

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"slices"
	"strings"
)

type ValidationError struct {
	Message string
}

func (e ValidationError) Error() string {
	return e.Message
}

var supportedLayoutsV1 = []SlideLayout{
	LayoutTitle,
	LayoutSection,
	LayoutTitleBullets,
	LayoutTwoColumn,
	LayoutTable,
	LayoutClosing,
}

var supportedLayoutsV2 = []SlideLayout{
	LayoutTitle,
	LayoutSection,
	LayoutTitleBullets,
	LayoutTwoColumn,
	LayoutTable,
	LayoutClosing,
	LayoutCoverHero,
	LayoutChapterDivider,
	LayoutTOCGrid,
	LayoutCardGrid,
	LayoutComparisonCards,
	LayoutTimelineItinerary,
	LayoutSummaryMatrix,
	LayoutRecommendation,
}

var (
	v1TextBlockTypes = []BlockType{
		BlockTypeParagraph,
		BlockTypeBulletList,
		BlockTypeNumberedList,
		BlockTypeQuote,
	}
	v2CoverBlockTypes = []BlockType{
		BlockTypeImage,
		BlockTypeBadge,
		BlockTypeRichText,
		BlockTypeCallout,
	}
	v2ChapterDividerBlockTypes = []BlockType{
		BlockTypeImage,
		BlockTypeBadge,
		BlockTypeRichText,
	}
	v2CardOnlyBlockTypes = []BlockType{
		BlockTypeCard,
	}
	v2SummaryMatrixBlockTypes = []BlockType{
		BlockTypeStat,
		BlockTypeTable,
		BlockTypeCallout,
	}
	v2RecommendationBlockTypes = []BlockType{
		BlockTypeImage,
		BlockTypeCallout,
		BlockTypeBadge,
	}
	v2TableBlockTypes = []BlockType{
		BlockTypeTable,
		BlockTypeCallout,
	}
	v2ClosingBlockTypes = []BlockType{
		BlockTypeParagraph,
		BlockTypeRichText,
		BlockTypeCallout,
		BlockTypeBadge,
		BlockTypeImage,
	}
	v2ToneValues = []string{
		"",
		"neutral",
		"accent",
		"success",
		"warning",
	}
	v2SpanEmphasisValues = []string{
		"",
		"strong",
		"emphasis",
		"accent",
	}
)

func ParseJSON(data []byte) (Document, error) {
	if err := rejectNullJSONFields(data); err != nil {
		return Document{}, err
	}

	var document Document

	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&document); err != nil {
		return Document{}, fmt.Errorf("decode presentation document: %w", err)
	}
	if err := decoder.Decode(new(struct{})); err != io.EOF {
		return Document{}, fmt.Errorf("decode presentation document: unexpected trailing content")
	}

	return Normalize(document)
}

func Validate(document Document) error {
	_, err := Normalize(document)
	return err
}

func Normalize(document Document) (Document, error) {
	normalized := Document{
		Version:       strings.TrimSpace(document.Version),
		SlideSize:     strings.TrimSpace(document.SlideSize),
		ThemePresetID: strings.TrimSpace(document.ThemePresetID),
		Slides:        make([]Slide, 0, len(document.Slides)),
	}

	if normalized.Version == "" {
		return Document{}, validationError("version is required")
	}
	if normalized.Version != VersionV1 && normalized.Version != VersionV2 {
		return Document{}, validationError(`version must be "v1" or "v2"`)
	}
	if normalized.SlideSize == "" {
		normalized.SlideSize = SlideSize16By9
	}
	if normalized.SlideSize != SlideSize16By9 {
		return Document{}, validationError("slideSize must be %q", SlideSize16By9)
	}
	if len(document.Slides) == 0 {
		return Document{}, validationError("slides must contain at least 1 slide")
	}

	switch normalized.Version {
	case VersionV1:
		if normalized.ThemePresetID != "" {
			return Document{}, validationError("themePresetId is not supported for %q documents", VersionV1)
		}
		for index, slide := range document.Slides {
			normalizedSlide, err := normalizeSlideV1(slide, fmt.Sprintf("slides[%d]", index))
			if err != nil {
				return Document{}, err
			}
			normalized.Slides = append(normalized.Slides, normalizedSlide)
		}
	case VersionV2:
		normalized.ThemePresetID = ResolveThemePresetID(normalized.ThemePresetID)
		for index, slide := range document.Slides {
			normalizedSlide, err := normalizeSlideV2(slide, fmt.Sprintf("slides[%d]", index))
			if err != nil {
				return Document{}, err
			}
			normalized.Slides = append(normalized.Slides, normalizedSlide)
		}
	}

	return normalized, nil
}

func normalizeSlideV1(slide Slide, path string) (Slide, error) {
	normalized := normalizedSlideBase(slide)

	if normalized.Layout == "" {
		return Slide{}, validationError("%s.layout is required", path)
	}
	if !slices.Contains(supportedLayoutsV1, normalized.Layout) {
		return Slide{}, validationError("%s.layout must be one of: title, section, title_bullets, two_column, table, closing", path)
	}
	if normalized.Title == "" {
		return Slide{}, validationError("%s.title is required", path)
	}

	switch normalized.Layout {
	case LayoutTitle, LayoutSection:
		if slide.Blocks != nil {
			return Slide{}, validationError("%s.blocks is not supported for %q slides", path, normalized.Layout)
		}
		if slide.Columns != nil {
			return Slide{}, validationError("%s.columns is not supported for %q slides", path, normalized.Layout)
		}
	case LayoutTitleBullets:
		if slide.Subtitle != nil {
			return Slide{}, validationError("%s.subtitle is not supported for %q slides", path, normalized.Layout)
		}
		if slide.Columns != nil {
			return Slide{}, validationError("%s.columns is not supported for %q slides", path, normalized.Layout)
		}

		blocks, err := normalizeBlocks(slide.Blocks, path+".blocks", v1TextBlockTypes, VersionV1)
		if err != nil {
			return Slide{}, err
		}
		if len(blocks) == 0 {
			return Slide{}, validationError("%s.blocks must contain at least 1 block", path)
		}
		if !hasAnyBlockType(blocks, BlockTypeBulletList, BlockTypeNumberedList) {
			return Slide{}, validationError("%s.blocks must contain at least 1 bullet_list or numbered_list block", path)
		}
		normalized.Blocks = blocks
	case LayoutTwoColumn:
		if slide.Subtitle != nil {
			return Slide{}, validationError("%s.subtitle is not supported for %q slides", path, normalized.Layout)
		}
		if slide.Blocks != nil {
			return Slide{}, validationError("%s.blocks is not supported for %q slides", path, normalized.Layout)
		}
		if len(slide.Columns) != 2 {
			return Slide{}, validationError("%s.columns must contain exactly 2 columns", path)
		}

		normalized.Columns = make([]Column, 0, len(slide.Columns))
		for index, column := range slide.Columns {
			normalizedColumn, err := normalizeColumn(column, fmt.Sprintf("%s.columns[%d]", path, index), v1TextBlockTypes, VersionV1)
			if err != nil {
				return Slide{}, err
			}
			normalized.Columns = append(normalized.Columns, normalizedColumn)
		}
	case LayoutTable:
		if slide.Subtitle != nil {
			return Slide{}, validationError("%s.subtitle is not supported for %q slides", path, normalized.Layout)
		}
		if slide.Columns != nil {
			return Slide{}, validationError("%s.columns is not supported for %q slides", path, normalized.Layout)
		}

		blocks, err := normalizeBlocks(slide.Blocks, path+".blocks", []BlockType{BlockTypeTable}, VersionV1)
		if err != nil {
			return Slide{}, err
		}
		if len(blocks) != 1 {
			return Slide{}, validationError("%s.blocks must contain exactly 1 table block", path)
		}
		normalized.Blocks = blocks
	case LayoutClosing:
		if slide.Columns != nil {
			return Slide{}, validationError("%s.columns is not supported for %q slides", path, normalized.Layout)
		}

		blocks, err := normalizeBlocks(slide.Blocks, path+".blocks", v1TextBlockTypes, VersionV1)
		if err != nil {
			return Slide{}, err
		}
		normalized.Blocks = blocks
	}

	return normalized, nil
}

func normalizeSlideV2(slide Slide, path string) (Slide, error) {
	normalized := normalizedSlideBase(slide)

	if normalized.Layout == "" {
		return Slide{}, validationError("%s.layout is required", path)
	}
	if !slices.Contains(supportedLayoutsV2, normalized.Layout) {
		return Slide{}, validationError("%s.layout must be one of: title, section, title_bullets, two_column, table, closing, cover_hero, chapter_divider, toc_grid, card_grid, comparison_cards, timeline_itinerary, summary_matrix, recommendation_split", path)
	}
	if normalized.Title == "" {
		return Slide{}, validationError("%s.title is required", path)
	}

	switch normalized.Layout {
	case LayoutTitle, LayoutSection, LayoutTitleBullets, LayoutTwoColumn:
		return normalizeSlideV1(slide, path)
	case LayoutCoverHero:
		if slide.Columns != nil {
			return Slide{}, validationError("%s.columns is not supported for %q slides", path, normalized.Layout)
		}
		blocks, err := normalizeBlocks(slide.Blocks, path+".blocks", v2CoverBlockTypes, VersionV2)
		if err != nil {
			return Slide{}, err
		}
		if len(blocks) == 0 {
			return Slide{}, validationError("%s.blocks must contain at least 1 block", path)
		}
		if len(blocks) > 3 {
			return Slide{}, validationError("%s.blocks must contain at most 3 blocks", path)
		}
		if countBlockType(blocks, BlockTypeImage) != 1 {
			return Slide{}, validationError("%s.blocks must contain exactly 1 image block", path)
		}
		normalized.Blocks = blocks
	case LayoutChapterDivider:
		if slide.Columns != nil {
			return Slide{}, validationError("%s.columns is not supported for %q slides", path, normalized.Layout)
		}
		blocks, err := normalizeBlocks(slide.Blocks, path+".blocks", v2ChapterDividerBlockTypes, VersionV2)
		if err != nil {
			return Slide{}, err
		}
		if len(blocks) > 2 {
			return Slide{}, validationError("%s.blocks must contain at most 2 blocks", path)
		}
		if countBlockType(blocks, BlockTypeImage) > 1 {
			return Slide{}, validationError("%s.blocks must contain at most 1 image block", path)
		}
		normalized.Blocks = blocks
	case LayoutTOCGrid:
		if slide.Columns != nil {
			return Slide{}, validationError("%s.columns is not supported for %q slides", path, normalized.Layout)
		}
		blocks, err := normalizeBlocks(slide.Blocks, path+".blocks", v2CardOnlyBlockTypes, VersionV2)
		if err != nil {
			return Slide{}, err
		}
		if len(blocks) < 2 || len(blocks) > 8 {
			return Slide{}, validationError("%s.blocks must contain between 2 and 8 card blocks", path)
		}
		normalized.Blocks = blocks
	case LayoutCardGrid:
		if slide.Columns != nil {
			return Slide{}, validationError("%s.columns is not supported for %q slides", path, normalized.Layout)
		}
		blocks, err := normalizeBlocks(slide.Blocks, path+".blocks", v2CardOnlyBlockTypes, VersionV2)
		if err != nil {
			return Slide{}, err
		}
		if len(blocks) < 2 || len(blocks) > 6 {
			return Slide{}, validationError("%s.blocks must contain between 2 and 6 card blocks", path)
		}
		normalized.Blocks = blocks
	case LayoutComparisonCards:
		if slide.Columns != nil {
			return Slide{}, validationError("%s.columns is not supported for %q slides", path, normalized.Layout)
		}
		blocks, err := normalizeBlocks(slide.Blocks, path+".blocks", v2CardOnlyBlockTypes, VersionV2)
		if err != nil {
			return Slide{}, err
		}
		if len(blocks) != 2 {
			return Slide{}, validationError("%s.blocks must contain exactly 2 card blocks", path)
		}
		normalized.Blocks = blocks
	case LayoutTimelineItinerary:
		if slide.Columns != nil {
			return Slide{}, validationError("%s.columns is not supported for %q slides", path, normalized.Layout)
		}
		blocks, err := normalizeBlocks(slide.Blocks, path+".blocks", v2CardOnlyBlockTypes, VersionV2)
		if err != nil {
			return Slide{}, err
		}
		if len(blocks) < 2 || len(blocks) > 6 {
			return Slide{}, validationError("%s.blocks must contain between 2 and 6 card blocks", path)
		}
		normalized.Blocks = blocks
	case LayoutSummaryMatrix:
		if slide.Columns != nil {
			return Slide{}, validationError("%s.columns is not supported for %q slides", path, normalized.Layout)
		}
		blocks, err := normalizeBlocks(slide.Blocks, path+".blocks", v2SummaryMatrixBlockTypes, VersionV2)
		if err != nil {
			return Slide{}, err
		}
		if len(blocks) < 2 || len(blocks) > 6 {
			return Slide{}, validationError("%s.blocks must contain between 2 and 6 blocks", path)
		}
		if countBlockType(blocks, BlockTypeTable) != 1 {
			return Slide{}, validationError("%s.blocks must contain exactly 1 table block", path)
		}
		if countBlockType(blocks, BlockTypeStat) < 1 {
			return Slide{}, validationError("%s.blocks must contain at least 1 stat block", path)
		}
		normalized.Blocks = blocks
	case LayoutRecommendation:
		if slide.Columns != nil {
			return Slide{}, validationError("%s.columns is not supported for %q slides", path, normalized.Layout)
		}
		blocks, err := normalizeBlocks(slide.Blocks, path+".blocks", v2RecommendationBlockTypes, VersionV2)
		if err != nil {
			return Slide{}, err
		}
		if len(blocks) < 2 || len(blocks) > 3 {
			return Slide{}, validationError("%s.blocks must contain between 2 and 3 blocks", path)
		}
		if countBlockType(blocks, BlockTypeImage) != 1 {
			return Slide{}, validationError("%s.blocks must contain exactly 1 image block", path)
		}
		if countBlockType(blocks, BlockTypeCallout) != 1 {
			return Slide{}, validationError("%s.blocks must contain exactly 1 callout block", path)
		}
		normalized.Blocks = blocks
	case LayoutTable:
		if slide.Columns != nil {
			return Slide{}, validationError("%s.columns is not supported for %q slides", path, normalized.Layout)
		}
		blocks, err := normalizeBlocks(slide.Blocks, path+".blocks", v2TableBlockTypes, VersionV2)
		if err != nil {
			return Slide{}, err
		}
		if len(blocks) < 1 || len(blocks) > 2 {
			return Slide{}, validationError("%s.blocks must contain between 1 and 2 blocks", path)
		}
		if countBlockType(blocks, BlockTypeTable) != 1 {
			return Slide{}, validationError("%s.blocks must contain exactly 1 table block", path)
		}
		normalized.Blocks = blocks
	case LayoutClosing:
		if slide.Columns != nil {
			return Slide{}, validationError("%s.columns is not supported for %q slides", path, normalized.Layout)
		}
		blocks, err := normalizeBlocks(slide.Blocks, path+".blocks", v2ClosingBlockTypes, VersionV2)
		if err != nil {
			return Slide{}, err
		}
		if len(blocks) > 3 {
			return Slide{}, validationError("%s.blocks must contain at most 3 blocks", path)
		}
		normalized.Blocks = blocks
	}

	return normalized, nil
}

func normalizedSlideBase(slide Slide) Slide {
	normalized := Slide{
		Layout: SlideLayout(strings.TrimSpace(string(slide.Layout))),
		Title:  strings.TrimSpace(slide.Title),
	}
	if slide.Subtitle != nil {
		subtitle := strings.TrimSpace(*slide.Subtitle)
		normalized.Subtitle = &subtitle
	}
	return normalized
}

func normalizeColumn(column Column, path string, allowedBlockTypes []BlockType, version string) (Column, error) {
	normalized := Column{
		Heading: strings.TrimSpace(column.Heading),
	}

	blocks, err := normalizeBlocks(column.Blocks, path+".blocks", allowedBlockTypes, version)
	if err != nil {
		return Column{}, err
	}
	if len(blocks) == 0 {
		return Column{}, validationError("%s.blocks must contain at least 1 block", path)
	}

	normalized.Blocks = blocks
	return normalized, nil
}

func normalizeBlocks(blocks []Block, path string, allowedBlockTypes []BlockType, version string) ([]Block, error) {
	normalized := make([]Block, 0, len(blocks))
	for index, block := range blocks {
		normalizedBlock, err := normalizeBlock(block, fmt.Sprintf("%s[%d]", path, index), version)
		if err != nil {
			return nil, err
		}
		if !slices.Contains(allowedBlockTypes, normalizedBlock.Type) {
			return nil, validationError("%s.type %q is not supported in this layout", fmt.Sprintf("%s[%d]", path, index), normalizedBlock.Type)
		}
		normalized = append(normalized, normalizedBlock)
	}
	return normalized, nil
}

func normalizeBlock(block Block, path string, version string) (Block, error) {
	normalized := Block{
		Type: BlockType(strings.TrimSpace(string(block.Type))),
		Tone: strings.TrimSpace(block.Tone),
	}
	for _, field := range []struct {
		src *string
		dst **string
	}{
		{src: block.Title, dst: &normalized.Title},
		{src: block.Text, dst: &normalized.Text},
		{src: block.Attribution, dst: &normalized.Attribution},
		{src: block.AssetRef, dst: &normalized.AssetRef},
		{src: block.AltText, dst: &normalized.AltText},
		{src: block.Caption, dst: &normalized.Caption},
		{src: block.Body, dst: &normalized.Body},
		{src: block.Label, dst: &normalized.Label},
		{src: block.Value, dst: &normalized.Value},
	} {
		if field.src != nil {
			value := strings.TrimSpace(*field.src)
			*field.dst = &value
		}
	}
	if len(block.Items) > 0 {
		items, err := normalizeStringSlice(block.Items, path+".items", true)
		if err != nil {
			return Block{}, err
		}
		normalized.Items = items
	}
	if len(block.Header) > 0 {
		header, err := normalizeStringSlice(block.Header, path+".header", true)
		if err != nil {
			return Block{}, err
		}
		normalized.Header = header
	}
	if len(block.Rows) > 0 {
		rows, err := normalizeRows(block.Rows, path+".rows", len(block.Header))
		if err != nil {
			return Block{}, err
		}
		normalized.Rows = rows
	}
	if len(block.Spans) > 0 {
		normalized.Spans = make([]TextSpan, 0, len(block.Spans))
		for index, span := range block.Spans {
			normalizedSpan, err := normalizeTextSpan(span, fmt.Sprintf("%s.spans[%d]", path, index))
			if err != nil {
				return Block{}, err
			}
			normalized.Spans = append(normalized.Spans, normalizedSpan)
		}
	}

	if normalized.Type == "" {
		return Block{}, validationError("%s.type is required", path)
	}

	switch normalized.Type {
	case BlockTypeParagraph:
		if version == VersionV2 && hasV2OnlyFields(block) {
			return Block{}, validationError("%s contains fields that are not supported for paragraph blocks", path)
		}
		if isNilOrEmpty(normalized.Text) {
			return Block{}, validationError("%s.text is required for paragraph blocks", path)
		}
		if block.Items != nil {
			return Block{}, validationError("%s.items is not supported for paragraph blocks", path)
		}
		if block.Header != nil || block.Rows != nil {
			return Block{}, validationError("%s.header and %s.rows are only supported for table blocks", path, path)
		}
		if block.Attribution != nil {
			return Block{}, validationError("%s.attribution is only supported for quote blocks", path)
		}
	case BlockTypeQuote:
		if version == VersionV2 && hasV2OnlyFields(block) {
			return Block{}, validationError("%s contains fields that are not supported for quote blocks", path)
		}
		if isNilOrEmpty(normalized.Text) {
			return Block{}, validationError("%s.text is required for quote blocks", path)
		}
		if block.Items != nil {
			return Block{}, validationError("%s.items is not supported for quote blocks", path)
		}
		if block.Header != nil || block.Rows != nil {
			return Block{}, validationError("%s.header and %s.rows are only supported for table blocks", path, path)
		}
	case BlockTypeBulletList, BlockTypeNumberedList:
		if version == VersionV2 && hasV2OnlyFields(block) {
			return Block{}, validationError("%s contains fields that are not supported for %q blocks", path, normalized.Type)
		}
		if block.Text != nil {
			return Block{}, validationError("%s.text is not supported for %q blocks", path, normalized.Type)
		}
		if block.Attribution != nil {
			return Block{}, validationError("%s.attribution is only supported for quote blocks", path)
		}
		if block.Header != nil || block.Rows != nil {
			return Block{}, validationError("%s.header and %s.rows are only supported for table blocks", path, path)
		}
		items, err := normalizeStringSlice(block.Items, path+".items", true)
		if err != nil {
			return Block{}, err
		}
		if len(items) == 0 {
			return Block{}, validationError("%s.items must contain at least 1 item", path)
		}
		normalized.Items = items
	case BlockTypeTable:
		if version == VersionV2 && (block.Title != nil || block.AssetRef != nil || block.AltText != nil || block.Body != nil || block.Label != nil || block.Value != nil || block.Spans != nil || block.Tone != "") {
			return Block{}, validationError("%s contains fields that are not supported for table blocks", path)
		}
		if block.Text != nil {
			return Block{}, validationError("%s.text is not supported for table blocks", path)
		}
		if block.Attribution != nil {
			return Block{}, validationError("%s.attribution is only supported for quote blocks", path)
		}
		if block.Items != nil {
			return Block{}, validationError("%s.items is only supported for bullet_list and numbered_list blocks", path)
		}

		header, err := normalizeStringSlice(block.Header, path+".header", true)
		if err != nil {
			return Block{}, err
		}
		if len(header) == 0 {
			return Block{}, validationError("%s.header must contain at least 1 column", path)
		}

		rows, err := normalizeRows(block.Rows, path+".rows", len(header))
		if err != nil {
			return Block{}, err
		}
		if len(rows) == 0 {
			return Block{}, validationError("%s.rows must contain at least 1 row", path)
		}

		normalized.Header = header
		normalized.Rows = rows
	case BlockTypeImage:
		if version != VersionV2 {
			return Block{}, validationError("%s.type must be one of: paragraph, bullet_list, numbered_list, table, quote", path)
		}
		if isNilOrEmpty(normalized.AssetRef) {
			return Block{}, validationError("%s.assetRef is required for image blocks", path)
		}
		if block.Text != nil || block.Items != nil || block.Header != nil || block.Rows != nil || block.Attribution != nil || block.Body != nil || block.Label != nil || block.Value != nil || block.Spans != nil || block.Tone != "" || block.Title != nil {
			return Block{}, validationError("%s contains fields that are not supported for image blocks", path)
		}
	case BlockTypeCard:
		if version != VersionV2 {
			return Block{}, validationError("%s.type must be one of: paragraph, bullet_list, numbered_list, table, quote", path)
		}
		if isNilOrEmpty(normalized.Title) {
			return Block{}, validationError("%s.title is required for card blocks", path)
		}
		if block.Text != nil || block.Items != nil || block.Header != nil || block.Rows != nil || block.Attribution != nil || block.Value != nil || block.Spans != nil || block.Tone != "" {
			return Block{}, validationError("%s contains fields that are not supported for card blocks", path)
		}
	case BlockTypeStat:
		if version != VersionV2 {
			return Block{}, validationError("%s.type must be one of: paragraph, bullet_list, numbered_list, table, quote", path)
		}
		if isNilOrEmpty(normalized.Value) {
			return Block{}, validationError("%s.value is required for stat blocks", path)
		}
		if isNilOrEmpty(normalized.Label) {
			return Block{}, validationError("%s.label is required for stat blocks", path)
		}
		if block.Title != nil || block.Text != nil || block.Items != nil || block.Header != nil || block.Rows != nil || block.Attribution != nil || block.AssetRef != nil || block.AltText != nil || block.Caption != nil || block.Spans != nil || block.Tone != "" {
			return Block{}, validationError("%s contains fields that are not supported for stat blocks", path)
		}
	case BlockTypeBadge:
		if version != VersionV2 {
			return Block{}, validationError("%s.type must be one of: paragraph, bullet_list, numbered_list, table, quote", path)
		}
		if isNilOrEmpty(normalized.Text) {
			return Block{}, validationError("%s.text is required for badge blocks", path)
		}
		if !slices.Contains(v2ToneValues, normalized.Tone) {
			return Block{}, validationError(`%s.tone must be one of: neutral, accent, success, warning`, path)
		}
		if block.Title != nil || block.Items != nil || block.Header != nil || block.Rows != nil || block.Attribution != nil || block.AssetRef != nil || block.AltText != nil || block.Caption != nil || block.Body != nil || block.Label != nil || block.Value != nil || block.Spans != nil {
			return Block{}, validationError("%s contains fields that are not supported for badge blocks", path)
		}
	case BlockTypeCallout:
		if version != VersionV2 {
			return Block{}, validationError("%s.type must be one of: paragraph, bullet_list, numbered_list, table, quote", path)
		}
		if isNilOrEmpty(normalized.Title) {
			return Block{}, validationError("%s.title is required for callout blocks", path)
		}
		if isNilOrEmpty(normalized.Body) {
			return Block{}, validationError("%s.body is required for callout blocks", path)
		}
		if block.Text != nil || block.Items != nil || block.Header != nil || block.Rows != nil || block.Attribution != nil || block.AssetRef != nil || block.AltText != nil || block.Label != nil || block.Value != nil || block.Spans != nil || block.Tone != "" {
			return Block{}, validationError("%s contains fields that are not supported for callout blocks", path)
		}
	case BlockTypeRichText:
		if version != VersionV2 {
			return Block{}, validationError("%s.type must be one of: paragraph, bullet_list, numbered_list, table, quote", path)
		}
		if len(normalized.Spans) == 0 {
			return Block{}, validationError("%s.spans must contain at least 1 span", path)
		}
		if block.Title != nil || block.Text != nil || block.Items != nil || block.Header != nil || block.Rows != nil || block.Attribution != nil || block.AssetRef != nil || block.AltText != nil || block.Caption != nil || block.Body != nil || block.Label != nil || block.Value != nil || block.Tone != "" {
			return Block{}, validationError("%s contains fields that are not supported for rich_text blocks", path)
		}
	default:
		if version == VersionV1 {
			return Block{}, validationError("%s.type must be one of: paragraph, bullet_list, numbered_list, table, quote", path)
		}
		return Block{}, validationError("%s.type must be one of: paragraph, bullet_list, numbered_list, table, quote, image, card, stat, badge, callout, rich_text", path)
	}

	return normalized, nil
}

func normalizeTextSpan(span TextSpan, path string) (TextSpan, error) {
	normalized := TextSpan{
		Text:     strings.TrimSpace(span.Text),
		Emphasis: strings.TrimSpace(span.Emphasis),
		Lang:     strings.TrimSpace(span.Lang),
	}
	if normalized.Text == "" {
		return TextSpan{}, validationError("%s.text is required", path)
	}
	if !slices.Contains(v2SpanEmphasisValues, normalized.Emphasis) {
		return TextSpan{}, validationError(`%s.emphasis must be one of: strong, emphasis, accent`, path)
	}
	return normalized, nil
}

func normalizeStringSlice(values []string, path string, rejectEmpty bool) ([]string, error) {
	normalized := make([]string, 0, len(values))
	for index, value := range values {
		trimmed := strings.TrimSpace(value)
		if rejectEmpty && trimmed == "" {
			return nil, validationError("%s[%d] must not be empty", path, index)
		}
		normalized = append(normalized, trimmed)
	}
	return normalized, nil
}

func normalizeRows(rows [][]string, path string, width int) ([][]string, error) {
	normalized := make([][]string, 0, len(rows))
	for rowIndex, row := range rows {
		if len(row) != width {
			return nil, validationError("%s[%d] must contain exactly %d cells", path, rowIndex, width)
		}

		normalizedRow := make([]string, 0, len(row))
		for columnIndex, cell := range row {
			trimmed := strings.TrimSpace(cell)
			if trimmed == "" {
				return nil, validationError("%s[%d][%d] must not be empty", path, rowIndex, columnIndex)
			}
			normalizedRow = append(normalizedRow, trimmed)
		}
		normalized = append(normalized, normalizedRow)
	}

	return normalized, nil
}

func validationError(format string, args ...any) error {
	return ValidationError{Message: fmt.Sprintf(format, args...)}
}

func isNilOrEmpty(value *string) bool {
	return value == nil || *value == ""
}

func hasAnyBlockType(blocks []Block, blockTypes ...BlockType) bool {
	for _, block := range blocks {
		if slices.Contains(blockTypes, block.Type) {
			return true
		}
	}
	return false
}

func countBlockType(blocks []Block, blockType BlockType) int {
	count := 0
	for _, block := range blocks {
		if block.Type == blockType {
			count++
		}
	}
	return count
}

func hasV2OnlyFields(block Block) bool {
	return block.Title != nil ||
		block.AssetRef != nil ||
		block.AltText != nil ||
		block.Caption != nil ||
		block.Body != nil ||
		block.Label != nil ||
		block.Value != nil ||
		block.Spans != nil ||
		block.Tone != ""
}

func rejectNullJSONFields(data []byte) error {
	var value any
	if err := json.Unmarshal(data, &value); err != nil {
		return fmt.Errorf("decode presentation document: %w", err)
	}
	return rejectNullDocumentFields(value)
}

func rejectNullDocumentFields(value any) error {
	document, err := requireObject(value, "document")
	if err != nil {
		return err
	}

	for _, field := range []string{"version", "slideSize", "themePresetId"} {
		if err := rejectNullObjectField(document, field, field); err != nil {
			return err
		}
	}

	slides, ok, err := rejectNullArrayField(document, "slides", "slides")
	if err != nil || !ok {
		return err
	}
	for index, slide := range slides {
		if slide == nil {
			return validationError("slides[%d] must not be null", index)
		}
		if err := rejectNullSlideFields(slide, fmt.Sprintf("slides[%d]", index)); err != nil {
			return err
		}
	}
	return nil
}

func rejectNullSlideFields(value any, path string) error {
	slide, err := requireObject(value, path)
	if err != nil {
		return err
	}

	for _, field := range []string{"layout", "title", "subtitle"} {
		if err := rejectNullObjectField(slide, field, path+"."+field); err != nil {
			return err
		}
	}

	blocks, ok, err := rejectNullArrayField(slide, "blocks", path+".blocks")
	if err != nil {
		return err
	}
	if ok {
		for index, block := range blocks {
			if block == nil {
				return validationError("%s[%d] must not be null", path+".blocks", index)
			}
			if err := rejectNullBlockFields(block, fmt.Sprintf("%s[%d]", path+".blocks", index)); err != nil {
				return err
			}
		}
	}

	columns, ok, err := rejectNullArrayField(slide, "columns", path+".columns")
	if err != nil {
		return err
	}
	if ok {
		for index, column := range columns {
			if column == nil {
				return validationError("%s[%d] must not be null", path+".columns", index)
			}
			if err := rejectNullColumnFields(column, fmt.Sprintf("%s[%d]", path+".columns", index)); err != nil {
				return err
			}
		}
	}

	return nil
}

func rejectNullColumnFields(value any, path string) error {
	column, err := requireObject(value, path)
	if err != nil {
		return err
	}
	if err := rejectNullObjectField(column, "heading", path+".heading"); err != nil {
		return err
	}

	blocks, ok, err := rejectNullArrayField(column, "blocks", path+".blocks")
	if err != nil {
		return err
	}
	if ok {
		for index, block := range blocks {
			if block == nil {
				return validationError("%s[%d] must not be null", path+".blocks", index)
			}
			if err := rejectNullBlockFields(block, fmt.Sprintf("%s[%d]", path+".blocks", index)); err != nil {
				return err
			}
		}
	}
	return nil
}

func rejectNullBlockFields(value any, path string) error {
	block, err := requireObject(value, path)
	if err != nil {
		return err
	}

	for _, field := range []string{"type", "title", "text", "attribution", "assetRef", "altText", "caption", "body", "label", "value", "tone"} {
		if err := rejectNullObjectField(block, field, path+"."+field); err != nil {
			return err
		}
	}

	for _, field := range []string{"items", "header"} {
		values, ok, err := rejectNullArrayField(block, field, path+"."+field)
		if err != nil {
			return err
		}
		if !ok {
			continue
		}
		for index, item := range values {
			if item == nil {
				return validationError("%s[%d] must not be null", path+"."+field, index)
			}
		}
	}

	spans, ok, err := rejectNullArrayField(block, "spans", path+".spans")
	if err != nil {
		return err
	}
	if ok {
		for index, span := range spans {
			if span == nil {
				return validationError("%s[%d] must not be null", path+".spans", index)
			}
			if err := rejectNullSpanFields(span, fmt.Sprintf("%s[%d]", path+".spans", index)); err != nil {
				return err
			}
		}
	}

	rows, ok, err := rejectNullArrayField(block, "rows", path+".rows")
	if err != nil {
		return err
	}
	if !ok {
		return nil
	}
	for rowIndex, row := range rows {
		if row == nil {
			return validationError("%s[%d] must not be null", path+".rows", rowIndex)
		}
		cells, ok := row.([]any)
		if !ok {
			continue
		}
		for columnIndex, cell := range cells {
			if cell == nil {
				return validationError("%s[%d][%d] must not be null", path+".rows", rowIndex, columnIndex)
			}
		}
	}
	return nil
}

func rejectNullSpanFields(value any, path string) error {
	span, err := requireObject(value, path)
	if err != nil {
		return err
	}
	for _, field := range []string{"text", "emphasis", "lang"} {
		if err := rejectNullObjectField(span, field, path+"."+field); err != nil {
			return err
		}
	}
	return nil
}

func rejectNullObjectField(values map[string]any, key string, path string) error {
	value, ok := values[key]
	if ok && value == nil {
		return validationError("%s must not be null", path)
	}
	return nil
}

func rejectNullArrayField(values map[string]any, key string, path string) ([]any, bool, error) {
	value, ok := values[key]
	if !ok {
		return nil, false, nil
	}
	if value == nil {
		return nil, true, validationError("%s must not be null", path)
	}
	array, ok := value.([]any)
	if !ok {
		return nil, true, nil
	}
	return array, true, nil
}

func requireObject(value any, path string) (map[string]any, error) {
	object, ok := value.(map[string]any)
	if ok {
		return object, nil
	}
	return nil, validationError("%s must be a JSON object", path)
}
