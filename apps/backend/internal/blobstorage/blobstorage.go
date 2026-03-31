package blobstorage

import (
	"context"
	"errors"
	"fmt"
	"io"
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
	resp, err := c.Service.DownloadStream(ctx, c.ContainerName, blobPath, nil)
	if err != nil {
		return nil, fmt.Errorf("download blob %s: %w", blobPath, err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read blob %s: %w", blobPath, err)
	}

	return data, nil
}
