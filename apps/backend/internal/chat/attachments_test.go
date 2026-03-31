package chat

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"testing"
)

func TestReadAttachmentsAcceptsPNG(t *testing.T) {
	t.Parallel()

	header := buildFileHeader(t, "image.png", "image/png", []byte{
		0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n',
		0x00, 0x00, 0x00, 0x0d, 'I', 'H', 'D', 'R',
		0x00, 0x00, 0x00, 0x01,
	})

	attachments, err := ReadAttachments([]*multipart.FileHeader{header})
	if err != nil {
		t.Fatalf("ReadAttachments returned error: %v", err)
	}

	if len(attachments) != 1 {
		t.Fatalf("expected 1 attachment, got %d", len(attachments))
	}

	if attachments[0].MediaType != "image/png" {
		t.Fatalf("expected image/png, got %q", attachments[0].MediaType)
	}
}

func TestReadAttachmentsRejectsUnsupportedType(t *testing.T) {
	t.Parallel()

	header := buildFileHeader(t, "notes.txt", "text/plain", []byte("hello world"))

	_, err := ReadAttachments([]*multipart.FileHeader{header})
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	validationErr, ok := err.(ValidationError)
	if !ok {
		t.Fatalf("expected ValidationError, got %T", err)
	}

	if validationErr.StatusCode != http.StatusUnsupportedMediaType {
		t.Fatalf("expected status %d, got %d", http.StatusUnsupportedMediaType, validationErr.StatusCode)
	}
}

func buildFileHeader(t *testing.T, filename, contentType string, body []byte) *multipart.FileHeader {
	t.Helper()

	var buffer bytes.Buffer
	writer := multipart.NewWriter(&buffer)

	partHeader := textproto.MIMEHeader{}
	partHeader.Set("Content-Disposition", `form-data; name="attachments"; filename="`+filename+`"`)
	partHeader.Set("Content-Type", contentType)

	part, err := writer.CreatePart(partHeader)
	if err != nil {
		t.Fatalf("CreatePart: %v", err)
	}

	if _, err := part.Write(body); err != nil {
		t.Fatalf("Write: %v", err)
	}

	if err := writer.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	reader := multipart.NewReader(bytes.NewReader(buffer.Bytes()), writer.Boundary())
	form, err := reader.ReadForm(int64(len(buffer.Bytes())))
	if err != nil {
		t.Fatalf("ReadForm: %v", err)
	}

	files := form.File["attachments"]
	if len(files) != 1 {
		t.Fatalf("expected 1 file header, got %d", len(files))
	}

	return files[0]
}
