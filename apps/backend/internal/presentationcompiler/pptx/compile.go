package pptx

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/ivanlin/ulduar/apps/backend/internal/presentationdialect"
)

const (
	slideWidthEMU  = 12192000
	slideHeightEMU = 6858000
	notesWidthEMU  = 6858000
	notesHeightEMU = 9144000

	slideMarginXEMU = 457200
	slideMarginYEMU = 304800
	contentGapEMU   = 228600
)

var zipModifiedTime = time.Date(1980, time.January, 1, 0, 0, 0, 0, time.UTC)

type zipEntry struct {
	name string
	data []byte
}

type textBox struct {
	id         int
	name       string
	x          int
	y          int
	cx         int
	cy         int
	fillColor  string
	fillAlpha  int
	lineColor  string
	geometry   string
	paragraphs []textParagraph
}

type textParagraph struct {
	text        string
	runs        []textRun
	size        int
	bold        bool
	italic      bool
	align       string
	bullet      bulletKind
	bulletIndex int
	color       string
}

type textRun struct {
	text          string
	bold          bool
	italic        bool
	color         string
	lang          string
	latinFont     string
	eastAsianFont string
}

type bulletKind int

const (
	bulletNone bulletKind = iota
	bulletDisc
	bulletNumbered
)

func Compile(document presentationdialect.Document) ([]byte, error) {
	return CompileWithAssets(document, nil)
}

