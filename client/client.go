package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
)

// Client is the Sovrabase HTTP client. It is safe for concurrent use.
type Client struct {
	httpClient   *http.Client
	baseURL      string
	projectKey   string
	accessToken  string
	refreshToken string

	// OnTokenRefresh is called when tokens are refreshed.
	OnTokenRefresh func(accessToken, refreshToken string)

	mu sync.Mutex
}

// New creates a new Sovrabase client.
// baseURL should not end with a trailing slash.
// projectKey is the X-Project-Key value for the project.
func New(baseURL, projectKey string) *Client {
	// Trim trailing slashes from baseURL.
	baseURL = strings.TrimRight(baseURL, "/")

	return &Client{
		httpClient: &http.Client{},
		baseURL:    baseURL,
		projectKey: projectKey,
	}
}

// SetAuth stores the access and refresh tokens for authenticated requests.
func (c *Client) SetAuth(accessToken, refreshToken string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.accessToken = accessToken
	c.refreshToken = refreshToken
}

// AccessToken returns the current access token.
func (c *Client) AccessToken() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.accessToken
}

// RefreshToken returns the current refresh token.
func (c *Client) RefreshToken() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.refreshToken
}

// do performs an HTTP request with the standard headers and returns the response.
func (c *Client) do(req *http.Request) (*http.Response, error) {
	c.mu.Lock()
	accessToken := c.accessToken
	refreshToken := c.refreshToken
	c.mu.Unlock()

	req.Header.Set("X-Project-Key", c.projectKey)
	if accessToken != "" {
		req.Header.Set("Authorization", "Bearer "+accessToken)
	}
	if req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}

	// Auto-refresh on 401.
	if resp.StatusCode == http.StatusUnauthorized && refreshToken != "" {
		resp.Body.Close()
		if err := c.Refresh(); err != nil {
			return nil, fmt.Errorf("token refresh failed: %w", err)
		}

		// Retry the request with new token.
		c.mu.Lock()
		accessToken = c.accessToken
		c.mu.Unlock()

		req.Header.Set("Authorization", "Bearer "+accessToken)
		resp, err = c.httpClient.Do(req)
		if err != nil {
			return nil, err
		}
	}

	return resp, nil
}

// doJSON marshals the body, performs the request, and unmarshals the JSON response into target.
func (c *Client) doJSON(method, path string, body interface{}, target interface{}) error {
	var reqBody io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshal: %w", err)
		}
		reqBody = bytes.NewReader(data)
	}

	fullURL := c.baseURL + path
	req, err := http.NewRequest(method, fullURL, reqBody)
	if err != nil {
		return fmt.Errorf("request: %w", err)
	}

	resp, err := c.do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return parseError(resp)
	}

	if target != nil {
		if err := json.NewDecoder(resp.Body).Decode(target); err != nil {
			return fmt.Errorf("decode: %w", err)
		}
	}
	return nil
}

// doRaw performs a request and returns the raw response body reader.
// The caller must close the returned reader.
func (c *Client) doRaw(method, path string) (io.ReadCloser, int, http.Header, error) {
	fullURL := c.baseURL + path
	req, err := http.NewRequest(method, fullURL, nil)
	if err != nil {
		return nil, 0, nil, fmt.Errorf("request: %w", err)
	}

	// Remove default JSON content-type for raw requests.
	req.Header.Set("Content-Type", "")

	resp, err := c.do(req)
	if err != nil {
		return nil, 0, nil, err
	}

	if resp.StatusCode >= 400 {
		defer resp.Body.Close()
		return nil, resp.StatusCode, resp.Header, parseError(resp)
	}

	return resp.Body, resp.StatusCode, resp.Header, nil
}

// doMultipart performs a multipart form upload request.
func (c *Client) doMultipart(path string, body io.Reader, contentType string, target interface{}) error {
	fullURL := c.baseURL + path
	req, err := http.NewRequest(http.MethodPost, fullURL, body)
	if err != nil {
		return fmt.Errorf("request: %w", err)
	}
	req.Header.Set("Content-Type", contentType)

	resp, err := c.do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return parseError(resp)
	}

	if target != nil {
		if err := json.NewDecoder(resp.Body).Decode(target); err != nil {
			return fmt.Errorf("decode: %w", err)
		}
	}
	return nil
}

// buildQueryPath builds a URL path with query parameters.
func buildQueryPath(base string, params url.Values) string {
	if len(params) == 0 {
		return base
	}
	return base + "?" + params.Encode()
}

// parseError reads an error response from the server.
func parseError(resp *http.Response) error {
	var errResp struct {
		Error string `json:"error"`
	}
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if json.Unmarshal(body, &errResp) == nil && errResp.Error != "" {
		return fmt.Errorf("server error (%d): %s", resp.StatusCode, errResp.Error)
	}
	return fmt.Errorf("server error (%d): %s", resp.StatusCode, strings.TrimSpace(string(body)))
}
