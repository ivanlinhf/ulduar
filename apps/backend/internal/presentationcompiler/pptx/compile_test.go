package pptx

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"io"
	"strings"
	"testing"

	"github.com/ivanlin/ulduar/apps/backend/internal/presentationdialect"
)

func TestCompileProducesDeterministicPPTXForSupportedLayouts(t *testing.T) {
	t.Parallel()

	subtitle := "FY2026 Q1"
	sectionSubtitle := "Key messages"
	closingSubtitle := "Questions and discussion"

	document := presentationdialect.Document{
		Version: presentationdialect.VersionV1,
		Slides: []presentationdialect.Slide{
			{
				Layout:   presentationdialect.LayoutTitle,
				Title:    "Quarterly Business Review",
				Subtitle: &subtitle,
			},
			{
				Layout:   presentationdialect.LayoutSection,
				Title:    "Executive summary",
				Subtitle: &sectionSubtitle,
			},
			{
				Layout: presentationdialect.LayoutTitleBullets,
				Title:  "Highlights",
				Blocks: []presentationdialect.Block{
					{
						Type: presentationdialect.BlockTypeParagraph,
						Text: stringPtr("The quarter focused on reliability and launch readiness."),
					},
					{
						Type:  presentationdialect.BlockTypeBulletList,
						Items: []string{"Launch readiness improved from 61% to 88%", "Median response latency decreased by 23%"},
					},
				},
			},
			{
				Layout: presentationdialect.LayoutTwoColumn,
				Title:  "Opportunities and risks",
				Columns: []presentationdialect.Column{
					{
						Heading: "Opportunities",
						Blocks: []presentationdialect.Block{
							{
								Type:  presentationdialect.BlockTypeNumberedList,
								Items: []string{"Expand into two adjacent buyer segments", "Bundle onboarding services with premium tier"},
							},
						},
					},
					{
						Heading: "Risks",
						Blocks: []presentationdialect.Block{
							{
								Type:        presentationdialect.BlockTypeQuote,
								Text:        stringPtr("Customers are optimistic, but they expect a smoother rollout."),
								Attribution: stringPtr("March 2026 customer advisory board"),
							},
						},
					},
				},
			},
			{
				Layout: presentationdialect.LayoutTable,
				Title:  "KPI snapshot",
				Blocks: []presentationdialect.Block{
					{
						Type:   presentationdialect.BlockTypeTable,
						Header: []string{"Metric", "Current", "Target"},
						Rows: [][]string{
							{"Net revenue retention", "109%", "110%"},
							{"Gross margin", "37%", "35%"},
						},
					},
				},
			},
			{
				Layout:   presentationdialect.LayoutClosing,
				Title:    "Thank you",
				Subtitle: &closingSubtitle,
				Blocks: []presentationdialect.Block{
					{
						Type: presentationdialect.BlockTypeParagraph,
						Text: stringPtr("Next milestone: board-ready deck compilation after planner approval."),
					},
				},
			},
		},
	}

	first, err := Compile(document)
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}

	second, err := Compile(document)
	if err != nil {
		t.Fatalf("Compile() second call error = %v", err)
	}

	if !bytes.Equal(first, second) {
		t.Fatal("Compile() output is not deterministic")
	}

	entries := readZIPEntries(t, first)

	requiredEntries := []string{
		"[Content_Types].xml",
		"_rels/.rels",
		"docProps/app.xml",
		"docProps/core.xml",
		"ppt/presentation.xml",
		"ppt/_rels/presentation.xml.rels",
		"ppt/slideMasters/slideMaster1.xml",
		"ppt/slideMasters/_rels/slideMaster1.xml.rels",
		"ppt/slideLayouts/slideLayout1.xml",
		"ppt/slideLayouts/_rels/slideLayout1.xml.rels",
		"ppt/theme/theme1.xml",
		"ppt/slides/slide1.xml",
		"ppt/slides/slide2.xml",
		"ppt/slides/slide3.xml",
		"ppt/slides/slide4.xml",
		"ppt/slides/slide5.xml",
		"ppt/slides/slide6.xml",
		"ppt/slides/_rels/slide1.xml.rels",
		"ppt/slides/_rels/slide2.xml.rels",
		"ppt/slides/_rels/slide3.xml.rels",
		"ppt/slides/_rels/slide4.xml.rels",
		"ppt/slides/_rels/slide5.xml.rels",
		"ppt/slides/_rels/slide6.xml.rels",
	}
	for _, name := range requiredEntries {
		if _, ok := entries[name]; !ok {
			t.Fatalf("missing pptx entry %q", name)
		}
	}
	for name, content := range entries {
		if strings.HasSuffix(name, ".xml") || strings.HasSuffix(name, ".rels") {
			assertWellFormedXML(t, name, content)
		}
	}

	assertContains(t, entries["ppt/presentation.xml"], `cx="12192000" cy="6858000"`)
	assertContains(t, entries["ppt/presentation.xml"], `<p:sldId id="256" r:id="rId2"/>`)
	assertContains(t, entries["ppt/presentation.xml"], `<p:sldId id="261" r:id="rId7"/>`)

	assertContains(t, entries["ppt/slides/slide1.xml"], `Quarterly Business Review`)
	assertContains(t, entries["ppt/slides/slide1.xml"], `FY2026 Q1`)
	assertContains(t, entries["ppt/slides/slide1.xml"], `<a:latin typeface="Arial"/><a:solidFill><a:srgbClr val="666666"/></a:solidFill>`)
	assertContains(t, entries["ppt/slides/slide2.xml"], `Executive summary`)
	assertContains(t, entries["ppt/slides/slide2.xml"], `Key messages`)
	assertContains(t, entries["ppt/slides/slide3.xml"], `Launch readiness improved from 61% to 88%`)
	assertContains(t, entries["ppt/slides/slide4.xml"], `Opportunities`)
	assertContains(t, entries["ppt/slides/slide4.xml"], `March 2026 customer advisory board`)
	assertContains(t, entries["ppt/slides/slide5.xml"], `Metric | Current | Target`)
	assertContains(t, entries["ppt/slides/slide5.xml"], `Net revenue retention | 109% | 110%`)
	assertContains(t, entries["ppt/slides/slide6.xml"], `Thank you`)
	assertContains(t, entries["ppt/slides/slide6.xml"], `Next milestone: board-ready deck compilation after planner approval.`)

	assertContains(t, entries["ppt/slides/_rels/slide1.xml.rels"], `../slideLayouts/slideLayout1.xml`)
	assertContains(t, entries["docProps/app.xml"], `<Slides>6</Slides>`)
}

