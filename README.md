# FyVault Agent

Cross-platform secrets agent with eBPF kernel injection (Linux), Keychain integration (macOS), encrypted file store (Windows), HTTP/DB proxy, and a 30+ command CLI.

## Components

| Binary | Purpose |
|--------|---------|
| `fyvaultd` | Daemon — boots secrets, starts proxies, attaches eBPF, runs heartbeat/sync |
| `fyvault` | CLI — secrets, environments, sync, scan, import/export, rotate, run |
| `fyvault-shim` | AWS `credential_process` helper — serves AWS keys from keyring |
| `fyvault-health` | Health checker — queries daemon status via Unix socket / TCP |

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                        fyvaultd daemon                          │
├──────────┬──────────┬──────────┬──────────┬──────────┬──────────┤
│  Cloud   │  Keyring │  eBPF    │  HTTP    │  DB      │  Health  │
│  Client  │          │  Loader  │  Proxy   │  Proxy   │  Server  │
│  (mTLS)  │          │ (Linux)  │          │ (Pg v3)  │          │
├──────────┼──────────┼──────────┼──────────┼──────────┼──────────┤
│ Boot     │ Linux:   │ TC egress│ Header   │ Password │ Unix     │
│ Heartbeat│  keyctl  │ redirect │ injection│ injection│ socket / │
│ Sync     │ macOS:   │ to local │ with     │ via wire │ TCP      │
│          │  Keychain│ proxy    │ templates│ protocol │          │
│          │ Windows: │          │          │          │          │
│          │  AES file│          │          │          │          │
└──────────┴──────────┴──────────┴──────────┴──────────┴──────────┘
```

## Platform Support

| Feature | Linux | macOS | Windows |
|---------|-------|-------|---------|
| Kernel keyring | keyctl syscalls | Keychain | AES-256-GCM encrypted file |
| eBPF redirect | TC egress classifier | — (proxy only) | — (proxy only) |
| HTTP proxy | ✅ | ✅ | ✅ |
| DB proxy (PostgreSQL) | ✅ | ✅ | ✅ |
| AWS credential shim | ✅ | ✅ | ✅ |
| Service management | systemd | launchd | sc.exe |
| Health check | Unix socket | Unix socket | TCP |

## Install

```bash
curl -fsSL https://get.fyvault.com | bash
```

## Build

```bash
cd agent

# Build for current platform
make build

# Cross-compile all platforms
make build-all

# Build with eBPF (Linux only, requires clang + kernel headers)
make build-linux-full
```

### eBPF Compilation

The eBPF TC classifier requires compilation on a Linux machine:

```bash
# x86_64
make ebpf

# ARM64
make ebpf-arm64
```

Outputs `ebpf/tc_redirect.o` which the daemon loads at startup. The daemon searches:
1. `ebpf/tc_redirect.o` (development)
2. `/usr/lib/fyvault/tc_redirect.o` (package install)
3. `/opt/fyvault/ebpf/tc_redirect.o` (Docker/manual)

## CLI Commands

### Authentication
```bash
fyvault login              # Interactive login with 2FA support
fyvault logout             # Remove stored credentials
fyvault whoami             # Show current user and org
```

### Organizations
```bash
fyvault orgs               # List organizations
fyvault orgs:create <name> # Create organization
fyvault use <org-id>       # Set active organization
```

### Secrets
```bash
fyvault secrets                    # List secrets
fyvault secrets:create             # Create (interactive)
fyvault secrets:get <name>         # Get metadata
fyvault secrets:set <name> <value> # Update value
fyvault secrets:delete <name>      # Delete
fyvault secrets:versions <name>    # List versions
fyvault secrets:rotate <name>      # Rotate (auto-generate new value)
```

### Environments
```bash
fyvault envs                       # List environments
fyvault envs:create <name>         # Create environment
fyvault envs:pull <env>            # Pull all secrets (KEY=VALUE output)
```

### Run with Secrets
```bash
fyvault run --env=dev -- npm start           # Inject as env vars
fyvault run --env=production -- python app.py # Any command
eval $(fyvault envs:pull staging)            # Shell export
```

### Import / Export
```bash
fyvault import --env=dev --file=.env                    # Import .env
fyvault import --env=dev --format=json --file=secrets.json
fyvault export --env=production --format=env > .env.prod
fyvault export --env=staging --format=json
fyvault export --env=dev --format=yaml
```

### Platform Sync
```bash
fyvault sync vercel --env=prod --token=$TOKEN --project-id=prj_xxx
fyvault sync heroku --env=prod --token=$TOKEN --app=my-app
fyvault sync netlify --env=prod --token=$TOKEN --service-id=site_xxx
fyvault sync railway --env=prod --token=$TOKEN --project-id=prj_xxx
fyvault sync fly --env=prod --token=$TOKEN --app=my-app
fyvault sync render --env=prod --token=$TOKEN --service-id=srv_xxx
```

### Config Generation
```bash
fyvault generate k8s --env=prod --name=app-secrets       # Kubernetes Secret
fyvault generate docker --env=prod > .env                 # Docker .env
fyvault generate docker-compose --env=prod --service=api  # Compose snippet
fyvault generate terraform --env=prod > secrets.tfvars    # Terraform
fyvault generate ansible --env=prod > vars.yml            # Ansible
fyvault generate pulumi --env=prod --name=stack           # Pulumi
fyvault generate gitlab-ci --env=prod                     # GitLab CI
fyvault generate circleci --env=prod                      # CircleCI
```

### Security Scanning
```bash
fyvault scan --file=.env.backup                    # Scan a file
fyvault scan --text="AKIA..."                      # Scan text
cat logs.txt | fyvault scan --stdin                # Scan from pipe
```

### Devices
```bash
fyvault devices                                    # List devices
fyvault devices:register --name=prod-1             # Register
fyvault devices:assign <device> <secret>           # Assign secret
```

### Agent Management
```bash
fyvault doctor                # Check system requirements
fyvault agent:status          # Check daemon health
fyvault agent:logs            # View daemon logs
fyvault agent:restart         # Restart daemon
```

### Global Flags

```bash
--env <name>        # Environment (dev/staging/production)
--org <org-id>      # Override organization
--api-url <url>     # Override API URL
--format <table|json> # Output format
```

## Daemon (fyvaultd)

The daemon runs as a system service and handles the full secret lifecycle:

1. **Boot** — Authenticates via mTLS device cert, fetches assigned secrets from cloud
2. **Keyring** — Stores secrets in platform-native secure storage
3. **Proxy** — Starts HTTP and DB proxies on localhost that inject credentials
4. **eBPF** (Linux) — Attaches TC classifier to redirect outbound traffic through proxy
5. **Sync** — Heartbeat every 5 min, syncs updated secrets automatically
6. **Health** — Exposes health endpoint for monitoring

```bash
# Start manually (usually managed by systemd/launchd)
sudo fyvaultd --config /etc/fyvault/fyvault.conf

# Check health
fyvault-health
```

## Development

```bash
# Run tests
make test

# Build all platforms
make build-all

# Clean
make clean
```

## Links

- [FyVault](https://fyvault.com)
- [Documentation](https://fyvault.com/docs)
- [CLI Reference](https://fyvault.com/cli)
- [Node.js SDK](https://github.com/FybyteTech/fyvault-sdk-node)

## License

Proprietary. Copyright 2026 Fybyte.
