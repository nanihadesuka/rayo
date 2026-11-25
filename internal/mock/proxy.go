package mock

import (
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

// Proxy is the device's HTTP proxy. Calls to the hosts it opens reach the mock as /host/path, as an
// interceptor in the app would send them, so mocks and captures are the same either way. Calls to other
// hosts pass untouched. Calls sent to it as to the mock itself, by an app that still rewrites its URLs,
// reach the mock too.
type Proxy struct {
	mock *Mock
	ca   *CA
	open func(host string) bool
	// Asked says whether a host was named, as with --hosts: then a browser's calls to it are decrypted too.
	Asked  func(host string) bool
	server *http.Server

	mu        sync.Mutex
	passed    map[string]bool   // hosts tunnelled untouched
	refused   map[string]string // hosts whose TLS the app refused: their calls are refused, never sent on
	plainSeen map[string]bool   // hosts called over plain http, passed untouched
	browsers  map[string]bool   // hosts a browser called, such as a Custom Tab: tunnelled, as it trusts only the system CAs
	conns     map[net.Conn]bool // hijacked, which the server doesn't close
}

// CA is the file of the certificate it signs with.
func (p *Proxy) CA() string { return p.ca.Path }

// Passthrough are hosts never opened unless asked for: Google Play services, Firebase, analytics and crash
// reports. A suffix matches the host and its subdomains.
var Passthrough = []string{
	"google.com", "googleapis.com", "gstatic.com", "googleusercontent.com", "gvt1.com", "gvt2.com", "ggpht.com",
	"googlesyndication.com", "googleadservices.com", "doubleclick.net", "google-analytics.com", "googletagmanager.com",
	"app-measurement.com", "firebaseio.com", "firebaseapp.com", "crashlytics.com", "android.com",
	"sentry.io", "bugsnag.com", "instabug.com", "datadoghq.com", "datadoghq.eu", "newrelic.com", "nr-data.net",
	"appsflyer.com", "appsflyersdk.com", "adjust.com", "branch.io", "app.link", "amplitude.com", "mixpanel.com",
	"segment.io", "segment.com", "braze.com", "onesignal.com", "facebook.com", "facebook.net",
}

// Matches says whether a host is one of the suffixes, or under one.
func Matches(host string, suffixes []string) bool {
	host = strings.ToLower(strings.TrimSuffix(host, "."))
	for _, s := range suffixes {
		s = strings.ToLower(strings.TrimPrefix(strings.TrimSpace(s), "."))
		if s != "" && (host == s || strings.HasSuffix(host, "."+s)) {
			return true
		}
	}
	return false
}

// NewProxy puts a proxy in front of a mock, opening the hosts open says.
func NewProxy(m *Mock, ca *CA, open func(host string) bool) *Proxy {
	if m.real != nil { // the app follows a redirect, through the proxy again
		m.real.client.CheckRedirect = noFollow
	}
	return &Proxy{mock: m, ca: ca, open: open, passed: map[string]bool{}, refused: map[string]string{}, plainSeen: map[string]bool{}, browsers: map[string]bool{}, conns: map[net.Conn]bool{}}
}

// Start serves on a free port of this computer.
func (p *Proxy) Start() (port int, err error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	p.server = &http.Server{Handler: p, ErrorLog: log.New(io.Discard, "", 0)}
	go func() {
		if err := p.server.Serve(listener); !errors.Is(err, http.ErrServerClosed) {
			p.mock.fail(fmt.Errorf("the proxy stopped: %w", err))
		}
	}()
	return listener.Addr().(*net.TCPAddr).Port, nil
}

// Close also cuts the tunnels.
func (p *Proxy) Close() error {
	if p == nil || p.server == nil {
		return nil
	}
	err := p.server.Close()
	p.mu.Lock()
	defer p.mu.Unlock()
	for c := range p.conns {
		c.Close()
	}
	return err
}

// Notes say which hosts weren't opened, and why.
func (p *Proxy) Notes() []string {
	p.mu.Lock()
	defer p.mu.Unlock()
	var notes []string
	if list := sorted(p.passed); list != "" {
		notes = append(notes, "proxy, passed untouched: "+list)
	}
	if list := sorted(p.browsers); list != "" {
		notes = append(notes, "proxy, a browser's calls passed untouched, as it trusts only the system's CAs: "+list)
	}
	if list := sorted(p.plainSeen); list != "" {
		notes = append(notes, "proxy, plain http passed untouched: "+list)
	}
	return notes
}

// keep closes a connection with the proxy, and returns what forgets it.
func (p *Proxy) keep(c net.Conn) func() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.conns[c] = true
	return func() {
		p.mu.Lock()
		defer p.mu.Unlock()
		delete(p.conns, c)
	}
}

