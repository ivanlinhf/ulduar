package azureopenai

import (
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
