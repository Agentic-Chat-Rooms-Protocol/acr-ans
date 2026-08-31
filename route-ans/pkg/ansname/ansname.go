// Package ansname provides parsing and validation for ANSName identifiers.
// ANSName is the canonical identifier format for registered agents in the ANS ecosystem.
package ansname

import (
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

// Common errors
var (
	ErrInvalidFormat     = errors.New("invalid ANSName format")
	ErrMissingProtocol   = errors.New("missing protocol scheme")
	ErrMissingAgentName  = errors.New("missing agent name")
	ErrMissingCapability = errors.New("missing capability")
	ErrMissingProviderID = errors.New("missing provider ID")
	ErrMissingVersion    = errors.New("missing version")
	ErrMissingExtension  = errors.New("missing extension")
	ErrInvalidVersion    = errors.New("invalid version format")
	ErrInvalidProviderID = errors.New("invalid provider ID format")
)

// ANSName represents a parsed ANS name identifier.
// Canonical format: protocol://agentName.capability.providerID.version.extension
// Example: mcp://sentimentAnalyzer.textAnalysis.PID-1234.v1.0.0.example.com
type ANSName struct {
	// Protocol is the communication protocol scheme (e.g., "mcp", "a2a", "acp", "https")
	Protocol string

	// AgentName is the unique provider-assigned name for the agent service
	AgentName string

	// Capability is the high-level function the agent exposes
	Capability string

	// ProviderID is a non-semantic, unique identifier for the owning entity (e.g., "PID-1234")
	ProviderID string

	// Version is the semantic version bound to the agent's code (e.g., "v1.0.0")
	Version string

	// Extension is the fully-qualified domain name acting as trust anchor
	Extension string

	// Raw is the original unparsed ANSName string
	Raw string
}

// providerIDPattern matches valid ProviderID format (PID-XXXX)
var providerIDPattern = regexp.MustCompile(`^PID-[A-Za-z0-9]+$`)

// versionPattern matches semantic version format (v1.0.0, v1.2.3, etc.)
var versionPattern = regexp.MustCompile(`^v\d+\.\d+\.\d+$`)

// protocolPattern matches valid protocol schemes
var protocolPattern = regexp.MustCompile(`^[a-z][a-z0-9+.-]*$`)

// Parse parses an ANSName string into its components.
// Expected format: protocol://agentName.capability.providerID.version.extension
func Parse(raw string) (*ANSName, error) {
	if raw == "" {
		return nil, ErrInvalidFormat
	}

	// Parse as URL to extract protocol and path
	u, err := url.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidFormat, err)
	}

	// Extract and validate protocol
	protocol := strings.ToLower(u.Scheme)
	if protocol == "" {
		return nil, ErrMissingProtocol
	}
	if !protocolPattern.MatchString(protocol) {
		return nil, fmt.Errorf("%w: invalid protocol scheme '%s'", ErrInvalidFormat, protocol)
	}

	// Get the host part (contains the structured name)
	host := u.Host
	if host == "" {
		// Try Opaque for schemes that don't use //
		host = u.Opaque
	}
	if host == "" {
		return nil, ErrInvalidFormat
	}

	// Parse the structured name components
	// Format: agentName.capability.providerID.version.extension
	// Extension can contain multiple dots (e.g., example.co.uk)
	ans, err := parseComponents(protocol, host, raw)
	if err != nil {
		return nil, err
	}

	return ans, nil
}