func TestCompileRejectsInvalidDocuments(t *testing.T) {
	t.Parallel()

	_, err := Compile(presentationdialect.Document{
		Version: presentationdialect.VersionV1,
		Slides: []presentationdialect.Slide{
			{
				Layout: "agenda",
				Title:  "Agenda",
			},
		},
	})
	if err == nil {
		t.Fatal("Compile() error = nil, want validation error")
	}
	if !strings.Contains(err.Error(), `slides[0].layout must be one of: title, section, title_bullets, two_column, table, closing`) {
		t.Fatalf("Compile() error = %q", err)
	}
}

func TestCompileProducesPPTXForV2SemanticLayouts(t *testing.T) {
	t.Parallel()

	document := presentationdialect.Document{
		Version:       presentationdialect.VersionV2,
		ThemePresetID: stringPtr(presentationdialect.ThemePresetTravelEditorial),
		Slides: []presentationdialect.Slide{
			{
				Layout:   presentationdialect.LayoutCoverHero,
				Title:    "Kyoto in Four Days",
				Subtitle: stringPtr("Travel Editorial"),
				Blocks: []presentationdialect.Block{
					{
						Type:     presentationdialect.BlockTypeImage,
						AssetRef: stringPtr("attachment:cover-photo"),
						Caption:  stringPtr("Autumn light over Arashiyama"),
					},
					{
						Type: presentationdialect.BlockTypeRichText,
						Spans: []presentationdialect.TextSpan{
							{Text: "A calm city break with "},
							{Text: "京都", Lang: "ja", Emphasis: "accent"},
						},
					},
				},
			},
			{
				Layout: presentationdialect.LayoutComparisonCards,
				Title:  "Stay options",
				Blocks: []presentationdialect.Block{
					{
						Type:  presentationdialect.BlockTypeCard,
						Title: stringPtr("Gion"),
						Body:  stringPtr("Best for walkable evenings and classic streetscapes."),
					},
					{
						Type:  presentationdialect.BlockTypeCard,
						Title: stringPtr("Arashiyama"),
						Body:  stringPtr("Best for a slower pace and riverside views."),
					},
				},
			},
		},
	}

	data, err := Compile(document)
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}

	entries := readZIPEntries(t, data)
	assertContains(t, entries["ppt/slides/slide1.xml"], `Kyoto in Four Days`)
	assertContains(t, entries["ppt/slides/slide1.xml"], `Image asset: attachment:cover-photo`)
	assertContains(t, entries["ppt/slides/slide1.xml"], `Autumn light over Arashiyama`)
	assertContains(t, entries["ppt/slides/slide2.xml"], `Stay options`)
	assertContains(t, entries["ppt/slides/slide2.xml"], `Gion`)
	assertContains(t, entries["ppt/slides/slide2.xml"], `Arashiyama`)
}

