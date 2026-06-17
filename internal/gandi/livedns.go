package gandi

import (
	"context"
	"net/url"
)

// LiveDNSRecord is a single LiveDNS rrset.
type LiveDNSRecord struct {
	Name   string   `json:"rrset_name"`
	Type   string   `json:"rrset_type"`
	TTL    int64    `json:"rrset_ttl"`
	Values []string `json:"rrset_values"`
}

func recordPath(fqdn, name, rtype string) string {
	return "/v5/livedns/domains/" + url.PathEscape(fqdn) + "/records/" +
		url.PathEscape(name) + "/" + url.PathEscape(rtype)
}

// GetLiveDNSRecord fetches a single record by name and type.
func (c *Client) GetLiveDNSRecord(ctx context.Context, fqdn, name, rtype string) (*LiveDNSRecord, error) {
	var r LiveDNSRecord
	if err := c.do(ctx, "GET", recordPath(fqdn, name, rtype), nil, &r); err != nil {
		return nil, err
	}
	return &r, nil
}

// CreateLiveDNSRecord creates a record. Gandi rejects duplicates of the same
// (name, type), so callers should treat 409 as "already exists".
func (c *Client) CreateLiveDNSRecord(ctx context.Context, fqdn string, rec LiveDNSRecord) error {
	body := map[string]any{
		"rrset_name":   rec.Name,
		"rrset_type":   rec.Type,
		"rrset_ttl":    rec.TTL,
		"rrset_values": rec.Values,
	}
	return c.do(ctx, "POST", "/v5/livedns/domains/"+url.PathEscape(fqdn)+"/records", body, nil)
}

// UpdateLiveDNSRecord replaces the TTL and values of a record.
func (c *Client) UpdateLiveDNSRecord(ctx context.Context, fqdn string, rec LiveDNSRecord) error {
	body := map[string]any{
		"rrset_ttl":    rec.TTL,
		"rrset_values": rec.Values,
	}
	return c.do(ctx, "PUT", recordPath(fqdn, rec.Name, rec.Type), body, nil)
}

// DeleteLiveDNSRecord removes a record by name and type.
func (c *Client) DeleteLiveDNSRecord(ctx context.Context, fqdn, name, rtype string) error {
	return c.do(ctx, "DELETE", recordPath(fqdn, name, rtype), nil, nil)
}
