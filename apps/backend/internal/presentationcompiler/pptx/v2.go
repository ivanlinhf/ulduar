package pptx

import (
	"archive/zip"
	"bytes"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/ivanlin/ulduar/apps/backend/internal/presentationdialect"
)

type CompileAsset struct {
	Filename  string
	MediaType string
	Data      []byte
}

type themeFonts struct {
	Latin string
	CJK   string
}

type themePreset struct {
	ID          string
	Name        string
	Background  string
	Surface     string
	SurfaceAlt  string
	Text        string
	Muted       string
	Accent      string
	AccentAlt   string
	Success     string
	Warning     string
	InverseText string
	Outline     string
	Fonts       themeFonts
}

type mediaPart struct {
	partName    string
	target      string
	contentType string
	data        []byte
}

type compiledSlide struct {
	xml  string
	rels string
}

type slideRelationship struct {
	id      string
	relType string
	target  string
}

type slideBuilder struct {
	pkg             *v2PackageBuilder
	slide           presentationdialect.Slide
	preset          themePreset
	elements        []string
	rels            []slideRelationship
	relIDByAssetRef map[string]string
	nextID          int
}

type v2PackageBuilder struct {
	document         presentationdialect.Document
	preset           themePreset
	assets           map[string]CompileAsset
	mediaByAssetRef  map[string]mediaPart
	mediaOrder       []string
	imageConfigByRef map[string]image.Config
}

type pictureCrop struct {
	left   int
	top    int
	right  int
	bottom int
}

func compileV2(document presentationdialect.Document, assets map[string]CompileAsset) ([]byte, error) {
	builder := &v2PackageBuilder{
		document:         document,
		preset:           resolveThemePreset(dereferenceString(document.ThemePresetID)),
		assets:           cloneCompileAssets(assets),
		mediaByAssetRef:  make(map[string]mediaPart),
		imageConfigByRef: make(map[string]image.Config),
	}

	slides := make([]compiledSlide, 0, len(document.Slides))
	for _, slide := range document.Slides {
		compiled, err := builder.buildSlide(slide)
		if err != nil {
			return nil, err
		}
		slides = append(slides, compiled)
	}

	entries := make([]zipEntry, 0, 11+len(slides)*2+len(builder.mediaOrder))
	entries = append(entries,
		zipEntry{name: "[Content_Types].xml", data: []byte(buildContentTypesXMLV2(len(slides), builder.mediaEntries()))},
		zipEntry{name: "_rels/.rels", data: []byte(rootRelationshipsXML)},
		zipEntry{name: "docProps/app.xml", data: []byte(buildAppPropertiesXML(document))},
		zipEntry{name: "docProps/core.xml", data: []byte(corePropertiesXML)},
		zipEntry{name: "ppt/presentation.xml", data: []byte(buildPresentationXML(document))},
		zipEntry{name: "ppt/_rels/presentation.xml.rels", data: []byte(buildPresentationRelationshipsXML(len(slides)))},
		zipEntry{name: "ppt/slideMasters/slideMaster1.xml", data: []byte(buildSlideMasterXMLV2(builder.preset))},
		zipEntry{name: "ppt/slideMasters/_rels/slideMaster1.xml.rels", data: []byte(slideMasterRelationshipsXML)},
		zipEntry{name: "ppt/slideLayouts/slideLayout1.xml", data: []byte(slideLayoutXML)},
		zipEntry{name: "ppt/slideLayouts/_rels/slideLayout1.xml.rels", data: []byte(slideLayoutRelationshipsXML)},
		zipEntry{name: "ppt/theme/theme1.xml", data: []byte(buildThemeXMLV2(builder.preset))},
	)
	for index, slide := range slides {
		slideNumber := index + 1
		entries = append(entries,
			zipEntry{name: fmt.Sprintf("ppt/slides/slide%d.xml", slideNumber), data: []byte(slide.xml)},
			zipEntry{name: fmt.Sprintf("ppt/slides/_rels/slide%d.xml.rels", slideNumber), data: []byte(slide.rels)},
		)
	}
	for _, part := range builder.mediaEntries() {
		entries = append(entries, zipEntry{name: part.partName, data: part.data})
	}

	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	for _, entry := range entries {
		header := &zip.FileHeader{
			Name:     entry.name,
			Method:   zip.Deflate,
			Modified: zipModifiedTime,
		}
		fileWriter, err := writer.CreateHeader(header)
		if err != nil {
			_ = writer.Close()
			return nil, fmt.Errorf("create pptx entry %q: %w", entry.name, err)
		}
		if _, err := fileWriter.Write(entry.data); err != nil {
			_ = writer.Close()
			return nil, fmt.Errorf("write pptx entry %q: %w", entry.name, err)
		}
	}
	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("close pptx archive: %w", err)
	}

	return buffer.Bytes(), nil
}

func (b *v2PackageBuilder) buildSlide(slide presentationdialect.Slide) (compiledSlide, error) {
	sb := slideBuilder{
		pkg:             b,
		slide:           slide,
		preset:          b.preset,
		elements:        make([]string, 0, 16),
		rels:            make([]slideRelationship, 0, 4),
		relIDByAssetRef: make(map[string]string),
		nextID:          2,
	}

	sb.addBox(textBox{
		name:      "Background",
		x:         0,
		y:         0,
		cx:        slideWidthEMU,
		cy:        slideHeightEMU,
		fillColor: b.preset.Background,
		lineColor: b.preset.Background,
	})

	var err error
	switch slide.Layout {
	case presentationdialect.LayoutTitle, presentationdialect.LayoutSection, presentationdialect.LayoutTitleBullets, presentationdialect.LayoutTwoColumn, presentationdialect.LayoutClosing:
		sb.addLegacyTextBoxes(slideTextBoxesWithAccent(slide, b.preset.Accent))
	case presentationdialect.LayoutTable:
		err = sb.renderTableLayout(slide)
	case presentationdialect.LayoutCoverHero:
		err = sb.renderCoverHero(slide)
	case presentationdialect.LayoutChapterDivider:
		err = sb.renderChapterDivider(slide)
	case presentationdialect.LayoutTOCGrid, presentationdialect.LayoutCardGrid:
		err = sb.renderCardGrid(slide)
	case presentationdialect.LayoutComparisonCards:
		err = sb.renderComparisonCards(slide)
	case presentationdialect.LayoutTimelineItinerary:
		err = sb.renderTimeline(slide)
	case presentationdialect.LayoutSummaryMatrix:
		err = sb.renderSummaryMatrix(slide)
	case presentationdialect.LayoutRecommendation:
		err = sb.renderRecommendation(slide)
	default:
		sb.addLegacyTextBoxes(semanticContentSlideTextBoxes(slide, false, b.preset.Accent))
	}
	if err != nil {
		return compiledSlide{}, err
	}

	var slideXML strings.Builder
	slideXML.WriteString(xmlHeader)
	slideXML.WriteString(`<p:sld xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main">`)
	slideXML.WriteString(`<p:cSld name="`)
	slideXML.WriteString(escapeXML(slide.Title))
	slideXML.WriteString(`"><p:spTree>`)
	slideXML.WriteString(groupShapeXML)
	for _, element := range sb.elements {
		slideXML.WriteString(element)
	}
	slideXML.WriteString(`</p:spTree></p:cSld><p:clrMapOvr><a:masterClrMapping/></p:clrMapOvr></p:sld>`)

	return compiledSlide{
		xml:  slideXML.String(),
		rels: buildSlideRelationshipsXML(sb.rels),
	}, nil
}