func TestCompileWithAssetsProducesDeterministicPPTXForV2SemanticLayouts(t *testing.T) {
	t.Parallel()

	document := presentationdialect.Document{
		Version:       presentationdialect.VersionV2,
		ThemePresetID: stringPtr(presentationdialect.ThemePresetTravelEditorial),
		Slides: []presentationdialect.Slide{
			{
				Layout:   presentationdialect.LayoutCoverHero,
				Title:    "Kyoto in Four Days",
				Subtitle: stringPtr("Travel Editorial"),
				Blocks: []presentationdialect.Block{
					{Type: presentationdialect.BlockTypeImage, AssetRef: stringPtr("attachment:cover-photo"), Caption: stringPtr("Autumn light over Arashiyama")},
					{Type: presentationdialect.BlockTypeRichText, Spans: []presentationdialect.TextSpan{{Text: "A calm city break with "}, {Text: "京都", Lang: "ja", Emphasis: "accent"}}},
				},
			},
			{
				Layout: presentationdialect.LayoutChapterDivider,
				Title:  "Neighborhoods",
				Blocks: []presentationdialect.Block{
					{Type: presentationdialect.BlockTypeImage, AssetRef: stringPtr("theme:hero-image")},
					{Type: presentationdialect.BlockTypeBadge, Text: stringPtr("Curated stays")},
				},
			},
			{
				Layout: presentationdialect.LayoutCardGrid,
				Title:  "Stay options",
				Blocks: []presentationdialect.Block{
					{Type: presentationdialect.BlockTypeCard, Title: stringPtr("Gion"), Body: stringPtr("Best for walkable evenings and classic streetscapes."), AssetRef: stringPtr("attachment:cover-photo")},
					{Type: presentationdialect.BlockTypeCard, Title: stringPtr("Arashiyama"), Body: stringPtr("Best for a slower pace and riverside views.")},
				},
			},
			{
				Layout: presentationdialect.LayoutComparisonCards,
				Title:  "Compare districts",
				Blocks: []presentationdialect.Block{
					{Type: presentationdialect.BlockTypeCard, Title: stringPtr("Gion"), Body: stringPtr("Historic lanes and compact dining clusters.")},
					{Type: presentationdialect.BlockTypeCard, Title: stringPtr("Arashiyama"), Body: stringPtr("River views, bamboo groves, slower rhythm.")},
				},
			},
			{
				Layout: presentationdialect.LayoutTimelineItinerary,
				Title:  "Two-day sample",
				Blocks: []presentationdialect.Block{
					{Type: presentationdialect.BlockTypeCard, Label: stringPtr("Day 1"), Title: stringPtr("East Kyoto"), Body: stringPtr("Kiyomizu-dera, Gion, and evening strolls.")},
					{Type: presentationdialect.BlockTypeCard, Label: stringPtr("Day 2"), Title: stringPtr("West Kyoto"), Body: stringPtr("Arashiyama, river walk, and scenic cafés.")},
				},
			},
			{
				Layout: presentationdialect.LayoutTable,
				Title:  "Budget snapshot",
				Blocks: []presentationdialect.Block{
					{Type: presentationdialect.BlockTypeCallout, Title: stringPtr("Planning note"), Body: stringPtr("Reserve rail and temple tickets ahead of peak foliage weekends.")},
					{Type: presentationdialect.BlockTypeTable, Header: []string{"Metric", "Value", "Notes"}, Rows: [][]string{{"Hotel", "$220", "Central boutique stay"}, {"Transit", "$45", "IC card + rail segments"}}},
				},
			},
		},
	}
	assets := map[string]CompileAsset{
		"attachment:cover-photo": {Filename: "cover.png", MediaType: "image/png", Data: testPNGData()},
		"theme:hero-image":       {Filename: "hero.png", MediaType: "image/png", Data: testPNGData()},
	}

	first, err := CompileWithAssets(document, assets)
	if err != nil {
		t.Fatalf("CompileWithAssets() error = %v", err)
	}
	second, err := CompileWithAssets(document, assets)
	if err != nil {
		t.Fatalf("CompileWithAssets() second call error = %v", err)
	}
	if !bytes.Equal(first, second) {
		t.Fatal("CompileWithAssets() output is not deterministic")
	}

	entries := readZIPEntries(t, first)
	binaryEntries := readZIPBinaryEntries(t, first)

	assertContains(t, entries["ppt/theme/theme1.xml"], `Travel Editorial`)
	assertContains(t, entries["ppt/theme/theme1.xml"], `Noto Serif CJK JP`)
	assertContains(t, entries["ppt/slides/slide1.xml"], `<p:pic>`)
	assertContains(t, entries["ppt/slides/slide1.xml"], `lang="ja-JP"`)
	assertContains(t, entries["ppt/slides/slide1.xml"], `Noto Serif CJK JP`)
	assertContains(t, entries["ppt/slides/_rels/slide1.xml.rels"], `relationships/image`)
	assertContains(t, entries["ppt/slides/slide3.xml"], `<p:pic>`)
	assertContains(t, entries["ppt/slides/slide4.xml"], `Compare districts`)
	assertContains(t, entries["ppt/slides/slide5.xml"], `Day 1`)
	assertContains(t, entries["ppt/slides/slide6.xml"], `<a:tbl>`)
	assertContains(t, entries["ppt/slides/slide6.xml"], `Planning note`)
	assertContains(t, entries["ppt/slides/slide6.xml"], `Central boutique stay`)

	if _, ok := binaryEntries["ppt/media/image1.png"]; !ok {
		t.Fatal("missing ppt/media/image1.png")
	}
	if _, ok := binaryEntries["ppt/media/image2.png"]; !ok {
		t.Fatal("missing ppt/media/image2.png")
	}
}