func sorted(set map[string]bool) string {
	var list []string
	for k := range set {
		list = append(list, k)
	}
	sort.Strings(list)
	return strings.Join(list, ", ")
}

func (p *Proxy) ServeHTTP(w http.ResponseWriter, q *http.Request) {
	switch {
	case q.Method == http.MethodConnect:
		p.connect(w, q)
	case !q.URL.IsAbs() || p.itself(q.URL.Host):
		// Sent to the mock as it is, by an app that rewrites its URLs.
		q.URL.Scheme, q.URL.Host = "", ""
		p.mock.ServeHTTP(w, q)
	default:
		p.mu.Lock()
		p.plainSeen[q.URL.Hostname()] = true
		p.mu.Unlock()
		passPlain(w, q)
	}
}

// itself says whether an address is the device's own, where the app would find the mock.
func (p *Proxy) itself(hostPort string) bool {
	host := hostPort
	if h, _, err := net.SplitHostPort(hostPort); err == nil {
		host = h
	}
	return host == "localhost" || net.ParseIP(host).IsLoopback()
}

func (p *Proxy) connect(w http.ResponseWriter, q *http.Request) {
	host, port, err := net.SplitHostPort(q.Host)
	if err != nil {
		host, port = q.Host, "443"
	}
	p.mu.Lock()
	_, refused := p.refused[host]
	open := !refused && port == "443" && hostName.MatchString(host) && p.open(host)
	if !open && !refused {
		p.passed[host] = true
	}
	p.mu.Unlock()
	if refused { // to the real backend, it would be neither mocked nor captured
		http.Error(w, "rayo: the app doesn't trust rayo's CA for "+host+", so its calls are refused", http.StatusBadGateway)
		return
	}
	hj, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "can't tunnel", http.StatusInternalServerError)
		return
	}
	if !open {
		server, err := net.DialTimeout("tcp", net.JoinHostPort(host, port), 15*time.Second)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		client, _, err := hj.Hijack()
		if err != nil {
			server.Close()
			return
		}
		defer p.keep(client)()
		defer p.keep(server)()
		_, _ = io.WriteString(client, "HTTP/1.1 200 Connection Established\r\n\r\n")
		tunnel(client, server)
		return
	}
	client, _, err := hj.Hijack()
	if err != nil {
		return
	}
	defer p.keep(client)()
	_, _ = io.WriteString(client, "HTTP/1.1 200 Connection Established\r\n\r\n")
	// A browser, such as a Custom Tab with a login page, trusts only the system's CAs: it would show
	// "not private" instead of the page. Its hello is read first, and then passed on untouched.
	hello, browser := peekHello(client)
	if browser && !p.asked(host) {
		p.mu.Lock()
		p.browsers[host] = true
		p.mu.Unlock()
		server, err := net.DialTimeout("tcp", net.JoinHostPort(host, port), 15*time.Second)
		if err != nil {
			return
		}
		defer p.keep(server)()
		if _, err := server.Write(hello); err != nil {
			server.Close()
			return
		}
		tunnel(client, server)
		return
	}
	client = &replayed{Conn: client, first: hello}
	conn := tls.Server(client, &tls.Config{
		NextProtos: []string{"http/1.1"},
		GetCertificate: func(hello *tls.ClientHelloInfo) (*tls.Certificate, error) {
			return p.ca.For(host)
		},
	})
	_ = conn.SetDeadline(time.Now().Add(15 * time.Second))
	if err := conn.Handshake(); err != nil {
		p.mu.Lock()
		_, again := p.refused[host]
		p.refused[host] = handshakeReason(err)
		p.mu.Unlock()
		if !again {
			p.mock.fail(fmt.Errorf("pinned or untrusted CA: %s rejected rayo's CA (%s). Its calls would reach the real backend, "+
				"neither mocked nor captured, so they are refused. A debug build trusts it with the network_security_config of "+
				"rayo ca export, see rayo help connect-mocks; a host left out of --hosts passes untouched", host, handshakeReason(err)))
		}
		conn.Close()
		return
	}
	_ = conn.SetDeadline(time.Time{})
	served := &http.Server{ErrorLog: log.New(io.Discard, "", 0), Handler: http.HandlerFunc(func(w http.ResponseWriter, q *http.Request) {
		q.URL.Scheme, q.URL.Host = "", ""
		q.URL.Path = "/" + host + q.URL.Path
		if q.URL.RawPath != "" {
			q.URL.RawPath = "/" + host + q.URL.RawPath
		}
		q.RequestURI = q.URL.RequestURI()
		p.mock.ServeHTTP(w, q)
	})}
	_ = served.Serve(&oneConn{conn: conn, done: make(chan struct{})})
}

