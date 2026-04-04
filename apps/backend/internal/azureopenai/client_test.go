package azureopenai

import (
	"encoding/json"
	"strings"
	"testing"
)

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
	if response.Usage.InputTokens != 45 {
		t.Fatalf("response.Usage.InputTokens = %d", response.Usage.InputTokens)
	}
	if response.Usage.OutputTokens != 78 {
		t.Fatalf("response.Usage.OutputTokens = %d", response.Usage.OutputTokens)
	}
	if response.Usage.TotalTokens != 123 {
		t.Fatalf("response.Usage.TotalTokens = %d", response.Usage.TotalTokens)
	}
}
