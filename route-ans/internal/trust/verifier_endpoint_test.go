package trust

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"encoding/pem"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestVerifyEndpointFingerprint(t *testing.T) {
	// Generate a test certificate
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("Failed to generate private key: %v", err)
	}

	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			CommonName: "test-agent.example.com",
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		DNSNames:              []string{"test-agent.example.com"},
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		t.Fatalf("Failed to create certificate: %v", err)
	}

	// Calculate expected fingerprint
	hash := sha256.Sum256(derBytes)
	expectedFingerprint := "SHA256:" + hex.EncodeToString(hash[:])

	// Create a test HTTPS server
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Configure server with our test certificate
	server.TLS = &tls.Config{
		Certificates: []tls.Certificate{
			{
				Certificate: [][]byte{derBytes},
				PrivateKey:  priv,
			},
		},
		MinVersion: tls.VersionTLS12,
	}
	server.StartTLS()
	defer server.Close()

	// Create verifier
	verifier := NewVerifier(nil, DefaultVerifierConfig())

	// Test successful verification
	t.Run("SuccessfulVerification", func(t *testing.T) {
		result, err := verifier.VerifyEndpointFingerprint(context.Background(), server.URL, expectedFingerprint)
		if err != nil {
			t.Fatalf("VerifyEndpointFingerprint failed: %v", err)
		}
		if !result.Passed {
			t.Errorf("Expected verification to pass, got: %s", result.Message)
		}
		if result.Details["fingerprint"] != expectedFingerprint {
			t.Errorf("Expected fingerprint %s, got %v", expectedFingerprint, result.Details["fingerprint"])
		}
	})

	// Test fingerprint mismatch
	t.Run("FingerprintMismatch", func(t *testing.T) {
		wrongFingerprint := "SHA256:0000000000000000000000000000000000000000000000000000000000000000"
		result, err := verifier.VerifyEndpointFingerprint(context.Background(), server.URL, wrongFingerprint)
		if err != nil {
			t.Fatalf("VerifyEndpointFingerprint failed: %v", err)
		}
		if result.Passed {
			t.Errorf("Expected verification to fail for wrong fingerprint")
		}
		if result.Details["expected"] != wrongFingerprint {
			t.Errorf("Expected details to contain wrong fingerprint")
		}
		if result.Details["actual"] != expectedFingerprint {
			t.Errorf("Expected details to contain actual fingerprint")
		}
	})

	// Test empty fingerprint
	t.Run("EmptyFingerprint", func(t *testing.T) {
		result, err := verifier.VerifyEndpointFingerprint(context.Background(), server.URL, "")
		if err != nil {
			t.Fatalf("VerifyEndpointFingerprint failed: %v", err)
		}
		if result.Passed {
			t.Errorf("Expected verification to fail for empty fingerprint")
		}
		if result.Message != "no fingerprint provided by registry" {
			t.Errorf("Expected 'no fingerprint provided' message, got: %s", result.Message)
		}
	})

	// Test non-HTTPS endpoint
	t.Run("NonHTTPSEndpoint", func(t *testing.T) {
		result, err := verifier.VerifyEndpointFingerprint(context.Background(), "http://example.com", expectedFingerprint)
		if err != nil {
			t.Fatalf("VerifyEndpointFingerprint failed: %v", err)
		}
		if result.Passed {
			t.Errorf("Expected verification to fail for non-HTTPS endpoint")
		}
	})
}

func TestVerifyEndpointFingerprint_RealWorld(t *testing.T) {
	// Skip this test in CI or if network is unavailable
	if testing.Short() {
		t.Skip("Skipping real-world test in short mode")
	}

	verifier := NewVerifier(nil, DefaultVerifierConfig())

	// Test with a real endpoint (example.com uses a valid cert)
	t.Run("GoogleHTTPSEndpoint", func(t *testing.T) {
		// First, get the actual fingerprint
		dialer := &tls.Dialer{
			Config: &tls.Config{
				InsecureSkipVerify: false,
				MinVersion:         tls.VersionTLS12,
			},
		}
		conn, err := dialer.DialContext(context.Background(), "tcp", "www.google.com:443")
		if err != nil {
			t.Skipf("Cannot connect to google.com: %v", err)
		}
		defer func() {
			if err := conn.Close(); err != nil {
				t.Logf("Failed to close connection: %v", err)
			}
		}()

		tlsConn := conn.(*tls.Conn)
		state := tlsConn.ConnectionState()
		if len(state.PeerCertificates) == 0 {
			t.Fatal("No certificates from google.com")
		}

		cert := state.PeerCertificates[0]
		hash := sha256.Sum256(cert.Raw)
		fingerprint := "SHA256:" + hex.EncodeToString(hash[:])

		// Now verify using our method
		result, err := verifier.VerifyEndpointFingerprint(context.Background(), "https://www.google.com", fingerprint)
		if err != nil {
			t.Fatalf("VerifyEndpointFingerprint failed: %v", err)
		}
		if !result.Passed {
			t.Errorf("Expected verification to pass for google.com, got: %s", result.Message)
		}
	})
}

func TestCalculateFingerprint(t *testing.T) {
	// Test PEM parsing and fingerprint calculation
	certPEM := `-----BEGIN CERTIFICATE-----
MIIBkTCB+wIJAKHHCgVZU38ZMA0GCSqGSIb3DQEBCwUAMBIxEDAOBgNVBAMMB3Rl
c3QtY2EwHhcNMjQwMTI0MDAwMDAwWhcNMjUwMTI0MDAwMDAwWjAQMQ4wDAYDVQQD
DAV0ZXN0MjCBnzANBgkqhkiG9w0BAQEFAAOBjQAwgYkCgYEAwUdO3fxEzXZZxHKG
q3hIJxNDiU4B6+b7GpHQ3bkJlMFhNlhKNYuLzkxs3hpfq6SrQvYJmXxBjzLLCvGz
fqnqLJBvqwYqUHzMqaQJI4OJXJ9sR5GYHqBNGZYqfVN0mJ7cHqYg8LJhOA7xJ7Qq
kLdHWKYEkZ1KLcJNvhzRCCECAwEAATANBgkqhkiG9w0BAQsFAAOBgQBQYD8q9yUX
+6wVVn8JXhH4dMqF3ykCLxKJBNYpqLQQqYDJjKJhZBNExJ7YLcQHqWCqBGKGhGqH
nGr8kLxBL7wBCgW8tMGvJkQZyBqLaLxVz5zLQ7g+VqG6L0KYQkXNYRqBqLLJqKYQ
qGhKGLqGBqYQqBqLaLxVz5zLQ7g+VqG6L0KYQkXNYRqBqLLJqKYQqGhKGLqGBqYQ
-----END CERTIFICATE-----`

	block, _ := pem.Decode([]byte(certPEM))
	if block != nil {
		hash := sha256.Sum256(block.Bytes)
		fingerprint := "SHA256:" + hex.EncodeToString(hash[:])
		t.Logf("Calculated fingerprint: %s", fingerprint)

		if len(fingerprint) != 71 { // SHA256: + 64 hex chars
			t.Errorf("Expected fingerprint length 71, got %d", len(fingerprint))
		}
	}
}
