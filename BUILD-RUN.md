# Build and Run Scripts

This document describes the build and run scripts for the namespace-configuration-operator.

## Quick Start

```bash
# Build the operator
./build.sh -o bin/manager main.go

# Build and run the operator
./run-go.sh
```

---

## Build Script (`build.sh`)

Automatically sets version information (VERSION, COMMIT, BUILD_DATE) when building, eliminating the need to manually specify ldflags.

### Usage

```bash
./build.sh -o bin/manager main.go
```

### Automatic Version Detection

The script automatically sets:
- **VERSION**: From `git describe --tags --always --dirty`
- **COMMIT**: From `git rev-parse --short HEAD`
- **BUILD_DATE**: From current UTC timestamp

### Examples

#### Basic Build
```bash
./build.sh -o bin/manager main.go
```

#### Build with Race Detector
```bash
./build.sh -race -o bin/manager main.go
```

#### Override Version
```bash
VERSION=1.0.0 ./build.sh -o bin/manager main.go
```

#### Override All Parameters
```bash
VERSION=2.0.0 COMMIT=abc123 BUILD_DATE=2025-01-01T00:00:00Z ./build.sh -o bin/manager main.go
```

#### Build with Tags
```bash
./build.sh -tags debug -o bin/manager main.go
```

#### Additional Go Build Flags
```bash
./build.sh -ldflags "-s -w" -o bin/manager main.go
```

### Environment Variables

Override any parameter via environment variables:
- `VERSION`: Override version string
- `COMMIT`: Override commit hash
- `BUILD_DATE`: Override build date (ISO 8601 format)

---

## Run Script (`run-go.sh`)

Simple script to build and run the operator locally with proper log configuration.

### Usage

```bash
# Automatic build and run
./run-go.sh

# Skip build if already built manually
./run-go.sh --skip-build

# Stop running operator
./run-go.sh --stop

# Development mode (console logs)
./run-go.sh --dev

# Custom log level
./run-go.sh --log-level debug

# Development mode with debug logs
./run-go.sh --dev --log-level 2

# See help
./run-go.sh --help
```

### Options

- `--log-level <level>`: Set log level (error, info, debug, 0-10) [default: info]
- `--dev`: Enable development mode (console logs) [default: false]
- `--skip-build`: Skip the build step (use existing binary)
- `--stop`: Stop the running operator and exit
- `--help`: Show help message

### Auto-Stop Feature

The script automatically stops any running operator before starting a new one to prevent multiple instances:

```bash
./run-go.sh  # Will stop existing operator first if running
```

### Environment Variables

Override log configuration via environment variables:

```bash
ZAP_LOG_LEVEL=debug ZAP_DEVEL=true ./run-go.sh
```

### Test Cases

All options have been tested and verified:

#### Command-Line Options

1. **`--help`**: Shows help message with build.sh reference
   ```bash
   ./run-go.sh --help
   ```

2. **`--log-level error`**: Sets log level to error
   ```bash
   ./run-go.sh --skip-build --log-level error
   ```

3. **`--log-level info`**: Sets log level to info (default)
   ```bash
   ./run-go.sh --skip-build --log-level info
   ```

4. **`--log-level debug`**: Sets log level to debug
   ```bash
   ./run-go.sh --skip-build --log-level debug
   ```

5. **`--log-level 2`**: Sets numeric log level (verbosity level 2)
   ```bash
   ./run-go.sh --skip-build --log-level 2
   ```

6. **`--dev`**: Enables development mode (console logs)
   ```bash
   ./run-go.sh --skip-build --dev
   ```

7. **`--skip-build`**: Skips build when binary exists
   ```bash
   ./run-go.sh --skip-build
   ```

8. **`--skip-build` (missing binary)**: Automatically builds if binary is missing
   ```bash
   rm bin/manager
   ./run-go.sh --skip-build  # Automatically builds using build.sh if binary missing
   ```

9. **`--stop`**: Stops running operator
   ```bash
   ./run-go.sh --stop
   ```

10. **Auto-stop**: Automatically stops existing operator before starting
    ```bash
    ./run-go.sh  # Stops existing operator first if running
    ```

11. **Combinations**: Multiple flags work together
    ```bash
    ./run-go.sh --skip-build --dev --log-level debug
    ```

12. **Invalid option**: Correctly detects and shows error
    ```bash
    ./run-go.sh --invalid-option  # Shows error message
    ```

#### Environment Variables

13. **`ZAP_LOG_LEVEL` override**: Environment variable takes precedence
    ```bash
    ZAP_LOG_LEVEL=error ./run-go.sh --skip-build
    ```

14. **`ZAP_DEVEL` override**: Environment variable works
    ```bash
    ZAP_DEVEL=true ./run-go.sh --skip-build
    ```

#### Default Behavior

15. **Default run**: Automatically builds using build.sh and runs
    ```bash
    ./run-go.sh  # Builds and runs with default settings
    ```

---

## Integration

The `run-go.sh` script automatically calls `build.sh` when needed:

- **Default behavior**: Calls `build.sh` automatically if binary doesn't exist
- **With `--skip-build`**: Skips build if binary exists, auto-builds if missing
- **Version info**: All version information from `build.sh` is correctly embedded
- **Environment overrides**: Build parameters can be overridden via environment variables

### Example Flow

```bash
# First run: builds automatically
./run-go.sh
# → Calls build.sh
# → Sets version info
# → Runs operator

# Subsequent runs: can skip build
./run-go.sh --skip-build
# → Uses existing binary
# → Runs operator

# Missing binary: auto-builds even with --skip-build
rm bin/manager
./run-go.sh --skip-build
# → Detects missing binary
# → Automatically calls build.sh
# → Runs operator
```

