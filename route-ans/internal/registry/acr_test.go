package registry

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/route-ans/route-ans/pkg/ansname"
)

func TestACRAdapter_LookupAndVerify(t *testing.T) {
	// Mock ACR Daemon
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/healthz":
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]string{"status": "healthy"})
		case "/api/v1/audit/chain":
			w.WriteHeader(http.StatusOK)
			entries := []map[string]interface{}{
				{
					"index":      0,
					"state_hash": "5c5ba708209fa7282f0944d8802d77e126ef3b206a2b1946af2beeea4e91abed",
					"prev_hash":  "0000000000000000000000000000000000000000000000000000000000000000",
					"timestamp":  time.Now().Format(time.RFC3339),
				},
			}
			json.NewEncoder(w).Encode(entries)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	opts := Options{
		Timeout:  2 * time.Second,
		Priority: 1,
	}
	cfg := map[string]interface{}{
		"daemonUrl": server.URL,
	}

	adapter, err := NewACRAdapter(opts, cfg)
	if err != nil {
		t.Fatalf("failed to create ACR adapter: %v", err)
	}

	ctx := context.Background()

	// 1. Check Healthy
	healthy, err := adapter.Healthy(ctx)
	if err != nil || !healthy {
		t.Fatalf("expected healthy true, got %v (err: %v)", healthy, err)
	}

	// 2. Lookup by ANSName
	name, err := ansname.Parse("acr://consensusLead.deliberation.PID-ACR.v1.0.0.acr.network")
	if err != nil {
		t.Fatalf("failed to parse ansname: %v", err)
	}

	rec, err := adapter.Lookup(ctx, name)
	if err != nil {
		t.Fatalf("failed to lookup record: %v", err)
	}

	if rec.ANSName != name.String() {
		t.Errorf("expected ANSName %s, got %s", name.String(), rec.ANSName)
	}
	if rec.Status != "active" {
		t.Errorf("expected status active, got %s", rec.Status)
	}
	if rec.Certificates.PublicCert.Fingerprint == "" {
		t.Errorf("expected non-empty cert fingerprint")
	}

	// 3. Verify Record
	vRes, err := adapter.VerifyRecord(ctx, rec)
	if err != nil {
		t.Fatalf("failed to verify record: %v", err)
	}
	if !vRes.Valid {
		t.Errorf("expected record to be valid, got invalid: %+v", vRes.Checks)
	}
}
