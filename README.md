# FyVault Agent

The eBPF-powered agent daemon, CLI, and installer for [FyVault](https://fyvault.com) — secure, zero-trust secret delivery to Linux servers.

**Related repos:**
- [fyvault-cloud](https://github.com/fybyte/fyvault-cloud) — Cloud API (Node.js)
- [fyvault](https://github.com/fybyte/fyvault) — Frontend (Next.js dashboard)
- [fyvault-node](https://github.com/fybyte/fyvault-node) — Node.js SDK
- [fyvault-python](https://github.com/fybyte/fyvault-python) — Python SDK

## Components

| Binary | Description |
|--------|-------------|
| `fyvaultd` | Agent daemon — syncs secrets, attaches eBPF programs |
| `fyvault` | CLI — manage secrets, devices, orgs from the terminal |
| `fyvault-shim` | Process shim for secret injection |
| `fyvault-health` | Health check binary for monitoring |

## Build

```bash
make build
```

Binaries are output to `bin/`.

## Install (Linux)

```bash
curl -fsSL https://get.fyvault.com | sudo sh
```

Or see `installer/install.sh` for the full installer script.

## Development

```bash
go test ./...
go vet ./...
```

## Release

Tag a version to trigger CI release:

```bash
git tag v0.1.0
git push origin v0.1.0
```

Cross-compiled binaries for linux/darwin (amd64/arm64) are uploaded as GitHub Release assets.

## License

Proprietary. All rights reserved.
