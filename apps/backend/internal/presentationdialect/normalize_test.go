package presentationdialect

import (
	"strings"
	"testing"
)

func testStringPtr(value string) *string {
	return &value
}

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
						Text: testStringPtr(" Opening summary. "),
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
								Text:        testStringPtr(" Keep the format small. "),
								Attribution: testStringPtr(" Design notes "),
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
								Text: testStringPtr("Only a paragraph"),
							},
						},
					},
				},
			},
			wantErr: `slides[0].blocks must contain at least 1 bullet_list or numbered_list block`,
		},
		{
			name: "title rejects empty blocks field",
			document: Document{
				Version: VersionV1,
				Slides: []Slide{
					{
						Layout: LayoutTitle,
						Title:  "Hello",
						Blocks: []Block{},
					},
				},
			},
			wantErr: `slides[0].blocks is not supported for "title" slides`,
		},
		{
			name: "title bullets rejects empty subtitle field",
			document: Document{
				Version: VersionV1,
				Slides: []Slide{
					{
						Layout:   LayoutTitleBullets,
						Title:    "Overview",
						Subtitle: testStringPtr(""),
						Blocks: []Block{
							{
								Type:  BlockTypeBulletList,
								Items: []string{"One"},
							},
						},
					},
				},
			},
			wantErr: `slides[0].subtitle is not supported for "title_bullets" slides`,
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
			name: "two column rejects empty blocks field",
			document: Document{
				Version: VersionV1,
				Slides: []Slide{
					{
						Layout: LayoutTwoColumn,
						Title:  "Compare",
						Blocks: []Block{},
						Columns: []Column{
							{
								Blocks: []Block{
									{
										Type:  BlockTypeBulletList,
										Items: []string{"One"},
									},
								},
							},
							{
								Blocks: []Block{
									{
										Type:  BlockTypeBulletList,
										Items: []string{"Two"},
									},
								},
							},
						},
					},
				},
			},
			wantErr: `slides[0].blocks is not supported for "two_column" slides`,
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
		{
			name: "paragraph rejects empty items field",
			document: Document{
				Version: VersionV1,
				Slides: []Slide{
					{
						Layout: LayoutClosing,
						Title:  "Wrap up",
						Blocks: []Block{
							{
								Type:  BlockTypeParagraph,
								Text:  testStringPtr("Summary"),
								Items: []string{},
							},
						},
					},
				},
			},
			wantErr: `slides[0].blocks[0].items is not supported for paragraph blocks`,
		},
		{
			name: "numbered list rejects empty text field",
			document: Document{
				Version: VersionV1,
				Slides: []Slide{
					{
						Layout: LayoutClosing,
						Title:  "Wrap up",
						Blocks: []Block{
							{
								Type:  BlockTypeNumberedList,
								Text:  testStringPtr(""),
								Items: []string{"One"},
							},
						},
					},
				},
			},
			wantErr: `slides[0].blocks[0].text is not supported for "numbered_list" blocks`,
		},
		{
			name: "quote rejects empty header field",
			document: Document{
				Version: VersionV1,
				Slides: []Slide{
					{
						Layout: LayoutClosing,
						Title:  "Wrap up",
						Blocks: []Block{
							{
								Type:   BlockTypeQuote,
								Text:   testStringPtr("Keep it small"),
								Header: []string{},
							},
						},
					},
				},
			},
			wantErr: `slides[0].blocks[0].header and slides[0].blocks[0].rows are only supported for table blocks`,
		},
		{
			name: "table rejects empty attribution field",
			document: Document{
				Version: VersionV1,
				Slides: []Slide{
					{
						Layout: LayoutTable,
						Title:  "Metrics",
						Blocks: []Block{
							{
								Type:        BlockTypeTable,
								Header:      []string{"Metric"},
								Rows:        [][]string{{"Latency"}},
								Attribution: testStringPtr(""),
							},
						},
					},
				},
			},
			wantErr: `slides[0].blocks[0].attribution is only supported for quote blocks`,
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

func TestParseJSONRejectsDisallowedEmptyFields(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		payload string
		wantErr string
	}{
		{
			name: "title with empty blocks field",
			payload: `{
				"version": "v1",
				"slides": [
					{
						"layout": "title",
						"title": "Hello",
						"blocks": []
					}
				]
			}`,
			wantErr: `slides[0].blocks is not supported for "title" slides`,
		},
		{
			name: "title bullets with empty subtitle field",
			payload: `{
				"version": "v1",
				"slides": [
					{
						"layout": "title_bullets",
						"title": "Overview",
						"subtitle": "",
						"blocks": [
							{
								"type": "bullet_list",
								"items": ["One"]
							}
						]
					}
				]
			}`,
			wantErr: `slides[0].subtitle is not supported for "title_bullets" slides`,
		},
		{
			name: "paragraph with empty items field",
			payload: `{
				"version": "v1",
				"slides": [
					{
						"layout": "closing",
						"title": "Done",
						"blocks": [
							{
								"type": "paragraph",
								"text": "Summary",
								"items": []
							}
						]
					}
				]
			}`,
			wantErr: `slides[0].blocks[0].items is not supported for paragraph blocks`,
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			_, err := ParseJSON([]byte(test.payload))
			if err == nil {
				t.Fatal("ParseJSON() error = nil, want error")
			}
			if err.Error() != test.wantErr {
				t.Fatalf("ParseJSON() error = %q, want %q", err.Error(), test.wantErr)
			}
		})
	}
}