func CompileWithAssets(document presentationdialect.Document, assets map[string]CompileAsset) ([]byte, error) {
	normalized, err := presentationdialect.Normalize(document)
	if err != nil {
		return nil, fmt.Errorf("normalize presentation document: %w", err)
	}

	if normalized.Version == presentationdialect.VersionV2 {
		return compileV2(normalized, assets)
	}

	entries := make([]zipEntry, 0, 11+len(normalized.Slides)*2)
	entries = append(entries,
		zipEntry{name: "[Content_Types].xml", data: []byte(buildContentTypesXML(len(normalized.Slides)))},
		zipEntry{name: "_rels/.rels", data: []byte(rootRelationshipsXML)},
		zipEntry{name: "docProps/app.xml", data: []byte(buildAppPropertiesXML(normalized))},
		zipEntry{name: "docProps/core.xml", data: []byte(corePropertiesXML)},
		zipEntry{name: "ppt/presentation.xml", data: []byte(buildPresentationXML(normalized))},
		zipEntry{name: "ppt/_rels/presentation.xml.rels", data: []byte(buildPresentationRelationshipsXML(len(normalized.Slides)))},
		zipEntry{name: "ppt/slideMasters/slideMaster1.xml", data: []byte(slideMasterXML)},
		zipEntry{name: "ppt/slideMasters/_rels/slideMaster1.xml.rels", data: []byte(slideMasterRelationshipsXML)},
		zipEntry{name: "ppt/slideLayouts/slideLayout1.xml", data: []byte(slideLayoutXML)},
		zipEntry{name: "ppt/slideLayouts/_rels/slideLayout1.xml.rels", data: []byte(slideLayoutRelationshipsXML)},
		zipEntry{name: "ppt/theme/theme1.xml", data: []byte(themeXML)},
	)

	for index, slide := range normalized.Slides {
		slideNumber := index + 1
		entries = append(entries,
			zipEntry{name: fmt.Sprintf("ppt/slides/slide%d.xml", slideNumber), data: []byte(buildSlideXML(slide))},
			zipEntry{name: fmt.Sprintf("ppt/slides/_rels/slide%d.xml.rels", slideNumber), data: []byte(slideRelationshipsXML)},
		)
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

func buildContentTypesXML(slideCount int) string {
	var builder strings.Builder
	builder.WriteString(xmlHeader)
	builder.WriteString(`<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">`)
	builder.WriteString(`<Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>`)
	builder.WriteString(`<Default Extension="xml" ContentType="application/xml"/>`)
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

func buildAppPropertiesXML(document presentationdialect.Document) string {
	var builder strings.Builder
	builder.WriteString(xmlHeader)
	builder.WriteString(`<Properties xmlns="http://schemas.openxmlformats.org/officeDocument/2006/extended-properties" xmlns:vt="http://schemas.openxmlformats.org/officeDocument/2006/docPropsVTypes">`)
	builder.WriteString(`<Application>ulduar</Application>`)
	builder.WriteString(`<PresentationFormat>On-screen Show (16:9)</PresentationFormat>`)
	builder.WriteString(`<Slides>`)
	builder.WriteString(strconv.Itoa(len(document.Slides)))
	builder.WriteString(`</Slides>`)
	builder.WriteString(`<Notes>0</Notes><HiddenSlides>0</HiddenSlides><MMClips>0</MMClips><ScaleCrop>false</ScaleCrop>`)
	builder.WriteString(`<HeadingPairs><vt:vector size="2" baseType="variant"><vt:variant><vt:lpstr>Titles of Parts</vt:lpstr></vt:variant><vt:variant><vt:i4>`)
	builder.WriteString(strconv.Itoa(len(document.Slides)))
	builder.WriteString(`</vt:i4></vt:variant></vt:vector></HeadingPairs>`)
	builder.WriteString(`<TitlesOfParts><vt:vector size="`)
	builder.WriteString(strconv.Itoa(len(document.Slides)))
	builder.WriteString(`" baseType="lpstr">`)
	for _, slide := range document.Slides {
		builder.WriteString(`<vt:lpstr>`)
		builder.WriteString(escapeXML(slide.Title))
		builder.WriteString(`</vt:lpstr>`)
	}
	builder.WriteString(`</vt:vector></TitlesOfParts>`)
	builder.WriteString(`<Company></Company><LinksUpToDate>false</LinksUpToDate><SharedDoc>false</SharedDoc><HyperlinksChanged>false</HyperlinksChanged><AppVersion>1.0</AppVersion>`)
	builder.WriteString(`</Properties>`)

	return builder.String()
}

func buildPresentationXML(document presentationdialect.Document) string {
	var builder strings.Builder
	builder.WriteString(xmlHeader)
	builder.WriteString(`<p:presentation xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main">`)
	builder.WriteString(`<p:sldMasterIdLst><p:sldMasterId id="2147483648" r:id="rId1"/></p:sldMasterIdLst>`)
	builder.WriteString(`<p:sldIdLst>`)
	for index := range document.Slides {
		builder.WriteString(`<p:sldId id="`)
		builder.WriteString(strconv.Itoa(256 + index))
		builder.WriteString(`" r:id="rId`)
		builder.WriteString(strconv.Itoa(index + 2))
		builder.WriteString(`"/>`)
	}
	builder.WriteString(`</p:sldIdLst>`)
	builder.WriteString(`<p:sldSz cx="`)
	builder.WriteString(strconv.Itoa(slideWidthEMU))
	builder.WriteString(`" cy="`)
	builder.WriteString(strconv.Itoa(slideHeightEMU))
	builder.WriteString(`" type="screen16x9"/>`)
	builder.WriteString(`<p:notesSz cx="`)
	builder.WriteString(strconv.Itoa(notesWidthEMU))
	builder.WriteString(`" cy="`)
	builder.WriteString(strconv.Itoa(notesHeightEMU))
	builder.WriteString(`"/>`)
	builder.WriteString(`</p:presentation>`)

	return builder.String()
}

func buildPresentationRelationshipsXML(slideCount int) string {
	var builder strings.Builder
	builder.WriteString(xmlHeader)
	builder.WriteString(`<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">`)
	builder.WriteString(`<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slideMaster" Target="slideMasters/slideMaster1.xml"/>`)
	for slideNumber := 1; slideNumber <= slideCount; slideNumber++ {
		builder.WriteString(`<Relationship Id="rId`)
		builder.WriteString(strconv.Itoa(slideNumber + 1))
		builder.WriteString(`" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slide" Target="slides/slide`)
		builder.WriteString(strconv.Itoa(slideNumber))
		builder.WriteString(`.xml"/>`)
	}
	builder.WriteString(`</Relationships>`)

	return builder.String()
}

func buildSlideXML(slide presentationdialect.Slide) string {
	textBoxes := slideTextBoxes(slide)

	var builder strings.Builder
	builder.WriteString(xmlHeader)
	builder.WriteString(`<p:sld xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main">`)
	builder.WriteString(`<p:cSld name="`)
	builder.WriteString(escapeXML(slide.Title))
	builder.WriteString(`"><p:spTree>`)
	builder.WriteString(groupShapeXML)
	for _, textBox := range textBoxes {
		builder.WriteString(renderTextBox(textBox))
	}
	builder.WriteString(`</p:spTree></p:cSld><p:clrMapOvr><a:masterClrMapping/></p:clrMapOvr></p:sld>`)

	return builder.String()
}

func slideTextBoxes(slide presentationdialect.Slide) []textBox {
	return slideTextBoxesWithAccent(slide, "A45C40")
}

func slideTextBoxesWithAccent(slide presentationdialect.Slide, accentColor string) []textBox {
	switch slide.Layout {
	case presentationdialect.LayoutTitle:
		return titleSlideTextBoxes(slide)
	case presentationdialect.LayoutSection:
		return sectionSlideTextBoxes(slide)
	case presentationdialect.LayoutTitleBullets:
		return titleBulletsSlideTextBoxes(slide, accentColor)
	case presentationdialect.LayoutTwoColumn:
		return twoColumnSlideTextBoxes(slide, accentColor)
	case presentationdialect.LayoutTable:
		return tableSlideTextBoxes(slide, accentColor)
	case presentationdialect.LayoutClosing:
		return closingSlideTextBoxes(slide, accentColor)
	case presentationdialect.LayoutCoverHero:
		return semanticContentSlideTextBoxes(slide, true, accentColor)
	case presentationdialect.LayoutChapterDivider:
		return semanticContentSlideTextBoxes(slide, true, accentColor)
	case presentationdialect.LayoutTOCGrid:
		return semanticContentSlideTextBoxes(slide, false, accentColor)
	case presentationdialect.LayoutCardGrid:
		return semanticContentSlideTextBoxes(slide, false, accentColor)
	case presentationdialect.LayoutComparisonCards:
		return semanticContentSlideTextBoxes(slide, false, accentColor)
	case presentationdialect.LayoutTimelineItinerary:
		return semanticContentSlideTextBoxes(slide, false, accentColor)
	case presentationdialect.LayoutSummaryMatrix:
		return semanticContentSlideTextBoxes(slide, false, accentColor)
	case presentationdialect.LayoutRecommendation:
		return semanticContentSlideTextBoxes(slide, false, accentColor)
	default:
		return nil
	}
}

func semanticContentSlideTextBoxes(slide presentationdialect.Slide, centered bool, accentColor string) []textBox {
	textBoxes := []textBox{
		{
			id:   2,
			name: "Title",
			x:    slideMarginXEMU,
			y:    slideMarginYEMU,
			cx:   slideWidthEMU - 2*slideMarginXEMU,
			cy:   685800,
			paragraphs: []textParagraph{
				{text: slide.Title, size: 2200, bold: true},
			},
		},
	}

	bodyY := 1371600
	nextID := 3
	if slide.Subtitle != nil {
		textBoxes = append(textBoxes, textBox{
			id:   nextID,
			name: "Subtitle",
			x:    slideMarginXEMU,
			y:    1066800,
			cx:   slideWidthEMU - 2*slideMarginXEMU,
			cy:   457200,
			paragraphs: []textParagraph{
				{text: *slide.Subtitle, size: 1600, color: "666666"},
			},
		})
		nextID++
		bodyY = 1676400
	}

	paragraphs := blockParagraphsWithAccent(slide.Blocks, true, accentColor)
	if centered {
		paragraphs = centerParagraphs(paragraphs)
	}
	textBoxes = append(textBoxes, textBox{
		id:         nextID,
		name:       "Content",
		x:          slideMarginXEMU,
		y:          bodyY,
		cx:         slideWidthEMU - 2*slideMarginXEMU,
		cy:         slideHeightEMU - bodyY - slideMarginYEMU,
		paragraphs: paragraphs,
	})

	return textBoxes
}

func titleSlideTextBoxes(slide presentationdialect.Slide) []textBox {
	textBoxes := []textBox{
		{
			id:   2,
			name: "Title",
			x:    914400,
			y:    1767840,
			cx:   slideWidthEMU - 1828800,
			cy:   914400,
			paragraphs: []textParagraph{
				{text: slide.Title, size: 2800, bold: true, align: "ctr"},
			},
		},
	}
	if slide.Subtitle != nil {
		textBoxes = append(textBoxes, textBox{
			id:   3,
			name: "Subtitle",
			x:    1524000,
			y:    2743200,
			cx:   slideWidthEMU - 3048000,
			cy:   685800,
			paragraphs: []textParagraph{
				{text: *slide.Subtitle, size: 1800, align: "ctr", color: "666666"},
			},
		})
	}

	return textBoxes
}

func sectionSlideTextBoxes(slide presentationdialect.Slide) []textBox {
	textBoxes := []textBox{
		{
			id:   2,
			name: "Section Title",
			x:    914400,
			y:    1981200,
			cx:   slideWidthEMU - 1828800,
			cy:   914400,
			paragraphs: []textParagraph{
				{text: slide.Title, size: 2600, bold: true, align: "ctr"},
			},
		},
	}
	if slide.Subtitle != nil {
		textBoxes = append(textBoxes, textBox{
			id:   3,
			name: "Section Subtitle",
			x:    1524000,
			y:    2971800,
			cx:   slideWidthEMU - 3048000,
			cy:   685800,
			paragraphs: []textParagraph{
				{text: *slide.Subtitle, size: 1800, align: "ctr", color: "666666"},
			},
		})
	}

	return textBoxes
}

func titleBulletsSlideTextBoxes(slide presentationdialect.Slide, accentColor string) []textBox {
	return []textBox{
		{
			id:   2,
			name: "Title",
			x:    slideMarginXEMU,
			y:    slideMarginYEMU,
			cx:   slideWidthEMU - 2*slideMarginXEMU,
			cy:   685800,
			paragraphs: []textParagraph{
				{text: slide.Title, size: 2200, bold: true},
			},
		},
		{
			id:         3,
			name:       "Content",
			x:          914400,
			y:          1371600,
			cx:         slideWidthEMU - 1828800,
			cy:         slideHeightEMU - 1828800,
			paragraphs: blockParagraphsWithAccent(slide.Blocks, true, accentColor),
		},
	}
}

func twoColumnSlideTextBoxes(slide presentationdialect.Slide, accentColor string) []textBox {
	columnWidth := (slideWidthEMU - 2*slideMarginXEMU - contentGapEMU) / 2
	bodyY := 1371600
	bodyHeight := slideHeightEMU - bodyY - slideMarginYEMU
	leftX := slideMarginXEMU
	rightX := leftX + columnWidth + contentGapEMU
	nextID := 3

	textBoxes := []textBox{
		{
			id:   2,
			name: "Title",
			x:    slideMarginXEMU,
			y:    slideMarginYEMU,
			cx:   slideWidthEMU - 2*slideMarginXEMU,
			cy:   685800,
			paragraphs: []textParagraph{
				{text: slide.Title, size: 2200, bold: true},
			},
		},
	}

	for index, column := range slide.Columns {
		x := leftX
		if index == 1 {
			x = rightX
		}
		columnBodyY := bodyY
		columnBodyHeight := bodyHeight
		if column.Heading != "" {
			textBoxes = append(textBoxes, textBox{
				id:   nextID,
				name: fmt.Sprintf("Column %d Heading", index+1),
				x:    x,
				y:    bodyY,
				cx:   columnWidth,
				cy:   457200,
				paragraphs: []textParagraph{
					{text: column.Heading, size: 1600, bold: true},
				},
			})
			nextID++
			columnBodyY += 533400
			columnBodyHeight -= 533400
		}
		textBoxes = append(textBoxes, textBox{
			id:         nextID,
			name:       fmt.Sprintf("Column %d Content", index+1),
			x:          x,
			y:          columnBodyY,
			cx:         columnWidth,
			cy:         columnBodyHeight,
			paragraphs: blockParagraphsWithAccent(column.Blocks, true, accentColor),
		})
		nextID++
	}

	return textBoxes
}

func tableSlideTextBoxes(slide presentationdialect.Slide, accentColor string) []textBox {
	return []textBox{
		{
			id:   2,
			name: "Title",
			x:    slideMarginXEMU,
			y:    slideMarginYEMU,
			cx:   slideWidthEMU - 2*slideMarginXEMU,
			cy:   685800,
			paragraphs: []textParagraph{
				{text: slide.Title, size: 2200, bold: true},
			},
		},
		{
			id:         3,
			name:       "Table",
			x:          slideMarginXEMU,
			y:          1371600,
			cx:         slideWidthEMU - 2*slideMarginXEMU,
			cy:         slideHeightEMU - 1828800,
			paragraphs: blockParagraphsWithAccent(slide.Blocks, true, accentColor),
		},
	}
}

func closingSlideTextBoxes(slide presentationdialect.Slide, accentColor string) []textBox {
	textBoxes := []textBox{
		{
			id:   2,
			name: "Title",
			x:    914400,
			y:    1524000,
			cx:   slideWidthEMU - 1828800,
			cy:   762000,
			paragraphs: []textParagraph{
				{text: slide.Title, size: 2600, bold: true, align: "ctr"},
			},
		},
	}

	nextID := 3
	bodyY := 2514600
	if slide.Subtitle != nil {
		textBoxes = append(textBoxes, textBox{
			id:   nextID,
			name: "Subtitle",
			x:    1219200,
			y:    2362200,
			cx:   slideWidthEMU - 2438400,
			cy:   533400,
			paragraphs: []textParagraph{
				{text: *slide.Subtitle, size: 1700, align: "ctr", color: "666666"},
			},
		})
		nextID++
		bodyY = 3200400
	}
	if len(slide.Blocks) > 0 {
		textBoxes = append(textBoxes, textBox{
			id:         nextID,
			name:       "Closing Content",
			x:          1524000,
			y:          bodyY,
			cx:         slideWidthEMU - 3048000,
			cy:         slideHeightEMU - bodyY - 762000,
			paragraphs: centerParagraphs(blockParagraphsWithAccent(slide.Blocks, true, accentColor)),
		})
	}

	return textBoxes
}

func blockParagraphs(blocks []presentationdialect.Block) []textParagraph {
	return blockParagraphsWithAccent(blocks, true, "A45C40")
}

func blockParagraphsWithOptions(blocks []presentationdialect.Block, includeImageNotes bool) []textParagraph {
	return blockParagraphsWithAccent(blocks, includeImageNotes, "A45C40")
}

func blockParagraphsWithAccent(blocks []presentationdialect.Block, includeImageNotes bool, accentColor string) []textParagraph {
	paragraphs := make([]textParagraph, 0, len(blocks)*3)
	for blockIndex, block := range blocks {
		if blockIndex > 0 {
			paragraphs = append(paragraphs, textParagraph{size: 1000})
		}

		switch block.Type {
		case presentationdialect.BlockTypeParagraph:
			paragraphs = append(paragraphs, textParagraph{text: dereferenceString(block.Text), size: 1600})
		case presentationdialect.BlockTypeBulletList:
			for _, item := range block.Items {
				paragraphs = append(paragraphs, textParagraph{text: item, size: 1600, bullet: bulletDisc})
			}
		case presentationdialect.BlockTypeNumberedList:
			for index, item := range block.Items {
				paragraphs = append(paragraphs, textParagraph{text: item, size: 1600, bullet: bulletNumbered, bulletIndex: index + 1})
			}
		case presentationdialect.BlockTypeQuote:
			paragraphs = append(paragraphs, textParagraph{text: dereferenceString(block.Text), size: 1700, italic: true, color: "444444"})
			if block.Attribution != nil {
				paragraphs = append(paragraphs, textParagraph{text: "— " + *block.Attribution, size: 1400, italic: true, color: "666666"})
			}
		case presentationdialect.BlockTypeTable:
			paragraphs = append(paragraphs, textParagraph{text: strings.Join(block.Header, " | "), size: 1600, bold: true})
			for _, row := range block.Rows {
				paragraphs = append(paragraphs, textParagraph{text: strings.Join(row, " | "), size: 1500})
			}
		case presentationdialect.BlockTypeImage:
			if includeImageNotes {
				paragraphs = append(paragraphs, textParagraph{text: "Image asset: " + dereferenceString(block.AssetRef), size: 1500, italic: true, color: "666666"})
			}
			if block.Caption != nil && includeImageNotes {
				paragraphs = append(paragraphs, textParagraph{text: *block.Caption, size: 1400, color: "666666"})
			}
		case presentationdialect.BlockTypeBadge:
			paragraphs = append(paragraphs, textParagraph{text: strings.ToUpper(dereferenceString(block.Text)), size: 1300, bold: true, color: "666666"})
		case presentationdialect.BlockTypeRichText:
			paragraphs = append(paragraphs, textParagraph{runs: richTextRuns(block.Spans, accentColor), size: 1600})
		case presentationdialect.BlockTypeCallout:
			paragraphs = append(paragraphs, textParagraph{text: dereferenceString(block.Title), size: 1700, bold: true})
			paragraphs = append(paragraphs, textParagraph{text: dereferenceString(block.Body), size: 1600})
		case presentationdialect.BlockTypeCard:
			if block.Label != nil {
				paragraphs = append(paragraphs, textParagraph{text: *block.Label, size: 1300, bold: true, color: "666666"})
			}
			paragraphs = append(paragraphs, textParagraph{text: dereferenceString(block.Title), size: 1700, bold: true})
			if block.Body != nil {
				paragraphs = append(paragraphs, textParagraph{text: *block.Body, size: 1500})
			}
			if includeImageNotes && block.AssetRef != nil {
				paragraphs = append(paragraphs, textParagraph{text: "Image asset: " + *block.AssetRef, size: 1400, italic: true, color: "666666"})
			}
		case presentationdialect.BlockTypeStat:
			paragraphs = append(paragraphs, textParagraph{text: dereferenceString(block.Value), size: 1900, bold: true})
			paragraphs = append(paragraphs, textParagraph{text: dereferenceString(block.Label), size: 1500, color: "666666"})
			if block.Body != nil {
				paragraphs = append(paragraphs, textParagraph{text: *block.Body, size: 1400})
			}
		}
	}

	if len(paragraphs) == 0 {
		return []textParagraph{{size: 1200}}
	}

	return paragraphs
}

func centerParagraphs(paragraphs []textParagraph) []textParagraph {
	centered := make([]textParagraph, 0, len(paragraphs))
	for _, paragraph := range paragraphs {
		updated := paragraph
		updated.align = "ctr"
		centered = append(centered, updated)
	}

	return centered
}

func richTextRuns(spans []presentationdialect.TextSpan, accentColor string) []textRun {
	runs := make([]textRun, 0, len(spans))
	for _, span := range spans {
		emphasis := strings.TrimSpace(span.Emphasis)
		runs = append(runs, textRun{
			text:   span.Text,
			bold:   emphasis == "strong",
			italic: emphasis == "emphasis",
			color:  richTextColor(emphasis, accentColor),
			lang:   normalizeDrawingLanguage(span.Lang),
		})
	}
	return runs
}

func renderTextBox(textBox textBox) string {
	return renderTextBoxWithFonts(textBox, defaultLegacyFonts())
}

func renderTextBoxWithFonts(textBox textBox, fonts themeFonts) string {
	var builder strings.Builder
	builder.WriteString(`<p:sp><p:nvSpPr><p:cNvPr id="`)
	builder.WriteString(strconv.Itoa(textBox.id))
	builder.WriteString(`" name="`)
	builder.WriteString(escapeXML(textBox.name))
	builder.WriteString(`"/><p:cNvSpPr txBox="1"/><p:nvPr/></p:nvSpPr>`)
	builder.WriteString(`<p:spPr><a:xfrm><a:off x="`)
	builder.WriteString(strconv.Itoa(textBox.x))
	builder.WriteString(`" y="`)
	builder.WriteString(strconv.Itoa(textBox.y))
	builder.WriteString(`"/><a:ext cx="`)
	builder.WriteString(strconv.Itoa(textBox.cx))
	builder.WriteString(`" cy="`)
	builder.WriteString(strconv.Itoa(textBox.cy))
	builder.WriteString(`"/></a:xfrm><a:prstGeom prst="`)
	if textBox.geometry != "" {
		builder.WriteString(textBox.geometry)
	} else {
		builder.WriteString(`rect`)
	}
	builder.WriteString(`"><a:avLst/></a:prstGeom>`)
	if textBox.fillColor != "" {
		builder.WriteString(`<a:solidFill><a:srgbClr val="`)
		builder.WriteString(textBox.fillColor)
		builder.WriteString(`"`)
		if textBox.fillAlpha > 0 {
			builder.WriteString(`><a:alpha val="`)
			builder.WriteString(strconv.Itoa(textBox.fillAlpha))
			builder.WriteString(`"/></a:srgbClr></a:solidFill>`)
		} else {
			builder.WriteString(`/></a:solidFill>`)
		}
	} else {
		builder.WriteString(`<a:noFill/>`)
	}
	builder.WriteString(`<a:ln>`)
	if textBox.lineColor != "" {
		builder.WriteString(`<a:solidFill><a:srgbClr val="`)
		builder.WriteString(textBox.lineColor)
		builder.WriteString(`"/></a:solidFill>`)
	} else {
		builder.WriteString(`<a:noFill/>`)
	}
	builder.WriteString(`</a:ln></p:spPr>`)
	builder.WriteString(`<p:txBody><a:bodyPr wrap="square" rtlCol="0" anchor="t"/><a:lstStyle/>`)
	for _, paragraph := range textBox.paragraphs {
		builder.WriteString(renderParagraphWithFonts(paragraph, fonts))
	}
	builder.WriteString(`</p:txBody></p:sp>`)

	return builder.String()
}

func renderParagraph(paragraph textParagraph) string {
	return renderParagraphWithFonts(paragraph, defaultLegacyFonts())
}

func renderParagraphWithFonts(paragraph textParagraph, fonts themeFonts) string {
	size := paragraph.size
	if size == 0 {
		size = 1200
	}

	var builder strings.Builder
	builder.WriteString(`<a:p><a:pPr`)
	if paragraph.align != "" {
		builder.WriteString(` algn="`)
		builder.WriteString(paragraph.align)
		builder.WriteString(`"`)
	}
	switch paragraph.bullet {
	case bulletDisc, bulletNumbered:
		builder.WriteString(` marL="342900" indent="-228600"`)
	}
	builder.WriteString(`>`)
	switch paragraph.bullet {
	case bulletDisc:
		builder.WriteString(`<a:buChar char="•"/>`)
	case bulletNumbered:
		builder.WriteString(`<a:buAutoNum type="arabicPeriod" startAt="`)
		builder.WriteString(strconv.Itoa(paragraph.bulletIndex))
		builder.WriteString(`"/>`)
	default:
		builder.WriteString(`<a:buNone/>`)
	}
	builder.WriteString(`</a:pPr>`)

	runs := paragraph.runs
	if len(runs) == 0 && paragraph.text != "" {
		runs = []textRun{{
			text:   paragraph.text,
			bold:   paragraph.bold,
			italic: paragraph.italic,
			color:  paragraph.color,
		}}
	}

	if len(runs) == 0 {
		builder.WriteString(`<a:endParaRPr lang="en-US" sz="`)
		builder.WriteString(strconv.Itoa(size))
		builder.WriteString(`"/></a:p>`)
		return builder.String()
	}

	for _, run := range runs {
		if run.text == "" {
			continue
		}
		lang := run.lang
		if lang == "" {
			lang = "en-US"
		}
		builder.WriteString(`<a:r><a:rPr lang="`)
		builder.WriteString(lang)
		builder.WriteString(`" sz="`)
		builder.WriteString(strconv.Itoa(size))
		builder.WriteString(`"`)
		if run.bold {
			builder.WriteString(` b="1"`)
		}
		if run.italic {
			builder.WriteString(` i="1"`)
		}
		builder.WriteString(`><a:latin typeface="`)
		builder.WriteString(escapeXML(firstNonEmpty(run.latinFont, fonts.Latin, "Arial")))
		builder.WriteString(`"/>`)
		if eastAsianFont := firstNonEmpty(run.eastAsianFont, eastAsianFontForLang(lang, fonts)); eastAsianFont != "" {
			builder.WriteString(`<a:ea typeface="`)
			builder.WriteString(escapeXML(eastAsianFont))
			builder.WriteString(`"/><a:cs typeface="`)
			builder.WriteString(escapeXML(eastAsianFont))
			builder.WriteString(`"/>`)
		}
		if color := firstNonEmpty(run.color, paragraph.color); color != "" {
			builder.WriteString(`<a:solidFill><a:srgbClr val="`)
			builder.WriteString(color)
			builder.WriteString(`"/></a:solidFill>`)
		}
		builder.WriteString(`</a:rPr><a:t>`)
		builder.WriteString(escapeXML(run.text))
		builder.WriteString(`</a:t></a:r>`)
	}

	builder.WriteString(`<a:endParaRPr lang="en-US" sz="`)
	builder.WriteString(strconv.Itoa(size))
	builder.WriteString(`"/></a:p>`)
	return builder.String()
}

func dereferenceString(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func escapeXML(value string) string {
	var buffer bytes.Buffer
	if err := xml.EscapeText(&buffer, []byte(value)); err != nil {
		// xml.EscapeText writes to a bytes.Buffer here, so an error is not
		// expected in practice. Avoid panicking in this library helper on the
		// unreachable path and fall back to the original value instead.
		return value
	}
	return buffer.String()
}

const xmlHeader = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>`

const groupShapeXML = `<p:nvGrpSpPr><p:cNvPr id="1" name="Group 1"/><p:cNvGrpSpPr/><p:nvPr/></p:nvGrpSpPr><p:grpSpPr><a:xfrm><a:off x="0" y="0"/><a:ext cx="0" cy="0"/><a:chOff x="0" y="0"/><a:chExt cx="0" cy="0"/></a:xfrm></p:grpSpPr>`

const rootRelationshipsXML = xmlHeader + `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="ppt/presentation.xml"/><Relationship Id="rId2" Type="http://schemas.openxmlformats.org/package/2006/relationships/metadata/core-properties" Target="docProps/core.xml"/><Relationship Id="rId3" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/extended-properties" Target="docProps/app.xml"/></Relationships>`

const corePropertiesXML = xmlHeader + `<cp:coreProperties xmlns:cp="http://schemas.openxmlformats.org/package/2006/metadata/core-properties" xmlns:dc="http://purl.org/dc/elements/1.1/" xmlns:dcterms="http://purl.org/dc/terms/" xmlns:dcmitype="http://purl.org/dc/dcmitype/" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance"><dc:title>Ulduar presentation</dc:title><dc:creator>ulduar</dc:creator><cp:lastModifiedBy>ulduar</cp:lastModifiedBy><dcterms:created xsi:type="dcterms:W3CDTF">2026-01-01T00:00:00Z</dcterms:created><dcterms:modified xsi:type="dcterms:W3CDTF">2026-01-01T00:00:00Z</dcterms:modified></cp:coreProperties>`

const slideRelationshipsXML = xmlHeader + `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slideLayout" Target="../slideLayouts/slideLayout1.xml"/></Relationships>`

const slideMasterRelationshipsXML = xmlHeader + `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slideLayout" Target="../slideLayouts/slideLayout1.xml"/><Relationship Id="rId2" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/theme" Target="../theme/theme1.xml"/></Relationships>`

const slideLayoutRelationshipsXML = xmlHeader + `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slideMaster" Target="../slideMasters/slideMaster1.xml"/></Relationships>`

const slideLayoutXML = xmlHeader + `<p:sldLayout xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" type="blank" preserve="1"><p:cSld name="Blank Layout"><p:spTree><p:nvGrpSpPr><p:cNvPr id="1" name="Group 1"/><p:cNvGrpSpPr/><p:nvPr/></p:nvGrpSpPr><p:grpSpPr><a:xfrm><a:off x="0" y="0"/><a:ext cx="0" cy="0"/><a:chOff x="0" y="0"/><a:chExt cx="0" cy="0"/></a:xfrm></p:grpSpPr></p:spTree></p:cSld><p:clrMapOvr><a:masterClrMapping/></p:clrMapOvr></p:sldLayout>`

const slideMasterXML = xmlHeader + `<p:sldMaster xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main"><p:cSld name="Ulduar Master"><p:spTree><p:nvGrpSpPr><p:cNvPr id="1" name="Group 1"/><p:cNvGrpSpPr/><p:nvPr/></p:nvGrpSpPr><p:grpSpPr><a:xfrm><a:off x="0" y="0"/><a:ext cx="0" cy="0"/><a:chOff x="0" y="0"/><a:chExt cx="0" cy="0"/></a:xfrm></p:grpSpPr></p:spTree></p:cSld><p:clrMap bg1="lt1" tx1="dk1" bg2="lt2" tx2="dk2" accent1="accent1" accent2="accent2" accent3="accent3" accent4="accent4" accent5="accent5" accent6="accent6" hlink="hlink" folHlink="folHlink"/><p:sldLayoutIdLst><p:sldLayoutId id="2147483649" r:id="rId1"/></p:sldLayoutIdLst><p:txStyles><p:titleStyle><a:lvl1pPr algn="l"><a:defRPr sz="2800" b="1"/></a:lvl1pPr></p:titleStyle><p:bodyStyle><a:lvl1pPr marL="342900" indent="-228600"><a:buChar char="•"/><a:defRPr sz="1600"/></a:lvl1pPr></p:bodyStyle><p:otherStyle><a:defPPr><a:defRPr sz="1600"/></a:defPPr></p:otherStyle></p:txStyles></p:sldMaster>`

const themeXML = xmlHeader + `<a:theme xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" name="Ulduar Theme"><a:themeElements><a:clrScheme name="Ulduar"><a:dk1><a:sysClr val="windowText" lastClr="000000"/></a:dk1><a:lt1><a:sysClr val="window" lastClr="FFFFFF"/></a:lt1><a:dk2><a:srgbClr val="1F1F1F"/></a:dk2><a:lt2><a:srgbClr val="F3F4F6"/></a:lt2><a:accent1><a:srgbClr val="2563EB"/></a:accent1><a:accent2><a:srgbClr val="059669"/></a:accent2><a:accent3><a:srgbClr val="D97706"/></a:accent3><a:accent4><a:srgbClr val="7C3AED"/></a:accent4><a:accent5><a:srgbClr val="DC2626"/></a:accent5><a:accent6><a:srgbClr val="0891B2"/></a:accent6><a:hlink><a:srgbClr val="0000FF"/></a:hlink><a:folHlink><a:srgbClr val="800080"/></a:folHlink></a:clrScheme><a:fontScheme name="Ulduar"><a:majorFont><a:latin typeface="Arial"/><a:ea typeface=""/><a:cs typeface=""/></a:majorFont><a:minorFont><a:latin typeface="Arial"/><a:ea typeface=""/><a:cs typeface=""/></a:minorFont></a:fontScheme><a:fmtScheme name="Ulduar"><a:fillStyleLst><a:solidFill><a:schemeClr val="phClr"/></a:solidFill><a:gradFill rotWithShape="1"><a:gsLst><a:gs pos="0"><a:schemeClr val="phClr"><a:tint val="50000"/><a:satMod val="300000"/></a:schemeClr></a:gs><a:gs pos="35000"><a:schemeClr val="phClr"><a:tint val="37000"/><a:satMod val="300000"/></a:schemeClr></a:gs><a:gs pos="100000"><a:schemeClr val="phClr"><a:tint val="15000"/><a:satMod val="350000"/></a:schemeClr></a:gs></a:gsLst><a:lin ang="16200000" scaled="1"/></a:gradFill><a:gradFill rotWithShape="1"><a:gsLst><a:gs pos="0"><a:schemeClr val="phClr"><a:shade val="51000"/><a:satMod val="130000"/></a:schemeClr></a:gs><a:gs pos="80000"><a:schemeClr val="phClr"><a:shade val="93000"/><a:satMod val="130000"/></a:schemeClr></a:gs><a:gs pos="100000"><a:schemeClr val="phClr"><a:shade val="94000"/><a:satMod val="135000"/></a:schemeClr></a:gs></a:gsLst><a:lin ang="16200000" scaled="0"/></a:gradFill></a:fillStyleLst><a:lineStyleLst><a:ln w="9525" cap="flat" cmpd="sng" algn="ctr"><a:solidFill><a:schemeClr val="phClr"/></a:solidFill><a:prstDash val="solid"/></a:ln><a:ln w="25400" cap="flat" cmpd="sng" algn="ctr"><a:solidFill><a:schemeClr val="phClr"/></a:solidFill><a:prstDash val="solid"/></a:ln><a:ln w="38100" cap="flat" cmpd="sng" algn="ctr"><a:solidFill><a:schemeClr val="phClr"/></a:solidFill><a:prstDash val="solid"/></a:ln></a:lineStyleLst><a:effectStyleLst><a:effectStyle><a:effectLst/></a:effectStyle><a:effectStyle><a:effectLst/></a:effectStyle><a:effectStyle><a:effectLst><a:outerShdw blurRad="57150" dist="19050" dir="5400000" algn="ctr" rotWithShape="0"><a:srgbClr val="000000"><a:alpha val="38000"/></a:srgbClr></a:outerShdw></a:effectLst></a:effectStyle></a:effectStyleLst><a:bgFillStyleLst><a:solidFill><a:schemeClr val="phClr"/></a:solidFill><a:solidFill><a:schemeClr val="phClr"><a:tint val="95000"/><a:satMod val="170000"/></a:schemeClr></a:solidFill><a:gradFill rotWithShape="1"><a:gsLst><a:gs pos="0"><a:schemeClr val="phClr"><a:tint val="93000"/><a:satMod val="150000"/><a:shade val="98000"/><a:lumMod val="102000"/></a:schemeClr></a:gs><a:gs pos="50000"><a:schemeClr val="phClr"><a:tint val="98000"/><a:satMod val="130000"/><a:shade val="90000"/><a:lumMod val="103000"/></a:schemeClr></a:gs><a:gs pos="100000"><a:schemeClr val="phClr"><a:shade val="63000"/><a:satMod val="120000"/></a:schemeClr></a:gs></a:gsLst><a:path path="circle"><a:fillToRect l="50000" t="-80000" r="50000" b="180000"/></a:path></a:gradFill></a:bgFillStyleLst></a:fmtScheme></a:themeElements><a:objectDefaults/><a:extraClrSchemeLst/></a:theme>`