// peekHello reads a TLS client hello without answering it: what was read, and whether a browser sent it.
// Chrome, and the browsers and WebViews made from it, send GREASE values; Android's own TLS, which
// OkHttp uses, doesn't.
func peekHello(c net.Conn) (read []byte, browser bool) {
	rec := &recorder{Conn: c}
	_ = c.SetReadDeadline(time.Now().Add(15 * time.Second))
	_ = tls.Server(rec, &tls.Config{GetConfigForClient: func(h *tls.ClientHelloInfo) (*tls.Config, error) {
		for _, v := range append(append([]uint16{}, h.CipherSuites...), h.SupportedVersions...) {
			if grease(v) {
				browser = true
			}
		}
		return nil, errPeeked
	}}).Handshake()
	_ = c.SetReadDeadline(time.Time{})
	return rec.read, browser
}

func (p *Proxy) asked(host string) bool { return p.Asked != nil && p.Asked(host) }

// grease is a value of RFC 8701, which a client sends to keep servers from choking on new ones.
func grease(v uint16) bool { return v&0x0f0f == 0x0a0a && v>>8 == v&0xff }

var errPeeked = errors.New("peeked")

// recorder keeps what is read, and sends nothing: the alert of a hello cut short doesn't reach the app.
type recorder struct {
	net.Conn
	read []byte
}

func (r *recorder) Read(b []byte) (int, error) {
	n, err := r.Conn.Read(b)
	r.read = append(r.read, b[:n]...)
	return n, err
}

func (r *recorder) Write(b []byte) (int, error) { return len(b), nil }

// replayed reads again what was read already, before the rest.
type replayed struct {
	net.Conn
	first []byte
}

func (r *replayed) Read(b []byte) (int, error) {
	if len(r.first) > 0 {
		n := copy(b, r.first)
		r.first = r.first[n:]
		return n, nil
	}
	return r.Conn.Read(b)
}

var alert = regexp.MustCompile(`remote error: tls: (.*)`)

func handshakeReason(err error) string {
	if m := alert.FindStringSubmatch(err.Error()); m != nil {
		return "the app said " + m[1]
	}
	return "the app hung up"
}

func tunnel(a, b net.Conn) {
	done := make(chan struct{}, 2)
	pipe := func(to, from net.Conn) {
		_, _ = io.Copy(to, from)
		if c, ok := to.(interface{ CloseWrite() error }); ok {
			_ = c.CloseWrite()
		}
		done <- struct{}{}
	}
	go pipe(a, b)
	go pipe(b, a)
	<-done
	<-done
	a.Close()
	b.Close()
}

var plainClient = &http.Client{
	Timeout:       time.Minute,
	Transport:     &http.Transport{Proxy: nil},
	CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
}

func passPlain(w http.ResponseWriter, q *http.Request) {
	out, err := http.NewRequestWithContext(q.Context(), q.Method, q.URL.String(), q.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	out.Header = q.Header.Clone()
	out.Header.Del("Proxy-Connection")
	res, err := plainClient.Do(out)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	defer res.Body.Close()
	for k, v := range res.Header {
		w.Header()[k] = v
	}
	w.WriteHeader(res.StatusCode)
	_, _ = io.Copy(w, res.Body)
}

// oneConn is a listener of one connection, already open.
type oneConn struct {
	conn net.Conn
	once sync.Once
	used bool
	done chan struct{}
}

func (l *oneConn) Accept() (net.Conn, error) {
	if !l.used {
		l.used = true
		return &closing{Conn: l.conn, closed: func() { l.once.Do(func() { close(l.done) }) }}, nil
	}
	<-l.done
	return nil, net.ErrClosed
}
func (l *oneConn) Close() error   { l.once.Do(func() { close(l.done) }); return nil }
func (l *oneConn) Addr() net.Addr { return l.conn.LocalAddr() }

// closing tells the listener when the connection ends, so that Serve returns.
type closing struct {
	net.Conn
	closed func()
}

func (c *closing) Close() error {
	err := c.Conn.Close()
	c.closed()
	return err
}
