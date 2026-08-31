# Usage Examples

Common use cases and code examples for Route ANS Resolver.

## ANSName Format Requirements

**Important**: All requests must include a **valid version** in the ANSName, even when using version negotiation.

- **Full format**: `protocol://agentName.capability.providerID.version.extension`
- **Simplified format**: `protocol://version.host.domain`

The version in the ANSName is used for parsing and FQDN extraction. Use the `version` query parameter to override version selection.

## Basic Resolution

### Full Format (Mock Registry)

```bash
# Exact version
curl "http://localhost:8080/v1/resolve?name=mcp://chatbot.conversation.PID-5678.v1.2.3.example.com"
```

### Simplified Format (GoDaddy Registry)

```bash
# Exact version
curl "http://localhost:8080/v1/resolve?name=ans://v1.0.0.greeting.example.com"

# Latest version
curl "http://localhost:8080/v1/resolve?name=ans://v1.0.0.greeting.example.com&version=*"
```

## Version Negotiation

### With Full Format

```bash
# Any 1.x version
curl "http://localhost:8080/v1/resolve?name=mcp://chatbot.conversation.PID-5678.v1.0.0.example.com&version=1.x"

# Compatible with 1.2.0
curl "http://localhost:8080/v1/resolve?name=mcp://chatbot.conversation.PID-5678.v1.2.0.example.com&version=^1.2.0"

# Patch updates only
curl "http://localhost:8080/v1/resolve?name=mcp://chatbot.conversation.PID-5678.v1.2.3.example.com&version=~1.2.3"
```

### With Simplified Format (GoDaddy)

```bash
# Compatible versions
curl "http://localhost:8080/v1/resolve?name=ans://v1.0.0.greeting.example.com&version=^1.0.0"

# Greater than or equal
curl "http://localhost:8080/v1/resolve?name=ans://v1.0.0.greeting.example.com&version=>=1.0.0"

# Latest version
curl "http://localhost:8080/v1/resolve?name=ans://v1.0.0.greeting.example.com&version=*"
```

## Batch Resolution

```bash
curl -X POST http://localhost:8080/v1/resolve/batch \
  -H "Content-Type: application/json" \
  -d '{
    "names": [
      "mcp://agent1.capability.PID-123.v1.0.0.example.com",
      "a2a://agent2.capability.PID-456.v2.0.0.example.com"
    ]
  }'
```

## Next Steps

- **[Version Negotiation](reference/version-negotiation.md)** - Version range details
- **[Architecture](architecture/overview.md)** - System design
