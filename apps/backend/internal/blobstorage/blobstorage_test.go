package blobstorage

import (
	"io"
	"math"
	"strings"
	"testing"
)

func TestLimitedDownloadReaderReadsSentinelByte(t *testing.T) {
	t.Parallel()

	data, err := io.ReadAll(limitedDownloadReader(strings.NewReader("abcd"), 3))
	if err != nil {
		t.Fatalf("ReadAll() error = %v", err)
	}
	if string(data) != "abcd" {
		t.Fatalf("data = %q, want %q", string(data), "abcd")
	}
}

func TestLimitedDownloadReaderDoesNotOverflowMaxInt64(t *testing.T) {
	t.Parallel()

	data, err := io.ReadAll(limitedDownloadReader(strings.NewReader("abc"), math.MaxInt64))
	if err != nil {
		t.Fatalf("ReadAll() error = %v", err)
	}
	if string(data) != "abc" {
		t.Fatalf("data = %q, want %q", string(data), "abc")
	}
}
