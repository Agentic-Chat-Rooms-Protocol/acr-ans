package ansname

import (
	"testing"
)

func TestParseVersionRange(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantOp    string
		wantMajor int
		wantMinor int
		wantPatch int
		wantErr   bool
	}{
		// Exact versions
		{"exact version", "1.0.0", "=", 1, 0, 0, false},
		{"exact version with v", "v2.3.4", "=", 2, 3, 4, false},

		// Operators
		{"greater than or equal", ">=1.0.0", ">=", 1, 0, 0, false},
		{"greater than", ">1.2.3", ">", 1, 2, 3, false},
		{"less than or equal", "<=2.0.0", "<=", 2, 0, 0, false},
		{"less than", "<3.0.0", "<", 3, 0, 0, false},
		{"equals operator", "=1.5.0", "=", 1, 5, 0, false},

		// Caret ranges
		{"caret range", "^1.2.3", "^", 1, 2, 3, false},
		{"caret with v", "^v2.0.0", "^", 2, 0, 0, false},

		// Tilde ranges
		{"tilde range", "~1.2.3", "~", 1, 2, 3, false},
		{"tilde with v", "~v1.0.0", "~", 1, 0, 0, false},

		// Wildcards
		{"major wildcard", "1.x", "x", 1, -1, -1, false},
		{"major.minor wildcard", "1.2.x", "x", 1, 2, -1, false},

		// Any version
		{"asterisk", "*", "*", 0, 0, 0, false},
		{"empty", "", "*", 0, 0, 0, false},

		// Invalid formats
		{"invalid format", "abc", "", 0, 0, 0, true},
		{"incomplete version", "1.0", "", 0, 0, 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			vr, err := ParseVersionRange(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseVersionRange() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}
			if vr.operator != tt.wantOp {
				t.Errorf("operator = %v, want %v", vr.operator, tt.wantOp)
			}
			if vr.operator != "*" && vr.operator != "x" {
				if vr.major != tt.wantMajor {
					t.Errorf("major = %v, want %v", vr.major, tt.wantMajor)
				}
				if vr.minor != tt.wantMinor {
					t.Errorf("minor = %v, want %v", vr.minor, tt.wantMinor)
				}
				if vr.patch != tt.wantPatch {
					t.Errorf("patch = %v, want %v", vr.patch, tt.wantPatch)
				}
			}
		})
	}
}

func TestVersionRange_Matches(t *testing.T) {
	tests := []struct {
		name      string
		rangeStr  string
		version   string
		wantMatch bool
	}{
		// Exact matches
		{"exact match", "1.0.0", "v1.0.0", true},
		{"exact no match", "1.0.0", "v1.0.1", false},

		// Greater than
		{"gt match higher major", ">1.0.0", "v2.0.0", true},
		{"gt match higher minor", ">1.2.0", "v1.3.0", true},
		{"gt match higher patch", ">1.2.3", "v1.2.4", true},
		{"gt no match equal", ">1.0.0", "v1.0.0", false},
		{"gt no match lower", ">2.0.0", "v1.9.9", false},

		// Greater than or equal
		{"gte match higher", ">=1.0.0", "v2.0.0", true},
		{"gte match equal", ">=1.2.3", "v1.2.3", true},
		{"gte no match lower", ">=2.0.0", "v1.9.9", false},

		// Less than
		{"lt match lower", "<2.0.0", "v1.9.9", true},
		{"lt no match equal", "<1.0.0", "v1.0.0", false},
		{"lt no match higher", "<1.0.0", "v2.0.0", false},

		// Less than or equal
		{"lte match lower", "<=2.0.0", "v1.0.0", true},
		{"lte match equal", "<=1.2.3", "v1.2.3", true},
		{"lte no match higher", "<=1.0.0", "v2.0.0", false},

		// Caret ranges
		{"caret 1.x.x allows same major", "^1.2.3", "v1.5.0", true},
		{"caret 1.x.x allows higher minor", "^1.2.3", "v1.3.0", true},
		{"caret 1.x.x allows higher patch", "^1.2.3", "v1.2.5", true},
		{"caret 1.x.x rejects lower patch", "^1.2.3", "v1.2.2", false},
		{"caret 1.x.x rejects different major", "^1.2.3", "v2.0.0", false},

		{"caret 0.x.x allows same minor", "^0.2.3", "v0.2.5", true},
		{"caret 0.x.x rejects different minor", "^0.2.3", "v0.3.0", false},
		{"caret 0.x.x rejects different major", "^0.2.3", "v1.0.0", false},

		{"caret 0.0.x exact patch only", "^0.0.3", "v0.0.3", true},
		{"caret 0.0.x rejects other patch", "^0.0.3", "v0.0.4", false},

		// Tilde ranges
		{"tilde allows patch", "~1.2.3", "v1.2.5", true},
		{"tilde rejects minor", "~1.2.3", "v1.3.0", false},
		{"tilde rejects major", "~1.2.3", "v2.0.0", false},
		{"tilde rejects lower", "~1.2.3", "v1.2.2", false},

		// Wildcards
		{"major wildcard matches any minor/patch", "1.x", "v1.5.9", true},
		{"major wildcard rejects other major", "1.x", "v2.0.0", false},

		{"major.minor wildcard matches any patch", "1.2.x", "v1.2.9", true},
		{"major.minor wildcard rejects other minor", "1.2.x", "v1.3.0", false},

		// Any version
		{"asterisk matches anything", "*", "v99.99.99", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			vr, err := ParseVersionRange(tt.rangeStr)
			if err != nil {
				t.Fatalf("ParseVersionRange() error = %v", err)
			}

			name, err := Parse("mcp://agent.capability.PID-123." + tt.version + ".example.com")
			if err != nil {
				t.Fatalf("Parse() error = %v", err)
			}

			match, err := vr.Matches(name)
			if err != nil {
				t.Fatalf("Matches() error = %v", err)
			}

			if match != tt.wantMatch {
				t.Errorf("Matches() = %v, want %v", match, tt.wantMatch)
			}
		})
	}
}

