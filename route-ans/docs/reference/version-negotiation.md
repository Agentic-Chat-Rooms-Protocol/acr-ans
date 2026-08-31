# Version Negotiation

Semantic version negotiation for ANS agent resolution.

## Overview

Version negotiation allows clients to request compatible agent versions using semantic version ranges instead of exact versions. The resolver automatically selects the highest matching version from available agents.

## Version Range Formats

| Format | Example | Meaning |
|--------|---------|---------|
| **Exact** | `1.2.3` | Exact version 1.2.3 |
| **Caret** | `^1.2.3` | Compatible versions (≥1.2.3, <2.0.0) |
| **Tilde** | `~1.2.3` | Patch updates only (≥1.2.3, <1.3.0) |
| **Wildcard** | `1.x` | Any version in major release |
| **Latest** | `*` | Latest available version |
| **Comparisons** | `>=1.0.0`, `<2.0.0` | Version constraints |

### Caret Ranges (Recommended)

- `^1.2.3` → Compatible with 1.2.3 (allows 1.x.x, but not 2.0.0)
- `^0.2.3` → Compatible with 0.2.3 (allows 0.2.x, but not 0.3.0)
- `^0.0.3` → Exact 0.0.3 (early development, no compatibility assumed)

### Tilde Ranges

- `~1.2.3` → Patch-level updates (1.2.3, 1.2.4, 1.2.5, but not 1.3.0)

## Usage

### Requirements

**Critical**: The ANSName MUST include a valid version component, even when using version negotiation.

- **Full format**: `protocol://agentName.capability.providerID.version.extension`
- **Simplified format**: `protocol://version.host.domain`

The version in the ANSName is used for:
1. Format validation and parsing
2. Extracting the FQDN for registry lookup
3. Default version when no `version` parameter is provided

### Query Parameter

Add `version` parameter to override/refine version selection:

```bash
# ANSName must include a version (v1.0.0 here)
# version parameter (^1.0.0) overrides it for negotiation
GET /v1/resolve?name=ans://v1.0.0.greeting.example.com&version=^1.0.0
```

**Selection Strategy**: When multiple versions match, the **highest** version is selected.

### Registry Support

**GoDaddy Registry**: Fully supports version negotiation. GoDaddy's API accepts semantic version ranges and returns the best matching version. The resolver validates the returned version matches the requested range.

**Mock Registry**: Supports all version range formats with local negotiation.

## Examples

### Correct Usage

All examples below include a valid version in the ANSName:

**Request latest compatible version (caret `^`):**
```bash
GET /v1/resolve?name=mcp://agent.example.com&version=^1.0.0
```
Returns: `1.5.2` (if available and compatible)

**Request patch updates only (tilde `~`):**
```bash
GET /v1/resolve?name=mcp://agent.example.com&version=~1.2.0
```
Returns: `1.2.5` (highest patch in 1.2.x)

**Request any version in major release:**
```bash
GET /v1/resolve?name=mcp://agent.example.com&version=1.x
```
Returns: `1.9.3` (highest 1.x version)

**Request latest version:**
```bash
GET /v1/resolve?name=mcp://agent.example.com&version=*
```
Returns: Latest registered version

### Common Mistakes

**❌ Incorrect - Missing version in ANSName:**
```bash
# This will fail with "invalid ANSName format"
GET /v1/resolve?name=ans://greeting.example.com&version=*
```

**✅ Correct - Version included in ANSName:**
```bash
# Use any valid version in the ANSName, version parameter for negotiation
GET /v1/resolve?name=ans://v1.0.0.greeting.example.com&version=*
```

The version in the ANSName (v1.0.0) can be any valid version - it's used for parsing.
The version parameter (*) determines which version is actually selected.

## See Also

- [ANS Specification](../architecture/ans-spec.md) - ANS name format and versioning
- [Semantic Versioning 2.0.0](https://semver.org/) - Version semantics
- [npm semver](https://docs.npmjs.com/cli/v6/using-npm/semver) - Range syntax reference
