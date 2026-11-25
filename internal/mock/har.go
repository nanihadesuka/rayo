package mock

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"slices"
	"strings"
	"time"
)

// Importing: the calls of a HAR file, as mitmproxy, Charles or a browser write them, made into mocks as a
// capture is, private parts hidden the same way.

type harFile struct {
	Log struct {
		Entries []harEntry `json:"entries"`
	} `json:"log"`
}

type harEntry struct {
	StartedDateTime string `json:"startedDateTime"`
	Request         struct {
		Method   string    `json:"method"`
		URL      string    `json:"url"`
		Headers  []harPair `json:"headers"`
		PostData *struct {
			MimeType string    `json:"mimeType"`
			Text     string    `json:"text"`
			Encoding string    `json:"encoding"`
			Params   []harPair `json:"params"`
		} `json:"postData"`
	} `json:"request"`
	Response struct {
		Status  int       `json:"status"`
		Headers []harPair `json:"headers"`
		Content struct {
			MimeType string `json:"mimeType"`
			Text     string `json:"text"`
			Encoding string `json:"encoding"`
		} `json:"content"`
	} `json:"response"`
}

type harPair struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

func harHeaders(pairs []harPair) http.Header {
	h := http.Header{}
	for _, p := range pairs {
		if !strings.HasPrefix(p.Name, ":") { // HTTP/2's own
			h.Add(p.Name, p.Value)
		}
	}
	return h
}

// ImportHAR writes the calls of a HAR file in a new folder of mocks. With a base, only the calls to it
// are taken, without it in their paths; else every call to a host that isn't Google's, Firebase's,
// analytics or crash reports, with the host at the start of the path.
func ImportHAR(file, folder, base string, secrets map[string]string, s Settings) ([]string, error) {
	data, err := os.ReadFile(file)
	if err != nil {
		return nil, err
	}
	var har harFile
	if err := json.Unmarshal(data, &har); err != nil {
		return nil, fmt.Errorf("%s isn't a HAR file: %w", file, err)
	}
	var from *url.URL
	if base != "" {
		if from, err = url.Parse(strings.TrimSuffix(base, "/")); err != nil || from.Host == "" {
			return nil, fmt.Errorf("--capture-from: not an address: %q", base)
		}
	}
	// by time, those without one last, then as written: a total order, so the same file gives the same mocks
	entries := slices.Clone(har.Log.Entries)
	at := make(map[*harEntry]time.Time, len(entries))
	for i := range entries {
		if t, err := time.Parse(time.RFC3339Nano, entries[i].StartedDateTime); err == nil {
			at[&entries[i]] = t
		}
	}
	order := make([]int, len(entries))
	for i := range order {
		order[i] = i
	}
	slices.SortFunc(order, func(i, j int) int {
		x, okX := at[&entries[i]]
		y, okY := at[&entries[j]]
		switch {
		case okX && okY && !x.Equal(y):
			return x.Compare(y)
		case okX != okY:
			if okX {
				return -1
			}
			return 1
		}
		return i - j
	})
	sorted := make([]harEntry, len(entries))
	for k, i := range order {
		sorted[k] = entries[i]
	}
	entries = sorted
	r := &real{settings: s, secrets: secretValues(secrets)}
	var calls []captured
	left := map[string]int{}
	for _, e := range entries {
		q, res := e.Request, e.Response
		u, err := url.Parse(q.URL)
		switch {
		case err != nil || u.Scheme != "http" && u.Scheme != "https":
			left["not an http(s) address"]++
			continue
		case q.Method == http.MethodConnect || res.Status < 100:
			left["not answered"]++
			continue
		}
		at := *u
		if from != nil {
			if u.Scheme != from.Scheme || !strings.EqualFold(u.Host, from.Host) || !strings.HasPrefix(u.Path, from.Path) {
				left["to another backend"]++
				continue
			}
			at.Path = strings.TrimPrefix(u.Path, from.Path)
			at.RawPath = ""
			if !strings.HasPrefix(at.Path, "/") {
				at.Path = "/" + at.Path
			}
		} else {
			if Matches(u.Hostname(), Passthrough) {
				left["to Google, Firebase, analytics or crash reports"]++
				continue
			}
			at.Path = "/" + strings.ToLower(u.Hostname()) + u.Path
			if u.RawPath != "" {
				at.RawPath = "/" + strings.ToLower(u.Hostname()) + u.RawPath
			}
		}
		sent := http.Header(harHeaders(q.Headers))
		var body []byte
		if p := q.PostData; p != nil {
			if p.MimeType != "" && sent.Get("Content-Type") == "" {
				sent.Set("Content-Type", p.MimeType)
			}
			switch {
			case p.Encoding == "base64":
				if body, err = base64.StdEncoding.DecodeString(p.Text); err != nil {
					left["a request body not in base64"]++
					continue
				}
			case p.Text != "":
				body = []byte(p.Text)
			case len(p.Params) > 0:
				form := url.Values{}
				for _, kv := range p.Params {
					form.Add(kv.Name, kv.Value)
				}
				body = []byte(form.Encode())
			}
		}
		got := harHeaders(res.Headers)
		contentType := got.Get("Content-Type")
		if contentType == "" {
			contentType = res.Content.MimeType
		}
		answer := []byte(res.Content.Text)
		if res.Content.Encoding == "base64" {
			if answer, err = base64.StdEncoding.DecodeString(res.Content.Text); err != nil {
				left["an answer not in base64"]++
				continue
			}
		}
		c := captured{method: q.Method, uri: r.cleanURI(&at), status: res.Status, contentType: contentType, headers: r.cleanHeaders(got),
			request: r.cleanRequest(&http.Request{Method: q.Method, Header: sent, URL: &at}, []byte(decoded(sent, body)))}
		if binary(contentType, answer) {
			c.body, c.binary = base64.StdEncoding.EncodeToString(answer), true
		} else if c.body, err = r.cleanBody(answer); err != nil {
			left["an answer that can't be read: "+err.Error()]++
			continue
		}
		calls = append(calls, c)
	}
	var skipped []string
	for _, why := range sortedKeys(left) {
		skipped = append(skipped, fmt.Sprintf("%d calls %s", left[why], why))
	}
	if len(calls) == 0 {
		return nil, fmt.Errorf("%s has no calls to import: %s", file, strings.Join(skipped, "; "))
	}
	return saveCaptured(folder, calls, s, nil, skipped, func([]string) {})
}
