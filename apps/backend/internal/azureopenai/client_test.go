package azureopenai

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestCreateResponseRequestMarshalsWebSearchTools(t *testing.T) {
	t.Parallel()

	request := CreateResponseRequest{
		Input: []InputMessage{{
			Role: "user",
			Content: []InputContentItem{{
				Type: "input_text",
				Text: "hello",
			}},
		}},
		Tools: []Tool{{Type: "web_search"}},
	}

	data, err := json.Marshal(request)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	got := string(data)
	if !strings.Contains(got, `"tools":[{"type":"web_search"}]`) {
		t.Fatalf("marshaled request missing web_search tools: %s", got)
	}
}

func TestReadSSEStreamParsesDataFrames(t *testing.T) {
	t.Parallel()

	stream := strings.Join([]string{
		"event: response.output_text.delta",
		"data: {\"type\":\"response.output_text.delta\",\"delta\":\"Hel\"}",
		"",
		"data: {\"type\":\"response.output_text.delta\",\"delta\":\"lo\"}",
		"",
		"data: [DONE]",
		"",
	}, "\n")

	var events []string
	err := readSSEStream(strings.NewReader(stream), func(data string) error {
		events = append(events, data)
		return nil
	})
	if err != nil {
		t.Fatalf("readSSEStream() error = %v", err)
	}

	if len(events) != 3 {
		t.Fatalf("expected 3 events including DONE, got %d", len(events))
	}
	if events[0] != "{\"type\":\"response.output_text.delta\",\"delta\":\"Hel\"}" {
		t.Fatalf("unexpected first event: %q", events[0])
	}
	if events[1] != "{\"type\":\"response.output_text.delta\",\"delta\":\"lo\"}" {
		t.Fatalf("unexpected second event: %q", events[1])
	}
	if events[2] != "[DONE]" {
		t.Fatalf("unexpected done event: %q", events[2])
	}
}

func TestResponseDecodesUsage(t *testing.T) {
	t.Parallel()

	payload := []byte(`{
		"id": "resp_123",
		"model": "gpt-5",
		"status": "completed",
		"usage": {
			"input_tokens": 45,
			"output_tokens": 78,
			"total_tokens": 123
		}
	}`)

	var response Response
	if err := json.Unmarshal(payload, &response); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	if response.Usage == nil {
		t.Fatal("response.Usage is nil")
	}
	if response.Usage.InputTokens == nil || *response.Usage.InputTokens != 45 {
		t.Fatalf("response.Usage.InputTokens = %v", response.Usage.InputTokens)
	}
	if response.Usage.OutputTokens == nil || *response.Usage.OutputTokens != 78 {
		t.Fatalf("response.Usage.OutputTokens = %v", response.Usage.OutputTokens)
	}
	if response.Usage.TotalTokens == nil || *response.Usage.TotalTokens != 123 {
		t.Fatalf("response.Usage.TotalTokens = %v", response.Usage.TotalTokens)
	}
}

func TestResponseDecodesPartialUsageWithoutDefaultingMissingFieldsToZero(t *testing.T) {
	t.Parallel()

	payload := []byte(`{
		"id": "resp_124",
		"model": "gpt-5",
		"status": "completed",
		"usage": {
			"total_tokens": 123
		}
	}`)

	var response Response
	if err := json.Unmarshal(payload, &response); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	if response.Usage == nil {
		t.Fatal("response.Usage is nil")
	}
	if response.Usage.InputTokens != nil {
		t.Fatalf("response.Usage.InputTokens = %v, want nil", response.Usage.InputTokens)
	}
	if response.Usage.OutputTokens != nil {
		t.Fatalf("response.Usage.OutputTokens = %v, want nil", response.Usage.OutputTokens)
	}
	if response.Usage.TotalTokens == nil || *response.Usage.TotalTokens != 123 {
		t.Fatalf("response.Usage.TotalTokens = %v", response.Usage.TotalTokens)
	}
}

func TestStreamEventDecodesWebSearchOutputItems(t *testing.T) {
	t.Parallel()

	payload := []byte(`{
		"type": "response.output_item.done",
		"item": {
			"id": "ws_123",
			"type": "web_search_call",
			"status": "completed"
		}
	}`)

	var event StreamEvent
	if err := json.Unmarshal(payload, &event); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	if event.Item == nil {
		t.Fatal("event.Item is nil")
	}
	if event.Item.Type != "web_search_call" {
		t.Fatalf("event.Item.Type = %q", event.Item.Type)
	}
	if event.Item.Status != "completed" {
		t.Fatalf("event.Item.Status = %q", event.Item.Status)
	}
}

func TestResponseDecodesURLCitationAnnotations(t *testing.T) {
	t.Parallel()

	payload := []byte(`{
		"id": "resp_123",
		"status": "completed",
		"output": [{
			"type": "message",
			"role": "assistant",
			"content": [{
				"type": "output_text",
				"text": "Answer",
				"annotations": [{
					"type": "url_citation",
					"title": "Example",
					"url": "https://example.com",
					"location": {
						"start": 1,
						"end": 6
					}
				}]
			}]
		}]
	}`)

	var response Response
	if err := json.Unmarshal(payload, &response); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	annotations := response.Output[0].Content[0].Annotations
	if len(annotations) != 1 {
		t.Fatalf("len(annotations) = %d, want 1", len(annotations))
	}
	if annotations[0].URL != "https://example.com" {
		t.Fatalf("annotations[0].URL = %q", annotations[0].URL)
	}
	if annotations[0].StartIndex == nil || *annotations[0].StartIndex != 1 {
		t.Fatalf("annotations[0].StartIndex = %v", annotations[0].StartIndex)
	}
	if annotations[0].EndIndex == nil || *annotations[0].EndIndex != 6 {
		t.Fatalf("annotations[0].EndIndex = %v", annotations[0].EndIndex)
	}
}
