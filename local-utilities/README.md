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

**✨ Enhanced Feature: Automatic Compact-to-Pretty JSON Conversion**

This script has been enhanced to automatically convert the operator's compact JSON logs into human-readable pretty-printed JSON format. Even though the operator outputs compact JSON (single-line), this script transforms it into indented, formatted JSON for better readability.

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
- `--pretty-json` - Force pretty-print JSON logs (requires `jq`)
- `--no-pretty-json` - Disable JSON pretty-printing (show compact JSON as-is)
- `--no-color` - Disable colored output
- `-h, --help` - Show help message

**Examples:**
```bash
# Follow logs in real-time with automatic compact-to-pretty JSON conversion (default)
# The operator outputs compact JSON, but this script converts it to readable format
./local-utilities/monitor-operator-logs.sh

# Show logs from last 5 minutes (with pretty JSON conversion)
./local-utilities/monitor-operator-logs.sh --since 5m

# Show last 50 lines and exit (no follow) - logs are still converted to pretty JSON
./local-utilities/monitor-operator-logs.sh --tail 50 --no-follow

# Filter for specific patterns (e.g., reconcile, GroupConfig, error)
# Pretty JSON conversion still applies to filtered results
./local-utilities/monitor-operator-logs.sh -g 'reconcile'
./local-utilities/monitor-operator-logs.sh -g 'GroupConfig'
./local-utilities/monitor-operator-logs.sh -g 'error'

# Monitor errors in custom namespace
./local-utilities/monitor-operator-logs.sh -n my-namespace -g 'error'

# Force pretty-print JSON logs (auto-detection is default, but this ensures it)
./local-utilities/monitor-operator-logs.sh --pretty-json

# Disable JSON pretty-printing (show compact JSON as-is from operator)
./local-utilities/monitor-operator-logs.sh --no-pretty-json
```

**Usage Tips:**
1. **Follow logs in real-time** - The default behavior follows logs as they're generated, with automatic JSON pretty-printing
2. **Show specific number of lines** - Use `--tail <number>` with `--no-follow` to see a snapshot
3. **Filter logs** - Use `-g` or `--grep` to filter for specific patterns (controller names, log levels, etc.)
4. **Pretty-printing is automatic** - JSON logs are automatically detected and formatted by default (requires `jq`)
5. **Disable pretty-printing** - Use `--no-pretty-json` if you prefer raw compact JSON output

**Features:**
- Automatic pod discovery using label selectors
- **✨ Compact-to-Pretty JSON Conversion** - Automatically converts operator's compact JSON logs to readable pretty-printed format
- **JSON pretty-printing** - Automatically detects and pretty-prints JSON log lines in real-time (requires `jq`)
- Color-coded log levels (ERROR=red, WARN=yellow, INFO=green, DEBUG=blue)
- Highlights key terms (reconciling, NamespaceConfig, GroupConfig, UserConfig)
- Authentication check before executing
- Graceful error handling

**JSON Pretty-Printing Enhancement:**
- **Key Feature**: The operator outputs compact JSON (single-line format), but this script automatically converts it to indented, human-readable pretty JSON
- By default, the script auto-detects JSON log lines and pretty-prints them using `jq`
- This transformation makes the structured JSON logs from the operator much more readable and easier to debug
- The conversion happens in real-time as logs are streamed from the operator pod
- Requires `jq` to be installed: `brew install jq` (macOS) or `apt-get install jq` (Linux)
- Use `--pretty-json` to force pretty-printing, or `--no-pretty-json` to disable and see compact JSON as-is
- **Example transformation:**
  ```json
  // Compact JSON (from operator):
  {"level":"info","ts":"2025-12-09T18:35:14-06:00","logger":"setup","msg":"starting manager"}
  
  // Pretty JSON (after script enhancement):
  {
    "level": "info",
    "ts": "2025-12-09T18:35:14-06:00",
    "logger": "setup",
    "msg": "starting manager"
  }
  ```

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
