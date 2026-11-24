package mock

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestUnmockedIgnores(t *testing.T) {
	var u Unmocked
	for _, text := range []string{"graphql:Geocode", "api.shop.com/pings/**", "/favorites/*"} {
		i, err := ParseIgnored(text)
		if err != nil {
			t.Fatal(err)
		}
		u.Ignore = append(u.Ignore, i)
	}
	u.Ignore = append(u.Ignore, Ignored{Host: "guest.shop.com", Path: "/dismiss"})
	if err := u.Check(); err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		method, uri, body string
		want              bool
	}{
		{"POST", "/graphql", `{"operationName":"Geocode","query":"query Geocode { a }"}`, true},
		{"POST", "/graphql", `{"operationName":"Wallet","query":"query Wallet { a }"}`, false},
		{"GET", "/api.shop.com/pings/a/b", "", true},
		{"GET", "/eu.api.shop.com/pings/a", "", true},
		{"GET", "/other.com/pings/a", "", false},
		{"GET", "/favorites/1", "", true},
		{"GET", "/favorites/1/2", "", false},
		{"GET", "/guest.shop.com/dismiss", "", true},
		{"GET", "/dismiss", "", false},
	} {
		q := httptest.NewRequest(c.method, c.uri, strings.NewReader(c.body))
		if got := u.ignores(q, c.body); got != c.want {
			t.Errorf("%s %s: got %v", c.method, c.uri, got)
		}
	}
	for _, bad := range []string{"", "graphql:", "not a host/x"} {
		if _, err := ParseIgnored(bad); err == nil {
			t.Errorf("%q: no error", bad)
		}
	}
}
