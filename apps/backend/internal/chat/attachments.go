package chat

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"

	"github.com/ivanlin/ulduar/apps/backend/internal/filenames"
)

const (
	MaxMessageRequestBytes   int64 = 64 << 20
	MaxAttachmentBytes       int64 = 20 << 20
	MaxAttachmentsPerMessage       = 5
)

var (
	supportedMediaTypes = map[string]string{
		"application/pdf": "pdf",
		"image/gif":       "image",
		"image/jpeg":      "image",
		"image/png":       "image",
		"image/webp":      "image",
	}
)

type AttachmentUpload struct {
	Filename  string
	MediaType string
	Kind      string
	SizeBytes int64
	SHA256    string
	Data      []byte
}

func ReadAttachments(headers []*multipart.FileHeader) ([]AttachmentUpload, error) {
	if len(headers) > MaxAttachmentsPerMessage {
		return nil, ValidationError{
			StatusCode: http.StatusBadRequest,
			Message:    fmt.Sprintf("too many attachments: maximum %d files", MaxAttachmentsPerMessage),
		}
	}

	attachments := make([]AttachmentUpload, 0, len(headers))
	for _, header := range headers {
		attachment, err := readAttachment(header)
		if err != nil {
			return nil, err
		}

		attachments = append(attachments, attachment)
	}

	return attachments, nil
}

func readAttachment(header *multipart.FileHeader) (AttachmentUpload, error) {
	file, err := header.Open()
	if err != nil {
		return AttachmentUpload{}, fmt.Errorf("open attachment %q: %w", header.Filename, err)
	}
	defer file.Close()

	data, err := io.ReadAll(io.LimitReader(file, MaxAttachmentBytes+1))
	if err != nil {
		return AttachmentUpload{}, fmt.Errorf("read attachment %q: %w", header.Filename, err)
	}

	if int64(len(data)) == 0 {
		return AttachmentUpload{}, ValidationError{
			StatusCode: http.StatusBadRequest,
			Message:    fmt.Sprintf("attachment %q is empty", header.Filename),
		}
	}

	if int64(len(data)) > MaxAttachmentBytes {
		return AttachmentUpload{}, ValidationError{
			StatusCode: http.StatusRequestEntityTooLarge,
			Message:    fmt.Sprintf("attachment %q exceeds %d bytes", header.Filename, MaxAttachmentBytes),
		}
	}

	mediaType := http.DetectContentType(data)
	kind, ok := supportedMediaTypes[mediaType]
	if !ok {
		return AttachmentUpload{}, ValidationError{
			StatusCode: http.StatusUnsupportedMediaType,
			Message:    fmt.Sprintf("attachment %q has unsupported media type %q", header.Filename, mediaType),
		}
	}

	sum := sha256.Sum256(data)

	return AttachmentUpload{
		Filename:  filenames.Sanitize(header.Filename, "attachment"),
		MediaType: mediaType,
		Kind:      kind,
		SizeBytes: int64(len(data)),
		SHA256:    hex.EncodeToString(sum[:]),
		Data:      data,
	}, nil
}
