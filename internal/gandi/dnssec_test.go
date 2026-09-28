package gandi

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"testing"
)

func TestListDNSSECKeysPaginates(t *testing.T) {
	var pages []string
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v5/domain/domains/example.com/dnskeys" {
			t.Errorf("path = %s", r.URL.Path)
		}
		if got := r.URL.Query().Get("per_page"); got != strconv.Itoa(dnskeysPageSize) {
			t.Errorf("per_page = %q", got)
		}
		page := r.URL.Query().Get("page")
		pages = append(pages, page)
		n := dnskeysPageSize // full first page forces a second request
		if page != "1" {
			n = 1
		}
		keys := make([]DNSSECKey, n)
		for i := range keys {
			keys[i] = DNSSECKey{ID: len(pages)*1000 + i}
		}
		_ = json.NewEncoder(w).Encode(keys)
	})
	keys, err := c.ListDNSSECKeys(context.Background(), "example.com")
	if err != nil {
		t.Fatal(err)
	}
	if len(keys) != dnskeysPageSize+1 {
		t.Fatalf("len(keys) = %d, want %d", len(keys), dnskeysPageSize+1)
	}
	if len(pages) != 2 || pages[0] != "1" || pages[1] != "2" {
		t.Fatalf("pages requested = %v, want [1 2]", pages)
	}
}

func TestListDNSSECKeysKeepsSharingID(t *testing.T) {
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if q.Get("sharing_id") != "org-123" || q.Get("page") != "1" {
			t.Errorf("query = %s", r.URL.RawQuery)
		}
		w.Write([]byte(`[]`))
	}, WithSharingID("org-123"))
	if _, err := c.ListDNSSECKeys(context.Background(), "example.com"); err != nil {
		t.Fatal(err)
	}
}

func TestDNSSECKeyDecode(t *testing.T) {
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`[{"id":42,"algorithm":13,"type":"ksk","public_key":"AbC=","keytag":2371,"digest":"DEAD","digest_type":2,"href":"x"}]`))
	})
	k, err := c.GetDNSSECKey(context.Background(), "example.com", 42)
	if err != nil {
		t.Fatal(err)
	}
	want := DNSSECKey{ID: 42, Algorithm: 13, Type: "ksk", PublicKey: "AbC=", KeyTag: 2371, Digest: "DEAD", DigestType: 2}
	if *k != want {
		t.Fatalf("key = %+v, want %+v", *k, want)
	}
}

func TestGetDNSSECKeyNotFound(t *testing.T) {
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`[{"id":1}]`))
	})
	_, err := c.GetDNSSECKey(context.Background(), "example.com", 2)
	if !IsNotFound(err) {
		t.Fatalf("IsNotFound = false, want true (err=%v)", err)
	}
}

func TestFindDNSSECKeyByPublicKeyIgnoresWhitespace(t *testing.T) {
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`[{"id":1,"public_key":"AAAA"},{"id":2,"public_key":"BBBBCCCC"}]`))
	})
	k, err := c.FindDNSSECKeyByPublicKey(context.Background(), "example.com", " BBBB CCCC\n")
	if err != nil {
		t.Fatal(err)
	}
	if k == nil || k.ID != 2 {
		t.Fatalf("key = %+v, want id 2", k)
	}
	k, err = c.FindDNSSECKeyByPublicKey(context.Background(), "example.com", "ZZZZ")
	if err != nil || k != nil {
		t.Fatalf("missing key: got %+v, %v; want nil, nil", k, err)
	}
}

func TestCreateDNSSECKeyBody(t *testing.T) {
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v5/domain/domains/example.com/dnskeys" {
			t.Errorf("%s %s", r.Method, r.URL.Path)
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if body["algorithm"] != float64(13) || body["type"] != "ksk" || body["public_key"] != "AbC=" {
			t.Errorf("body = %v", body)
		}
		w.WriteHeader(http.StatusAccepted)
		w.Write([]byte(`{"message":"DNSSEC key created"}`))
	})
	if err := c.CreateDNSSECKey(context.Background(), "example.com", 13, "ksk", "AbC="); err != nil {
		t.Fatal(err)
	}
}

func TestDeleteDNSSECKeyPath(t *testing.T) {
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete || r.URL.Path != "/v5/domain/domains/example.com/dnskeys/42" {
			t.Errorf("%s %s", r.Method, r.URL.Path)
		}
		w.WriteHeader(http.StatusAccepted)
	})
	if err := c.DeleteDNSSECKey(context.Background(), "example.com", 42); err != nil {
		t.Fatal(err)
	}
}

func TestWaitForDNSSECKeyReturnsKey(t *testing.T) {
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`[{"id":7,"public_key":"AbC="}]`))
	})
	k, err := c.WaitForDNSSECKey(context.Background(), "example.com", "AbC=")
	if err != nil {
		t.Fatal(err)
	}
	if k.ID != 7 {
		t.Fatalf("id = %d, want 7", k.ID)
	}
}

func TestWaitForDNSSECKeyGone(t *testing.T) {
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`[{"id":1}]`))
	})
	if err := c.WaitForDNSSECKeyGone(context.Background(), "example.com", 2); err != nil {
		t.Fatal(err)
	}
}

func TestWaitForDNSSECKeyPropagatesErrors(t *testing.T) {
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	})
	if _, err := c.WaitForDNSSECKey(context.Background(), "example.com", "AbC="); err == nil {
		t.Fatal("want error, got nil")
	}
	if err := c.WaitForDNSSECKeyGone(context.Background(), "example.com", 1); err == nil {
		t.Fatal("want error, got nil")
	}
}

func TestPollRespectsContext(t *testing.T) {
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`[]`)) // key never appears
	})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := c.WaitForDNSSECKey(ctx, "example.com", "AbC="); err == nil {
		t.Fatal("want context error, got nil")
	}
}

func TestNormalizePublicKey(t *testing.T) {
	for in, want := range map[string]string{
		"AbC=":      "AbC=",
		"Ab C=":     "AbC=",
		"\tAb\nC= ": "AbC=",
		"":          "",
		"A B C":     "ABC",
	} {
		if got := NormalizePublicKey(in); got != want {
			t.Errorf("NormalizePublicKey(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestGetDomainLiveDNS(t *testing.T) {
	for body, want := range map[string]*bool{
		`{"current":"other","dnssec_available":true}`:  ptr(true),
		`{"current":"other","dnssec_available":false}`: ptr(false),
		`{"current":"other"}`:                          nil,
	} {
		c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/v5/domain/domains/example.com/livedns" {
				t.Errorf("path = %s", r.URL.Path)
			}
			w.Write([]byte(body))
		})
		l, err := c.GetDomainLiveDNS(context.Background(), "example.com")
		if err != nil {
			t.Fatal(err)
		}
		if (want == nil) != (l.DNSSECAvailable == nil) || (want != nil && *want != *l.DNSSECAvailable) {
			t.Errorf("%s: DNSSECAvailable = %v, want %v", body, l.DNSSECAvailable, want)
		}
	}
}

func ptr[T any](v T) *T { return &v }
