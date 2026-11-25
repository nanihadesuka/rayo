package mock

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestProxy(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("USERPROFILE", t.TempDir())
	ca, err := LoadCA()
	if err != nil {
		t.Fatal(err)
	}
	m := New()
	r, err := ParseRule([]byte(`{"request":{"method":"GET","url":"/api.example.com/hello?a=1"},"response":{"status":200,"body":"hi"}}`))
	if err != nil {
		t.Fatal(err)
	}
	if err := m.Add(r); err != nil {
		t.Fatal(err)
	}
	if !m.Hosts()["api.example.com"] {
		t.Fatalf("hosts: %v", m.Hosts())
	}
	p := NewProxy(m, ca, func(host string) bool { return m.Hosts()[host] })
	port, err := p.Start()
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	proxy, _ := url.Parse(fmt.Sprintf("http://127.0.0.1:%d", port))
	client := func(trust bool) *http.Client {
		roots := x509.NewCertPool()
		if trust {
			roots.AppendCertsFromPEM(ca.PEM())
		}
		return &http.Client{Transport: &http.Transport{Proxy: http.ProxyURL(proxy), TLSClientConfig: &tls.Config{RootCAs: roots}}}
	}
	get := func(c *http.Client, address string) (string, error) {
		res, err := c.Get(address)
		if err != nil {
			return "", err
		}
		defer res.Body.Close()
		data, _ := io.ReadAll(res.Body)
		return fmt.Sprintf("%d %s", res.StatusCode, data), nil
	}

	if got, err := get(client(true), "https://api.example.com/hello?a=1"); err != nil || got != "200 hi" {
		t.Fatalf("mocked: %q %v", got, err)
	}
	if got, err := get(client(true), "https://api.example.com/other"); err != nil || !strings.HasPrefix(got, "404 not mocked: GET /api.example.com/other") {
		t.Fatalf("not mocked: %q %v", got, err)
	}
	// An app that still rewrites its URLs reaches the mock through the same port.
	direct := &http.Client{}
	if got, err := get(direct, fmt.Sprintf("http://127.0.0.1:%d/api.example.com/hello?a=1", port)); err != nil || got != "200 hi" {
		t.Fatalf("interceptor: %q %v", got, err)
	}
	// An app that doesn't trust the CA: an error of the run at once, as its calls would reach the real
	// backend; and its calls after are refused, never sent on.
	if _, err := get(client(false), "https://api.example.com/hello?a=1"); err == nil {
		t.Fatal("an untrusted CA was accepted")
	}
	var failed error
	for limit := time.Now().Add(2 * time.Second); time.Now().Before(limit); time.Sleep(10 * time.Millisecond) {
		// the proxy tells it once its side of the handshake ends, which may be after the app's
		if failed = m.Err(); failed != nil {
			break
		}
	}
	if failed == nil || !strings.Contains(failed.Error(), "pinned or untrusted CA: api.example.com rejected rayo's CA") {
		t.Fatalf("err: %v", failed)
	}
	if _, err := get(client(false), "https://api.example.com/hello?a=1"); err == nil || strings.Count(m.Err().Error(), "pinned") != 1 {
		t.Fatalf("a refused host was sent on, or told twice: %v, %v", err, m.Err())
	}
	calls := m.Calls()
	if len(calls) != 3 || calls[0].URI != "/api.example.com/hello?a=1" {
		t.Fatalf("calls: %v", calls)
	}
}

func TestMatches(t *testing.T) {
	for host, want := range map[string]bool{"example.com": true, "api.example.com": true, "notexample.com": false, "x.io": false} {
		if Matches(host, []string{"example.com"}) != want {
			t.Errorf("%s: want %v", host, want)
		}
	}
}

func TestGrease(t *testing.T) {
	for v, want := range map[uint16]bool{0x0a0a: true, 0xfafa: true, 0x1a1a: true, 0x1a2a: false, 0x1301: false, 0x0304: false} {
		if grease(v) != want {
			t.Errorf("%#x: %t", v, !want)
		}
	}
}
