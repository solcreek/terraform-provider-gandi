package gandi

import (
	"context"
	"fmt"
	"net/url"
	"sort"
	"time"
)

// Host is a glue record: a nameserver name registered at the registry with
// one or more IP addresses, e.g. ns1.example.com -> 203.0.113.1.
type Host struct {
	Name string   `json:"name"`
	FQDN string   `json:"fqdn,omitempty"`
	IPs  []string `json:"ips"`
}

func hostsPath(fqdn string) string {
	return "/v5/domain/domains/" + url.PathEscape(fqdn) + "/hosts"
}

// ListHosts returns all glue records for a domain.
func (c *Client) ListHosts(ctx context.Context, fqdn string) ([]Host, error) {
	var hosts []Host
	if err := c.do(ctx, "GET", hostsPath(fqdn), nil, &hosts); err != nil {
		return nil, err
	}
	return hosts, nil
}

// GetHost returns a single glue record by its short name (e.g. "ns1").
func (c *Client) GetHost(ctx context.Context, fqdn, name string) (*Host, error) {
	var h Host
	if err := c.do(ctx, "GET", hostsPath(fqdn)+"/"+url.PathEscape(name), nil, &h); err != nil {
		return nil, err
	}
	return &h, nil
}

// CreateHost creates a glue record.
func (c *Client) CreateHost(ctx context.Context, fqdn, name string, ips []string) error {
	body := map[string]any{"name": name, "ips": ips}
	return c.do(ctx, "POST", hostsPath(fqdn), body, nil)
}

// UpdateHost replaces the IPs of an existing glue record.
func (c *Client) UpdateHost(ctx context.Context, fqdn, name string, ips []string) error {
	body := map[string]any{"ips": ips}
	return c.do(ctx, "PUT", hostsPath(fqdn)+"/"+url.PathEscape(name), body, nil)
}

// DeleteHost removes a glue record.
func (c *Client) DeleteHost(ctx context.Context, fqdn, name string) error {
	return c.do(ctx, "DELETE", hostsPath(fqdn)+"/"+url.PathEscape(name), nil, nil)
}

// Glue record changes are asynchronous: the API responds 202 ("in progress")
// and the host becomes consistent a few seconds later. These helpers poll until
// the registry reflects the desired state so Terraform state stays accurate.

const (
	hostPollInterval = 2 * time.Second
	hostPollTimeout  = 90 * time.Second
)

// WaitForHostIPs polls until the host exists with exactly the given IPs.
func (c *Client) WaitForHostIPs(ctx context.Context, fqdn, name string, ips []string) error {
	want := sortedCopy(ips)
	return poll(ctx, func() (bool, error) {
		h, err := c.GetHost(ctx, fqdn, name)
		if err != nil {
			if IsNotFound(err) {
				return false, nil
			}
			return false, err
		}
		return equalStringSets(sortedCopy(h.IPs), want), nil
	}, fmt.Sprintf("glue record %s.%s to reach desired IPs", name, fqdn))
}

// WaitForHostGone polls until the host no longer exists.
func (c *Client) WaitForHostGone(ctx context.Context, fqdn, name string) error {
	return poll(ctx, func() (bool, error) {
		_, err := c.GetHost(ctx, fqdn, name)
		if err == nil {
			return false, nil
		}
		if IsNotFound(err) {
			return true, nil
		}
		return false, err
	}, fmt.Sprintf("glue record %s.%s to be deleted", name, fqdn))
}

func poll(ctx context.Context, check func() (bool, error), desc string) error {
	deadline := time.Now().Add(hostPollTimeout)
	for {
		done, err := check()
		if err != nil {
			return err
		}
		if done {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("timed out after %s waiting for %s", hostPollTimeout, desc)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(hostPollInterval):
		}
	}
}

func sortedCopy(in []string) []string {
	out := append([]string(nil), in...)
	sort.Strings(out)
	return out
}

func equalStringSets(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