func (b *v2PackageBuilder) mediaEntries() []mediaPart {
	entries := make([]mediaPart, 0, len(b.mediaOrder))
	for _, assetRef := range b.mediaOrder {
		entries = append(entries, b.mediaByAssetRef[assetRef])
	}
	return entries
}

func (b *v2PackageBuilder) ensureMediaPart(assetRef string) (mediaPart, error) {
	if part, ok := b.mediaByAssetRef[assetRef]; ok {
		return part, nil
	}

	asset, ok := b.assets[assetRef]
	if !ok || len(asset.Data) == 0 {
		return mediaPart{}, fmt.Errorf("missing image asset %q", assetRef)
	}

	ext, contentType, err := mediaExtension(asset)
	if err != nil {
		return mediaPart{}, err
	}
	index := len(b.mediaOrder) + 1
	part := mediaPart{
		partName:    fmt.Sprintf("ppt/media/image%d.%s", index, ext),
		target:      fmt.Sprintf("../media/image%d.%s", index, ext),
		contentType: contentType,
		data:        asset.Data,
	}
	b.mediaByAssetRef[assetRef] = part
	b.mediaOrder = append(b.mediaOrder, assetRef)
	return part, nil
}

func (b *v2PackageBuilder) hasAsset(assetRef string) bool {
	asset, ok := b.assets[assetRef]
	return ok && len(asset.Data) > 0
}

func (b *slideBuilder) addLegacyTextBoxes(boxes []textBox) {
	for _, box := range boxes {
		b.addBox(box)
	}
}

func (b *slideBuilder) addBox(box textBox) {
	if box.id == 0 {
		box.id = b.nextShapeID()
	}
	b.elements = append(b.elements, renderTextBoxWithFonts(box, b.preset.Fonts))
}

func (b *slideBuilder) nextShapeID() int {
	id := b.nextID
	b.nextID++
	return id
}

func (b *slideBuilder) addPicture(assetRef string, x, y, cx, cy int, description string, cropToFill bool) error {
	part, err := b.pkg.ensureMediaPart(assetRef)
	if err != nil {
		return err
	}
	relID, ok := b.relIDByAssetRef[assetRef]
	if !ok {
		relID = fmt.Sprintf("rId%d", len(b.rels)+2)
		b.relIDByAssetRef[assetRef] = relID
		b.rels = append(b.rels, slideRelationship{
			id:      relID,
			relType: "http://schemas.openxmlformats.org/officeDocument/2006/relationships/image",
			target:  part.target,
		})
	}
	var crop pictureCrop
	if cropToFill {
		crop = b.pkg.imageCropToFill(assetRef, cx, cy)
	}
	b.elements = append(b.elements, renderPictureXML(b.nextShapeID(), description, relID, x, y, cx, cy, crop))
	return nil
}

func (b *slideBuilder) addTable(x, y, cx, cy int, header []string, rows [][]string, headerFill string) {
	b.elements = append(b.elements, renderTableXML(b.nextShapeID(), x, y, cx, cy, header, rows, headerFill, b.preset))
}

func (b *slideBuilder) renderCoverHero(slide presentationdialect.Slide) error {
	imageBlock := firstBlockOfType(slide.Blocks, presentationdialect.BlockTypeImage)
	hasImage := imageBlock != nil && imageBlock.AssetRef != nil && b.pkg.hasAsset(dereferenceString(imageBlock.AssetRef))
	if hasImage {
		if err := b.addPicture(dereferenceString(imageBlock.AssetRef), 0, 0, slideWidthEMU, slideHeightEMU, blockAssetDescription(imageBlock, slide.Title), true); err != nil {
			return err
		}
		b.addBox(textBox{
			name:      "Overlay",
			x:         0,
			y:         0,
			cx:        slideWidthEMU,
			cy:        slideHeightEMU,
			fillColor: "111827",
			fillAlpha: 42000,
			lineColor: "111827",
		})
	}

	textColor := b.preset.Text
	mutedColor := b.preset.Muted
	if hasImage {
		textColor = b.preset.InverseText
		mutedColor = b.preset.InverseText
	}
	b.addBox(textBox{
		name: "Cover Title",
		x:    762000,
		y:    1447800,
		cx:   5943600,
		cy:   1219200,
		paragraphs: []textParagraph{{
			text:  slide.Title,
			size:  3200,
			bold:  true,
			color: textColor,
		}},
	})
	if slide.Subtitle != nil {
		b.addBox(textBox{
			name: "Cover Subtitle",
			x:    762000,
			y:    2743200,
			cx:   5486400,
			cy:   457200,
			paragraphs: []textParagraph{{
				text:  *slide.Subtitle,
				size:  1700,
				color: mutedColor,
			}},
		})
	}

	bodyBlocks := nonImageBlocks(slide.Blocks)
	if !hasImage {
		bodyBlocks = slide.Blocks
	}
	if len(bodyBlocks) > 0 {
		includeImageNotes := !hasImage
		b.addBox(textBox{
			name:      "Cover Body Panel",
			x:         762000,
			y:         3657600,
			cx:        5181600,
			cy:        1828800,
			fillColor: firstNonEmpty(boolColor(hasImage, "111827", ""), b.preset.SurfaceAlt),
			fillAlpha: boolAlpha(hasImage, 65000),
			lineColor: "",
			geometry:  "roundRect",
			paragraphs: centerParagraphs(
				recolorParagraphs(blockParagraphsWithAccent(bodyBlocks, includeImageNotes, b.preset.Accent), textColor, mutedColor),
			),
		})
	}
	if imageBlock != nil && imageBlock.Caption != nil {
		b.addBox(textBox{
			name: "Cover Caption",
			x:    7010400,
			y:    5943600,
			cx:   4267200,
			cy:   304800,
			paragraphs: []textParagraph{{
				text:  *imageBlock.Caption,
				size:  1200,
				align: "r",
				color: mutedColor,
			}},
		})
	}
	return nil
}

