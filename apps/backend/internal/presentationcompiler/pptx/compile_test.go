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
		"ppt/slideLayouts/slideLayout1.xml",
		"ppt/theme/theme1.xml",
		"ppt/slides/slide1.xml",
		"ppt/slides/slide2.xml",
		"ppt/slides/slide3.xml",
		"ppt/slides/slide4.xml",
		"ppt/slides/slide5.xml",
		"ppt/slides/slide6.xml",
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

func readZIPEntries(t *testing.T, data []byte) map[string]string {
	t.Helper()

	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("zip.NewReader() error = %v", err)
	}

	entries := make(map[string]string, len(reader.File))
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
		entries[file.Name] = content.String()
	}

	return entries
}

func assertContains(t *testing.T, content string, needle string) {
	t.Helper()

	if !strings.Contains(content, needle) {
		t.Fatalf("content missing %q\ncontent=%s", needle, content)
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