// parseComponents extracts and validates the individual components from the host string
//
//nolint:gocyclo // High complexity due to format parsing logic; refactoring would reduce readability
func parseComponents(protocol, host, raw string) (*ANSName, error) {
	parts := strings.Split(host, ".")

	// Try simplified format first (GoDaddy): protocol://version.host.domain
	// Example: ans://v1.0.0.greeting.neelanjan.dev
	if len(parts) >= 3 && len(parts[0]) > 0 && parts[0][0] == 'v' {
		potentialVersion := parts[0] + "." + parts[1] + "." + parts[2]
		if versionPattern.MatchString(potentialVersion) {
			// This is simplified format
			if len(parts) < 4 {
				return nil, fmt.Errorf("%w: need at least version and host", ErrInvalidFormat)
			}
			agentName := parts[3]                     // First part after version is agent name
			extension := strings.Join(parts[4:], ".") // Rest is the domain

			// For simplified format, we need at least agent.domain (2 parts after version)
			if extension == "" {
				return nil, fmt.Errorf("%w: need at least agent name and domain", ErrInvalidFormat)
			}

			return &ANSName{
				Protocol:   protocol,
				AgentName:  agentName,
				Capability: "default",  // No capability in simplified format
				ProviderID: "PID-0000", // No provider in simplified format
				Version:    potentialVersion,
				Extension:  extension,
				Raw:        raw,
			}, nil
		}
	}

	// Fall back to full format parsing
	// Minimum parts: agentName.capability.providerID.v1.0.0.extension (at least 7 parts)
	// Version like v1.0.0 takes 3 parts after dot split
	if len(parts) < 7 {
		return nil, fmt.Errorf("%w: expected at least 7 dot-separated components for full format or version.host.domain for simplified format", ErrInvalidFormat)
	}

	// Find the version start component (starts with 'v' followed by digits)
	// Version format is vX.Y.Z which spans 3 dots
	versionStartIdx := -1
	for i := 0; i < len(parts)-2; i++ {
		if len(parts[i]) > 0 && parts[i][0] == 'v' {
			// Try to form a version from this and next 2 parts
			potentialVersion := parts[i] + "." + parts[i+1] + "." + parts[i+2]
			if versionPattern.MatchString(potentialVersion) {
				versionStartIdx = i
				break
			}
		}
	}

	if versionStartIdx == -1 {
		return nil, ErrMissingVersion
	}

	// Need at least: agentName, capability, providerID before version
	if versionStartIdx < 3 {
		return nil, fmt.Errorf("%w: need agentName, capability, and providerID before version", ErrInvalidFormat)
	}

	// Version takes 3 parts (vX, Y, Z), need at least one part after for extension
	versionEndIdx := versionStartIdx + 2
	if versionEndIdx >= len(parts)-1 {
		return nil, ErrMissingExtension
	}

	// Extract components
	agentName := parts[0]
	capability := parts[1]
	providerID := parts[versionStartIdx-1]
	version := parts[versionStartIdx] + "." + parts[versionStartIdx+1] + "." + parts[versionStartIdx+2]
	extension := strings.Join(parts[versionEndIdx+1:], ".")

	// Handle case where capability might span multiple parts
	// If providerID is at index > 2, join intermediate parts as capability
	if versionStartIdx > 3 {
		capability = strings.Join(parts[1:versionStartIdx-1], ".")
	}

	// Validate components
	if agentName == "" {
		return nil, ErrMissingAgentName
	}
	if capability == "" {
		return nil, ErrMissingCapability
	}
	if !providerIDPattern.MatchString(providerID) {
		return nil, fmt.Errorf("%w: '%s' does not match PID-XXXX format", ErrInvalidProviderID, providerID)
	}
	if !versionPattern.MatchString(version) {
		return nil, fmt.Errorf("%w: '%s' does not match vX.Y.Z format", ErrInvalidVersion, version)
	}
	if extension == "" {
		return nil, ErrMissingExtension
	}

	return &ANSName{
		Protocol:   protocol,
		AgentName:  agentName,
		Capability: capability,
		ProviderID: providerID,
		Version:    version,
		Extension:  extension,
		Raw:        raw,
	}, nil
}

// String returns the canonical string representation of the ANSName
func (a *ANSName) String() string {
	return fmt.Sprintf("%s://%s.%s.%s.%s.%s",
		a.Protocol,
		a.AgentName,
		a.Capability,
		a.ProviderID,
		a.Version,
		a.Extension,
	)
}

// FQDN returns the fully qualified domain name for the agent.
// This is constructed as: agentName.extension
func (a *ANSName) FQDN() string {
	return fmt.Sprintf("%s.%s", a.AgentName, a.Extension)
}

// CacheKey returns a unique key suitable for caching this ANSName
func (a *ANSName) CacheKey() string {
	return a.String()
}

// Validate checks if the ANSName is valid and complete
func (a *ANSName) Validate() error {
	if a.Protocol == "" {
		return ErrMissingProtocol
	}
	if !protocolPattern.MatchString(a.Protocol) {
		return fmt.Errorf("%w: invalid protocol", ErrInvalidFormat)
	}
	if a.AgentName == "" {
		return ErrMissingAgentName
	}
	if a.Capability == "" {
		return ErrMissingCapability
	}
	if a.ProviderID == "" {
		return ErrMissingProviderID
	}
	if !providerIDPattern.MatchString(a.ProviderID) {
		return ErrInvalidProviderID
	}
	if a.Version == "" {
		return ErrMissingVersion
	}
	if !versionPattern.MatchString(a.Version) {
		return ErrInvalidVersion
	}
	if a.Extension == "" {
		return ErrMissingExtension
	}
	return nil
}

// VersionComponents parses the semantic version and returns major, minor, patch
func (a *ANSName) VersionComponents() (major, minor, patch int, err error) {
	// Remove 'v' prefix
	v := strings.TrimPrefix(a.Version, "v")
	_, err = fmt.Sscanf(v, "%d.%d.%d", &major, &minor, &patch)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("failed to parse version: %w", err)
	}
	return major, minor, patch, nil
}

// IsNewerThan returns true if this ANSName has a higher version than other
func (a *ANSName) IsNewerThan(other *ANSName) (bool, error) {
	aMajor, aMinor, aPatch, err := a.VersionComponents()
	if err != nil {
		return false, err
	}

	oMajor, oMinor, oPatch, err := other.VersionComponents()
	if err != nil {
		return false, err
	}

	if aMajor != oMajor {
		return aMajor > oMajor, nil
	}
	if aMinor != oMinor {
		return aMinor > oMinor, nil
	}
	return aPatch > oPatch, nil
}

// SameAgent returns true if both ANSNames refer to the same agent (ignoring version)
func (a *ANSName) SameAgent(other *ANSName) bool {
	return a.Protocol == other.Protocol &&
		a.AgentName == other.AgentName &&
		a.Capability == other.Capability &&
		a.ProviderID == other.ProviderID &&
		a.Extension == other.Extension
}

// ValidProtocols returns a list of known valid protocols
func ValidProtocols() []string {
	return []string{"a2a", "mcp", "acp", "https", "http"}
}

// IsKnownProtocol returns true if the protocol is a recognized ANS protocol
func IsKnownProtocol(protocol string) bool {
	known := map[string]bool{
		"a2a":   true,
		"mcp":   true,
		"acp":   true,
		"https": true,
		"http":  true,
	}
	return known[strings.ToLower(protocol)]
}
