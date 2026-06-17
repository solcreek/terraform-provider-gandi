package gandi

import (
	"context"
	"net/url"
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
