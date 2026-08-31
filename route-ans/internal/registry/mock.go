// Package registry provides mock adapter for testing.
package registry

import (
	"context"
	"sync"
	"time"

	"github.com/route-ans/route-ans/pkg/ansname"
)

// mockAdapter is a mock registry adapter for testing
type mockAdapter struct {
	mu       sync.RWMutex
	records  map[string]*Record
	name     string
	priority int
	delay    time.Duration
}

// NewMockAdapter creates a new mock registry adapter
func NewMockAdapter(opts Options, config map[string]interface{}) (Adapter, error) {
	delay := 10 * time.Millisecond
	if d, ok := config["mockDelay"].(string); ok {
		if parsed, err := time.ParseDuration(d); err == nil {
			delay = parsed
		}
	}

	return &mockAdapter{
		records:  make(map[string]*Record),
		name:     "mock",
		priority: opts.Priority,
		delay:    delay,
	}, nil
}

// AddRecord adds a record to the mock registry (for testing)
func (m *mockAdapter) AddRecord(record *Record) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.records[record.ANSName] = record
}

// Lookup queries the mock registry
func (m *mockAdapter) Lookup(ctx context.Context, name *ansname.ANSName) (*Record, error) {
	// Simulate network delay
	if m.delay > 0 {
		select {
		case <-time.After(m.delay):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	record, exists := m.records[name.String()]
	if !exists {
		return nil, &ErrNotFound{ANSName: name.String()}
	}

	return record, nil
}

// LookupByFQDN queries for all versions
func (m *mockAdapter) LookupByFQDN(ctx context.Context, fqdn string) ([]*Record, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var results []*Record
	for _, record := range m.records {
		if record.FQDN == fqdn {
			results = append(results, record)
		}
	}

	return results, nil
}

// GetMerkleProof returns a mock Merkle proof
func (m *mockAdapter) GetMerkleProof(ctx context.Context, ansName string) (*MerkleProof, error) {
	return &MerkleProof{
		LeafIndex: 12345,
		TreeSize:  100000,
		Hashes:    [][]byte{{0x01, 0x02}, {0x03, 0x04}},
		RootHash:  []byte{0xab, 0xcd, 0xef},
		Timestamp: time.Now(),
	}, nil
}

// GetSignedTreeHead returns a mock signed tree head
func (m *mockAdapter) GetSignedTreeHead(ctx context.Context) (*SignedTreeHead, error) {
	return &SignedTreeHead{
		TreeSize:  100000,
		RootHash:  []byte{0xab, 0xcd, 0xef},
		Timestamp: time.Now(),
		Signature: []byte{0x01, 0x02, 0x03},
		KeyID:     "mock-key-1",
	}, nil
}

// VerifyRecord performs mock verification
func (m *mockAdapter) VerifyRecord(ctx context.Context, record *Record) (*VerificationResult, error) {
	return &VerificationResult{
		Valid: true,
		Checks: map[string]CheckResult{
			"signature":    {Passed: true, Message: "mock signature valid"},
			"merkle_proof": {Passed: true, Message: "mock proof valid"},
			"certificate":  {Passed: true, Message: "mock cert valid"},
		},
		Timestamp: time.Now(),
	}, nil
}

// Subscribe is not supported for mock adapter
func (m *mockAdapter) Subscribe(ctx context.Context, handler EventHandler) error {
	// Mock adapter doesn't support event streaming
	<-ctx.Done()
	return ctx.Err()
}

// Name returns the adapter name
func (m *mockAdapter) Name() string {
	return m.name
}

// Priority returns the adapter priority
func (m *mockAdapter) Priority() int {
	return m.priority
}

// Healthy always returns true for mock
func (m *mockAdapter) Healthy(ctx context.Context) (bool, error) {
	return true, nil
}

// Close releases resources
func (m *mockAdapter) Close() error {
	return nil
}

// CreateMockRecord creates a sample record for testing
func CreateMockRecord(protocol, agentName, capability, providerID, version, extension string) *Record {
	ansName := protocol + "://" + agentName + "." + capability + "." + providerID + "." + version + "." + extension
	fqdn := agentName + "." + extension

	return &Record{
		ANSName:  ansName,
		FQDN:     fqdn,
		Protocol: protocol,
		Version:  version,
		Status:   "active",
		Endpoint: "https://" + fqdn + ":8443",
		Certificates: CertificateInfo{
			PublicCert: CertDetails{
				Fingerprint: "sha256:abcd1234",
				IssuedAt:    time.Now().Add(-24 * time.Hour),
				ExpiresAt:   time.Now().Add(90 * 24 * time.Hour),
				Issuer:      "Let's Encrypt",
			},
			PrivateCert: CertDetails{
				Fingerprint: "sha256:efgh5678",
				IssuedAt:    time.Now().Add(-24 * time.Hour),
				ExpiresAt:   time.Now().Add(365 * 24 * time.Hour),
				Issuer:      "ANS Private CA",
			},
		},
		Metadata: AgentMetadata{
			DisplayName:  agentName + " Agent",
			Description:  "A " + capability + " agent for testing",
			Capabilities: []string{capability, "testing"},
			Protocols:    []string{protocol},
		},
		RegistrarID:  "ra-mock",
		TTL:          5 * time.Minute,
		RegisteredAt: time.Now().Add(-7 * 24 * time.Hour),
		UpdatedAt:    time.Now().Add(-1 * time.Hour),
		ExpiresAt:    time.Now().Add(90 * 24 * time.Hour),
	}
}

func init() {
	Register("mock", func(opts Options, config map[string]interface{}) (Adapter, error) {
		return NewMockAdapter(opts, config)
	})
}
