package chat

import (
	"encoding/json"
	"testing"

	"github.com/ivanlin/ulduar/apps/backend/internal/azureopenai"
)

func TestDecodeContentKeepsOlderMessagesWithoutCitationsReadable(t *testing.T) {
	t.Parallel()

	content, err := DecodeContent(json.RawMessage(`{"parts":[{"type":"text","text":"hello"}]}`))
	if err != nil {
		t.Fatalf("DecodeContent() error = %v", err)
	}

	if len(content.Parts) != 1 {
		t.Fatalf("len(content.Parts) = %d, want 1", len(content.Parts))
	}
	if len(content.Parts[0].Citations) != 0 {
		t.Fatalf("len(content.Parts[0].Citations) = %d, want 0", len(content.Parts[0].Citations))
	}
}

func TestNewAssistantContentFromResponsePersistsCitations(t *testing.T) {
	t.Parallel()

	start := 5
	end := 18
	data, err := NewAssistantContentFromResponse(azureopenai.Response{
		Output: []azureopenai.ResponseItem{{
			Type: "message",
			Role: "assistant",
			Content: []azureopenai.ResponseContentItem{{
				Type: "output_text",
				Text: "Battery breakthrough",
				Annotations: []azureopenai.ResponseAnnotation{{
					Type:       "url_citation",
					Title:      "Example source",
					URL:        "https://example.com/source",
					StartIndex: &start,
					EndIndex:   &end,
				}},
			}},
		}},
	})
	if err != nil {
		t.Fatalf("NewAssistantContentFromResponse() error = %v", err)
	}

	content, err := DecodeContent(data)
	if err != nil {
		t.Fatalf("DecodeContent() error = %v", err)
	}

	if len(content.Parts) != 1 {
		t.Fatalf("len(content.Parts) = %d, want 1", len(content.Parts))
	}
	if len(content.Parts[0].Citations) != 1 {
		t.Fatalf("len(content.Parts[0].Citations) = %d, want 1", len(content.Parts[0].Citations))
	}
	if content.Parts[0].Citations[0].URL != "https://example.com/source" {
		t.Fatalf("content.Parts[0].Citations[0].URL = %q", content.Parts[0].Citations[0].URL)
	}
	if content.Parts[0].Citations[0].StartIndex == nil || *content.Parts[0].Citations[0].StartIndex != start {
		t.Fatalf("content.Parts[0].Citations[0].StartIndex = %v", content.Parts[0].Citations[0].StartIndex)
	}
	if content.Parts[0].Citations[0].EndIndex == nil || *content.Parts[0].Citations[0].EndIndex != end {
		t.Fatalf("content.Parts[0].Citations[0].EndIndex = %v", content.Parts[0].Citations[0].EndIndex)
	}
}

func TestNewAssistantContentFromResponsePreservesAnnotatedTextWhitespace(t *testing.T) {
	t.Parallel()

	start := 2
	end := 9
	data, err := NewAssistantContentFromResponse(azureopenai.Response{
		Output: []azureopenai.ResponseItem{{
			Type: "message",
			Role: "assistant",
			Content: []azureopenai.ResponseContentItem{{
				Type: "output_text",
				Text: "\n Grounded answer ",
				Annotations: []azureopenai.ResponseAnnotation{{
					Type:       "url_citation",
					URL:        "https://example.com/source",
					StartIndex: &start,
					EndIndex:   &end,
				}},
			}},
		}},
	})
	if err != nil {
		t.Fatalf("NewAssistantContentFromResponse() error = %v", err)
	}

	content, err := DecodeContent(data)
	if err != nil {
		t.Fatalf("DecodeContent() error = %v", err)
	}

	if got := content.Parts[0].Text; got != "\n Grounded answer " {
		t.Fatalf("content.Parts[0].Text = %q, want %q", got, "\n Grounded answer ")
	}
	if content.Parts[0].Citations[0].StartIndex == nil || *content.Parts[0].Citations[0].StartIndex != start {
		t.Fatalf("content.Parts[0].Citations[0].StartIndex = %v", content.Parts[0].Citations[0].StartIndex)
	}
	if content.Parts[0].Citations[0].EndIndex == nil || *content.Parts[0].Citations[0].EndIndex != end {
		t.Fatalf("content.Parts[0].Citations[0].EndIndex = %v", content.Parts[0].Citations[0].EndIndex)
	}
}

func TestNewAssistantContentFromResponseAllowsMissingItemTypeForAssistantText(t *testing.T) {
	t.Parallel()

	data, err := NewAssistantContentFromResponse(azureopenai.Response{
		Output: []azureopenai.ResponseItem{{
			Role: "assistant",
			Content: []azureopenai.ResponseContentItem{{
				Type: "output_text",
				Text: "Streamed answer",
			}},
		}},
	})
	if err != nil {
		t.Fatalf("NewAssistantContentFromResponse() error = %v", err)
	}

	content, err := DecodeContent(data)
	if err != nil {
		t.Fatalf("DecodeContent() error = %v", err)
	}

	if len(content.Parts) != 1 {
		t.Fatalf("len(content.Parts) = %d, want 1", len(content.Parts))
	}
	if content.Parts[0].Text != "Streamed answer" {
		t.Fatalf("content.Parts[0].Text = %q", content.Parts[0].Text)
	}
}

func TestNewAssistantContentFromResponseIgnoresNonMessageOutputItems(t *testing.T) {
	t.Parallel()

	data, err := NewAssistantContentFromResponse(azureopenai.Response{
		OutputText: "Final answer",
		Output: []azureopenai.ResponseItem{{
			Type: "web_search_call",
			Content: []azureopenai.ResponseContentItem{{
				Type: "output_text",
				Text: "raw tool trace",
			}},
		}},
	})
	if err != nil {
		t.Fatalf("NewAssistantContentFromResponse() error = %v", err)
	}

	content, err := DecodeContent(data)
	if err != nil {
		t.Fatalf("DecodeContent() error = %v", err)
	}

	if len(content.Parts) != 1 {
		t.Fatalf("len(content.Parts) = %d, want 1", len(content.Parts))
	}
	if content.Parts[0].Text != "Final answer" {
		t.Fatalf("content.Parts[0].Text = %q, want %q", content.Parts[0].Text, "Final answer")
	}
}

func TestNewTextContentWithCitationsPreservesTextForOffsets(t *testing.T) {
	t.Parallel()

	start := 2
	end := 7
	data, err := NewTextContentWithCitations("\n cited text ", []MessageCitation{{
		URL:        "https://example.com/source",
		StartIndex: &start,
		EndIndex:   &end,
	}})
	if err != nil {
		t.Fatalf("NewTextContentWithCitations() error = %v", err)
	}

	content, err := DecodeContent(data)
	if err != nil {
		t.Fatalf("DecodeContent() error = %v", err)
	}

	if len(content.Parts) != 1 {
		t.Fatalf("len(content.Parts) = %d, want 1", len(content.Parts))
	}
	if got := content.Parts[0].Text; got != "\n cited text " {
		t.Fatalf("content.Parts[0].Text = %q, want %q", got, "\n cited text ")
	}
	if content.Parts[0].Citations[0].StartIndex == nil || *content.Parts[0].Citations[0].StartIndex != start {
		t.Fatalf("content.Parts[0].Citations[0].StartIndex = %v", content.Parts[0].Citations[0].StartIndex)
	}
	if content.Parts[0].Citations[0].EndIndex == nil || *content.Parts[0].Citations[0].EndIndex != end {
		t.Fatalf("content.Parts[0].Citations[0].EndIndex = %v", content.Parts[0].Citations[0].EndIndex)
	}
}
