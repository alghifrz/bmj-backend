package storage

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"bmj-backend/internal/config"
)

const requestTimeout = 20 * time.Second

// Client uploads and deletes objects in a Supabase Storage bucket.
type Client struct {
	baseURL    string
	apiKey     string
	bucket     string
	httpClient *http.Client
}

// New returns a client when Supabase storage settings are present.
// It returns nil when storage is not configured.
func New(cfg config.Config) *Client {
	baseURL := strings.TrimRight(strings.TrimSpace(cfg.SupabaseURL), "/")
	apiKey := strings.TrimSpace(cfg.SupabaseServiceRoleKey)
	bucket := strings.TrimSpace(cfg.SupabaseStorageBucket)
	if baseURL == "" || apiKey == "" || bucket == "" || strings.Contains(bucket, "/") || strings.Contains(bucket, "..") {
		return nil
	}

	parsed, err := url.Parse(baseURL)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "https" && parsed.Scheme != "http") {
		return nil
	}

	return &Client{
		baseURL: baseURL,
		apiKey:  apiKey,
		bucket:  bucket,
		httpClient: &http.Client{
			Timeout: requestTimeout,
		},
	}
}

// Upload stores an object and returns its public URL.
func (c *Client) Upload(ctx context.Context, path, contentType string, body []byte) (string, error) {
	if c == nil {
		return "", fmt.Errorf("storage is not configured")
	}
	if !validObjectPath(path) {
		return "", fmt.Errorf("invalid storage path")
	}

	endpoint := c.baseURL + "/storage/v1/object/" + url.PathEscape(c.bucket) + "/" + escapePath(path)
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	c.authorize(request)
	request.Header.Set("Content-Type", contentType)
	request.Header.Set("Cache-Control", "public, max-age=31536000")

	response, err := c.httpClient.Do(request)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 1<<20))
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return "", fmt.Errorf("storage upload status %d", response.StatusCode)
	}

	return c.baseURL + "/storage/v1/object/public/" + url.PathEscape(c.bucket) + "/" + escapePath(path), nil
}

// Remove deletes objects by their storage paths.
func (c *Client) Remove(ctx context.Context, paths []string) error {
	if c == nil || len(paths) == 0 {
		return nil
	}
	safe := make([]string, 0, len(paths))
	for _, path := range paths {
		if validObjectPath(path) {
			safe = append(safe, path)
		}
	}
	if len(safe) == 0 {
		return nil
	}

	payload := bytes.NewBufferString(`{"prefixes":[`)
	for index, path := range safe {
		if index > 0 {
			payload.WriteByte(',')
		}
		payload.WriteString(fmt.Sprintf("%q", path))
	}
	payload.WriteString(`]}`)

	endpoint := c.baseURL + "/storage/v1/object/" + url.PathEscape(c.bucket)
	request, err := http.NewRequestWithContext(ctx, http.MethodDelete, endpoint, payload)
	if err != nil {
		return err
	}
	c.authorize(request)
	request.Header.Set("Content-Type", "application/json")

	response, err := c.httpClient.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 1<<20))
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("storage delete status %d", response.StatusCode)
	}
	return nil
}

// ObjectPath returns the bucket path for a public URL created by this client.
func (c *Client) ObjectPath(publicURL string) (string, bool) {
	if c == nil {
		return "", false
	}
	prefix := c.baseURL + "/storage/v1/object/public/" + url.PathEscape(c.bucket) + "/"
	if !strings.HasPrefix(publicURL, prefix) {
		return "", false
	}
	path, err := url.PathUnescape(strings.TrimPrefix(publicURL, prefix))
	if err != nil || !validObjectPath(path) {
		return "", false
	}
	return path, true
}

func (c *Client) authorize(request *http.Request) {
	request.Header.Set("Authorization", "Bearer "+c.apiKey)
	request.Header.Set("apikey", c.apiKey)
}

func escapePath(path string) string {
	parts := strings.Split(path, "/")
	for index, part := range parts {
		parts[index] = url.PathEscape(part)
	}
	return strings.Join(parts, "/")
}

func validObjectPath(path string) bool {
	if path == "" || strings.HasPrefix(path, "/") || strings.Contains(path, "..") || strings.Contains(path, "\\") {
		return false
	}
	return true
}