func (b *slideBuilder) renderChapterDivider(slide presentationdialect.Slide) error {
	imageBlock := firstBlockOfType(slide.Blocks, presentationdialect.BlockTypeImage)
	hasImage := imageBlock != nil && imageBlock.AssetRef != nil && b.pkg.hasAsset(dereferenceString(imageBlock.AssetRef))
	if hasImage {
		if err := b.addPicture(dereferenceString(imageBlock.AssetRef), 0, 0, 3657600, slideHeightEMU, blockAssetDescription(imageBlock, slide.Title), true); err != nil {
			return err
		}
	}
	b.addBox(textBox{
		name:      "Divider Accent",
		x:         3898900,
		y:         1447800,
		cx:        152400,
		cy:        1524000,
		fillColor: b.preset.Accent,
		lineColor: b.preset.Accent,
	})
	b.addBox(textBox{
		name: "Divider Title",
		x:    4267200,
		y:    1524000,
		cx:   7010400,
		cy:   1066800,
		paragraphs: []textParagraph{{
			text:  slide.Title,
			size:  2800,
			bold:  true,
			color: b.preset.Text,
		}},
	})
	if slide.Subtitle != nil {
		b.addBox(textBox{
			name: "Divider Subtitle",
			x:    4267200,
			y:    2590800,
			cx:   6400800,
			cy:   457200,
			paragraphs: []textParagraph{{
				text:  *slide.Subtitle,
				size:  1600,
				color: b.preset.Muted,
			}},
		})
	}
	body := nonImageBlocks(slide.Blocks)
	if !hasImage {
		body = slide.Blocks
	}
	if len(body) > 0 {
		b.addBox(textBox{
			name:       "Divider Body",
			x:          4267200,
			y:          3352800,
			cx:         6553200,
			cy:         1828800,
			paragraphs: recolorParagraphs(blockParagraphsWithAccent(body, !hasImage, b.preset.Accent), b.preset.Text, b.preset.Muted),
		})
	}
	return nil
}

func (b *slideBuilder) renderCardGrid(slide presentationdialect.Slide) error {
	b.addBox(textBox{
		name: "Grid Title",
		x:    slideMarginXEMU,
		y:    slideMarginYEMU,
		cx:   slideWidthEMU - 2*slideMarginXEMU,
		cy:   685800,
		paragraphs: []textParagraph{{
			text:  slide.Title,
			size:  2200,
			bold:  true,
			color: b.preset.Text,
		}},
	})
	startY := 1371600
	if slide.Subtitle != nil {
		b.addBox(textBox{
			name: "Grid Subtitle",
			x:    slideMarginXEMU,
			y:    1066800,
			cx:   slideWidthEMU - 2*slideMarginXEMU,
			cy:   304800,
			paragraphs: []textParagraph{{
				text:  *slide.Subtitle,
				size:  1400,
				color: b.preset.Muted,
			}},
		})
		startY = 1600200
	}

	columns := 2
	if len(slide.Blocks) >= 5 {
		columns = 3
	}
	rows := maxInt((len(slide.Blocks)+columns-1)/columns, 1)
	cardGap := 228600
	cardWidth := (slideWidthEMU - 2*slideMarginXEMU - (columns-1)*cardGap) / columns
	cardHeight := (slideHeightEMU - startY - slideMarginYEMU - (rows-1)*cardGap) / rows
	for index, block := range slide.Blocks {
		col := index % columns
		row := index / columns
		x := slideMarginXEMU + col*(cardWidth+cardGap)
		y := startY + row*(cardHeight+cardGap)
		if err := b.addCard(block, x, y, cardWidth, cardHeight); err != nil {
			return err
		}
	}
	return nil
}

func (b *slideBuilder) renderComparisonCards(slide presentationdialect.Slide) error {
	b.addBox(textBox{
		name:       "Comparison Title",
		x:          slideMarginXEMU,
		y:          slideMarginYEMU,
		cx:         slideWidthEMU - 2*slideMarginXEMU,
		cy:         685800,
		paragraphs: []textParagraph{{text: slide.Title, size: 2200, bold: true, color: b.preset.Text}},
	})
	cardWidth := (slideWidthEMU - 2*slideMarginXEMU - contentGapEMU) / 2
	for index, block := range slide.Blocks {
		x := slideMarginXEMU
		if index == 1 {
			x += cardWidth + contentGapEMU
		}
		if err := b.addCard(block, x, 1524000, cardWidth, 4114800); err != nil {
			return err
		}
	}
	return nil
}

func (b *slideBuilder) renderTimeline(slide presentationdialect.Slide) error {
	b.addBox(textBox{
		name:       "Timeline Title",
		x:          slideMarginXEMU,
		y:          slideMarginYEMU,
		cx:         slideWidthEMU - 2*slideMarginXEMU,
		cy:         685800,
		paragraphs: []textParagraph{{text: slide.Title, size: 2200, bold: true, color: b.preset.Text}},
	})
	if slide.Subtitle != nil {
		b.addBox(textBox{
			name:       "Timeline Subtitle",
			x:          slideMarginXEMU,
			y:          1066800,
			cx:         slideWidthEMU - 2*slideMarginXEMU,
			cy:         304800,
			paragraphs: []textParagraph{{text: *slide.Subtitle, size: 1400, color: b.preset.Muted}},
		})
	}
	panelY := 1600200
	panelHeight := (slideHeightEMU - panelY - slideMarginYEMU - (len(slide.Blocks)-1)*152400) / maxInt(len(slide.Blocks), 1)
	for index, block := range slide.Blocks {
		y := panelY + index*(panelHeight+152400)
		b.addBox(textBox{
			name:      fmt.Sprintf("Timeline Dot %d", index+1),
			x:         762000,
			y:         y + 152400,
			cx:        152400,
			cy:        152400,
			fillColor: b.preset.Accent,
			lineColor: b.preset.Accent,
			geometry:  "ellipse",
		})
		b.addBox(textBox{
			name:      fmt.Sprintf("Timeline Panel %d", index+1),
			x:         1143000,
			y:         y,
			cx:        slideWidthEMU - 1905000,
			cy:        panelHeight,
			fillColor: b.preset.Surface,
			lineColor: b.preset.Outline,
			geometry:  "roundRect",
			paragraphs: recolorParagraphs(
				cardParagraphs(block, false),
				b.preset.Text,
				b.preset.Muted,
			),
		})
	}
	return nil
}

