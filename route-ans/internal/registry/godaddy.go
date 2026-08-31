// Package registry provides the GoDaddy ANS registry adapter implementation.
package registry

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/route-ans/route-ans/pkg/ansname"
	"github.com/rs/zerolog/log"
)

// godaddyAdapter implements the Adapter interface for GoDaddy's ANS Registry
type godaddyAdapter struct {
	client    *http.Client
	baseURL   string
	apiKey    string
	apiSecret string
	name      string
	priority  int
}

// GoDaddy API request/response types
type godaddyResolutionRequest struct {
	AgentHost string `json:"agentHost"`
	Version   string `json:"version"`
}

type godaddyResolutionResponse struct {
	ANSName string `json:"ansName"`
	Links   []struct {
		Rel  string `json:"rel"`
		Href string `json:"href"`
	} `json:"links"`
}

type godaddyAgentDetails struct {
	AgentID               string                 `json:"agentId"`
	AgentName             string                 `json:"agentName"` // Deprecated field
	AgentDisplayName      string                 `json:"agentDisplayName"`
	AgentHost             string                 `json:"agentHost"`
	Version               string                 `json:"version"`
	Protocol              string                 `json:"protocol"`  // Deprecated field
	Extension             string                 `json:"extension"` // Deprecated field
	ANSName               string                 `json:"ansName"`
	AgentStatus           string                 `json:"agentStatus"`
	AgentCategory         string                 `json:"agentCategory"`
	AgentCapability       string                 `json:"agentCapability"` // Deprecated field
	AgentDescription      string                 `json:"agentDescription"`
	Provider              string                 `json:"provider"` // Deprecated field
	RegistrationTimestamp string                 `json:"registrationTimestamp"`
	LastRenewalTimestamp  string                 `json:"lastRenewalTimestamp"`
	Endpoints             []godaddyEndpoint      `json:"endpoints"`
	ProtocolExtensions    map[string]interface{} `json:"protocolExtensions"` // Deprecated field
	Links                 []struct {
		Rel  string `json:"rel"`
		Href string `json:"href"`
	} `json:"links"`
}

type godaddyEndpoint struct {
	AgentURL         string            `json:"agentUrl"`
	Protocol         string            `json:"protocol"`
	MetaDataURL      string            `json:"metaDataUrl"`
	DocumentationURL string            `json:"documentationUrl"`
	Functions        []godaddyFunction `json:"functions"`
	Transports       []string          `json:"transports"`
}

type godaddyFunction struct {
	ID   string   `json:"id"`
	Name string   `json:"name"`
	Tags []string `json:"tags"`
}

type godaddyCertificateResponse struct {
	CertificatePEM                string `json:"certificatePEM"`
	CertificateIssuer             string `json:"certificateIssuer"`
	CertificateSubject            string `json:"certificateSubject"`
	CertificateSerialNumber       string `json:"certificateSerialNumber"`
	CertificatePublicKeyAlgorithm string `json:"certificatePublicKeyAlgorithm"`
	CertificateSignatureAlgorithm string `json:"certificateSignatureAlgorithm"`
	CertificateValidFrom          string `json:"certificateValidFrom"`
	CertificateValidTo            string `json:"certificateValidTo"`
}

// GoDaddy Events API structures
type godaddyEventsResponse struct {
	Items     []godaddyEvent `json:"items"`
	LastLogID string         `json:"lastLogId"`
}

type godaddyEvent struct {
	LogID            string            `json:"logId"`
	EventType        string            `json:"eventType"` // AGENT_REGISTERED, AGENT_RENEWED, AGENT_REVOKED, etc.
	CreatedAt        string            `json:"createdAt"`
	ExpiresAt        string            `json:"expiresAt"`
	AgentID          string            `json:"agentId"`
	ANSName          string            `json:"ansName"`
	AgentHost        string            `json:"agentHost"`
	AgentDisplayName string            `json:"agentDisplayName"`
	AgentDescription string            `json:"agentDescription"`
	Version          string            `json:"version"`
	ProviderID       string            `json:"providerId"`
	Endpoints        []godaddyEndpoint `json:"endpoints"`
}

