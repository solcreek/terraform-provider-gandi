package gandi

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

// DNSSECKey is a DNSKEY submitted to the registry. Gandi derives the DS
// record (keytag/digest) from it; there is no way to submit a raw DS.
type DNSSECKey struct {
	ID         int    `json:"id"`
	Algorithm  int    `json:"algorithm"`
	Type       string `json:"type"`
	PublicKey  string `json:"public_key"`
	KeyTag     int    `json:"keytag"`
	Digest     string `json:"digest"`
	DigestType int    `json:"digest_type"`
}

func dnskeysPath(fqdn string) string {
	return "/v5/domain/domains/" + url.PathEscape(fqdn) + "/dnskeys"
}

// dnskeysPageSize is the per_page used when listing keys. A domain rarely has
// more than a handful, but the endpoint is paginated so we walk every page.
const dnskeysPageSize = 100

// ListDNSSECKeys returns all DNSSEC keys registered for a domain.
func (c *Client) ListDNSSECKeys(ctx context.Context, fqdn string) ([]DNSSECKey, error) {
	var all []DNSSECKey
	for page := 1; ; page++ {
		var keys []DNSSECKey
		p := fmt.Sprintf("%s?per_page=%d&page=%d", dnskeysPath(fqdn), dnskeysPageSize, page)
		if err := c.do(ctx, "GET", p, nil, &keys); err != nil {
			return nil, err
		}
		all = append(all, keys...)
		if len(keys) < dnskeysPageSize {
			return all, nil
		}
	}
}

// GetDNSSECKey returns a single key by id. The API has no per-key GET, so this
// filters the list; a missing key yields a not-found *APIError.
func (c *Client) GetDNSSECKey(ctx context.Context, fqdn string, id int) (*DNSSECKey, error) {
	keys, err := c.ListDNSSECKeys(ctx, fqdn)
	if err != nil {
		return nil, err
	}
	for i := range keys {
		if keys[i].ID == id {
			return &keys[i], nil
		}
	}
	return nil, &APIError{StatusCode: 404, Cause: "CAUSE_NOTFOUND",
		Message: fmt.Sprintf("DNSSEC key %d not found on %s", id, fqdn)}
}

// FindDNSSECKeyByPublicKey returns the key whose public key matches pk
// (ignoring whitespace), or nil if there is none.
func (c *Client) FindDNSSECKeyByPublicKey(ctx context.Context, fqdn, pk string) (*DNSSECKey, error) {
	keys, err := c.ListDNSSECKeys(ctx, fqdn)
	if err != nil {
		return nil, err
	}
	want := NormalizePublicKey(pk)
	for i := range keys {
		if NormalizePublicKey(keys[i].PublicKey) == want {
			return &keys[i], nil
		}
	}
	return nil, nil
}

// CreateDNSSECKey submits a DNSKEY. The API answers 202 without the new id;
// use WaitForDNSSECKey to learn it.
func (c *Client) CreateDNSSECKey(ctx context.Context, fqdn string, algorithm int, keyType, publicKey string) error {
	body := map[string]any{"algorithm": algorithm, "type": keyType, "public_key": publicKey}
	return c.do(ctx, "POST", dnskeysPath(fqdn), body, nil)
}

// DeleteDNSSECKey removes a key by id.
func (c *Client) DeleteDNSSECKey(ctx context.Context, fqdn string, id int) error {
	return c.do(ctx, "DELETE", dnskeysPath(fqdn)+"/"+strconv.Itoa(id), nil, nil)
}

// WaitForDNSSECKey polls until a key with the given public key appears and
// returns it.
func (c *Client) WaitForDNSSECKey(ctx context.Context, fqdn, publicKey string) (*DNSSECKey, error) {
	var found *DNSSECKey
	err := poll(ctx, func() (bool, error) {
		k, err := c.FindDNSSECKeyByPublicKey(ctx, fqdn, publicKey)
		if err != nil {
			return false, err
		}
		found = k
		return k != nil, nil
	}, fmt.Sprintf("DNSSEC key to appear on %s", fqdn))
	return found, err
}

// WaitForDNSSECKeyGone polls until the key with the given id no longer exists.
func (c *Client) WaitForDNSSECKeyGone(ctx context.Context, fqdn string, id int) error {
	return poll(ctx, func() (bool, error) {
		_, err := c.GetDNSSECKey(ctx, fqdn, id)
		if err == nil {
			return false, nil
		}
		if IsNotFound(err) {
			return true, nil
		}
		return false, err
	}, fmt.Sprintf("DNSSEC key %d on %s to be deleted", id, fqdn))
}

// NormalizePublicKey strips whitespace so a key pasted from zone-file output
// (which splits the base64 into space-separated chunks) compares equal to the
// compact form the API returns.
func NormalizePublicKey(pk string) string {
	return strings.Join(strings.Fields(pk), "")
}
