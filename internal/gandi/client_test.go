package gandi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func testClient(t *testing.T, h http.HandlerFunc, opts ...Option) *Client {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return New("test-pat", append([]Option{WithBaseURL(srv.URL)}, opts...)...)
}

func TestAuthorizationHeader(t *testing.T) {
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer test-pat" {
			t.Errorf("Authorization = %q, want %q", got, "Bearer test-pat")
		}
		w.Write([]byte(`["ns1.example.net"]`))
	})
	if _, err := c.GetNameservers(context.Background(), "example.com"); err != nil {
		t.Fatal(err)
	}
}

func TestSharingIDQueryParam(t *testing.T) {
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("sharing_id"); got != "org-123" {
			t.Errorf("sharing_id = %q, want org-123", got)
		}
		w.Write([]byte(`[]`))
	}, WithSharingID("org-123"))
	if _, err := c.ListHosts(context.Background(), "example.com"); err != nil {
		t.Fatal(err)
	}
}

func TestNotFound(t *testing.T) {
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"message":"not found"}`))
	})
	_, err := c.GetDomain(context.Background(), "missing.example")
	if !IsNotFound(err) {
		t.Fatalf("IsNotFound = false, want true (err=%v)", err)
	}
}

func TestNotFoundViaCause(t *testing.T) {
	// Glue records return 400 with cause CAUSE_NOTFOUND instead of a 404.
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"code":400,"cause":"CAUSE_NOTFOUND","message":"Host 'ns1.example.com' doesn't exist"}`))
	})
	_, err := c.GetHost(context.Background(), "example.com", "ns1")
	if !IsNotFound(err) {
		t.Fatalf("IsNotFound = false for 400+CAUSE_NOTFOUND, want true (err=%v)", err)
	}
}

func TestUnauthorizedHint(t *testing.T) {
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"message":"unauthorized"}`))
	})
	_, err := c.GetDomain(context.Background(), "example.com")
	if err == nil || !strings.Contains(err.Error(), "Personal Access Token") {
		t.Fatalf("want PAT hint in error, got %v", err)
	}
}

func TestRetryOn429(t *testing.T) {
	var calls int32
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		if atomic.AddInt32(&calls, 1) == 1 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.Write([]byte(`["ns1.example.net"]`))
	})
	ns, err := c.GetNameservers(context.Background(), "example.com")
	if err != nil {
		t.Fatal(err)
	}
	if atomic.LoadInt32(&calls) != 2 {
		t.Fatalf("calls = %d, want 2 (one 429 then success)", calls)
	}
	if len(ns) != 1 || ns[0] != "ns1.example.net" {
		t.Fatalf("ns = %v", ns)
	}
}

func TestSetNameserversBody(t *testing.T) {
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("method = %s, want PUT", r.Method)
		}
		var body struct {
			Nameservers []string `json:"nameservers"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if len(body.Nameservers) != 2 {
			t.Errorf("nameservers = %v, want 2 entries", body.Nameservers)
		}
		w.WriteHeader(http.StatusOK)
	})
	err := c.SetNameservers(context.Background(), "example.com", []string{"a.ns", "b.ns"})
	if err != nil {
		t.Fatal(err)
	}
}

func TestLiveDNSRecordRoundTrip(t *testing.T) {
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"rrset_name":"www","rrset_type":"A","rrset_ttl":3600,"rrset_values":["203.0.113.1"]}`))
	})
	rec, err := c.GetLiveDNSRecord(context.Background(), "example.com", "www", "A")
	if err != nil {
		t.Fatal(err)
	}
	if rec.TTL != 3600 || rec.Name != "www" || len(rec.Values) != 1 {
		t.Fatalf("rec = %+v", rec)
	}
}

func TestTimeoutIsConfigurable(t *testing.T) {
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(50 * time.Millisecond)
		w.Write([]byte(`[]`))
	}, WithTimeout(10*time.Millisecond))
	_, err := c.ListHosts(context.Background(), "example.com")
	if err == nil {
		t.Fatal("expected timeout error, got nil")
	}
}
