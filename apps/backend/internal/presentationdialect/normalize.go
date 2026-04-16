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

var supportedLayouts = []SlideLayout{
	LayoutTitle,
	LayoutSection,
	LayoutTitleBullets,
	LayoutTwoColumn,
	LayoutTable,
	LayoutClosing,
}

var textBlockTypes = []BlockType{
	BlockTypeParagraph,
	BlockTypeBulletList,
	BlockTypeNumberedList,
	BlockTypeQuote,
}

func ParseJSON(data []byte) (Document, error) {
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
		Version:   strings.TrimSpace(document.Version),
		SlideSize: strings.TrimSpace(document.SlideSize),
		Slides:    make([]Slide, 0, len(document.Slides)),
	}

	if normalized.Version == "" {
		return Document{}, validationError("version is required")
	}
	if normalized.Version != VersionV1 {
		return Document{}, validationError("version must be %q", VersionV1)
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

	for index, slide := range document.Slides {
		normalizedSlide, err := normalizeSlide(slide, fmt.Sprintf("slides[%d]", index))
		if err != nil {
			return Document{}, err
		}
		normalized.Slides = append(normalized.Slides, normalizedSlide)
	}

	return normalized, nil
}

func normalizeSlide(slide Slide, path string) (Slide, error) {
	normalized := Slide{
		Layout:  SlideLayout(strings.TrimSpace(string(slide.Layout))),
		Title:   strings.TrimSpace(slide.Title),
		Blocks:  []Block{},
		Columns: []Column{},
	}
	if slide.Subtitle != nil {
		subtitle := strings.TrimSpace(*slide.Subtitle)
		normalized.Subtitle = &subtitle
	}

	if normalized.Layout == "" {
		return Slide{}, validationError("%s.layout is required", path)
	}
	if !slices.Contains(supportedLayouts, normalized.Layout) {
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

		blocks, err := normalizeBlocks(slide.Blocks, path+".blocks", textBlockTypes)
		if err != nil {
			return Slide{}, err
		}
		if len(blocks) == 0 {
			return Slide{}, validationError("%s.blocks must contain at least 1 block", path)
		}

		hasList := false
		for _, block := range blocks {
			if block.Type == BlockTypeBulletList || block.Type == BlockTypeNumberedList {
				hasList = true
				break
			}
		}
		if !hasList {
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
			normalizedColumn, err := normalizeColumn(column, fmt.Sprintf("%s.columns[%d]", path, index), textBlockTypes)
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

		blocks, err := normalizeBlocks(slide.Blocks, path+".blocks", []BlockType{BlockTypeTable})
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

		blocks, err := normalizeBlocks(slide.Blocks, path+".blocks", textBlockTypes)
		if err != nil {
			return Slide{}, err
		}
		normalized.Blocks = blocks
	}

	return normalized, nil
}

func normalizeColumn(column Column, path string, allowedBlockTypes []BlockType) (Column, error) {
	normalized := Column{
		Heading: strings.TrimSpace(column.Heading),
		Blocks:  []Block{},
	}

	blocks, err := normalizeBlocks(column.Blocks, path+".blocks", allowedBlockTypes)
	if err != nil {
		return Column{}, err
	}
	if len(blocks) == 0 {
		return Column{}, validationError("%s.blocks must contain at least 1 block", path)
	}

	normalized.Blocks = blocks
	return normalized, nil
}

func normalizeBlocks(blocks []Block, path string, allowedBlockTypes []BlockType) ([]Block, error) {
	normalized := make([]Block, 0, len(blocks))
	for index, block := range blocks {
		normalizedBlock, err := normalizeBlock(block, fmt.Sprintf("%s[%d]", path, index))
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

func normalizeBlock(block Block, path string) (Block, error) {
	normalized := Block{
		Type:   BlockType(strings.TrimSpace(string(block.Type))),
		Items:  []string{},
		Header: []string{},
		Rows:   [][]string{},
	}
	if block.Text != nil {
		text := strings.TrimSpace(*block.Text)
		normalized.Text = &text
	}
	if block.Attribution != nil {
		attribution := strings.TrimSpace(*block.Attribution)
		normalized.Attribution = &attribution
	}

	if normalized.Type == "" {
		return Block{}, validationError("%s.type is required", path)
	}

	switch normalized.Type {
	case BlockTypeParagraph:
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
	default:
		return Block{}, validationError("%s.type must be one of: paragraph, bullet_list, numbered_list, table, quote", path)
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
	return ValidationError{
		Message: fmt.Sprintf(format, args...),
	}
}

func isNilOrEmpty(value *string) bool {
	return value == nil || *value == ""
}
