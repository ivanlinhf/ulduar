package azureopenai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"
)

const defaultAPIVersion = "v1"

type HTTPDoer interface {
	Do(req *http.Request) (*http.Response, error)
}

type Client interface {
	CreateResponse(ctx context.Context, request CreateResponseRequest) (Response, error)
	GetResponse(ctx context.Context, responseID string) (Response, error)
	StreamResponse(ctx context.Context, request CreateResponseRequest, onEvent func(StreamEvent) error) error
}

type RESTClient struct {
	baseURL    string
	apiKey     string
	apiVersion string
	deployment string
	httpClient HTTPDoer
}

type CreateResponseRequest struct {
	Model              string `json:"model,omitempty"`
	Input              any    `json:"input"`
	Instructions       string `json:"instructions,omitempty"`
	PreviousResponseID string `json:"previous_response_id,omitempty"`
	Store              *bool  `json:"store,omitempty"`
	Stream             bool   `json:"stream,omitempty"`
}

type InputMessage struct {
	Type    string             `json:"type,omitempty"`
	Role    string             `json:"role"`
	Content []InputContentItem `json:"content"`
}

type InputContentItem struct {
	Type     string `json:"type"`
	Text     string `json:"text,omitempty"`
	ImageURL string `json:"image_url,omitempty"`
	FileData string `json:"file_data,omitempty"`
	FileID   string `json:"file_id,omitempty"`
	Filename string `json:"filename,omitempty"`
	Detail   string `json:"detail,omitempty"`
}

type Response struct {
	ID         string         `json:"id"`
	Object     string         `json:"object"`
	Model      string         `json:"model"`
	Status     string         `json:"status"`
	OutputText string         `json:"output_text"`
	Output     []ResponseItem `json:"output"`
	Error      *ResponseError `json:"error,omitempty"`
}

type ResponseItem struct {
	ID      string                `json:"id"`
	Type    string                `json:"type"`
	Role    string                `json:"role"`
	Status  string                `json:"status"`
	Content []ResponseContentItem `json:"content"`
}

type ResponseContentItem struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type ResponseError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type StreamEvent struct {
	Type     string         `json:"type"`
	Delta    string         `json:"delta,omitempty"`
	Response *Response      `json:"response,omitempty"`
	Error    *ResponseError `json:"error,omitempty"`
}

type APIError struct {
	StatusCode int
	Message    string
}

func (e APIError) Error() string {
	return fmt.Sprintf("azure openai api returned status %d: %s", e.StatusCode, e.Message)
}

func NewClient(endpoint, apiKey, apiVersion, deployment string) (*RESTClient, error) {
	if strings.TrimSpace(endpoint) == "" {
		return nil, fmt.Errorf("azure openai endpoint must not be empty")
	}

	if strings.TrimSpace(apiKey) == "" {
		return nil, fmt.Errorf("azure openai api key must not be empty")
	}

	if strings.TrimSpace(deployment) == "" {
		return nil, fmt.Errorf("azure openai deployment must not be empty")
	}

	baseURL, err := normalizeBaseURL(endpoint)
	if err != nil {
		return nil, err
	}

	version := strings.TrimSpace(apiVersion)
	if version == "" {
		version = defaultAPIVersion
	}

	return &RESTClient{
		baseURL:    baseURL,
		apiKey:     apiKey,
		apiVersion: version,
		deployment: deployment,
		httpClient: &http.Client{},
	}, nil
}

func (c *RESTClient) CreateResponse(ctx context.Context, request CreateResponseRequest) (Response, error) {
	if strings.TrimSpace(request.Model) == "" {
		request.Model = c.deployment
	}

	return c.doJSON(ctx, http.MethodPost, "/responses", request)
}

func (c *RESTClient) GetResponse(ctx context.Context, responseID string) (Response, error) {
	responseID = strings.TrimSpace(responseID)
	if responseID == "" {
		return Response{}, fmt.Errorf("response id must not be empty")
	}

	return c.doJSON(ctx, http.MethodGet, path.Join("/responses", responseID), nil)
}

func (c *RESTClient) StreamResponse(ctx context.Context, request CreateResponseRequest, onEvent func(StreamEvent) error) error {
	if strings.TrimSpace(request.Model) == "" {
		request.Model = c.deployment
	}
	request.Stream = true

	return c.doStream(ctx, "/responses", request, onEvent)
}