func (b *slideBuilder) renderTableLayout(slide presentationdialect.Slide) error {
	b.addBox(textBox{
		name:       "Table Title",
		x:          slideMarginXEMU,
		y:          slideMarginYEMU,
		cx:         slideWidthEMU - 2*slideMarginXEMU,
		cy:         685800,
		paragraphs: []textParagraph{{text: slide.Title, size: 2200, bold: true, color: b.preset.Text}},
	})
	tableBlock := firstBlockOfType(slide.Blocks, presentationdialect.BlockTypeTable)
	callout := firstBlockOfType(slide.Blocks, presentationdialect.BlockTypeCallout)
	tableY := 1371600
	if callout != nil {
		b.addBox(textBox{
			name:      "Table Callout",
			x:         slideMarginXEMU,
			y:         1219200,
			cx:        slideWidthEMU - 2*slideMarginXEMU,
			cy:        685800,
			fillColor: b.preset.SurfaceAlt,
			lineColor: b.preset.Outline,
			geometry:  "roundRect",
			paragraphs: recolorParagraphs(
				blockParagraphsWithAccent([]presentationdialect.Block{*callout}, false, b.preset.Accent),
				b.preset.Text,
				b.preset.Muted,
			),
		})
		tableY = 2286000
	}
	if tableBlock != nil {
		b.addTable(slideMarginXEMU, tableY, slideWidthEMU-2*slideMarginXEMU, slideHeightEMU-tableY-slideMarginYEMU, tableBlock.Header, tableBlock.Rows, b.preset.Accent)
	}
	return nil
}

func (b *slideBuilder) renderSummaryMatrix(slide presentationdialect.Slide) error {
	b.addBox(textBox{
		name:       "Summary Title",
		x:          slideMarginXEMU,
		y:          slideMarginYEMU,
		cx:         slideWidthEMU - 2*slideMarginXEMU,
		cy:         685800,
		paragraphs: []textParagraph{{text: slide.Title, size: 2200, bold: true, color: b.preset.Text}},
	})
	stats := blocksOfType(slide.Blocks, presentationdialect.BlockTypeStat)
	tableBlock := firstBlockOfType(slide.Blocks, presentationdialect.BlockTypeTable)
	callout := firstBlockOfType(slide.Blocks, presentationdialect.BlockTypeCallout)
	statWidth := (slideWidthEMU - 2*slideMarginXEMU - maxInt(len(stats)-1, 0)*contentGapEMU) / maxInt(len(stats), 1)
	for index, stat := range stats {
		x := slideMarginXEMU + index*(statWidth+contentGapEMU)
		b.addBox(textBox{
			name:      fmt.Sprintf("Stat %d", index+1),
			x:         x,
			y:         1219200,
			cx:        statWidth,
			cy:        914400,
			fillColor: b.preset.Surface,
			lineColor: b.preset.Outline,
			geometry:  "roundRect",
			paragraphs: recolorParagraphs(
				blockParagraphsWithAccent([]presentationdialect.Block{stat}, false, b.preset.Accent),
				b.preset.Text,
				b.preset.Muted,
			),
		})
	}
	if tableBlock != nil {
		tableWidth := slideWidthEMU - 2*slideMarginXEMU
		if callout != nil {
			tableWidth -= 2743200 + contentGapEMU
			b.addBox(textBox{
				name:      "Summary Callout",
				x:         slideMarginXEMU + tableWidth + contentGapEMU,
				y:         2514600,
				cx:        2743200,
				cy:        1828800,
				fillColor: b.preset.SurfaceAlt,
				lineColor: b.preset.Outline,
				geometry:  "roundRect",
				paragraphs: recolorParagraphs(
					blockParagraphsWithAccent([]presentationdialect.Block{*callout}, false, b.preset.Accent),
					b.preset.Text,
					b.preset.Muted,
				),
			})
		}
		b.addTable(slideMarginXEMU, 2514600, tableWidth, 2895600, tableBlock.Header, tableBlock.Rows, b.preset.Accent)
	}
	return nil
}

func (b *slideBuilder) renderRecommendation(slide presentationdialect.Slide) error {
	imageBlock := firstBlockOfType(slide.Blocks, presentationdialect.BlockTypeImage)
	hasImage := imageBlock != nil && imageBlock.AssetRef != nil && b.pkg.hasAsset(dereferenceString(imageBlock.AssetRef))
	if hasImage {
		if err := b.addPicture(dereferenceString(imageBlock.AssetRef), 0, 0, 4876800, slideHeightEMU, blockAssetDescription(imageBlock, slide.Title), true); err != nil {
			return err
		}
	}
	b.addBox(textBox{
		name:       "Recommendation Title",
		x:          5334000,
		y:          1219200,
		cx:         5943600,
		cy:         685800,
		paragraphs: []textParagraph{{text: slide.Title, size: 2400, bold: true, color: b.preset.Text}},
	})
	body := nonImageBlocks(slide.Blocks)
	if !hasImage {
		body = slide.Blocks
	}
	if len(body) > 0 {
		b.addBox(textBox{
			name:      "Recommendation Body",
			x:         5334000,
			y:         1981200,
			cx:        5334000,
			cy:        2895600,
			fillColor: b.preset.Surface,
			lineColor: b.preset.Outline,
			geometry:  "roundRect",
			paragraphs: recolorParagraphs(
				blockParagraphsWithAccent(body, !hasImage, b.preset.Accent),
				b.preset.Text,
				b.preset.Muted,
			),
		})
	}
	return nil
}

