package client

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"errors"
	"math/big"
	"net"
	"strings"
	"testing"
	"time"
)

// TestDiagnoseDialError covers the PEN-32 connection-diagnosis logic: an opaque
// dial failure must be turned into an actionable transport-level message that
// distinguishes the misconfigurations that otherwise all look identical.
func TestDiagnoseDialError(t *testing.T) {
	opaque := errors.New("context deadline exceeded")

	t.Run("nothing listening -> TCP connect failed", func(t *testing.T) {
		// Reserve a port then close it so the address is almost certainly dead.
		l, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		addr := l.Addr().String()
		_ = l.Close()

		c := NewGRPCClient(addr, &ClientOptions{Insecure: true, ConnectTimeout: time.Second})
		got := c.diagnoseDialError(opaque).Error()
		if !strings.Contains(got, "TCP connect failed") {
			t.Fatalf("expected TCP-connect-failed diagnosis, got: %s", got)
		}
	})

	t.Run("plaintext client against TLS server -> server requires TLS", func(t *testing.T) {
		addr := startTLSListener(t)
		c := NewGRPCClient(addr, &ClientOptions{Insecure: false, TLSConfig: nil, ConnectTimeout: time.Second})
		got := c.diagnoseDialError(opaque).Error()
		if !strings.Contains(got, "server requires TLS") {
			t.Fatalf("expected server-requires-TLS diagnosis, got: %s", got)
		}
	})

	t.Run("TLS client against plaintext server -> server is plaintext", func(t *testing.T) {
		addr := startPlaintextListener(t)
		c := NewGRPCClient(addr, &ClientOptions{Insecure: false, TLSConfig: &tls.Config{}, ConnectTimeout: time.Second})
		got := c.diagnoseDialError(opaque).Error()
		if !strings.Contains(got, "server is plaintext") {
			t.Fatalf("expected server-is-plaintext diagnosis, got: %s", got)
		}
	})
}

func TestServerSpeaksTLS(t *testing.T) {
	if got := serverSpeaksTLS(startTLSListener(t)); !got {
		t.Error("expected serverSpeaksTLS=true for a TLS listener")
	}
	if got := serverSpeaksTLS(startPlaintextListener(t)); got {
		t.Error("expected serverSpeaksTLS=false for a plaintext listener")
	}
}

func TestTLSHint(t *testing.T) {
	cases := []struct {
		errText string
		want    string
	}{
		{"x509: certificate is valid for dev02.brown.chat, dev02, not dev02.home.lan", "Hostname mismatch"},
		{"x509: certificate signed by unknown authority", "wrong or stale"},
		{"x509: certificate has expired or is not yet valid", "expired or not yet valid"},
		{"some other tls error", "Check `ca_cert`"},
	}
	for _, tc := range cases {
		if got := tlsHint(errors.New(tc.errText)); !strings.Contains(got, tc.want) {
			t.Errorf("tlsHint(%q) = %q, want substring %q", tc.errText, got, tc.want)
		}
	}
}

func TestHostFromAddr(t *testing.T) {
	cases := map[string]string{
		"dev02.brown.chat:50051": "dev02.brown.chat",
		"10.0.10.52:50051":       "10.0.10.52",
		"dev02":                  "dev02", // no port
	}
	for in, want := range cases {
		if got := hostFromAddr(in); got != want {
			t.Errorf("hostFromAddr(%q) = %q, want %q", in, got, want)
		}
	}
}

// startTLSListener spins up a TLS listener with a throwaway self-signed cert and
// returns its address. It accepts and immediately closes connections.
func startTLSListener(t *testing.T) string {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "test"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		DNSNames:     []string{"localhost"},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	cert := tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key}
	l, err := tls.Listen("tcp", "127.0.0.1:0", &tls.Config{Certificates: []tls.Certificate{cert}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = l.Close() })
	go acceptAndClose(l)
	return l.Addr().String()
}

// startPlaintextListener spins up a bare TCP listener and returns its address.
func startPlaintextListener(t *testing.T) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = l.Close() })
	go acceptAndClose(l)
	return l.Addr().String()
}

func acceptAndClose(l net.Listener) {
	for {
		conn, err := l.Accept()
		if err != nil {
			return
		}
		// For the TLS listener, complete the handshake so probes see a real TLS peer.
		if tc, ok := conn.(*tls.Conn); ok {
			_ = tc.Handshake()
		}
		_ = conn.Close()
	}
}
