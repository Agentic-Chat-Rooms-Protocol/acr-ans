package ansname

import (
	"testing"
)

func TestParse(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    *ANSName
		wantErr bool
		errType error
	}{
		{
			name:  "valid canonical ANSName",
			input: "mcp://sentimentAnalyzer.textAnalysis.PID-1234.v1.0.0.example.com",
			want: &ANSName{
				Protocol:   "mcp",
				AgentName:  "sentimentAnalyzer",
				Capability: "textAnalysis",
				ProviderID: "PID-1234",
				Version:    "v1.0.0",
				Extension:  "example.com",
				Raw:        "mcp://sentimentAnalyzer.textAnalysis.PID-1234.v1.0.0.example.com",
			},
			wantErr: false,
		},
		{
			name:  "valid a2a protocol",
			input: "a2a://TextProc.Translate.PID-ACME.v2.1.0.acme.co.uk",
			want: &ANSName{
				Protocol:   "a2a",
				AgentName:  "TextProc",
				Capability: "Translate",
				ProviderID: "PID-ACME",
				Version:    "v2.1.0",
				Extension:  "acme.co.uk",
				Raw:        "a2a://TextProc.Translate.PID-ACME.v2.1.0.acme.co.uk",
			},
			wantErr: false,
		},
		{
			name:  "valid with subdomain extension",
			input: "https://myagent.api.PID-5678.v0.0.1.api.example.org",
			want: &ANSName{
				Protocol:   "https",
				AgentName:  "myagent",
				Capability: "api",
				ProviderID: "PID-5678",
				Version:    "v0.0.1",
				Extension:  "api.example.org",
				Raw:        "https://myagent.api.PID-5678.v0.0.1.api.example.org",
			},
			wantErr: false,
		},
		{
			name:    "empty string",
			input:   "",
			wantErr: true,
			errType: ErrInvalidFormat,
		},
		{
			name:    "missing protocol",
			input:   "sentimentAnalyzer.textAnalysis.PID-1234.v1.0.0.example.com",
			wantErr: true,
			errType: ErrMissingProtocol,
		},
		{
			name:    "invalid provider ID format",
			input:   "mcp://agent.capability.INVALID.v1.0.0.example.com",
			wantErr: true,
			errType: ErrInvalidProviderID,
		},
		{
			name:    "invalid version format",
			input:   "mcp://agent.capability.PID-1234.1.0.0.example.com",
			wantErr: true,
			errType: ErrMissingVersion,
		},
		{
			name:    "too few components",
			input:   "mcp://agent.example.com",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Parse(tt.input)

			if tt.wantErr {
				if err == nil {
					t.Errorf("Parse() expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("Parse() unexpected error: %v", err)
				return
			}

			if got.Protocol != tt.want.Protocol {
				t.Errorf("Protocol = %v, want %v", got.Protocol, tt.want.Protocol)
			}
			if got.AgentName != tt.want.AgentName {
				t.Errorf("AgentName = %v, want %v", got.AgentName, tt.want.AgentName)
			}
			if got.Capability != tt.want.Capability {
				t.Errorf("Capability = %v, want %v", got.Capability, tt.want.Capability)
			}
			if got.ProviderID != tt.want.ProviderID {
				t.Errorf("ProviderID = %v, want %v", got.ProviderID, tt.want.ProviderID)
			}
			if got.Version != tt.want.Version {
				t.Errorf("Version = %v, want %v", got.Version, tt.want.Version)
			}
			if got.Extension != tt.want.Extension {
				t.Errorf("Extension = %v, want %v", got.Extension, tt.want.Extension)
			}
		})
	}
}

func TestANSName_String(t *testing.T) {
	ans := &ANSName{
		Protocol:   "mcp",
		AgentName:  "sentimentAnalyzer",
		Capability: "textAnalysis",
		ProviderID: "PID-1234",
		Version:    "v1.0.0",
		Extension:  "example.com",
	}

	want := "mcp://sentimentAnalyzer.textAnalysis.PID-1234.v1.0.0.example.com"
	got := ans.String()

	if got != want {
		t.Errorf("String() = %v, want %v", got, want)
	}
}

func TestANSName_FQDN(t *testing.T) {
	ans := &ANSName{
		Protocol:   "mcp",
		AgentName:  "sentimentAnalyzer",
		Capability: "textAnalysis",
		ProviderID: "PID-1234",
		Version:    "v1.0.0",
		Extension:  "example.com",
	}

	want := "sentimentAnalyzer.example.com"
	got := ans.FQDN()

	if got != want {
		t.Errorf("FQDN() = %v, want %v", got, want)
	}
}

func TestANSName_VersionComponents(t *testing.T) {
	ans := &ANSName{Version: "v1.2.3"}

	major, minor, patch, err := ans.VersionComponents()
	if err != nil {
		t.Errorf("VersionComponents() unexpected error: %v", err)
	}

	if major != 1 || minor != 2 || patch != 3 {
		t.Errorf("VersionComponents() = %d.%d.%d, want 1.2.3", major, minor, patch)
	}
}

func TestANSName_IsNewerThan(t *testing.T) {
	tests := []struct {
		name string
		v1   string
		v2   string
		want bool
	}{
		{"major version higher", "v2.0.0", "v1.0.0", true},
		{"minor version higher", "v1.2.0", "v1.1.0", true},
		{"patch version higher", "v1.1.2", "v1.1.1", true},
		{"same version", "v1.0.0", "v1.0.0", false},
		{"lower version", "v1.0.0", "v2.0.0", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := &ANSName{Version: tt.v1}
			b := &ANSName{Version: tt.v2}

			got, err := a.IsNewerThan(b)
			if err != nil {
				t.Errorf("IsNewerThan() unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("IsNewerThan() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestANSName_SameAgent(t *testing.T) {
	a := &ANSName{
		Protocol:   "mcp",
		AgentName:  "agent",
		Capability: "cap",
		ProviderID: "PID-1234",
		Version:    "v1.0.0",
		Extension:  "example.com",
	}

	// Same agent, different version
	b := &ANSName{
		Protocol:   "mcp",
		AgentName:  "agent",
		Capability: "cap",
		ProviderID: "PID-1234",
		Version:    "v2.0.0",
		Extension:  "example.com",
	}

	if !a.SameAgent(b) {
		t.Error("SameAgent() should return true for same agent with different version")
	}

	// Different agent
	c := &ANSName{
		Protocol:   "mcp",
		AgentName:  "other",
		Capability: "cap",
		ProviderID: "PID-1234",
		Version:    "v1.0.0",
		Extension:  "example.com",
	}

	if a.SameAgent(c) {
		t.Error("SameAgent() should return false for different agents")
	}
}

func TestANSName_Validate(t *testing.T) {
	valid := &ANSName{
		Protocol:   "mcp",
		AgentName:  "agent",
		Capability: "cap",
		ProviderID: "PID-1234",
		Version:    "v1.0.0",
		Extension:  "example.com",
	}

	if err := valid.Validate(); err != nil {
		t.Errorf("Validate() unexpected error for valid ANSName: %v", err)
	}

	// Test missing protocol
	invalid := &ANSName{
		AgentName:  "agent",
		Capability: "cap",
		ProviderID: "PID-1234",
		Version:    "v1.0.0",
		Extension:  "example.com",
	}

	if err := invalid.Validate(); err == nil {
		t.Error("Validate() should return error for missing protocol")
	}
}
