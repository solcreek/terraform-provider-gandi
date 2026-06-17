// Package gandi is a minimal, dependency-free (stdlib only) client for the
// subset of the Gandi v5 API that this Terraform provider needs:
// domain info, nameservers, glue records (hosts) and LiveDNS records.
//
// It intentionally does NOT depend on github.com/go-gandi/go-gandi so that the
// provider owns its own HTTP behaviour: configurable timeout, 429 back-off and
// clear error messages.
package gandi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// DefaultBaseURL is the production Gandi API. Override for the sandbox.
const DefaultBaseURL = "https://api.gandi.net"

// SandboxBaseURL is the Gandi sandbox API.
const SandboxBaseURL = "https://api.sandbox.gandi.net"

// Client is a Gandi v5 API client.
type Client struct {
	http      *http.Client
	baseURL   string
	pat       string
	sharingID string
}

// Option configures a Client.
type Option func(*Client)

// WithBaseURL overrides the API base URL (e.g. the sandbox).
func WithBaseURL(u string) Option {
	return func(c *Client) {
		if u != "" {
			c.baseURL = strings.TrimRight(u, "/")
		}
	}
}

// WithSharingID scopes requests to a Gandi organization (sharing_id).
func WithSharingID(s string) Option { return func(c *Client) { c.sharingID = s } }

// WithTimeout sets the per-request timeout. This is the whole reason this
// client exists instead of go-gandi, whose default 5s timeout is not
// configurable through the upstream Terraform provider.
func WithTimeout(d time.Duration) Option {
	return func(c *Client) {
		if d > 0 {
			c.http.Timeout = d
		}
	}
}

// WithHTTPClient injects a custom *http.Client (mainly for tests).
func WithHTTPClient(h *http.Client) Option {
	return func(c *Client) {
		if h != nil {
			c.http = h
		}
	}
}

// New builds a Client authenticated with a Personal Access Token (PAT).
func New(pat string, opts ...Option) *Client {
	c := &Client{
		http:    &http.Client{Timeout: 30 * time.Second},
		baseURL: DefaultBaseURL,
		pat:     pat,
	}
	for _, o := range opts {
		o(c)
	}
	return c
}

// APIError is returned for any non-2xx response.
type APIError struct {
	StatusCode int
	Message    string
	Cause      string
	Body       string
}

func (e *APIError) Error() string {
	if e.Message != "" {
		return fmt.Sprintf("gandi API error %d: %s", e.StatusCode, e.Message)
	}
	return fmt.Sprintf("gandi API error %d: %s", e.StatusCode, strings.TrimSpace(e.Body))
}

// IsNotFound reports whether err represents a missing resource. Gandi is not
// consistent here: most endpoints return 404, but some (e.g. glue records)
// return 400 with cause "CAUSE_NOTFOUND".
func IsNotFound(err error) bool {
	var ae *APIError
	if !errors.As(err, &ae) {
		return false
	}
	return ae.StatusCode == http.StatusNotFound || ae.Cause == "CAUSE_NOTFOUND"
}

// standardError matches Gandi's error envelope.
type standardError struct {
	Message string `json:"message"`
	Cause   string `json:"cause"`
	Errors  []struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	} `json:"errors"`
}

// do performs a request, marshalling body (if non-nil) and unmarshalling the
// response into out (if non-nil). It retries on HTTP 429 honouring Retry-After.
func (c *Client) do(ctx context.Context, method, path string, body, out any) error {
	var payload []byte
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshal request body: %w", err)
		}
		payload = b
	}

	url := c.baseURL + path
	if c.sharingID != "" {
		sep := "?"
		if strings.Contains(url, "?") {
			sep = "&"
		}
		url += sep + "sharing_id=" + c.sharingID
	}

	const maxAttempts = 4
	var lastErr error
	for attempt := 0; attempt < maxAttempts; attempt++ {
		var rdr io.Reader
		if payload != nil {
			rdr = bytes.NewReader(payload)
		}
		req, err := http.NewRequestWithContext(ctx, method, url, rdr)
		if err != nil {
			return err
		}
		req.Header.Set("Authorization", "Bearer "+c.pat)
		req.Header.Set("Accept", "application/json")
		if payload != nil {
			req.Header.Set("Content-Type", "application/json")
		}

		resp, err := c.http.Do(req)
		if err != nil {
			return fmt.Errorf("%s %s: %w", method, path, err)
		}

		// Rate limited: back off and retry.
		if resp.StatusCode == http.StatusTooManyRequests && attempt < maxAttempts-1 {
			wait := retryAfter(resp.Header.Get("Retry-After"), attempt)
			io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(wait):
			}
			lastErr = &APIError{StatusCode: resp.StatusCode, Message: "rate limited"}
			continue
		}

		respBody, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		if resp.StatusCode < 200 || resp.StatusCode > 299 {
			return parseError(resp.StatusCode, respBody)
		}

		if out != nil && len(respBody) > 0 {
			if err := json.Unmarshal(respBody, out); err != nil {
				return fmt.Errorf("decode response: %w", err)
			}
		}
		return nil
	}
	return lastErr
}

func parseError(status int, body []byte) error {
	ae := &APIError{StatusCode: status, Body: string(body)}
	var se standardError
	if json.Unmarshal(body, &se) == nil {
		ae.Cause = se.Cause
		if se.Message != "" {
			ae.Message = se.Message
		} else if len(se.Errors) > 0 {
			parts := make([]string, 0, len(se.Errors))
			for _, e := range se.Errors {
				parts = append(parts, e.Name+": "+e.Description)
			}
			ae.Message = strings.Join(parts, ", ")
		}
	}
	// Make credential problems actionable: a PAT can be invalid, expired, or
	// lack the permission/organization scope for this call.
	switch status {
	case http.StatusUnauthorized:
		ae.Message = strings.TrimSpace(ae.Message + " (the Personal Access Token is missing, invalid, or expired — check GANDI_PAT)")
	case http.StatusForbidden:
		ae.Message = strings.TrimSpace(ae.Message + " (the Personal Access Token lacks permission or organization scope for this resource)")
	}
	return ae
}

func retryAfter(header string, attempt int) time.Duration {
	if header != "" {
		if secs, err := strconv.Atoi(strings.TrimSpace(header)); err == nil && secs >= 0 {
			return time.Duration(secs) * time.Second
		}
	}
	// Exponential fallback: 1s, 2s, 4s.
	return time.Duration(1<<attempt) * time.Second
}
