// Package ansname provides version negotiation for ANS resolution.
package ansname

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// VersionRange represents a version constraint for ANS resolution.
// Supports semantic versioning ranges like ">=1.0.0", "^2.1.0", "~1.2.3", "1.x", "*"
type VersionRange struct {
	Original string
	operator string
	major    int
	minor    int
	patch    int
}

var (
	// Version range patterns
	exactPattern      = regexp.MustCompile(`^v?(\d+)\.(\d+)\.(\d+)$`)                 // 1.0.0
	rangePattern      = regexp.MustCompile(`^(>=|>|<=|<|=)\s*v?(\d+)\.(\d+)\.(\d+)$`) // >=1.0.0
	caretPattern      = regexp.MustCompile(`^\^\s*v?(\d+)\.(\d+)\.(\d+)$`)            // ^1.2.3
	tildePattern      = regexp.MustCompile(`^~\s*v?(\d+)\.(\d+)\.(\d+)$`)             // ~1.2.3
	majorPattern      = regexp.MustCompile(`^v?(\d+)\.x$`)                            // 1.x
	majorMinorPattern = regexp.MustCompile(`^v?(\d+)\.(\d+)\.x$`)                     // 1.2.x
)

// ParseVersionRange parses a version range string into a VersionRange.
// Supported formats:
//   - Exact: "1.0.0", "v1.0.0"
//   - Operators: ">=1.0.0", ">1.0.0", "<=2.0.0", "<2.0.0", "=1.0.0"
//   - Caret: "^1.2.3" (allows changes that don't modify left-most non-zero digit)
//   - Tilde: "~1.2.3" (allows patch-level changes)
//   - Wildcards: "1.x" (any minor/patch), "1.2.x" (any patch)
//   - Any: "*" (any version)
func ParseVersionRange(rangeStr string) (*VersionRange, error) {
	rangeStr = strings.TrimSpace(rangeStr)

	if rangeStr == "" || rangeStr == "*" {
		return &VersionRange{
			Original: rangeStr,
			operator: "*",
		}, nil
	}

	// Try exact version
	if matches := exactPattern.FindStringSubmatch(rangeStr); matches != nil {
		major, _ := strconv.Atoi(matches[1])
		minor, _ := strconv.Atoi(matches[2])
		patch, _ := strconv.Atoi(matches[3])
		return &VersionRange{
			Original: rangeStr,
			operator: "=",
			major:    major,
			minor:    minor,
			patch:    patch,
		}, nil
	}

	// Try operator ranges (>=, >, <=, <, =)
	if matches := rangePattern.FindStringSubmatch(rangeStr); matches != nil {
		major, _ := strconv.Atoi(matches[2])
		minor, _ := strconv.Atoi(matches[3])
		patch, _ := strconv.Atoi(matches[4])
		return &VersionRange{
			Original: rangeStr,
			operator: matches[1],
			major:    major,
			minor:    minor,
			patch:    patch,
		}, nil
	}

	// Try caret range (^1.2.3)
	if matches := caretPattern.FindStringSubmatch(rangeStr); matches != nil {
		major, _ := strconv.Atoi(matches[1])
		minor, _ := strconv.Atoi(matches[2])
		patch, _ := strconv.Atoi(matches[3])
		return &VersionRange{
			Original: rangeStr,
			operator: "^",
			major:    major,
			minor:    minor,
			patch:    patch,
		}, nil
	}

	// Try tilde range (~1.2.3)
	if matches := tildePattern.FindStringSubmatch(rangeStr); matches != nil {
		major, _ := strconv.Atoi(matches[1])
		minor, _ := strconv.Atoi(matches[2])
		patch, _ := strconv.Atoi(matches[3])
		return &VersionRange{
			Original: rangeStr,
			operator: "~",
			major:    major,
			minor:    minor,
			patch:    patch,
		}, nil
	}

	// Try major wildcard (1.x)
	if matches := majorPattern.FindStringSubmatch(rangeStr); matches != nil {
		major, _ := strconv.Atoi(matches[1])
		return &VersionRange{
			Original: rangeStr,
			operator: "x",
			major:    major,
			minor:    -1,
			patch:    -1,
		}, nil
	}

	// Try major.minor wildcard (1.2.x)
	if matches := majorMinorPattern.FindStringSubmatch(rangeStr); matches != nil {
		major, _ := strconv.Atoi(matches[1])
		minor, _ := strconv.Atoi(matches[2])
		return &VersionRange{
			Original: rangeStr,
			operator: "x",
			major:    major,
			minor:    minor,
			patch:    -1,
		}, nil
	}

	return nil, fmt.Errorf("invalid version range format: %s", rangeStr)
}