func (c *RESTClient) BaseURL() string {
	return c.baseURL
}

func (c *RESTClient) Deployment() string {
	return c.deployment
}

func (c *RESTClient) APIVersion() string {
	return c.apiVersion
}

func (c *RESTClient) doJSON(ctx context.Context, method, endpointPath string, payload any) (Response, error) {
	var body io.Reader
	if payload != nil {
		data, err := json.Marshal(payload)
		if err != nil {
			return Response{}, fmt.Errorf("marshal request: %w", err)
		}
		body = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+endpointPath, body)
	if err != nil {
		return Response{}, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("api-key", c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return Response{}, fmt.Errorf("perform request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		message, readErr := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
		if readErr != nil {
			return Response{}, fmt.Errorf("read error response: %w", readErr)
		}

		return Response{}, APIError{
			StatusCode: resp.StatusCode,
			Message:    strings.TrimSpace(string(message)),
		}
	}

	var response Response
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return Response{}, fmt.Errorf("decode response: %w", err)
	}

	return response, nil
}

func (c *RESTClient) doStream(ctx context.Context, endpointPath string, payload any, onEvent func(StreamEvent) error) error {
	body, err := marshalRequestBody(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+endpointPath, body)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("api-key", c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("perform request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		message, readErr := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
		if readErr != nil {
			return fmt.Errorf("read error response: %w", readErr)
		}

		return APIError{
			StatusCode: resp.StatusCode,
			Message:    strings.TrimSpace(string(message)),
		}
	}

	if err := readSSEStream(resp.Body, func(data string) error {
		if data == "[DONE]" {
			return nil
		}

		var event StreamEvent
		if err := json.Unmarshal([]byte(data), &event); err != nil {
			return fmt.Errorf("decode stream event: %w", err)
		}

		if onEvent == nil {
			return nil
		}

		return onEvent(event)
	}); err != nil {
		return fmt.Errorf("read event stream: %w", err)
	}

	return nil
}

func marshalRequestBody(payload any) (io.Reader, error) {
	if payload == nil {
		return nil, nil
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	return bytes.NewReader(data), nil
}

func readSSEStream(body io.Reader, onData func(data string) error) error {
	reader := io.Reader(body)
	buf := make([]byte, 0, 4096)
	lineBuf := make([]byte, 1)
	dataLines := make([]string, 0, 4)

	flush := func() error {
		if len(dataLines) == 0 {
			return nil
		}
		data := strings.Join(dataLines, "\n")
		dataLines = dataLines[:0]
		return onData(data)
	}

	for {
		n, err := reader.Read(lineBuf)
		if n > 0 {
			b := lineBuf[0]
			buf = append(buf, b)
			if b != '\n' {
				continue
			}

			line := strings.TrimRight(string(buf), "\r\n")
			buf = buf[:0]

			if line == "" {
				if err := flush(); err != nil {
					return err
				}
				continue
			}

			if strings.HasPrefix(line, ":") {
				continue
			}

			field, value, ok := strings.Cut(line, ":")
			if !ok {
				continue
			}
			value = strings.TrimPrefix(value, " ")

			if field == "data" {
				dataLines = append(dataLines, value)
			}
		}

		if err != nil {
			if err == io.EOF {
				if len(buf) > 0 {
					line := strings.TrimRight(string(buf), "\r\n")
					if strings.HasPrefix(line, "data:") {
						dataLines = append(dataLines, strings.TrimPrefix(strings.TrimPrefix(line, "data:"), " "))
					}
				}
				return flush()
			}
			return err
		}
	}
}

func normalizeBaseURL(endpoint string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(endpoint))
	if err != nil {
		return "", fmt.Errorf("parse azure openai endpoint: %w", err)
	}

	if parsed.Scheme == "" || parsed.Host == "" {
		return "", fmt.Errorf("azure openai endpoint must be an absolute url")
	}

	cleanPath := strings.TrimSuffix(parsed.Path, "/")
	if !strings.HasSuffix(cleanPath, "/openai/v1") {
		cleanPath = strings.TrimSuffix(cleanPath, "/openai")
		cleanPath = cleanPath + "/openai/v1"
	}

	parsed.Path = cleanPath
	parsed.RawPath = cleanPath
	parsed.RawQuery = ""
	parsed.Fragment = ""

	return strings.TrimSuffix(parsed.String(), "/"), nil
}
