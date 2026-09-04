#!/bin/bash

# Script to monitor namespace-configuration-operator logs
# Usage: ./monitor-operator-logs.sh [OPTIONS]
#
# Options:
#   -n, --namespace <namespace>  Operator namespace (default: namespace-configuration-operator)
#   -f, --follow                 Follow logs in real-time (default: true)
#   --since <duration>           Show logs since duration (e.g., 5m, 1h, 2d)
#   --tail <lines>               Number of lines to show from end (default: 100)
#   -g, --grep <pattern>         Filter logs by pattern
#   --pretty-json                Pretty-print JSON logs (default: auto-detect)
#   --no-pretty-json             Don't pretty-print JSON logs
#   --no-color                   Disable colored output
#   -h, --help                   Show this help message

set -euo pipefail

# Default values
NAMESPACE="namespace-configuration-operator"
FOLLOW=true
SINCE=""
TAIL=100
GREP_PATTERN=""
USE_COLOR=true
SHOW_HELP=false
PRETTY_JSON="auto"  # auto, true, false

# Colors
if [[ -t 1 ]]; then
    RED='\033[0;31m'
    GREEN='\033[0;32m'
    YELLOW='\033[0;33m'
    BLUE='\033[0;34m'
    MAGENTA='\033[0;35m'
    CYAN='\033[0;36m'
    NC='\033[0m' # No Color
else
    RED=''
    GREEN=''
    YELLOW=''
    BLUE=''
    MAGENTA=''
    CYAN=''
    NC=''
fi

# Parse arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        -n|--namespace)
            if [[ $# -lt 2 || -z "$2" ]]; then
                echo -e "${RED}Error: --namespace requires an argument.${NC}"
                exit 1
            fi
            NAMESPACE="$2"
            shift 2
            ;;
        -f|--follow)
            FOLLOW=true
            shift
            ;;
        --no-follow)
            FOLLOW=false
            shift
            ;;
        --since)
            if [[ $# -lt 2 || -z "$2" ]]; then
                echo -e "${RED}Error: --since requires an argument.${NC}"
                exit 1
            fi
            SINCE="$2"
            shift 2
            ;;
        --tail)
            if [[ $# -ge 2 && ! "$2" =~ ^- ]]; then
                TAIL="$2"
                shift 2
            else
                # TAIL already defaults to 100, so no assignment needed.
                shift 1
            fi
            ;;
        -g|--grep)
            if [[ $# -lt 2 || -z "$2" ]]; then
                echo -e "${RED}Error: --grep requires an argument.${NC}"
                exit 1
            fi
            GREP_PATTERN="$2"
            shift 2
            ;;
        --pretty-json)
            PRETTY_JSON="true"
            shift
            ;;
        --no-pretty-json)
            PRETTY_JSON="false"
            shift
            ;;
        --no-color)
            USE_COLOR=false
            RED=''
            GREEN=''
            YELLOW=''
            BLUE=''
            MAGENTA=''
            CYAN=''
            NC=''
            shift
            ;;
        -h|--help)
            SHOW_HELP=true
            shift
            ;;
        *)
            echo -e "${RED}Unknown option: $1${NC}"
            SHOW_HELP=true
            shift
            ;;
    esac
done

# Show help if requested
if [ "$SHOW_HELP" = true ]; then
    echo "Usage: $0 [OPTIONS]"
    echo ""
    echo "Monitor namespace-configuration-operator logs with filtering and formatting."
    echo ""
    echo "Options:"
    echo "  -n, --namespace <namespace>  Operator namespace (default: namespace-configuration-operator)"
    echo "  -f, --follow                 Follow logs in real-time (default: true)"
    echo "  --no-follow                  Don't follow logs, just show and exit"
    echo "  --since <duration>           Show logs since duration (e.g., 5m, 1h, 2d)"
    echo "  --tail <lines>               Number of lines to show from end (default: 100)"
    echo "  -g, --grep <pattern>         Filter logs by pattern"
    echo "  --pretty-json                Pretty-print JSON logs (default: auto-detect)"
    echo "  --no-pretty-json             Don't pretty-print JSON logs"
    echo "  --no-color                   Disable colored output"
    echo "  -h, --help                   Show this help message"
    echo ""
    echo "Examples:"
    echo "  $0                                    # Follow logs with defaults"
    echo "  $0 --since 5m                         # Show logs from last 5 minutes"
    echo "  $0 -g 'GroupConfig'                   # Filter for GroupConfig logs"
    echo "  $0 --tail 50 --no-follow              # Show last 50 lines and exit"
    echo "  $0 -n my-namespace --grep 'error'     # Monitor errors in custom namespace"
    echo "  $0 --pretty-json                      # Force pretty-print JSON logs"
    echo "  $0 --no-pretty-json                   # Disable JSON pretty-printing"
    exit 0
fi

# Function to check if oc is authenticated
check_oc_auth() {
    if ! oc whoami &>/dev/null; then
        echo -e "${RED}❌ Not authenticated to OpenShift cluster${NC}"
        echo -e "${YELLOW}Please run: oc login${NC}"
        exit 1
    fi
}