func (b *slideBuilder) addCard(block presentationdialect.Block, x, y, cx, cy int) error {
	b.addBox(textBox{
		name:      "Card Panel",
		x:         x,
		y:         y,
		cx:        cx,
		cy:        cy,
		fillColor: b.preset.Surface,
		lineColor: b.preset.Outline,
		geometry:  "roundRect",
	})
	contentX := x + 152400
	contentY := y + 152400
	contentWidth := cx - 304800
	if block.AssetRef != nil && b.pkg.hasAsset(dereferenceString(block.AssetRef)) {
		if err := b.addPicture(*block.AssetRef, contentX, contentY, contentWidth, 1371600, dereferenceString(block.Title), true); err != nil {
			return err
		}
		contentY += 1600200
	}
	includeImageNote := block.AssetRef != nil && !b.pkg.hasAsset(dereferenceString(block.AssetRef))
	b.addBox(textBox{
		name:       "Card Content",
		x:          contentX,
		y:          contentY,
		cx:         contentWidth,
		cy:         cy - (contentY - y) - 152400,
		paragraphs: recolorParagraphs(cardParagraphs(block, includeImageNote), b.preset.Text, b.preset.Muted),
	})
	return nil
}

func cardParagraphs(block presentationdialect.Block, includeImageNote bool) []textParagraph {
	return blockParagraphsWithOptions([]presentationdialect.Block{block}, includeImageNote)
}

func blockAssetDescription(block *presentationdialect.Block, fallback string) string {
	if block != nil {
		for _, candidate := range []*string{block.AltText, block.Caption, block.Title} {
			if text := strings.TrimSpace(dereferenceString(candidate)); text != "" {
				return text
			}
		}
	}
	return strings.TrimSpace(fallback)
}

func renderPictureXML(id int, description, relID string, x, y, cx, cy int, crop pictureCrop) string {
	var builder strings.Builder
	builder.WriteString(`<p:pic><p:nvPicPr><p:cNvPr id="`)
	builder.WriteString(strconv.Itoa(id))
	builder.WriteString(`" name="Picture `)
	builder.WriteString(strconv.Itoa(id))
	builder.WriteString(`" descr="`)
	builder.WriteString(escapeXML(description))
	builder.WriteString(`"/><p:cNvPicPr><a:picLocks noChangeAspect="1"/></p:cNvPicPr><p:nvPr/></p:nvPicPr>`)
	builder.WriteString(`<p:blipFill><a:blip r:embed="`)
	builder.WriteString(relID)
	builder.WriteString(`"/>`)
	if crop.left > 0 || crop.top > 0 || crop.right > 0 || crop.bottom > 0 {
		builder.WriteString(`<a:srcRect`)
		if crop.left > 0 {
			builder.WriteString(` l="`)
			builder.WriteString(strconv.Itoa(crop.left))
			builder.WriteString(`"`)
		}
		if crop.top > 0 {
			builder.WriteString(` t="`)
			builder.WriteString(strconv.Itoa(crop.top))
			builder.WriteString(`"`)
		}
		if crop.right > 0 {
			builder.WriteString(` r="`)
			builder.WriteString(strconv.Itoa(crop.right))
			builder.WriteString(`"`)
		}
		if crop.bottom > 0 {
			builder.WriteString(` b="`)
			builder.WriteString(strconv.Itoa(crop.bottom))
			builder.WriteString(`"`)
		}
		builder.WriteString(`/>`)
	}
	builder.WriteString(`<a:stretch><a:fillRect/></a:stretch></p:blipFill>`)
	builder.WriteString(`<p:spPr><a:xfrm><a:off x="`)
	builder.WriteString(strconv.Itoa(x))
	builder.WriteString(`" y="`)
	builder.WriteString(strconv.Itoa(y))
	builder.WriteString(`"/><a:ext cx="`)
	builder.WriteString(strconv.Itoa(cx))
	builder.WriteString(`" cy="`)
	builder.WriteString(strconv.Itoa(cy))
	builder.WriteString(`"/></a:xfrm><a:prstGeom prst="rect"><a:avLst/></a:prstGeom><a:ln><a:noFill/></a:ln></p:spPr></p:pic>`)
	return builder.String()
}

func renderTableXML(id, x, y, cx, cy int, header []string, rows [][]string, headerFill string, preset themePreset) string {
	totalRows := len(rows) + 1
	rowHeight := cy / maxInt(totalRows, 1)
	columnWidth := cx / maxInt(len(header), 1)
	var builder strings.Builder
	builder.WriteString(`<p:graphicFrame><p:nvGraphicFramePr><p:cNvPr id="`)
	builder.WriteString(strconv.Itoa(id))
	builder.WriteString(`" name="Table `)
	builder.WriteString(strconv.Itoa(id))
	builder.WriteString(`"/><p:cNvGraphicFramePr/><p:nvPr/></p:nvGraphicFramePr><p:xfrm><a:off x="`)
	builder.WriteString(strconv.Itoa(x))
	builder.WriteString(`" y="`)
	builder.WriteString(strconv.Itoa(y))
	builder.WriteString(`"/><a:ext cx="`)
	builder.WriteString(strconv.Itoa(cx))
	builder.WriteString(`" cy="`)
	builder.WriteString(strconv.Itoa(cy))
	builder.WriteString(`"/></p:xfrm><a:graphic><a:graphicData uri="http://schemas.openxmlformats.org/drawingml/2006/table"><a:tbl><a:tblPr firstRow="1" bandRow="1"><a:tableStyleId>{5940675A-B579-460E-94D1-54222C63F5DA}</a:tableStyleId></a:tblPr><a:tblGrid>`)
	for range header {
		builder.WriteString(`<a:gridCol w="`)
		builder.WriteString(strconv.Itoa(columnWidth))
		builder.WriteString(`"/>`)
	}
	builder.WriteString(`</a:tblGrid>`)
	builder.WriteString(renderTableRowXML(header, rowHeight, true, headerFill, preset))
	for _, row := range rows {
		builder.WriteString(renderTableRowXML(row, rowHeight, false, preset.Surface, preset))
	}
	builder.WriteString(`</a:tbl></a:graphicData></a:graphic></p:graphicFrame>`)
	return builder.String()
}

