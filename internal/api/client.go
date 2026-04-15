package api

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
	"time"
)

type TokenSource interface {
	AccessToken(ctx context.Context) (string, error)
}

type Client struct {
	baseURL     *url.URL
	httpClient  *http.Client
	tokenSource TokenSource
}

func NewClient(baseURL string, httpClient *http.Client, tokenSource TokenSource) (*Client, error) {
	parsedURL, err := url.Parse(strings.TrimSpace(baseURL))
	if err != nil {
		return nil, fmt.Errorf("parse api url: %w", err)
	}

	if parsedURL.Scheme == "" || parsedURL.Host == "" {
		return nil, fmt.Errorf("api url must include scheme and host")
	}

	if httpClient == nil {
		httpClient = &http.Client{Timeout: 15 * time.Second}
	}

	return &Client{
		baseURL:     parsedURL,
		httpClient:  httpClient,
		tokenSource: tokenSource,
	}, nil
}

func (c *Client) NewRequest(ctx context.Context, method string, requestPath string, body any) (*http.Request, error) {
	return c.newRequest(ctx, method, requestPath, body, true)
}

func (c *Client) NewRequestWithHeaders(ctx context.Context, method string, requestPath string, body any, headers map[string]string) (*http.Request, error) {
	req, err := c.newRequest(ctx, method, requestPath, body, true)
	if err != nil {
		return nil, err
	}

	for key, value := range headers {
		req.Header.Set(key, value)
	}

	return req, nil
}

func (c *Client) newRequest(ctx context.Context, method string, requestPath string, body any, includeAuth bool) (*http.Request, error) {
	endpoint := *c.baseURL
	endpoint.Path = path.Join(c.baseURL.Path, requestPath)

	var reader io.Reader
	if body != nil {
		payload, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshal request body: %w", err)
		}
		reader = bytes.NewReader(payload)
	}

	req, err := http.NewRequestWithContext(ctx, method, endpoint.String(), reader)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}

	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	if includeAuth && c.tokenSource != nil {
		token, err := c.tokenSource.AccessToken(ctx)
		if err != nil {
			return nil, fmt.Errorf("load access token: %w", err)
		}
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
	}

	return req, nil
}

func (c *Client) Do(req *http.Request) (*http.Response, error) {
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}

	return resp, nil
}

func (c *Client) DoStream(req *http.Request) (*http.Response, error) {
	streamClient := *c.httpClient
	streamClient.Timeout = 0

	resp, err := streamClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("send stream request: %w", err)
	}

	return resp, nil
}
