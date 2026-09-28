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

// DomainLiveDNS is the LiveDNS/DNSSEC status of a domain. The DNSSEC flags are
// optional in the API response, hence pointers: nil means "not reported".
type DomainLiveDNS struct {
	Current             string   `json:"current"`
	Nameservers         []string `json:"nameservers"`
	DNSSECAvailable     *bool    `json:"dnssec_available"`
	LiveDNSSECAvailable *bool    `json:"livednssec_available"`
}

// GetDomainLiveDNS returns the LiveDNS status of a domain, including whether
// its registry accepts DNSSEC keys.
func (c *Client) GetDomainLiveDNS(ctx context.Context, fqdn string) (*DomainLiveDNS, error) {
	var l DomainLiveDNS
	if err := c.do(ctx, "GET", "/v5/domain/domains/"+url.PathEscape(fqdn)+"/livedns", nil, &l); err != nil {
		return nil, err
	}
	return &l, nil
}