func renderTableRowXML(cells []string, height int, header bool, fillColor string, preset themePreset) string {
	textColor := preset.Text
	if header {
		textColor = preset.InverseText
	}
	var builder strings.Builder
	builder.WriteString(`<a:tr h="`)
	builder.WriteString(strconv.Itoa(height))
	builder.WriteString(`">`)
	for _, cell := range cells {
		builder.WriteString(`<a:tc><a:txBody><a:bodyPr/><a:lstStyle/><a:p><a:pPr algn="ctr"><a:buNone/></a:pPr><a:r><a:rPr lang="en-US" sz="1300"`)
		if header {
			builder.WriteString(` b="1"`)
		}
		builder.WriteString(`><a:latin typeface="`)
		builder.WriteString(escapeXML(firstNonEmpty(preset.Fonts.Latin, "Arial")))
		builder.WriteString(`"/>`)
		if preset.Fonts.CJK != "" {
			builder.WriteString(`<a:ea typeface="`)
			builder.WriteString(escapeXML(preset.Fonts.CJK))
			builder.WriteString(`"/><a:cs typeface="`)
			builder.WriteString(escapeXML(preset.Fonts.CJK))
			builder.WriteString(`"/>`)
		}
		builder.WriteString(`<a:solidFill><a:srgbClr val="`)
		builder.WriteString(textColor)
		builder.WriteString(`"/></a:solidFill></a:rPr><a:t>`)
		builder.WriteString(escapeXML(cell))
		builder.WriteString(`</a:t></a:r><a:endParaRPr lang="en-US" sz="1300"/></a:p></a:txBody><a:tcPr><a:solidFill><a:srgbClr val="`)
		builder.WriteString(fillColor)
		builder.WriteString(`"/></a:solidFill><a:lnL w="12700"><a:solidFill><a:srgbClr val="`)
		builder.WriteString(preset.Outline)
		builder.WriteString(`"/></a:solidFill></a:lnL><a:lnR w="12700"><a:solidFill><a:srgbClr val="`)
		builder.WriteString(preset.Outline)
		builder.WriteString(`"/></a:solidFill></a:lnR><a:lnT w="12700"><a:solidFill><a:srgbClr val="`)
		builder.WriteString(preset.Outline)
		builder.WriteString(`"/></a:solidFill></a:lnT><a:lnB w="12700"><a:solidFill><a:srgbClr val="`)
		builder.WriteString(preset.Outline)
		builder.WriteString(`"/></a:solidFill></a:lnB></a:tcPr></a:tc>`)
	}
	builder.WriteString(`</a:tr>`)
	return builder.String()
}

func buildSlideRelationshipsXML(rels []slideRelationship) string {
	var builder strings.Builder
	builder.WriteString(xmlHeader)
	builder.WriteString(`<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">`)
	builder.WriteString(`<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slideLayout" Target="../slideLayouts/slideLayout1.xml"/>`)
	for _, rel := range rels {
		builder.WriteString(`<Relationship Id="`)
		builder.WriteString(rel.id)
		builder.WriteString(`" Type="`)
		builder.WriteString(rel.relType)
		builder.WriteString(`" Target="`)
		builder.WriteString(rel.target)
		builder.WriteString(`"/>`)
	}
	builder.WriteString(`</Relationships>`)
	return builder.String()
}

func buildContentTypesXMLV2(slideCount int, media []mediaPart) string {
	var builder strings.Builder
	builder.WriteString(xmlHeader)
	builder.WriteString(`<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">`)
	builder.WriteString(`<Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>`)
	builder.WriteString(`<Default Extension="xml" ContentType="application/xml"/>`)
	seenContentTypes := map[string]struct{}{}
	for _, part := range media {
		ext := strings.TrimPrefix(filepath.Ext(part.partName), ".")
		key := ext + ":" + part.contentType
		if _, ok := seenContentTypes[key]; ok {
			continue
		}
		seenContentTypes[key] = struct{}{}
		builder.WriteString(`<Default Extension="`)
		builder.WriteString(ext)
		builder.WriteString(`" ContentType="`)
		builder.WriteString(part.contentType)
		builder.WriteString(`"/>`)
	}
	builder.WriteString(`<Override PartName="/docProps/app.xml" ContentType="application/vnd.openxmlformats-officedocument.extended-properties+xml"/>`)
	builder.WriteString(`<Override PartName="/docProps/core.xml" ContentType="application/vnd.openxmlformats-package.core-properties+xml"/>`)
	builder.WriteString(`<Override PartName="/ppt/presentation.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.presentation.main+xml"/>`)
	builder.WriteString(`<Override PartName="/ppt/slideMasters/slideMaster1.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.slideMaster+xml"/>`)
	builder.WriteString(`<Override PartName="/ppt/slideLayouts/slideLayout1.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.slideLayout+xml"/>`)
	builder.WriteString(`<Override PartName="/ppt/theme/theme1.xml" ContentType="application/vnd.openxmlformats-officedocument.theme+xml"/>`)
	for slideNumber := 1; slideNumber <= slideCount; slideNumber++ {
		builder.WriteString(`<Override PartName="/ppt/slides/slide`)
		builder.WriteString(strconv.Itoa(slideNumber))
		builder.WriteString(`.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.slide+xml"/>`)
	}
	builder.WriteString(`</Types>`)
	return builder.String()
}

func buildSlideMasterXMLV2(preset themePreset) string {
	var builder strings.Builder
	builder.WriteString(xmlHeader)
	builder.WriteString(`<p:sldMaster xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main"><p:cSld name="`)
	builder.WriteString(escapeXML(preset.Name))
	builder.WriteString(` Master"><p:bg><p:bgPr><a:solidFill><a:srgbClr val="`)
	builder.WriteString(preset.Background)
	builder.WriteString(`"/></a:solidFill><a:effectLst/></p:bgPr></p:bg><p:spTree><p:nvGrpSpPr><p:cNvPr id="1" name="Group 1"/><p:cNvGrpSpPr/><p:nvPr/></p:nvGrpSpPr><p:grpSpPr><a:xfrm><a:off x="0" y="0"/><a:ext cx="0" cy="0"/><a:chOff x="0" y="0"/><a:chExt cx="0" cy="0"/></a:xfrm></p:grpSpPr></p:spTree></p:cSld><p:clrMap bg1="lt1" tx1="dk1" bg2="lt2" tx2="dk2" accent1="accent1" accent2="accent2" accent3="accent3" accent4="accent4" accent5="accent5" accent6="accent6" hlink="hlink" folHlink="folHlink"/><p:sldLayoutIdLst><p:sldLayoutId id="2147483649" r:id="rId1"/></p:sldLayoutIdLst><p:txStyles><p:titleStyle><a:lvl1pPr algn="l"><a:defRPr sz="2800" b="1"/></a:lvl1pPr></p:titleStyle><p:bodyStyle><a:lvl1pPr><a:defRPr sz="1600"/></a:lvl1pPr></p:bodyStyle><p:otherStyle><a:defPPr><a:defRPr sz="1600"/></a:defPPr></p:otherStyle></p:txStyles></p:sldMaster>`)
	return builder.String()
}