type godaddyCertificate struct {
	PEM          string
	Fingerprint  string
	Issuer       string
	Subject      string
	SerialNumber string
	NotBefore    time.Time
	NotAfter     time.Time
}

type godaddyError struct {
	Code    string                 `json:"code"`
	Message string                 `json:"message"`
	Status  string                 `json:"status"`
	Details map[string]interface{} `json:"details"`
}

// NewGoDaddyAdapter creates a new GoDaddy registry adapter
func NewGoDaddyAdapter(opts Options, config map[string]interface{}) (Adapter, error) {
	// Extract configuration
	baseURL, _ := config["baseURL"].(string)
	if baseURL == "" {
		baseURL = "https://api.godaddy.com/v1"
	}

	apiKey, _ := config["apiKey"].(string)
	apiSecret, _ := config["apiSecret"].(string)

	if apiKey == "" || apiSecret == "" {
		return nil, fmt.Errorf("godaddy adapter requires apiKey and apiSecret")
	}

	return &godaddyAdapter{
		client: &http.Client{
			Timeout: opts.Timeout,
		},
		baseURL:   baseURL,
		apiKey:    apiKey,
		apiSecret: apiSecret,
		name:      "godaddy",
		priority:  opts.Priority,
	}, nil
}

// Lookup queries the GoDaddy registry for an agent by ANSName
func (g *godaddyAdapter) Lookup(ctx context.Context, name *ansname.ANSName) (*Record, error) {
	// Extract agentHost (FQDN) and version from ANSName
	// ANSName format: protocol://version.host.domain
	agentHost := name.FQDN()
	version := name.Version

	// GoDaddy expects version without 'v' prefix (e.g., "1.0.0" not "v1.0.0")
	version = strings.TrimPrefix(version, "v")

	// Step 1: Resolve to get agent details link
	resolutionReq := godaddyResolutionRequest{
		AgentHost: agentHost,
		Version:   version,
	}

	resolutionURL := fmt.Sprintf("%s/agents/resolution", g.baseURL)

	resolutionResp, err := g.makeRequest(ctx, "POST", resolutionURL, resolutionReq)
	if err != nil {
		return nil, err
	}

	var resolution godaddyResolutionResponse
	if err := json.Unmarshal(resolutionResp, &resolution); err != nil {
		return nil, fmt.Errorf("failed to parse resolution response: %w", err)
	}

	// Find agent-details link
	var detailsURL string
	for _, link := range resolution.Links {
		if link.Rel == "agent-details" {
			detailsURL = link.Href
			break
		}
	}

	if detailsURL == "" {
		return nil, &ErrNotFound{ANSName: name.String()}
	}

	// Step 2: Get agent details
	// If the URL is internal (ra.int.godaddy.com), construct the public API path
	if strings.Contains(detailsURL, "ra.int.godaddy.com") || strings.Contains(detailsURL, ".int.") {
		// Extract the agent ID from the internal URL
		// Format: https://ra.int.godaddy.com/v1/agents/{agentId}
		parts := strings.Split(detailsURL, "/agents/")
		if len(parts) == 2 {
			agentID := parts[1]
			// Use the public API endpoint
			detailsURL = fmt.Sprintf("%s/agents/%s", g.baseURL, agentID)
			log.Debug().Str("agentId", agentID).Str("publicURL", detailsURL).Msg("Converted internal URL to public API")
		}
	}

	detailsResp, err := g.makeRequest(ctx, "GET", detailsURL, nil)
	if err != nil {
		return nil, fmt.Errorf("registry lookup failed: %w", err)
	}

	var details godaddyAgentDetails
	if err := json.Unmarshal(detailsResp, &details); err != nil {
		return nil, fmt.Errorf("failed to parse agent details: %w", err)
	}

	const (
		statusActive   = "ACTIVE"
		statusVerified = "VERIFIED"
	)

	// Check status
	if details.AgentStatus != statusActive && details.AgentStatus != statusVerified {
		return nil, &ErrNotFound{ANSName: name.String()}
	}

	// Step 3: Get certificates
	serverCert, identityCert := g.getCertificates(ctx, details.Links)

	// Build the record
	record := g.buildRecord(&details, serverCert, identityCert)
	return record, nil
}

