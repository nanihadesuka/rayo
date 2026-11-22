package live

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// How another process of this computer uses a session.

// door is where a session is, and what proves that one may use it.
type door struct {
	Port  int    `json:"port"`
	Token string `json:"token"`
}

func doorFile(output string) string { return filepath.Join(output, "live.json") }

const tokenHeader = "X-Rayo-Token"

// listen takes requests of this computer that have the token, which only this user can read.
func listen(output string, requests chan<- Request) (func(), error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}
	secret := make([]byte, 16)
	if _, err := rand.Read(secret); err != nil {
		return nil, errors.Join(err, l.Close())
	}
	here := door{l.Addr().(*net.TCPAddr).Port, hex.EncodeToString(secret)}
	data, err := json.Marshal(here)
	if err == nil {
		err = os.MkdirAll(output, 0o755)
	}
	if err == nil {
		_ = os.Remove(doorFile(output)) // of a session that didn't end well; it may have other permissions
		err = os.WriteFile(doorFile(output), data, 0o600)
	}
	if err != nil {
		return nil, errors.Join(err, l.Close())
	}
	done := make(chan struct{})
	server := &http.Server{ReadHeaderTimeout: 10 * time.Second, Handler: http.HandlerFunc(func(w http.ResponseWriter, q *http.Request) {
		if q.Method != http.MethodPost || q.Header.Get(tokenHeader) != here.Token {
			http.Error(w, "not allowed", http.StatusForbidden)
			return
		}
		r := Request{answer: make(chan Reply, 1)}
		if err := json.NewDecoder(io.LimitReader(q.Body, 10<<20)).Decode(&r); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		select {
		case requests <- r:
		case <-done:
			http.Error(w, "the session is over", http.StatusGone)
			return
		}
		select {
		case a := <-r.answer:
			_ = json.NewEncoder(w).Encode(a) // whoever asked may have gone
		case <-done:
			http.Error(w, "the session is over", http.StatusGone)
		}
	})}
	go func() { _ = server.Serve(l) }()
	return func() {
		_ = os.Remove(doorFile(output))
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		time.AfterFunc(200*time.Millisecond, func() { close(done) }) // the last answer goes out first
		_ = server.Shutdown(ctx)
	}, nil
}

// Ask sends a request to the session that is live in an output folder.
func Ask(output string, q Request) (*Reply, error) {
	none := errors.New("no session is live here: start one with rayo live")
	data, err := os.ReadFile(doorFile(output))
	if err != nil {
		return nil, none
	}
	var there door
	if err := json.Unmarshal(data, &there); err != nil {
		return nil, none
	}
	body, err := json.Marshal(q)
	if err != nil {
		return nil, err
	}
	out, err := http.NewRequest(http.MethodPost, fmt.Sprintf("http://127.0.0.1:%d/", there.Port), bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	out.Header.Set(tokenHeader, there.Token)
	res, err := http.DefaultClient.Do(out)
	if err != nil {
		return nil, none
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		text, _ := io.ReadAll(io.LimitReader(res.Body, 1000))
		return nil, errors.New(strings.TrimSpace(string(text)))
	}
	a := &Reply{}
	return a, json.NewDecoder(res.Body).Decode(a)
}