func buildThemeXMLV2(preset themePreset) string {
	var builder strings.Builder
	builder.WriteString(xmlHeader)
	builder.WriteString(`<a:theme xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" name="`)
	builder.WriteString(escapeXML(preset.Name))
	builder.WriteString(`"><a:themeElements><a:clrScheme name="`)
	builder.WriteString(escapeXML(preset.Name))
	builder.WriteString(`"><a:dk1><a:srgbClr val="`)
	builder.WriteString(preset.Text)
	builder.WriteString(`"/></a:dk1><a:lt1><a:srgbClr val="`)
	builder.WriteString(preset.Background)
	builder.WriteString(`"/></a:lt1><a:dk2><a:srgbClr val="`)
	builder.WriteString(preset.SurfaceAlt)
	builder.WriteString(`"/></a:dk2><a:lt2><a:srgbClr val="`)
	builder.WriteString(preset.Surface)
	builder.WriteString(`"/></a:lt2><a:accent1><a:srgbClr val="`)
	builder.WriteString(preset.Accent)
	builder.WriteString(`"/></a:accent1><a:accent2><a:srgbClr val="`)
	builder.WriteString(preset.AccentAlt)
	builder.WriteString(`"/></a:accent2><a:accent3><a:srgbClr val="`)
	builder.WriteString(preset.Success)
	builder.WriteString(`"/></a:accent3><a:accent4><a:srgbClr val="`)
	builder.WriteString(preset.Warning)
	builder.WriteString(`"/></a:accent4><a:accent5><a:srgbClr val="`)
	builder.WriteString(preset.SurfaceAlt)
	builder.WriteString(`"/></a:accent5><a:accent6><a:srgbClr val="`)
	builder.WriteString(preset.Outline)
	builder.WriteString(`"/></a:accent6><a:hlink><a:srgbClr val="`)
	builder.WriteString(preset.Accent)
	builder.WriteString(`"/></a:hlink><a:folHlink><a:srgbClr val="`)
	builder.WriteString(preset.AccentAlt)
	builder.WriteString(`"/></a:folHlink></a:clrScheme><a:fontScheme name="`)
	builder.WriteString(escapeXML(preset.Name))
	builder.WriteString(` Fonts"><a:majorFont><a:latin typeface="`)
	builder.WriteString(escapeXML(preset.Fonts.Latin))
	builder.WriteString(`"/><a:ea typeface="`)
	builder.WriteString(escapeXML(preset.Fonts.CJK))
	builder.WriteString(`"/><a:cs typeface="`)
	builder.WriteString(escapeXML(preset.Fonts.CJK))
	builder.WriteString(`"/></a:majorFont><a:minorFont><a:latin typeface="`)
	builder.WriteString(escapeXML(preset.Fonts.Latin))
	builder.WriteString(`"/><a:ea typeface="`)
	builder.WriteString(escapeXML(preset.Fonts.CJK))
	builder.WriteString(`"/><a:cs typeface="`)
	builder.WriteString(escapeXML(preset.Fonts.CJK))
	builder.WriteString(`"/></a:minorFont></a:fontScheme><a:fmtScheme name="`)
	builder.WriteString(escapeXML(preset.Name))
	builder.WriteString(`"><a:fillStyleLst><a:solidFill><a:schemeClr val="phClr"/></a:solidFill><a:solidFill><a:srgbClr val="`)
	builder.WriteString(preset.Surface)
	builder.WriteString(`"/></a:solidFill><a:solidFill><a:srgbClr val="`)
	builder.WriteString(preset.SurfaceAlt)
	builder.WriteString(`"/></a:solidFill></a:fillStyleLst><a:lineStyleLst><a:ln w="9525"><a:solidFill><a:srgbClr val="`)
	builder.WriteString(preset.Outline)
	builder.WriteString(`"/></a:solidFill></a:ln><a:ln w="19050"><a:solidFill><a:srgbClr val="`)
	builder.WriteString(preset.Outline)
	builder.WriteString(`"/></a:solidFill></a:ln><a:ln w="28575"><a:solidFill><a:srgbClr val="`)
	builder.WriteString(preset.Accent)
	builder.WriteString(`"/></a:solidFill></a:ln></a:lineStyleLst><a:effectStyleLst><a:effectStyle><a:effectLst/></a:effectStyle><a:effectStyle><a:effectLst/></a:effectStyle><a:effectStyle><a:effectLst/></a:effectStyle></a:effectStyleLst><a:bgFillStyleLst><a:solidFill><a:srgbClr val="`)
	builder.WriteString(preset.Background)
	builder.WriteString(`"/></a:solidFill><a:solidFill><a:srgbClr val="`)
	builder.WriteString(preset.Surface)
	builder.WriteString(`"/></a:solidFill><a:solidFill><a:srgbClr val="`)
	builder.WriteString(preset.SurfaceAlt)
	builder.WriteString(`"/></a:solidFill></a:bgFillStyleLst></a:fmtScheme></a:themeElements><a:objectDefaults/><a:extraClrSchemeLst/></a:theme>`)
	return builder.String()
}

func resolveThemePreset(id string) themePreset {
	definition := presentationdialect.ResolveThemePresetDesign(id)
	return themePreset{
		ID:          definition.Metadata.ID,
		Name:        definition.Metadata.Label,
		Background:  definition.Palette.Background,
		Surface:     definition.Palette.Surface,
		SurfaceAlt:  definition.Palette.SurfaceAlt,
		Text:        definition.Palette.Text,
		Muted:       definition.Palette.Muted,
		Accent:      definition.Palette.Accent,
		AccentAlt:   definition.Palette.AccentAlt,
		Success:     definition.Palette.Success,
		Warning:     definition.Palette.Warning,
		InverseText: definition.Palette.InverseText,
		Outline:     definition.Palette.Outline,
		Fonts: themeFonts{
			Latin: definition.Fonts.Latin,
			CJK:   definition.Fonts.CJK,
		},
	}
}