func TestCompileWithAssetsUsesPresetAccentForRichText(t *testing.T) {
	t.Parallel()

	document := presentationdialect.Document{
		Version:       presentationdialect.VersionV2,
		ThemePresetID: stringPtr(presentationdialect.ThemePresetGeneralClean),
		Slides: []presentationdialect.Slide{{
			Layout: presentationdialect.LayoutClosing,
			Title:  "Status",
			Blocks: []presentationdialect.Block{{
				Type: presentationdialect.BlockTypeRichText,
				Spans: []presentationdialect.TextSpan{
					{Text: "All systems "},
					{Text: "nominal", Emphasis: "accent"},
				},
			}},
		}},
	}

	data, err := CompileWithAssets(document, nil)
	if err != nil {
		t.Fatalf("CompileWithAssets() error = %v", err)
	}

	entries := readZIPEntries(t, data)
	assertContains(t, entries["ppt/slides/slide1.xml"], `srgbClr val="2563EB"`)
	assertNotContains(t, entries["ppt/slides/slide1.xml"], `srgbClr val="A45C40"`)
}

func TestCompileV2HandlesEmptyCardGrid(t *testing.T) {
	t.Parallel()

	document := presentationdialect.Document{
		Version:       presentationdialect.VersionV2,
		ThemePresetID: stringPtr(presentationdialect.ThemePresetGeneralClean),
		Slides: []presentationdialect.Slide{{
			Layout: presentationdialect.LayoutCardGrid,
			Title:  "Empty grid",
		}},
	}

	data, err := compileV2(document, nil)
	if err != nil {
		t.Fatalf("compileV2() error = %v", err)
	}

	entries := readZIPEntries(t, data)
	assertContains(t, entries["ppt/slides/slide1.xml"], `Empty grid`)
}

