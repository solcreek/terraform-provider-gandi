package gandi

import (
	"context"
	"net/url"
)

// Domain is the subset of the domain info object we expose.
type Domain struct {
	FQDN        string     `json:"fqdn"`
	FQDNUnicode string     `json:"fqdn_unicode"`
	TLD         string     `json:"tld"`
	ID          string     `json:"id"`
	Status      []string   `json:"status"`
	Nameservers []string   `json:"nameservers"`
	Tags        []string   `json:"tags"`
	Autorenew   *Autorenew `json:"autorenew"`
	Dates       struct {
		CreatedAt           string `json:"created_at"`
		UpdatedAt           string `json:"updated_at"`
		RegistryEndsAt      string `json:"registry_ends_at"`
		RenewBeginsAt       string `json:"renew_begins_at"`
		HoldBeginsAt        string `json:"hold_begins_at"`
		HoldEndsAt          string `json:"hold_ends_at"`
		DeletesAt           string `json:"deletes_at"`
		RestoreEndsAt       string `json:"restore_ends_at"`
		PendingDeleteEndsAt string `json:"pending_delete_ends_at"`
	} `json:"dates"`
}

// Autorenew is a domain's automatic renewal setting, as embedded in the domain
// info object. Dates are the scheduled renewal attempts.
type Autorenew struct {
	Enabled  bool     `json:"enabled"`
	Duration int      `json:"duration"`
	OrgID    string   `json:"org_id"`
	Dates    []string `json:"dates"`
}

// AutorenewRequest is the body of PATCH /domain/domains/{domain}/autorenew.
// Duration (years, 1–9) and OrgID (the organization billed) are optional.
type AutorenewRequest struct {
	Enabled  bool   `json:"enabled"`
	Duration int    `json:"duration,omitempty"`
	OrgID    string `json:"org_id,omitempty"`
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

// SetAutorenew changes a domain's automatic renewal setting. It does not renew
// the domain or charge anything by itself.
func (c *Client) SetAutorenew(ctx context.Context, fqdn string, req AutorenewRequest) error {
	return c.do(ctx, "PATCH", "/v5/domain/domains/"+url.PathEscape(fqdn)+"/autorenew", req, nil)
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
