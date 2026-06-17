package gandi

import (
	"context"
	"net/url"
)

// Domain is the subset of the domain info object we expose.
type Domain struct {
	FQDN        string   `json:"fqdn"`
	FQDNUnicode string   `json:"fqdn_unicode"`
	TLD         string   `json:"tld"`
	ID          string   `json:"id"`
	Status      []string `json:"status"`
	Nameservers []string `json:"nameservers"`
	Tags        []string `json:"tags"`
	Dates       struct {
		CreatedAt      string `json:"created_at"`
		UpdatedAt      string `json:"updated_at"`
		RegistryEndsAt string `json:"registry_ends_at"`
	} `json:"dates"`
}

// GetDomain fetches a single domain by FQDN.
func (c *Client) GetDomain(ctx context.Context, fqdn string) (*Domain, error) {
	var d Domain
	if err := c.do(ctx, "GET", "/v5/domain/domains/"+url.PathEscape(fqdn), nil, &d); err != nil {
		return nil, err
	}
	return &d, nil
}

// GetNameservers returns the registry-level nameservers for a domain.
func (c *Client) GetNameservers(ctx context.Context, fqdn string) ([]string, error) {
	var ns []string
	if err := c.do(ctx, "GET", "/v5/domain/domains/"+url.PathEscape(fqdn)+"/nameservers", nil, &ns); err != nil {
		return nil, err
	}
	return ns, nil
}

// SetNameservers replaces the registry-level nameservers for a domain.
func (c *Client) SetNameservers(ctx context.Context, fqdn string, ns []string) error {
	body := map[string][]string{"nameservers": ns}
	return c.do(ctx, "PUT", "/v5/domain/domains/"+url.PathEscape(fqdn)+"/nameservers", body, nil)
}
