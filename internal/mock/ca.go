package mock

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// CA signs the certificates the proxy shows the app. It is a project's, kept with its flows so that
// every checkout signs alike, or else made once per computer. Only its certificate goes into the app,
// and only into its debug build.
type CA struct {
	Path string // of the certificate; the key is next to it
	cert *x509.Certificate
	key  *ecdsa.PrivateKey

	mu    sync.Mutex
	hosts map[string]*tls.Certificate
}

// CAFolder is where a computer's own CA is kept: ~/.rayo.
func CAFolder() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".rayo"), nil
}

// ProjectCA is the folder a project keeps its CA in, next to its suite or at its root.
const ProjectCA = ".rayo/ca"

// FindCA looks for a project's CA: .rayo/ca with a ca.pem, in from and the folders above it, up to
// the root of the repository (the folder with .git). It is empty if there is none.
func FindCA(from string) string {
	dir, err := filepath.Abs(from)
	if err != nil {
		return ""
	}
	for {
		folder := filepath.Join(dir, filepath.FromSlash(ProjectCA))
		if _, err := os.Stat(filepath.Join(folder, "ca.pem")); err == nil {
			return folder
		}
		if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
			return ""
		}
		up := filepath.Dir(dir)
		if up == dir {
			return ""
		}
		dir = up
	}
}

// LoadCA reads the CA in a folder, a project's, which must be there. An empty folder is the CA of
// this computer, in ~/.rayo, made the first time.
func LoadCA(folder string) (*CA, error) {
	if folder == "" {
		home, err := CAFolder()
		if err != nil {
			return nil, err
		}
		if _, err := os.Stat(filepath.Join(home, "ca.pem")); errors.Is(err, os.ErrNotExist) {
			if err := MakeCA(home); err != nil {
				return nil, fmt.Errorf("couldn't make the CA in %s: %w", home, err)
			}
		}
		folder = home
	}
	certFile, keyFile := filepath.Join(folder, "ca.pem"), filepath.Join(folder, "ca-key.pem")
	if _, err := os.Stat(certFile); errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("there is no CA in %s: it takes ca.pem and ca-key.pem; rayo ca new %s makes one", filepath.ToSlash(folder), filepath.ToSlash(folder))
	}
	pair, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return nil, fmt.Errorf("the CA in %s: %w", folder, err)
	}
	cert, err := x509.ParseCertificate(pair.Certificate[0])
	if err != nil {
		return nil, err
	}
	key, ok := pair.PrivateKey.(*ecdsa.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("the CA in %s: not an ECDSA key", folder)
	}
	return &CA{Path: certFile, cert: cert, key: key, hosts: map[string]*tls.Certificate{}}, nil
}

// MakeCA makes a new CA in a folder, as ca.pem and ca-key.pem; it won't replace one that is there.
func MakeCA(folder string) error {
	certFile, keyFile := filepath.Join(folder, "ca.pem"), filepath.Join(folder, "ca-key.pem")
	for _, f := range []string{certFile, keyFile} {
		if _, err := os.Stat(f); err == nil {
			return fmt.Errorf("%s is there already", filepath.ToSlash(f))
		}
	}
	return makeCA(folder, certFile, keyFile)
}

func makeCA(folder, certFile, keyFile string) error {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return err
	}
	host, _ := os.Hostname()
	template := &x509.Certificate{
		SerialNumber:          serial(),
		Subject:               pkix.Name{CommonName: "rayo CA " + host, Organization: []string{"rayo, for tests only"}},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().AddDate(10, 0, 0),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign | x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
		IsCA:                  true,
		MaxPathLenZero:        true,
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		return err
	}
	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(folder, 0o700); err != nil {
		return err
	}
	if err := os.WriteFile(keyFile, pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER}), 0o600); err != nil {
		return err
	}
	return os.WriteFile(certFile, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0o644)
}

func serial() *big.Int {
	n, _ := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 120))
	return n
}

// PEM is the certificate, to put in the app.
func (ca *CA) PEM() []byte {
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: ca.cert.Raw})
}

// For is a certificate for a host, signed by the CA.
func (ca *CA) For(host string) (*tls.Certificate, error) {
	ca.mu.Lock()
	defer ca.mu.Unlock()
	if c := ca.hosts[host]; c != nil {
		return c, nil
	}
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, err
	}
	template := &x509.Certificate{
		SerialNumber: serial(),
		Subject:      pkix.Name{CommonName: host},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().AddDate(1, 0, 0),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	if ip := net.ParseIP(host); ip != nil {
		template.IPAddresses = []net.IP{ip}
	} else {
		template.DNSNames = []string{host}
	}
	der, err := x509.CreateCertificate(rand.Reader, template, ca.cert, &key.PublicKey, ca.key)
	if err != nil {
		return nil, err
	}
	c := &tls.Certificate{Certificate: [][]byte{der, ca.cert.Raw}, PrivateKey: key}
	ca.hosts[host] = c
	return c, nil
}
