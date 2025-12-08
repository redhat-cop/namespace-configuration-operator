# Local Utilities

Helper scripts for developing, debugging, and managing the namespace-configuration-operator.

## Scripts

### `create-dockerhub-secret.sh`

Simple utility to create the Docker Hub registry secret required by Kyverno policies.

**Usage:**
```bash
# Interactive mode (prompts for credentials, run from repository root)
./local-utilities/create-dockerhub-secret.sh

# With environment variables
DOCKERHUB_USERNAME=your-username \
DOCKERHUB_PASSWORD=your-password \
DOCKERHUB_EMAIL=your-email@example.com \
./local-utilities/create-dockerhub-secret.sh
```

**What it does:**
- Creates namespace if it doesn't exist
- Replaces existing secret if found
- Creates `dockerhub-secret` in `namespace-configuration-operator` namespace

**Related Documentation:**
- See `kyverno-policies/README.md` for information about Kyverno policies that use this secret

---

### `generate-policies.sh`

Generate Kyverno policies from templates using envsubst. Processes all `env-*.yaml.tpl` files in the `kyverno-policies` directory.

**Usage:**
```bash
# Set your Docker Hub username (required)
export DOCKERHUB_USERNAME=your-username

# Optional: Set log level configuration
export ZAP_LOG_LEVEL=info
export ZAP_DEVEL=false

# Generate all policies (run from repository root)
./local-utilities/generate-policies.sh

# Or pass username as argument
./local-utilities/generate-policies.sh your-username
```

**What it does:**
1. Reads all `env-*.yaml.tpl` files from `kyverno-policies/` directory
2. Replaces environment variable placeholders:
   - `${DOCKERHUB_USERNAME}` - Docker Hub username (required)
   - `${ZAP_LOG_LEVEL}` - Log level (optional, defaults from template)
   - `${ZAP_DEVEL}` - Development mode (optional, defaults from template)
3. Generates corresponding `.yaml` files (without `env-` prefix and `.tpl` extension)

**Related Documentation:**
- See `kyverno-policies/README-TEMPLATES.md` for detailed template usage instructions

---

### `monitor-operator-logs.sh`

Monitor namespace-configuration-operator logs with filtering and formatting.

**Usage:**
```bash
./local-utilities/monitor-operator-logs.sh [OPTIONS]
```

**Options:**
- `-n, --namespace <namespace>` - Operator namespace (default: namespace-configuration-operator)
- `-f, --follow` - Follow logs in real-time (default: true)
- `--no-follow` - Don't follow logs, just show and exit
- `--since <duration>` - Show logs since duration (e.g., 5m, 1h, 2d)
- `--tail <lines>` - Number of lines to show from end (default: 100)
- `-g, --grep <pattern>` - Filter logs by pattern
- `--no-color` - Disable colored output
- `-h, --help` - Show help message

**Examples:**
```bash
# Follow logs with defaults (last 100 lines)
./local-utilities/monitor-operator-logs.sh

# Show logs from last 5 minutes
./local-utilities/monitor-operator-logs.sh --since 5m

# Filter for GroupConfig related logs
./local-utilities/monitor-operator-logs.sh -g 'GroupConfig'

# Show last 50 lines and exit (no follow)
./local-utilities/monitor-operator-logs.sh --tail 50 --no-follow

# Monitor errors in custom namespace
./local-utilities/monitor-operator-logs.sh -n my-namespace --grep 'error'
```

**Features:**
- Automatic pod discovery using label selectors
- Color-coded log levels (ERROR=red, WARN=yellow, INFO=green, DEBUG=blue)
- Highlights key terms (reconciling, NamespaceConfig, GroupConfig, UserConfig)
- Authentication check before executing
- Graceful error handling

**Prerequisites:**
- Authenticated to OpenShift cluster (`oc login`)
- namespace-configuration-operator deployed and running

---

## Quick Reference

| Script | Purpose | Location |
|--------|---------|----------|
| `create-dockerhub-secret.sh` | Create Docker Hub registry secret | `local-utilities/` |
| `generate-policies.sh` | Generate Kyverno policies from templates | `local-utilities/` |
| `monitor-operator-logs.sh` | Monitor operator logs | `local-utilities/` |

---

## Contributing

When adding new scripts:
1. Make scripts executable: `chmod +x local-utilities/your-script.sh`
2. Add shebang: `#!/bin/bash`
3. Include usage documentation in script comments
4. Update this README with script description and usage
5. Add error handling and validation
6. Support both `oc` and `kubectl` commands when possible