func TestCompileWithAssetsReturnsUnprefixedMediaErrors(t *testing.T) {
	t.Parallel()

	assetRef := "attachment:unsupported"
	document := presentationdialect.Document{
		Version: presentationdialect.VersionV2,
		Slides: []presentationdialect.Slide{{
			Layout: presentationdialect.LayoutRecommendation,
			Title:  "Unsupported image",
			Blocks: []presentationdialect.Block{{
				Type:     presentationdialect.BlockTypeImage,
				AssetRef: &assetRef,
			}, {
				Type:  presentationdialect.BlockTypeCallout,
				Title: stringPtr("Note"),
				Body:  stringPtr("Body"),
			}},
		}},
	}

	_, err := CompileWithAssets(document, map[string]CompileAsset{
		assetRef: {
			Filename:  "cover.gif",
			MediaType: "image/gif",
			Data:      testPNGData(),
		},
	})
	if err == nil {
		t.Fatal("CompileWithAssets() error = nil, want error")
	}
	assertContains(t, err.Error(), `unsupported pptx image media type "image/gif"`)
	assertNotContains(t, err.Error(), `compile pptx:`)
}

func TestEnsureMediaPartReusesOwnedAssetBytes(t *testing.T) {
	t.Parallel()

	builder := &v2PackageBuilder{
		assets: map[string]CompileAsset{
			"attachment:cover": {
				Filename:  "cover.png",
				MediaType: "image/png",
				Data:      testPNGData(),
			},
		},
		mediaByAssetRef: make(map[string]mediaPart),
	}

	part, err := builder.ensureMediaPart("attachment:cover")
	if err != nil {
		t.Fatalf("ensureMediaPart() error = %v", err)
	}
	asset := builder.assets["attachment:cover"]
	if len(part.data) == 0 || len(asset.Data) == 0 {
		t.Fatal("expected non-empty media data")
	}
	if &part.data[0] != &asset.Data[0] {
		t.Fatal("ensureMediaPart() cloned asset bytes instead of reusing the owned copy")
	}
}

func TestRenderTextBoxWithFontsAddsEmptyParagraphWhenParagraphsMissing(t *testing.T) {
	t.Parallel()

	xml := renderTextBoxWithFonts(textBox{
		id:   1,
		name: "Background",
		x:    0,
		y:    0,
		cx:   slideWidthEMU,
		cy:   slideHeightEMU,
	}, defaultLegacyFonts())

	assertContains(t, xml, `<p:txBody>`)
	assertContains(t, xml, `<a:p>`)
}