func TestNegotiateVersion(t *testing.T) {
	// Create test candidates
	v100, _ := Parse("mcp://agent.capability.PID-123.v1.0.0.example.com")
	v101, _ := Parse("mcp://agent.capability.PID-123.v1.0.1.example.com")
	v110, _ := Parse("mcp://agent.capability.PID-123.v1.1.0.example.com")
	v120, _ := Parse("mcp://agent.capability.PID-123.v1.2.0.example.com")
	v200, _ := Parse("mcp://agent.capability.PID-123.v2.0.0.example.com")
	v210, _ := Parse("mcp://agent.capability.PID-123.v2.1.0.example.com")

	tests := []struct {
		name        string
		candidates  []*ANSName
		rangeStr    string
		wantVersion string
		wantErr     bool
	}{
		{
			name:        "exact version match",
			candidates:  []*ANSName{v100, v110, v120},
			rangeStr:    "1.1.0",
			wantVersion: "v1.1.0",
			wantErr:     false,
		},
		{
			name:        "gte picks highest",
			candidates:  []*ANSName{v100, v101, v110, v120},
			rangeStr:    ">=1.0.0",
			wantVersion: "v1.2.0",
			wantErr:     false,
		},
		{
			name:        "lt picks highest below threshold",
			candidates:  []*ANSName{v100, v110, v120, v200},
			rangeStr:    "<2.0.0",
			wantVersion: "v1.2.0",
			wantErr:     false,
		},
		{
			name:        "caret picks latest compatible",
			candidates:  []*ANSName{v100, v110, v120, v200},
			rangeStr:    "^1.0.0",
			wantVersion: "v1.2.0",
			wantErr:     false,
		},
		{
			name:        "tilde picks latest patch",
			candidates:  []*ANSName{v100, v101, v110},
			rangeStr:    "~1.0.0",
			wantVersion: "v1.0.1",
			wantErr:     false,
		},
		{
			name:        "wildcard picks highest in range",
			candidates:  []*ANSName{v100, v110, v120, v200, v210},
			rangeStr:    "1.x",
			wantVersion: "v1.2.0",
			wantErr:     false,
		},
		{
			name:        "asterisk picks absolute highest",
			candidates:  []*ANSName{v100, v110, v200, v210},
			rangeStr:    "*",
			wantVersion: "v2.1.0",
			wantErr:     false,
		},
		{
			name:        "no match returns error",
			candidates:  []*ANSName{v100, v110},
			rangeStr:    ">=2.0.0",
			wantVersion: "",
			wantErr:     true,
		},
		{
			name:        "empty candidates returns error",
			candidates:  []*ANSName{},
			rangeStr:    "1.0.0",
			wantVersion: "",
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := NegotiateVersion(tt.candidates, tt.rangeStr)
			if (err != nil) != tt.wantErr {
				t.Errorf("NegotiateVersion() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}
			if result.Version != tt.wantVersion {
				t.Errorf("NegotiateVersion() version = %v, want %v", result.Version, tt.wantVersion)
			}
		})
	}
}

func TestVersionRange_String(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"1.0.0", "1.0.0"},
		{">=1.2.3", ">=1.2.3"},
		{"^1.0.0", "^1.0.0"},
		{"~1.2.3", "~1.2.3"},
		{"1.x", "1.x"},
		{"*", "*"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			vr, err := ParseVersionRange(tt.input)
			if err != nil {
				t.Fatalf("ParseVersionRange() error = %v", err)
			}
			if got := vr.String(); got != tt.want {
				t.Errorf("String() = %v, want %v", got, tt.want)
			}
		})
	}
}