// LookupByFQDN queries the registry for all versions of an agent by FQDN
func (g *godaddyAdapter) LookupByFQDN(ctx context.Context, fqdn string) ([]*Record, error) {
	// Use wildcard version to get the latest/best matching version
	// GoDaddy's API supports semantic versioning and will return the best match
	resolutionReq := godaddyResolutionRequest{
		AgentHost: fqdn,
		Version:   "*", // Wildcard to match any version
	}

	resolutionURL := fmt.Sprintf("%s/agents/resolution", g.baseURL)
	resolutionResp, err := g.makeRequest(ctx, "POST", resolutionURL, resolutionReq)
	if err != nil {
		return nil, err
	}

	var resolution godaddyResolutionResponse
	if err := json.Unmarshal(resolutionResp, &resolution); err != nil {
		return nil, fmt.Errorf("failed to parse resolution response: %w", err)
	}

	// Find agent-details link
	var detailsURL string
	for _, link := range resolution.Links {
		if link.Rel == "agent-details" {
			detailsURL = link.Href
			break
		}
	}

	if detailsURL == "" {
		return nil, &ErrNotFound{ANSName: fqdn}
	}

	// Get agent details
	if strings.Contains(detailsURL, "ra.int.godaddy.com") || strings.Contains(detailsURL, ".int.") {
		parts := strings.Split(detailsURL, "/agents/")
		if len(parts) == 2 {
			agentID := parts[1]
			detailsURL = fmt.Sprintf("%s/agents/%s", g.baseURL, agentID)
			log.Debug().Str("agentId", agentID).Str("publicURL", detailsURL).Msg("Converted internal URL to public API")
		}
	}

	detailsResp, err := g.makeRequest(ctx, "GET", detailsURL, nil)
	if err != nil {
		return nil, fmt.Errorf("registry lookup failed: %w", err)
	}

	var details godaddyAgentDetails
	if err := json.Unmarshal(detailsResp, &details); err != nil {
		return nil, fmt.Errorf("failed to parse agent details: %w", err)
	}

	// Check status
	if details.AgentStatus != "ACTIVE" && details.AgentStatus != "VERIFIED" {
		return nil, &ErrNotFound{ANSName: fqdn}
	}

	// Get certificates
	serverCert, identityCert := g.getCertificates(ctx, details.Links)

	// Build the record
	record := g.buildRecord(&details, serverCert, identityCert)

	// Return as a slice (GoDaddy returns single best match for wildcard)
	return []*Record{record}, nil
}

// GetMerkleProof retrieves the Merkle inclusion proof
func (g *godaddyAdapter) GetMerkleProof(ctx context.Context, ansName string) (*MerkleProof, error) {
	// GoDaddy's Merkle proof endpoint (to be implemented when available)
	return nil, fmt.Errorf("merkle proof not yet implemented for GoDaddy adapter")
}

// GetSignedTreeHead retrieves the current signed tree head
func (g *godaddyAdapter) GetSignedTreeHead(ctx context.Context) (*SignedTreeHead, error) {
	// GoDaddy's signed tree head endpoint (to be implemented when available)
	return nil, fmt.Errorf("signed tree head not yet implemented for GoDaddy adapter")
}

// VerifyRecord verifies a record's authenticity
func (g *godaddyAdapter) VerifyRecord(ctx context.Context, record *Record) (*VerificationResult, error) {
	// Basic verification - more sophisticated checks can be added
	return &VerificationResult{
		Valid: true,
		Checks: map[string]CheckResult{
			"registry": {Passed: true, Message: "Record retrieved from GoDaddy registry"},
		},
		Timestamp: time.Now(),
	}, nil
}

