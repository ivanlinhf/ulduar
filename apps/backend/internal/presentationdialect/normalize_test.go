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

func TestNormalizeIsIdempotent(t *testing.T) {
	t.Parallel()

	document := Document{
		Version: " v1 ",
		Slides: []Slide{
			{
				Layout:   LayoutTitle,
				Title:    " Welcome ",
				Subtitle: testStringPtr(" FY2026 "),
			},
			{
				Layout: LayoutTitleBullets,
				Title:  " Overview ",
				Blocks: []Block{
					{
						Type: BlockTypeParagraph,
						Text: testStringPtr(" Summary "),
					},
					{
						Type:  BlockTypeBulletList,
						Items: []string{" One "},
					},
				},
			},
			{
				Layout: LayoutTwoColumn,
				Title:  " Compare ",
				Columns: []Column{
					{
						Blocks: []Block{
							{
								Type:  BlockTypeBulletList,
								Items: []string{" Left "},
							},
						},
					},
					{
						Blocks: []Block{
							{
								Type:        BlockTypeQuote,
								Text:        testStringPtr(" Right "),
								Attribution: testStringPtr(" Notes "),
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
	}

	normalized, err := Normalize(document)
	if err != nil {
		t.Fatalf("Normalize() first pass error = %v", err)
	}

	if err := Validate(normalized); err != nil {
		t.Fatalf("Validate() normalized document error = %v", err)
	}

	renormalized, err := Normalize(normalized)
	if err != nil {
		t.Fatalf("Normalize() second pass error = %v", err)
	}

	if renormalized.Slides[0].Blocks != nil {
		t.Fatalf("renormalized.Slides[0].Blocks = %#v, want nil for forbidden field", renormalized.Slides[0].Blocks)
	}
	if renormalized.Slides[0].Columns != nil {
		t.Fatalf("renormalized.Slides[0].Columns = %#v, want nil for forbidden field", renormalized.Slides[0].Columns)
	}
	if renormalized.Slides[1].Blocks == nil {
		t.Fatal("renormalized.Slides[1].Blocks = nil, want canonical empty-or-populated slice for allowed field")
	}
	if renormalized.Slides[2].Blocks != nil {
		t.Fatalf("renormalized.Slides[2].Blocks = %#v, want nil for forbidden field", renormalized.Slides[2].Blocks)
	}
	if renormalized.Slides[2].Columns == nil {
		t.Fatal("renormalized.Slides[2].Columns = nil, want populated columns slice")
	}
	if renormalized.Slides[2].Columns[1].Blocks[0].Items != nil {
		t.Fatalf("renormalized.Slides[2].Columns[1].Blocks[0].Items = %#v, want nil for unsupported quote field", renormalized.Slides[2].Columns[1].Blocks[0].Items)
	}
	if renormalized.Slides[3].Blocks == nil {
		t.Fatal("renormalized.Slides[3].Blocks = nil, want canonical empty slice for optional allowed field")
	}
	if len(renormalized.Slides[3].Blocks) != 0 {
		t.Fatalf("len(renormalized.Slides[3].Blocks) = %d, want 0", len(renormalized.Slides[3].Blocks))
	}
}

func TestNormalizeV2AppliesPresetFallbackAndTrimsSemanticBlocks(t *testing.T) {
	t.Parallel()

	document, err := Normalize(Document{
		Version:       VersionV2,
		ThemePresetID: testStringPtr(" unknown_preset "),
		Slides: []Slide{
			{
				Layout:   LayoutCoverHero,
				Title:    " Kyoto Escape ",
				Subtitle: testStringPtr(" 4 days in autumn "),
				Blocks: []Block{
					{
						Type:     BlockTypeImage,
						AssetRef: testStringPtr(" attachment:cover-photo "),
						Caption:  testStringPtr(" Maple season in Arashiyama "),
					},
					{
						Type: BlockTypeRichText,
						Spans: []TextSpan{
							{Text: " Slow travel ", Emphasis: " strong "},
							{Text: " 京都 ", Lang: " ja "},
						},
					},
				},
			},
			{
				Layout: LayoutSummaryMatrix,
				Title:  "Trip snapshot",
				Blocks: []Block{
					{
						Type:  BlockTypeStat,
						Value: testStringPtr(" 4 "),
						Label: testStringPtr(" Days "),
					},
					{
						Type:   BlockTypeTable,
						Header: []string{"Area", "Why go"},
						Rows: [][]string{
							{" Arashiyama ", " Bamboo grove and river walk "},
						},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("Normalize() error = %v", err)
	}

	if document.ThemePresetID == nil || *document.ThemePresetID != ThemePresetGeneralClean {
		t.Fatalf("document.ThemePresetID = %v, want %q", document.ThemePresetID, ThemePresetGeneralClean)
	}
	if got := *document.Slides[0].Blocks[0].AssetRef; got != "attachment:cover-photo" {
		t.Fatalf("document.Slides[0].Blocks[0].AssetRef = %q", got)
	}
	if got := document.Slides[0].Blocks[1].Spans[0].Emphasis; got != "strong" {
		t.Fatalf("document.Slides[0].Blocks[1].Spans[0].Emphasis = %q", got)
	}
	if got := document.Slides[0].Blocks[1].Spans[1].Lang; got != "ja" {
		t.Fatalf("document.Slides[0].Blocks[1].Spans[1].Lang = %q", got)
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
			name: "v1 theme preset id pointer rejected",
			document: Document{
				Version:       VersionV1,
				ThemePresetID: testStringPtr(""),
				Slides: []Slide{
					{
						Layout: LayoutTitle,
						Title:  "Hello",
					},
				},
			},
			wantErr: `themePresetId is not supported for "v1" documents`,
		},
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
			name: "table rows before header yields header error",
			document: Document{
				Version: VersionV2,
				Slides: []Slide{
					{
						Layout: LayoutTable,
						Title:  "Metrics",
						Blocks: []Block{
							{
								Type: BlockTypeTable,
								Rows: [][]string{{"Latency"}},
							},
						},
					},
				},
			},
			wantErr: `slides[0].blocks[0].header must contain at least 1 column`,
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

func TestNormalizeRejectsInvalidV2Documents(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		document Document
		wantErr  string
	}{
		{
			name: "legacy layout rejects v2-only block fields",
			document: Document{
				Version: VersionV2,
				Slides: []Slide{{
					Layout: LayoutTitleBullets,
					Title:  "Agenda",
					Blocks: []Block{{
						Type:  BlockTypeBulletList,
						Items: []string{"One"},
						Tone:  testStringPtr(""),
					}},
				}},
			},
			wantErr: `slides[0].blocks[0] contains fields that are not supported for "bullet_list" blocks`,
		},
		{
			name: "cover hero requires image",
			document: Document{
				Version: VersionV2,
				Slides: []Slide{{
					Layout: LayoutCoverHero,
					Title:  "Travel",
					Blocks: []Block{{
						Type: BlockTypeRichText,
						Spans: []TextSpan{
							{Text: "Hello"},
						},
					}},
				}},
			},
			wantErr: `slides[0].blocks must contain exactly 1 image block`,
		},
		{
			name: "table rejects caption",
			document: Document{
				Version: VersionV2,
				Slides: []Slide{{
					Layout: LayoutTable,
					Title:  "Snapshot",
					Blocks: []Block{{
						Type:    BlockTypeTable,
						Header:  []string{"Metric"},
						Rows:    [][]string{{"Value"}},
						Caption: testStringPtr("Unsupported"),
					}},
				}},
			},
			wantErr: `slides[0].blocks[0] contains fields that are not supported for table blocks`,
		},
		{
			name: "card rejects image-only fields",
			document: Document{
				Version: VersionV2,
				Slides: []Slide{{
					Layout: LayoutComparisonCards,
					Title:  "Options",
					Blocks: []Block{{
						Type:    BlockTypeCard,
						Title:   testStringPtr("Gion"),
						AltText: testStringPtr("Unsupported"),
					}, {
						Type:  BlockTypeCard,
						Title: testStringPtr("Arashiyama"),
					}},
				}},
			},
			wantErr: `slides[0].blocks[0] contains fields that are not supported for card blocks`,
		},
		{
			name: "callout rejects caption",
			document: Document{
				Version: VersionV2,
				Slides: []Slide{{
					Layout: LayoutRecommendation,
					Title:  "Stay",
					Blocks: []Block{{
						Type:     BlockTypeImage,
						AssetRef: testStringPtr("attachment:cover"),
					}, {
						Type:    BlockTypeCallout,
						Title:   testStringPtr("Recommendation"),
						Body:    testStringPtr("Stay central"),
						Caption: testStringPtr("Unsupported"),
					}},
				}},
			},
			wantErr: `slides[0].blocks[1] contains fields that are not supported for callout blocks`,
		},
		{
			name: "badge tone invalid",
			document: Document{
				Version: VersionV2,
				Slides: []Slide{{
					Layout: LayoutClosing,
					Title:  "Done",
					Blocks: []Block{{
						Type: BlockTypeBadge,
						Text: testStringPtr("Packed"),
						Tone: testStringPtr("loud"),
					}},
				}},
			},
			wantErr: `slides[0].blocks[0].tone must be one of: neutral, accent, success, warning`,
		},
		{
			name: "summary matrix requires stat",
			document: Document{
				Version: VersionV2,
				Slides: []Slide{{
					Layout: LayoutSummaryMatrix,
					Title:  "Snapshot",
					Blocks: []Block{{
						Type:   BlockTypeTable,
						Header: []string{"Metric"},
						Rows:   [][]string{{"Value"}},
					}, {
						Type:  BlockTypeCallout,
						Title: testStringPtr("Note"),
						Body:  testStringPtr("Missing stat"),
					}},
				}},
			},
			wantErr: `slides[0].blocks must contain at least 1 stat block`,
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
			name: "v1 theme preset id present but empty",
			payload: `{
				"version": "v1",
				"themePresetId": "",
				"slides": [
					{
						"layout": "title",
						"title": "Hello"
					}
				]
			}`,
			wantErr: `themePresetId is not supported for "v1" documents`,
		},
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
		{
			name: "paragraph with empty tone field",
			payload: `{
				"version": "v2",
				"slides": [
					{
						"layout": "closing",
						"title": "Done",
						"blocks": [
							{
								"type": "paragraph",
								"text": "Summary",
								"tone": ""
							}
						]
					}
				]
			}`,
			wantErr: `slides[0].blocks[0] contains fields that are not supported for paragraph blocks`,
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

func TestParseJSONRejectsNullV2Fields(t *testing.T) {
	t.Parallel()

	_, err := ParseJSON([]byte(`{
		"version": "v2",
		"themePresetId": null,
		"slides": [
			{
				"layout": "cover_hero",
				"title": "Kyoto",
				"blocks": [
					{
						"type": "image",
						"assetRef": "attachment:cover"
					}
				]
			}
		]
	}`))
	if err == nil {
		t.Fatal("ParseJSON() error = nil, want error")
	}
	if err.Error() != `themePresetId must not be null` {
		t.Fatalf("ParseJSON() error = %q", err.Error())
	}
}
