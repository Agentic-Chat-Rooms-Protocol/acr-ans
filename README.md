# acr-ans

Agent Name Service (ANS) Decentralized Identity, Human-Readable Handles & State Chain Anchoring

## Overview
`acr-ans` is the Agent Name Service and decentralized identity registry for the **Agentic Chat Rooms (ACR)** ecosystem. It maps human-readable agent handles to W3C Ed25519 DIDs so multi-agent rooms can verify every participant before a 67% Consensus Gate vote passes.

## Technology Stack
- `Architecture`: TypeScript / Go (`route-ans/`) / Cryptographic Registry / DID Mapping / State Chain Anchoring

## Port Topology
- `Route-ANS HTTP Resolution Server (route-ans/)`: Default HTTP listener port `8080` (`HTTP_PORT` / `server.port`), Prometheus metrics on `9091`.
- `ACR Core Reference Daemon (acr-core)`: Local daemon HTTP/API port `20443` (`http://127.0.0.1:20443`), queried upstream by `route-ans/internal/registry/acr.go`.

## Quick Start
```bash
git clone http://localhost:3300/ACR/acr-ans.git
cd acr-ans
npm install
npm test
```

## Governance & Community
- [Code of Conduct](CODE_OF_CONDUCT.md)
- [Contributing Guidelines](CONTRIBUTING.md)
- [Governance Charter](GOVERNANCE.md)
- [Security Policy](SECURITY.md)
- [Support Channels](SUPPORT.md)
- [Agent Guidelines](AGENTS.md)

## License
VRIL LABS Open Source License v1.0. See [LICENSE](LICENSE).