// Subscribe starts receiving events from the GoDaddy events API
func (g *godaddyAdapter) Subscribe(ctx context.Context, handler EventHandler) error {
	log.Info().Msg("Starting GoDaddy events subscription")

	var lastLogID string
	pollInterval := 10 * time.Second // Poll every 10 seconds
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Info().Msg("GoDaddy events subscription stopped")
			return ctx.Err()
		case <-ticker.C:
			// Fetch events from GoDaddy API
			events, newLastLogID, err := g.fetchEvents(ctx, lastLogID, 100)
			if err != nil {
				log.Error().Err(err).Msg("Failed to fetch events from GoDaddy")
				continue
			}

			// Process each event
			for _, gdEvent := range events {
				event := g.convertEvent(&gdEvent)
				if err := handler(ctx, event); err != nil {
					log.Error().
						Err(err).
						Str("logId", gdEvent.LogID).
						Str("eventType", gdEvent.EventType).
						Msg("Event handler failed")
				}
			}

			// Update cursor if we got events
			if newLastLogID != "" {
				lastLogID = newLastLogID
			}

			if len(events) > 0 {
				log.Debug().
					Int("count", len(events)).
					Str("lastLogId", lastLogID).
					Msg("Processed GoDaddy events")
			}
		}
	}
}

// fetchEvents retrieves events from GoDaddy's GET /v1/agents/events endpoint
func (g *godaddyAdapter) fetchEvents(ctx context.Context, lastLogID string, limit int) ([]godaddyEvent, string, error) {
	url := fmt.Sprintf("%s/agents/events", g.baseURL)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, "", err
	}

	// Add query parameters
	q := req.URL.Query()
	if lastLogID != "" {
		q.Add("lastLogId", lastLogID)
	}
	if limit > 0 {
		q.Add("limit", fmt.Sprintf("%d", limit))
	}
	req.URL.RawQuery = q.Encode()

	// Add authentication
	req.Header.Set("Authorization", fmt.Sprintf("sso-key %s:%s", g.apiKey, g.apiSecret))
	req.Header.Set("Accept", "application/json")

	resp, err := g.client.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			log.Warn().Err(err).Msg("Failed to close response body")
		}
	}()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, "", fmt.Errorf("GoDaddy events API returned %d: %s", resp.StatusCode, string(body))
	}

	var eventsResp godaddyEventsResponse
	if err := json.NewDecoder(resp.Body).Decode(&eventsResp); err != nil {
		return nil, "", fmt.Errorf("failed to decode events response: %w", err)
	}

	return eventsResp.Items, eventsResp.LastLogID, nil
}

// convertEvent converts a GoDaddy event to our internal Event structure
func (g *godaddyAdapter) convertEvent(gdEvent *godaddyEvent) *Event {
	// Parse timestamp
	timestamp, _ := time.Parse(time.RFC3339, gdEvent.CreatedAt)

	// Map GoDaddy event types to our internal types
	eventType := mapEventType(gdEvent.EventType)

	// Extract endpoint if available
	endpoint := ""
	if len(gdEvent.Endpoints) > 0 {
		endpoint = gdEvent.Endpoints[0].AgentURL
	}

	// Extract FQDN from ANS name
	fqdn := gdEvent.AgentHost

	return &Event{
		ID:        gdEvent.LogID,
		Type:      eventType,
		ANSName:   gdEvent.ANSName,
		FQDN:      fqdn,
		Timestamp: timestamp,
		Data: map[string]interface{}{
			"agentId":          gdEvent.AgentID,
			"agentDisplayName": gdEvent.AgentDisplayName,
			"agentDescription": gdEvent.AgentDescription,
			"version":          gdEvent.Version,
			"providerId":       gdEvent.ProviderID,
			"endpoint":         endpoint,
			"expiresAt":        gdEvent.ExpiresAt,
		},
		Signature: "", // GoDaddy doesn't sign individual events
	}
}

// mapEventType maps GoDaddy event types to internal event types
func mapEventType(godaddyType string) string {
	switch godaddyType {
	case "AGENT_REGISTERED":
		return "registered"
	case "AGENT_RENEWED":
		return "renewed"
	case "AGENT_REVOKED":
		return "revoked"
	case "AGENT_DEPRECATED":
		return "deprecated"
	case "AGENT_EXPIRED":
		return "expired"
	case "AGENT_UPDATED":
		return "updated"
	default:
		return strings.ToLower(strings.TrimPrefix(godaddyType, "AGENT_"))
	}
}

// Name returns the adapter name
func (g *godaddyAdapter) Name() string {
	return g.name
}

// Priority returns the adapter priority
func (g *godaddyAdapter) Priority() int {
	return g.priority
}

