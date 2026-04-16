package presentationdialect

import (
	"strings"
	"testing"
)

func TestNormalizeAppliesDefaultsAndTrimsValues(t *testing.T) {
	t.Parallel()

	document, err := Normalize(Document{
		Version: " v1 ",
		Slides: []Slide{
			{
				Layout: LayoutTitleBullets,
				Title:  " Product overview ",
				Blocks: []Block{
					{
						Type: BlockTypeParagraph,
						Text: " Opening summary. ",
					},
					{
						Type:  BlockTypeBulletList,
						Items: []string{" Fast setup ", " Deterministic compiler "},
					},
				},
			},
			{
				Layout: LayoutTwoColumn,
				Title:  " Compare options ",
				Columns: []Column{
					{
						Heading: " Option A ",
						Blocks: []Block{
							{
								Type:  BlockTypeNumberedList,
								Items: []string{" First ", " Second "},
							},
						},
					},
					{
						Blocks: []Block{
							{
								Type:        BlockTypeQuote,
								Text:        " Keep the format small. ",
								Attribution: " Design notes ",
							},
						},
					},
				},
			},
			{
				Layout: LayoutClosing,
				Title:  " Thanks ",
			},
		},
	})
	if err != nil {
		t.Fatalf("Normalize() error = %v", err)
	}

	if document.Version != VersionV1 {
		t.Fatalf("document.Version = %q, want %q", document.Version, VersionV1)
	}
	if document.SlideSize != SlideSize16By9 {
		t.Fatalf("document.SlideSize = %q, want %q", document.SlideSize, SlideSize16By9)
	}
	if document.Slides[0].Title != "Product overview" {
		t.Fatalf("document.Slides[0].Title = %q", document.Slides[0].Title)
	}
	if got := document.Slides[0].Blocks[1].Items[0]; got != "Fast setup" {
		t.Fatalf("document.Slides[0].Blocks[1].Items[0] = %q", got)
	}
	if got := document.Slides[1].Columns[0].Heading; got != "Option A" {
		t.Fatalf("document.Slides[1].Columns[0].Heading = %q", got)
	}
	if len(document.Slides[2].Blocks) != 0 {
		t.Fatalf("len(document.Slides[2].Blocks) = %d, want 0", len(document.Slides[2].Blocks))
	}
	if len(document.Slides[2].Columns) != 0 {
		t.Fatalf("len(document.Slides[2].Columns) = %d, want 0", len(document.Slides[2].Columns))
	}
}

func TestNormalizeRejectsInvalidDocuments(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		document Document
		wantErr  string
	}{
		{
			name: "unsupported layout",
			document: Document{
				Version: VersionV1,
				Slides: []Slide{
					{
						Layout: "agenda",
						Title:  "Agenda",
					},
				},
			},
			wantErr: `slides[0].layout must be one of: title, section, title_bullets, two_column, table, closing`,
		},
		{
			name: "unsupported block type",
			document: Document{
				Version: VersionV1,
				Slides: []Slide{
					{
						Layout: LayoutClosing,
						Title:  "Done",
						Blocks: []Block{
							{
								Type: "chart",
							},
						},
					},
				},
			},
			wantErr: `slides[0].blocks[0].type must be one of: paragraph, bullet_list, numbered_list, table, quote`,
		},
		{
			name: "invalid slide size",
			document: Document{
				Version:   VersionV1,
				SlideSize: "4:3",
				Slides: []Slide{
					{
						Layout: LayoutTitle,
						Title:  "Hello",
					},
				},
			},
			wantErr: `slideSize must be "16:9"`,
		},
		{
			name: "title bullets requires list block",
			document: Document{
				Version: VersionV1,
				Slides: []Slide{
					{
						Layout: LayoutTitleBullets,
						Title:  "Overview",
						Blocks: []Block{
							{
								Type: BlockTypeParagraph,
								Text: "Only a paragraph",
							},
						},
					},
				},
			},
			wantErr: `slides[0].blocks must contain at least 1 bullet_list or numbered_list block`,
		},
		{
			name: "two column requires exactly two columns",
			document: Document{
				Version: VersionV1,
				Slides: []Slide{
					{
						Layout: LayoutTwoColumn,
						Title:  "Compare",
						Columns: []Column{
							{
								Blocks: []Block{
									{
										Type:  BlockTypeBulletList,
										Items: []string{"One"},
									},
								},
							},
						},
					},
				},
			},
			wantErr: `slides[0].columns must contain exactly 2 columns`,
		},
		{
			name: "table row width mismatch",
			document: Document{
				Version: VersionV1,
				Slides: []Slide{
					{
						Layout: LayoutTable,
						Title:  "Metrics",
						Blocks: []Block{
							{
								Type:   BlockTypeTable,
								Header: []string{"Metric", "Value"},
								Rows: [][]string{
									{"Latency"},
								},
							},
						},
					},
				},
			},
			wantErr: `slides[0].blocks[0].rows[0] must contain exactly 2 cells`,
		},
		{
			name: "table block not allowed in closing layout",
			document: Document{
				Version: VersionV1,
				Slides: []Slide{
					{
						Layout: LayoutClosing,
						Title:  "Wrap up",
						Blocks: []Block{
							{
								Type:   BlockTypeTable,
								Header: []string{"A"},
								Rows:   [][]string{{"B"}},
							},
						},
					},
				},
			},
			wantErr: `slides[0].blocks[0].type "table" is not supported in this layout`,
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			_, err := Normalize(test.document)
			if err == nil {
				t.Fatal("Normalize() error = nil, want error")
			}
			if err.Error() != test.wantErr {
				t.Fatalf("Normalize() error = %q, want %q", err.Error(), test.wantErr)
			}
		})
	}
}

func TestParseJSONRejectsUnknownFields(t *testing.T) {
	t.Parallel()

	_, err := ParseJSON([]byte(`{
		"version": "v1",
		"slides": [
			{
				"layout": "title",
				"title": "Hello",
				"unexpected": true
			}
		]
	}`))
	if err == nil {
		t.Fatal("ParseJSON() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("ParseJSON() error = %q, want unknown field error", err.Error())
	}
}