# Function to find operator pod
find_operator_pod() {
    local pod
    # Try multiple label selectors
    pod=$(oc get pods -n "$NAMESPACE" \
        -l control-plane=namespace-configuration-operator \
        -o jsonpath='{.items[0].metadata.name}' 2>/dev/null)
    
    if [ -z "$pod" ]; then
        # Fallback to generic controller-manager label
        pod=$(oc get pods -n "$NAMESPACE" \
            -l control-plane=controller-manager \
            -o jsonpath='{.items[0].metadata.name}' 2>/dev/null)
    fi
    
    if [ -z "$pod" ]; then
        echo -e "${RED}❌ No operator pod found in namespace: $NAMESPACE${NC}"
        echo -e "${YELLOW}Looking for pods with labels: control-plane=namespace-configuration-operator or control-plane=controller-manager${NC}"
        echo ""
        echo -e "${CYAN}Available pods in namespace:${NC}"
        oc get pods -n "$NAMESPACE" 2>/dev/null || echo "Namespace not found"
        exit 1
    fi
    
    echo "$pod"
}

# Function to check if jq is available
check_jq() {
    if ! command -v jq &> /dev/null; then
        return 1
    fi
    return 0
}

# Function to detect if a line is JSON
is_json_line() {
    local line="$1"
    # Check if line starts with { and ends with } (basic JSON detection)
    if [[ "$line" =~ ^\{.*\}$ ]]; then
        return 0
    fi
    return 1
}

# Function to pretty-print JSON logs
pretty_print_json() {
    if [ "$PRETTY_JSON" = "false" ] || ! check_jq; then
        # Don't pretty-print or jq not available, just pass through
        cat
        return
    fi
    
    local line
    while IFS= read -r line || [ -n "$line" ]; do
        # Skip empty lines
        if [ -z "$line" ]; then
            echo
            continue
        fi
        
        # Determine if we should try to pretty-print this line
        local should_pretty=false
        if [ "$PRETTY_JSON" = "true" ]; then
            should_pretty=true
        elif [ "$PRETTY_JSON" = "auto" ] && is_json_line "$line"; then
            should_pretty=true
        fi
        
        if [ "$should_pretty" = true ]; then
            # Try to pretty-print with jq (with color support)
            # jq -C enables color output, . pretty-prints
            if echo "$line" | jq -C '.' 2>/dev/null; then
                # Successfully pretty-printed JSON
                continue
            else
                # Not valid JSON or jq failed, print as-is
                echo "$line"
            fi
        else
            # Not JSON or auto-detect said no, print as-is
            echo "$line"
        fi
    done
}

# Function to colorize log lines
colorize_logs() {
    if [ "$USE_COLOR" = false ]; then
        cat
    else
        sed -e "s/\(ERROR\|Error\|error\)/${RED}&${NC}/g" \
            -e "s/\(WARN\|Warning\|warning\)/${YELLOW}&${NC}/g" \
            -e "s/\(INFO\|Info\)/${GREEN}&${NC}/g" \
            -e "s/\(DEBUG\|Debug\)/${BLUE}&${NC}/g" \
            -e "s/\(reconciling\|Reconciling\)/${CYAN}&${NC}/g" \
            -e "s/\(NamespaceConfig\)/${MAGENTA}&${NC}/g" \
            -e "s/\(GroupConfig\)/${MAGENTA}&${NC}/g" \
            -e "s/\(UserConfig\)/${MAGENTA}&${NC}/g"
    fi
}

# Main execution
echo -e "${CYAN}🔍 Namespace Configuration Operator Log Monitor${NC}"
echo -e "${CYAN}================================================${NC}"
echo ""

# Check authentication
check_oc_auth

echo -e "${GREEN}✅ Authenticated as: $(oc whoami)${NC}"
echo -e "${GREEN}✅ Cluster: $(oc whoami --show-server)${NC}"
echo ""

# Find operator pod
echo -e "${BLUE}🔎 Finding operator pod in namespace: $NAMESPACE${NC}"
POD_NAME=$(find_operator_pod)
echo -e "${GREEN}✅ Found pod: $POD_NAME${NC}"
echo ""

# Build log command
LOG_CMD="oc logs -n $NAMESPACE $POD_NAME"

if [ "$FOLLOW" = true ]; then
    LOG_CMD="$LOG_CMD -f"
fi

if [ -n "$SINCE" ]; then
    LOG_CMD="$LOG_CMD --since=$SINCE"
else
    LOG_CMD="$LOG_CMD --tail=$TAIL"
fi

# Show command being executed
echo -e "${BLUE}📋 Executing: $LOG_CMD${NC}"
if [ -n "$GREP_PATTERN" ]; then
    echo -e "${BLUE}🔍 Filtering for pattern: $GREP_PATTERN${NC}"
fi
if [ "$PRETTY_JSON" != "false" ]; then
    if check_jq; then
        if [ "$PRETTY_JSON" = "true" ]; then
            echo -e "${BLUE}✨ Pretty-printing JSON logs (forced)${NC}"
        else
            echo -e "${BLUE}✨ Pretty-printing JSON logs (auto-detect)${NC}"
        fi
    else
        echo -e "${YELLOW}⚠️  jq not found - JSON pretty-printing disabled. Install jq for better log formatting.${NC}"
        echo -e "${YELLOW}   Install: brew install jq (macOS) or apt-get install jq (Linux)${NC}"
    fi
fi
echo ""
echo -e "${CYAN}================================================${NC}"
echo ""

# Execute logs command with optional grep, JSON pretty-printing, and colorization
if [ -n "$GREP_PATTERN" ]; then
    if [ "$PRETTY_JSON" != "false" ] && check_jq; then
        eval "$LOG_CMD" | grep --line-buffered "$GREP_PATTERN" | pretty_print_json | colorize_logs
    else
    eval "$LOG_CMD" | grep --line-buffered "$GREP_PATTERN" | colorize_logs
    fi
else
    if [ "$PRETTY_JSON" != "false" ] && check_jq; then
        eval "$LOG_CMD" | pretty_print_json | colorize_logs
else
    eval "$LOG_CMD" | colorize_logs
    fi
fi