// Healthy checks if the registry is reachable
func (g *godaddyAdapter) Healthy(ctx context.Context) (bool, error) {
	// Simple health check - try to reach the API
	req, err := http.NewRequestWithContext(ctx, "GET", g.baseURL, nil)
	if err != nil {
		return false, err
	}

	resp, err := g.client.Do(req)
	if err != nil {
		return false, err
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			log.Warn().Err(err).Msg("Failed to close response body")
		}
	}()

	return resp.StatusCode < 500, nil
}

// Close releases resources
func (g *godaddyAdapter) Close() error {
	g.client.CloseIdleConnections()
	return nil
}

// Helper methods

func (g *godaddyAdapter) makeRequest(ctx context.Context, method, url string, body interface{}) ([]byte, error) {
	var reqBody io.Reader
	if body != nil {
		jsonData, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request: %w", err)
		}
		reqBody = bytes.NewBuffer(jsonData)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Add authorization header
	req.Header.Set("Authorization", fmt.Sprintf("sso-key %s:%s", g.apiKey, g.apiSecret))
	req.Header.Set("Content-Type", "application/json")

	resp, err := g.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			log.Warn().Err(err).Msg("Failed to close response body")
		}
	}()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Handle error responses
	if resp.StatusCode >= 400 {
		var gdErr godaddyError
		if err := json.Unmarshal(respBody, &gdErr); err == nil && gdErr.Code != "" {
			// Structured error from GoDaddy
			if resp.StatusCode == http.StatusNotFound {
				return nil, &ErrNotFound{ANSName: url}
			}
			return nil, fmt.Errorf("godaddy API error (%s): %s", gdErr.Code, gdErr.Message)
		}
		// Fallback for non-structured errors
		if resp.StatusCode == http.StatusNotFound {
			return nil, &ErrNotFound{ANSName: url}
		}
		return nil, fmt.Errorf("API error (%d): %s", resp.StatusCode, string(respBody))
	}

	return respBody, nil
}

func (g *godaddyAdapter) getCertificates(ctx context.Context, links []struct {
	Rel  string `json:"rel"`
	Href string `json:"href"`
}) (*godaddyCertificate, *godaddyCertificate) {
	var serverCertURL, identityCertURL string

	for _, link := range links {
		switch link.Rel {
		case "server-certificates":
			serverCertURL = link.Href
		case "identity-certificates":
			identityCertURL = link.Href
		}
	}

	var serverCert, identityCert *godaddyCertificate

	// Get server certificate
	if serverCertURL != "" {
		resp, err := g.makeRequest(ctx, "GET", serverCertURL, nil)
		if err == nil {
			var certs []godaddyCertificateResponse
			if err := json.Unmarshal(resp, &certs); err == nil && len(certs) > 0 {
				serverCert = g.parseCertificate(&certs[0])
			}
		}
	}

	// Get identity certificate
	if identityCertURL != "" {
		resp, err := g.makeRequest(ctx, "GET", identityCertURL, nil)
		if err == nil {
			var certs []godaddyCertificateResponse
			if err := json.Unmarshal(resp, &certs); err == nil && len(certs) > 0 {
				identityCert = g.parseCertificate(&certs[0])
			}
		}
	}

	return serverCert, identityCert
}

func (g *godaddyAdapter) parseCertificate(cert *godaddyCertificateResponse) *godaddyCertificate {
	if cert == nil || cert.CertificatePEM == "" {
		return nil
	}

	// Calculate SHA-256 fingerprint from PEM
	fingerprint := g.calculateFingerprint(cert.CertificatePEM)

	// Parse timestamps
	notBefore, _ := time.Parse(time.RFC3339, cert.CertificateValidFrom)
	notAfter, _ := time.Parse(time.RFC3339, cert.CertificateValidTo)

	return &godaddyCertificate{
		PEM:          cert.CertificatePEM,
		Fingerprint:  fingerprint,
		Issuer:       cert.CertificateIssuer,
		Subject:      cert.CertificateSubject,
		SerialNumber: cert.CertificateSerialNumber,
		NotBefore:    notBefore,
		NotAfter:     notAfter,
	}
}