func defaultLegacyFonts() themeFonts {
	return themeFonts{Latin: "Arial"}
}

func (b *v2PackageBuilder) imageCropToFill(assetRef string, targetWidth, targetHeight int) pictureCrop {
	if targetWidth <= 0 || targetHeight <= 0 {
		return pictureCrop{}
	}
	cfg, ok := b.imageConfigByRef[assetRef]
	if !ok {
		asset, ok := b.assets[assetRef]
		if !ok {
			return pictureCrop{}
		}
		decoded, _, err := image.DecodeConfig(bytes.NewReader(asset.Data))
		if err != nil || decoded.Width <= 0 || decoded.Height <= 0 {
			return pictureCrop{}
		}
		b.imageConfigByRef[assetRef] = decoded
		cfg = decoded
	}
	return imageCropToFillConfig(cfg, targetWidth, targetHeight)
}

func imageCropToFillConfig(cfg image.Config, targetWidth, targetHeight int) pictureCrop {
	if cfg.Width <= 0 || cfg.Height <= 0 || targetWidth <= 0 || targetHeight <= 0 {
		return pictureCrop{}
	}
	imageAspect := float64(cfg.Width) / float64(cfg.Height)
	targetAspect := float64(targetWidth) / float64(targetHeight)
	if imageAspect > targetAspect {
		visibleWidth := targetAspect / imageAspect
		crop := int(((1 - visibleWidth) / 2) * 100000)
		return pictureCrop{left: crop, right: crop}
	}
	if imageAspect < targetAspect {
		visibleHeight := imageAspect / targetAspect
		crop := int(((1 - visibleHeight) / 2) * 100000)
		return pictureCrop{top: crop, bottom: crop}
	}
	return pictureCrop{}
}

func mediaExtension(asset CompileAsset) (string, string, error) {
	switch strings.ToLower(strings.TrimSpace(asset.MediaType)) {
	case "image/png":
		return "png", "image/png", nil
	case "image/jpeg", "image/jpg":
		return "jpeg", "image/jpeg", nil
	default:
		return "", "", fmt.Errorf("unsupported pptx image media type %q", asset.MediaType)
	}
}

func nonImageBlocks(blocks []presentationdialect.Block) []presentationdialect.Block {
	filtered := make([]presentationdialect.Block, 0, len(blocks))
	for _, block := range blocks {
		if block.Type == presentationdialect.BlockTypeImage {
			continue
		}
		filtered = append(filtered, block)
	}
	return filtered
}

func firstBlockOfType(blocks []presentationdialect.Block, blockType presentationdialect.BlockType) *presentationdialect.Block {
	for index := range blocks {
		if blocks[index].Type == blockType {
			return &blocks[index]
		}
	}
	return nil
}

func blocksOfType(blocks []presentationdialect.Block, blockType presentationdialect.BlockType) []presentationdialect.Block {
	filtered := make([]presentationdialect.Block, 0, len(blocks))
	for _, block := range blocks {
		if block.Type == blockType {
			filtered = append(filtered, block)
		}
	}
	return filtered
}

func recolorParagraphs(paragraphs []textParagraph, textColor string, mutedColor string) []textParagraph {
	colored := make([]textParagraph, 0, len(paragraphs))
	for _, paragraph := range paragraphs {
		updated := paragraph
		if len(updated.runs) > 0 {
			updated.runs = append([]textRun(nil), updated.runs...)
			for index, run := range updated.runs {
				if run.color == "" {
					if run.italic || (!run.bold && strings.HasPrefix(run.text, "— ")) {
						run.color = mutedColor
					} else {
						run.color = textColor
					}
					updated.runs[index] = run
				}
			}
		} else if updated.color == "" && updated.text != "" {
			if updated.italic {
				updated.color = mutedColor
			} else {
				updated.color = textColor
			}
		}
		colored = append(colored, updated)
	}
	return colored
}

func richTextColor(emphasis string, accentColor string) string {
	switch emphasis {
	case "accent":
		return accentColor
	default:
		return ""
	}
}

func normalizeDrawingLanguage(lang string) string {
	lang = strings.TrimSpace(lang)
	if lang == "" {
		return ""
	}
	switch {
	case strings.EqualFold(lang, "ja"), strings.EqualFold(lang, "ja-jp"):
		return "ja-JP"
	case strings.EqualFold(lang, "zh"), strings.EqualFold(lang, "zh-cn"):
		return "zh-CN"
	case strings.EqualFold(lang, "zh-tw"):
		return "zh-TW"
	case strings.EqualFold(lang, "ko"), strings.EqualFold(lang, "ko-kr"):
		return "ko-KR"
	default:
		return lang
	}
}

func eastAsianFontForLang(lang string, fonts themeFonts) string {
	lang = strings.ToLower(strings.TrimSpace(lang))
	if strings.HasPrefix(lang, "ja") || strings.HasPrefix(lang, "zh") || strings.HasPrefix(lang, "ko") {
		return fonts.CJK
	}
	return ""
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func maxInt(value int, fallback int) int {
	if value > 0 {
		return value
	}
	return fallback
}

func boolColor(condition bool, whenTrue string, whenFalse string) string {
	if condition {
		return whenTrue
	}
	return whenFalse
}

func boolAlpha(condition bool, alpha int) int {
	if condition {
		return alpha
	}
	return 0
}

func cloneCompileAssets(assets map[string]CompileAsset) map[string]CompileAsset {
	if len(assets) == 0 {
		return nil
	}
	cloned := make(map[string]CompileAsset, len(assets))
	for key, asset := range assets {
		cloned[key] = CompileAsset{
			Filename:  asset.Filename,
			MediaType: asset.MediaType,
			Data:      slicesClone(asset.Data),
		}
	}
	return cloned
}

func slicesClone(data []byte) []byte {
	if len(data) == 0 {
		return nil
	}
	cloned := make([]byte, len(data))
	copy(cloned, data)
	return cloned
}
