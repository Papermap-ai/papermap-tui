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

// refresherTokenSource is an optional capability a TokenSource may implement.
// When the backend rejects a token the client still considers valid, the
// client calls ForceRefresh to mint a new one and replays the request once.
// TokenStore implements this; simple test stubs can omit it.
type refresherTokenSource interface {
	ForceRefresh(ctx context.Context) (string, error)
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
		httpClient = &http.Client{
			Timeout:       15 * time.Second,
			CheckRedirect: defaultCheckRedirect(),
		}
	}

	return &Client{
		baseURL:     parsedURL,
		httpClient:  httpClient,
		tokenSource: tokenSource,
	}, nil
}

func defaultCheckRedirect() func(*http.Request, []*http.Request) error {
	return func(req *http.Request, via []*http.Request) error {
		if len(via) >= 10 {
			return fmt.Errorf("stopped after 10 redirects")
		}
		// Strip Authorization header on cross-host redirects.
		// Same-host redirects (e.g. http -> https) are preserved.
		if via[0].URL.Host != req.URL.Host {
			req.Header.Del("Authorization")
		}
		return nil
	}
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

	if resp.StatusCode == http.StatusUnauthorized && req.Header.Get("Authorization") != "" {
		if retried, retryErr, ok := c.retryOn401(c.httpClient, req, resp); ok {
			return retried, retryErr
		}
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

	if resp.StatusCode == http.StatusUnauthorized && req.Header.Get("Authorization") != "" {
		if retried, retryErr, ok := c.retryOn401(&streamClient, req, resp); ok {
			return retried, retryErr
		}
	}

	return resp, nil
}

// retryOn401 mints a fresh access token via the token source's optional
// ForceRefresh capability and replays req once. The bool return is true when
// the retry path ran (regardless of success); when false the caller falls
// through with the original 401 response. On refresh failure the returned
// error wraps auth.ErrSessionExpired so the app routes to the session-expired
// flow instead of surfacing a stale 401.
func (c *Client) retryOn401(httpClient *http.Client, req *http.Request, resp *http.Response) (*http.Response, error, bool) {
	refresher, ok := c.tokenSource.(refresherTokenSource)
	if !ok || refresher == nil {
		return resp, nil, false
	}

	token, err := refresher.ForceRefresh(req.Context())
	if err != nil {
		_ = resp.Body.Close()
		return nil, err, true
	}

	replayed, err := replayRequest(httpClient, req, token)
	if err != nil {
		_ = resp.Body.Close()
		return nil, fmt.Errorf("replay after refresh: %w", err), true
	}

	_ = resp.Body.Close()
	return replayed, nil, true
}

// replayRequest rebuilds req for one-shot retry: it re-reads the body via
// req.GetBody (set by http.NewRequestWithContext for byte buffers, nil for
// GET bodies), swaps the Authorization header for the refreshed token, and
// drops req from its previous transport via an isolated client clone.
func replayRequest(base *http.Client, req *http.Request, token string) (*http.Response, error) {
	var body io.ReadCloser
	if req.GetBody != nil {
		rb, err := req.GetBody()
		if err != nil {
			return nil, fmt.Errorf("replay request body: %w", err)
		}
		body = io.NopCloser(rb)
	} else if req.Body != nil {
		// Body already consumed and no GetBody; cannot replay safely.
		return nil, fmt.Errorf("request body is not replayable")
	}

	clone := *req
	clone.Body = body
	clone.Header = req.Header.Clone()
	clone.Header.Set("Authorization", "Bearer "+strings.TrimSpace(token))

	return base.Do(&clone)
}