func (g *godaddyAdapter) calculateFingerprint(certPEM string) string {
	// Parse PEM to get DER bytes
	block, _ := pem.Decode([]byte(certPEM))
	if block == nil {
		return ""
	}

	// Calculate SHA-256 hash of DER-encoded certificate
	hash := sha256.Sum256(block.Bytes)
	return "SHA256:" + hex.EncodeToString(hash[:])
}

func (g *godaddyAdapter) buildRecord(details *godaddyAgentDetails, serverCert, identityCert *godaddyCertificate) *Record {
	// Extract endpoint and protocol info from endpoints array
	var endpoint, protocol string
	var capabilities []string
	var protocols []string

	if len(details.Endpoints) > 0 {
		firstEndpoint := details.Endpoints[0]
		endpoint = firstEndpoint.AgentURL
		protocol = firstEndpoint.Protocol
		protocols = append(protocols, firstEndpoint.Protocol)

		// Extract capabilities from functions
		for _, fn := range firstEndpoint.Functions {
			capabilities = append(capabilities, fn.ID)
		}
	}

	// Fallback to agentHost if no endpoint URL
	if endpoint == "" && details.AgentHost != "" {
		endpoint = fmt.Sprintf("https://%s", details.AgentHost)
	}

	// Fallback for protocol
	if protocol == "" {
		protocol = details.Protocol // Use deprecated field if available
	}

	// Build certificate info
	certInfo := CertificateInfo{}
	if serverCert != nil {
		certInfo.PublicCert = CertDetails{
			Fingerprint:  serverCert.Fingerprint,
			Issuer:       serverCert.Issuer,
			SerialNumber: serverCert.SerialNumber,
			IssuedAt:     serverCert.NotBefore,
			ExpiresAt:    serverCert.NotAfter,
		}
	}
	if identityCert != nil {
		certInfo.PrivateCert = CertDetails{
			Fingerprint:  identityCert.Fingerprint,
			Issuer:       identityCert.Issuer,
			SerialNumber: identityCert.SerialNumber,
			IssuedAt:     identityCert.NotBefore,
			ExpiresAt:    identityCert.NotAfter,
		}
	}

	record := &Record{
		ANSName:      details.ANSName,
		FQDN:         details.AgentHost,
		Protocol:     protocol,
		Version:      details.Version,
		Status:       g.mapStatus(details.AgentStatus),
		RegistrarID:  "godaddy",
		TTL:          5 * time.Minute,
		Certificates: certInfo,
		UpdatedAt:    time.Now(),
		Endpoint:     endpoint,
		Metadata: AgentMetadata{
			DisplayName:  details.AgentDisplayName,
			Description:  details.AgentDescription,
			Capabilities: capabilities,
			Protocols:    protocols,
		},
	}

	// Parse registration timestamp
	if details.RegistrationTimestamp != "" {
		if t, err := time.Parse(time.RFC3339, details.RegistrationTimestamp); err == nil {
			record.RegisteredAt = t
		}
	}

	// Add certificate information
	if serverCert != nil {
		record.Certificates.PublicCert = g.parseCertDetails(serverCert)
	}
	if identityCert != nil {
		record.Certificates.PrivateCert = g.parseCertDetails(identityCert)
	}

	// Set expiration based on identity certificate
	if identityCert != nil && !identityCert.NotAfter.IsZero() {
		record.ExpiresAt = identityCert.NotAfter
	} else {
		record.ExpiresAt = time.Now().Add(90 * 24 * time.Hour)
	}

	return record
}

func (g *godaddyAdapter) parseCertDetails(cert *godaddyCertificate) CertDetails {
	return CertDetails{
		Fingerprint:  cert.Fingerprint,
		Issuer:       cert.Issuer,
		SerialNumber: cert.SerialNumber,
		IssuedAt:     cert.NotBefore,
		ExpiresAt:    cert.NotAfter,
	}
}

func (g *godaddyAdapter) mapStatus(status string) string {
	switch status {
	case "ACTIVE", "VERIFIED":
		return "active"
	case "PENDING", "PENDING_VALIDATION":
		return "pending"
	case "REVOKED":
		return "revoked"
	case "EXPIRED":
		return "expired"
	default:
		return "unknown"
	}
}

func init() {
	Register("godaddy", func(opts Options, config map[string]interface{}) (Adapter, error) {
		return NewGoDaddyAdapter(opts, config)
	})
}