// Matches checks if the given ANSName version satisfies this version range.
//
//nolint:gocyclo // High complexity required for comprehensive version range matching logic
func (vr *VersionRange) Matches(name *ANSName) (bool, error) {
	major, minor, patch, err := name.VersionComponents()
	if err != nil {
		return false, err
	}

	switch vr.operator {
	case "*":
		return true, nil

	case "=":
		return major == vr.major && minor == vr.minor && patch == vr.patch, nil

	case ">":
		if major != vr.major {
			return major > vr.major, nil
		}
		if minor != vr.minor {
			return minor > vr.minor, nil
		}
		return patch > vr.patch, nil

	case ">=":
		if major != vr.major {
			return major > vr.major, nil
		}
		if minor != vr.minor {
			return minor > vr.minor, nil
		}
		return patch >= vr.patch, nil

	case "<":
		if major != vr.major {
			return major < vr.major, nil
		}
		if minor != vr.minor {
			return minor < vr.minor, nil
		}
		return patch < vr.patch, nil

	case "<=":
		if major != vr.major {
			return major < vr.major, nil
		}
		if minor != vr.minor {
			return minor < vr.minor, nil
		}
		return patch <= vr.patch, nil

	case "^": // Caret: allows changes that don't modify left-most non-zero
		if vr.major > 0 {
			// ^1.2.3 := >=1.2.3 <2.0.0
			if major != vr.major {
				return false, nil
			}
			if minor < vr.minor {
				return false, nil
			}
			if minor == vr.minor {
				return patch >= vr.patch, nil
			}
			return true, nil
		} else if vr.minor > 0 {
			// ^0.2.3 := >=0.2.3 <0.3.0
			if major != 0 || minor != vr.minor {
				return false, nil
			}
			return patch >= vr.patch, nil
		} else {
			// ^0.0.3 := >=0.0.3 <0.0.4
			return major == 0 && minor == 0 && patch == vr.patch, nil
		}

	case "~": // Tilde: allows patch-level changes
		// ~1.2.3 := >=1.2.3 <1.3.0
		if major != vr.major || minor != vr.minor {
			return false, nil
		}
		return patch >= vr.patch, nil

	case "x": // Wildcard
		if vr.minor == -1 {
			// 1.x: any minor/patch in major version 1
			return major == vr.major, nil
		}
		// 1.2.x: any patch in version 1.2
		return major == vr.major && minor == vr.minor, nil

	default:
		return false, fmt.Errorf("unknown operator: %s", vr.operator)
	}
}

// NegotiateVersion selects the best matching version from a list of candidates.
// Returns the highest version that satisfies the range, or nil if no match.
// Strategy: prefer the latest stable version that matches the range.
func NegotiateVersion(candidates []*ANSName, rangeStr string) (*ANSName, error) {
	if len(candidates) == 0 {
		return nil, fmt.Errorf("no candidates provided")
	}

	// Parse the version range
	versionRange, err := ParseVersionRange(rangeStr)
	if err != nil {
		return nil, fmt.Errorf("invalid version range: %w", err)
	}

	// Filter candidates that match the range
	var matches []*ANSName
	for _, candidate := range candidates {
		if match, err := versionRange.Matches(candidate); err == nil && match {
			matches = append(matches, candidate)
		}
	}

	if len(matches) == 0 {
		return nil, fmt.Errorf("no version matches range %s", rangeStr)
	}

	// Return the highest matching version
	best := matches[0]
	for _, candidate := range matches[1:] {
		if newer, err := candidate.IsNewerThan(best); err == nil && newer {
			best = candidate
		}
	}

	return best, nil
}

// String returns a string representation of the version range.
func (vr *VersionRange) String() string {
	return vr.Original
}
