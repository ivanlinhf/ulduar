package blobstorage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/blob"
)

type Client struct {
	Service       *azblob.Client
	ContainerName string
}

func Connect(accountName, accountKey, serviceURL, containerName string) (*Client, error) {
	credential, err := azblob.NewSharedKeyCredential(accountName, accountKey)
	if err != nil {
		return nil, fmt.Errorf("create azure storage credential: %w", err)
	}

	serviceClient, err := azblob.NewClientWithSharedKeyCredential(serviceURL, credential, nil)
	if err != nil {
		return nil, fmt.Errorf("create azure blob client: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if _, err := serviceClient.CreateContainer(ctx, containerName, nil); err != nil {
		var responseErr *azcore.ResponseError
		if !errors.As(err, &responseErr) || responseErr.StatusCode != http.StatusConflict {
			return nil, fmt.Errorf("create blob container %s: %w", containerName, err)
		}
	}

	return &Client{
		Service:       serviceClient,
		ContainerName: containerName,
	}, nil
}

func (c *Client) Upload(ctx context.Context, blobPath string, data []byte, contentType string) error {
	_, err := c.Service.UploadBuffer(ctx, c.ContainerName, blobPath, data, &azblob.UploadBufferOptions{
		HTTPHeaders: &blob.HTTPHeaders{
			BlobContentType: &contentType,
		},
	})
	if err != nil {
		return fmt.Errorf("upload blob %s: %w", blobPath, err)
	}

	return nil
}

func (c *Client) Delete(ctx context.Context, blobPath string) error {
	_, err := c.Service.DeleteBlob(ctx, c.ContainerName, blobPath, nil)
	if err != nil {
		return fmt.Errorf("delete blob %s: %w", blobPath, err)
	}

	return nil
}

func (c *Client) Download(ctx context.Context, blobPath string) ([]byte, error) {
	return c.download(ctx, blobPath, 0)
}

func (c *Client) DownloadWithinLimit(ctx context.Context, blobPath string, maxBytes int64) ([]byte, error) {
	return c.download(ctx, blobPath, maxBytes)
}

func (c *Client) download(ctx context.Context, blobPath string, maxBytes int64) ([]byte, error) {
	resp, err := c.Service.DownloadStream(ctx, c.ContainerName, blobPath, nil)
	if err != nil {
		return nil, fmt.Errorf("download blob %s: %w", blobPath, err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(limitedDownloadReader(resp.Body, maxBytes))
	if err != nil {
		return nil, fmt.Errorf("read blob %s: %w", blobPath, err)
	}
	if maxBytes > 0 && int64(len(data)) > maxBytes {
		return nil, fmt.Errorf("blob %s exceeds %d bytes", blobPath, maxBytes)
	}

	return data, nil
}

func limitedDownloadReader(r io.Reader, maxBytes int64) io.Reader {
	if maxBytes <= 0 {
		return r
	}
	limit := maxBytes
	if maxBytes < math.MaxInt64 {
		limit = maxBytes + 1
	}
	return io.LimitReader(r, limit)
}