func TestParseJSONRejectsNullFields(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		payload string
		wantErr string
	}{
		{
			name: "title bullets subtitle null",
			payload: `{
				"version": "v1",
				"slides": [
					{
						"layout": "title_bullets",
						"title": "Overview",
						"subtitle": null,
						"blocks": [
							{
								"type": "bullet_list",
								"items": ["One"]
							}
						]
					}
				]
			}`,
			wantErr: `slides[0].subtitle must not be null`,
		},
		{
			name: "title blocks null",
			payload: `{
				"version": "v1",
				"slides": [
					{
						"layout": "title",
						"title": "Hello",
						"blocks": null
					}
				]
			}`,
			wantErr: `slides[0].blocks must not be null`,
		},
		{
			name: "title bullets required blocks null",
			payload: `{
				"version": "v1",
				"slides": [
					{
						"layout": "title_bullets",
						"title": "Overview",
						"blocks": null
					}
				]
			}`,
			wantErr: `slides[0].blocks must not be null`,
		},
		{
			name: "two column required columns null",
			payload: `{
				"version": "v1",
				"slides": [
					{
						"layout": "two_column",
						"title": "Compare",
						"columns": null
					}
				]
			}`,
			wantErr: `slides[0].columns must not be null`,
		},
		{
			name: "paragraph items null",
			payload: `{
				"version": "v1",
				"slides": [
					{
						"layout": "closing",
						"title": "Done",
						"blocks": [
							{
								"type": "paragraph",
								"text": "Summary",
								"items": null
							}
						]
					}
				]
			}`,
			wantErr: `slides[0].blocks[0].items must not be null`,
		},
		{
			name: "column blocks null",
			payload: `{
				"version": "v1",
				"slides": [
					{
						"layout": "two_column",
						"title": "Compare",
						"columns": [
							{
								"heading": "Left",
								"blocks": null
							},
							{
								"blocks": [
									{
										"type": "bullet_list",
										"items": ["Two"]
									}
								]
							}
						]
					}
				]
			}`,
			wantErr: `slides[0].columns[0].blocks must not be null`,
		},
		{
			name: "table row cell null",
			payload: `{
				"version": "v1",
				"slides": [
					{
						"layout": "table",
						"title": "Metrics",
						"blocks": [
							{
								"type": "table",
								"header": ["Metric", "Value"],
								"rows": [["Latency", null]]
							}
						]
					}
				]
			}`,
			wantErr: `slides[0].blocks[0].rows[0][1] must not be null`,
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			_, err := ParseJSON([]byte(test.payload))
			if err == nil {
				t.Fatal("ParseJSON() error = nil, want error")
			}
			if err.Error() != test.wantErr {
				t.Fatalf("ParseJSON() error = %q, want %q", err.Error(), test.wantErr)
			}
		})
	}
}
