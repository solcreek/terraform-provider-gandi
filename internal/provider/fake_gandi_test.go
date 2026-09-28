package provider

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/solcreek/terraform-provider-gandi/internal/gandi"
)

// fakeGandi is an in-memory stand-in for the parts of the Gandi API the DNSSEC
// resources use, so their full Terraform lifecycle can be tested without
// credentials. Like the real API it answers create/delete with 202 and no id.
type fakeGandi struct {
	t   *testing.T
	srv *httptest.Server

	mu              sync.Mutex
	nextID          int
	keys            map[string][]gandi.DNSSECKey // domain -> keys
	dnssecAvailable *bool
	failMethod      string // requests with this method get a 400 while set
}

func newFakeGandi(t *testing.T) *fakeGandi {
	t.Helper()
	f := &fakeGandi{t: t, nextID: 100, keys: map[string][]gandi.DNSSECKey{}}
	f.srv = httptest.NewServer(http.HandlerFunc(f.serve))
	t.Cleanup(f.srv.Close)
	return f
}

func (f *fakeGandi) providerConfig() string {
	return fmt.Sprintf(`
provider "gandi" {
  personal_access_token = "test"
  api_url               = %q
}
`, f.srv.URL)
}

// addKey inserts a key directly, as if it were added in the dashboard.
func (f *fakeGandi) addKey(domain string, algorithm int, typ, pk string) gandi.DNSSECKey {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.nextID++
	k := gandi.DNSSECKey{
		ID: f.nextID, Algorithm: algorithm, Type: typ, PublicKey: pk,
		KeyTag: 20000 + f.nextID, Digest: fmt.Sprintf("DIGEST%d", f.nextID), DigestType: 2,
	}
	f.keys[domain] = append(f.keys[domain], k)
	return k
}

// removeKey deletes a key directly, as if it were removed in the dashboard.
func (f *fakeGandi) removeKey(domain string, id int) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	for i, k := range f.keys[domain] {
		if k.ID == id {
			f.keys[domain] = append(f.keys[domain][:i], f.keys[domain][i+1:]...)
			return true
		}
	}
	return false
}

// failRequests makes every request with the given HTTP method fail with a
// Gandi-style 400 until called again with "".
func (f *fakeGandi) failRequests(method string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.failMethod = method
}

func (f *fakeGandi) keyCount(domain string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.keys[domain])
}

func (f *fakeGandi) serve(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	fail := f.failMethod != "" && r.Method == f.failMethod
	f.mu.Unlock()
	if fail {
		f.json(w, http.StatusBadRequest, map[string]any{"code": 400, "cause": "Bad Request", "message": "injected failure"})
		return
	}

	rest, ok := strings.CutPrefix(r.URL.Path, "/v5/domain/domains/")
	if !ok {
		f.notFound(w)
		return
	}
	parts := strings.Split(rest, "/")
	domain := parts[0]

	switch {
	case len(parts) == 1 && r.Method == http.MethodGet:
		f.json(w, http.StatusOK, map[string]any{
			"fqdn": domain, "tld": domain[strings.LastIndex(domain, ".")+1:], "id": "dom-1",
			"status": []string{}, "nameservers": []string{"a.ns.example"},
			"dates": map[string]string{"registry_ends_at": "2030-01-01T00:00:00Z"},
		})
	case len(parts) == 2 && parts[1] == "livedns" && r.Method == http.MethodGet:
		body := map[string]any{"current": "other"}
		if f.dnssecAvailable != nil {
			body["dnssec_available"] = *f.dnssecAvailable
		}
		f.json(w, http.StatusOK, body)
	case len(parts) == 2 && parts[1] == "dnskeys" && r.Method == http.MethodGet:
		f.mu.Lock()
		keys := append([]gandi.DNSSECKey{}, f.keys[domain]...)
		f.mu.Unlock()
		f.json(w, http.StatusOK, keys)
	case len(parts) == 2 && parts[1] == "dnskeys" && r.Method == http.MethodPost:
		var body struct {
			Algorithm int    `json:"algorithm"`
			Type      string `json:"type"`
			PublicKey string `json:"public_key"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			f.t.Errorf("fake gandi: decode create body: %v", err)
		}
		// Stored compact, as the real API returns it.
		f.addKey(domain, body.Algorithm, body.Type, gandi.NormalizePublicKey(body.PublicKey))
		f.json(w, http.StatusAccepted, map[string]string{"message": "DNSSEC key created"})
	case len(parts) == 3 && parts[1] == "dnskeys" && r.Method == http.MethodDelete:
		id, _ := strconv.Atoi(parts[2])
		if !f.removeKey(domain, id) {
			f.notFound(w)
			return
		}
		f.json(w, http.StatusAccepted, map[string]string{"message": "DNSSEC key deleted"})
	default:
		f.notFound(w)
	}
}

func (f *fakeGandi) json(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func (f *fakeGandi) notFound(w http.ResponseWriter) {
	f.json(w, http.StatusNotFound, map[string]any{"code": 404, "message": "not found"})
}

func ptr[T any](v T) *T { return &v }
