package filenames

import "testing"

func TestSanitize(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		fallback string
		want     string
	}{
		{name: "basename and replace unsafe chars", input: "../ref image.png", fallback: "file", want: "ref-image.png"},
		{name: "collapse consecutive unsafe chars", input: "ref@@@###image.png", fallback: "file", want: "ref-image.png"},
		{name: "trim empty to fallback", input: "   ", fallback: "file", want: "file"},
		{name: "trim unsafe-only to fallback", input: "???", fallback: "file", want: "file"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := Sanitize(tt.input, tt.fallback); got != tt.want {
				t.Fatalf("Sanitize(%q, %q) = %q, want %q", tt.input, tt.fallback, got, tt.want)
			}
		})
	}
}