func TestCompileWithoutAssetsPreservesCardImagePlaceholder(t *testing.T) {
	t.Parallel()

	assetRef := "attachment:cover-photo"
	document := presentationdialect.Document{
		Version: presentationdialect.VersionV2,
		Slides: []presentationdialect.Slide{{
			Layout: presentationdialect.LayoutCardGrid,
			Title:  "Stay options",
			Blocks: []presentationdialect.Block{
				{
					Type:     presentationdialect.BlockTypeCard,
					Title:    stringPtr("Gion"),
					Body:     stringPtr("Best for walkable evenings."),
					AssetRef: &assetRef,
				},
				{
					Type:  presentationdialect.BlockTypeCard,
					Title: stringPtr("Arashiyama"),
					Body:  stringPtr("Best for a slower pace."),
				},
			},
		}},
	}

	data, err := Compile(document)
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}

	entries := readZIPEntries(t, data)
	assertContains(t, entries["ppt/slides/slide1.xml"], `Image asset: attachment:cover-photo`)
}

func readZIPEntries(t *testing.T, data []byte) map[string]string {
	t.Helper()

	binaryEntries := readZIPBinaryEntries(t, data)
	entries := make(map[string]string, len(binaryEntries))
	for name, content := range binaryEntries {
		entries[name] = string(content)
	}

	return entries
}

func readZIPBinaryEntries(t *testing.T, data []byte) map[string][]byte {
	t.Helper()

	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("zip.NewReader() error = %v", err)
	}

	entries := make(map[string][]byte, len(reader.File))
	for _, file := range reader.File {
		rc, err := file.Open()
		if err != nil {
			t.Fatalf("file.Open(%q) error = %v", file.Name, err)
		}
		var content bytes.Buffer
		if _, err := content.ReadFrom(rc); err != nil {
			_ = rc.Close()
			t.Fatalf("ReadFrom(%q) error = %v", file.Name, err)
		}
		if err := rc.Close(); err != nil {
			t.Fatalf("Close(%q) error = %v", file.Name, err)
		}
		entries[file.Name] = content.Bytes()
	}

	return entries
}

func assertContains(t *testing.T, content string, needle string) {
	t.Helper()

	if !strings.Contains(content, needle) {
		t.Fatalf("content missing %q\ncontent=%s", needle, content)
	}
}

func assertNotContains(t *testing.T, content string, needle string) {
	t.Helper()

	if strings.Contains(content, needle) {
		t.Fatalf("content unexpectedly contains %q\ncontent=%s", needle, content)
	}
}

func assertWellFormedXML(t *testing.T, name string, content string) {
	t.Helper()

	decoder := xml.NewDecoder(strings.NewReader(content))
	for {
		if _, err := decoder.Token(); err != nil {
			if err == io.EOF {
				return
			}
			t.Fatalf("xml parse %q error = %v\ncontent=%s", name, err, content)
		}
	}
}

func stringPtr(value string) *string {
	return &value
}

func testPNGData() []byte {
	return []byte{
		0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a,
		0x00, 0x00, 0x00, 0x0d, 0x49, 0x48, 0x44, 0x52,
		0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
		0x08, 0x06, 0x00, 0x00, 0x00, 0x1f, 0x15, 0xc4,
		0x89, 0x00, 0x00, 0x00, 0x0d, 0x49, 0x44, 0x41,
		0x54, 0x78, 0x9c, 0x63, 0x00, 0x01, 0x00, 0x00,
		0x05, 0x00, 0x01, 0x0d, 0x0a, 0x2d, 0xb4, 0x00,
		0x00, 0x00, 0x00, 0x49, 0x45, 0x4e, 0x44, 0xae,
		0x42, 0x60, 0x82,
	}
}
