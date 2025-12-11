# Resolved Issues Tracker - Namespace Configuration Operator

**Last Updated:** December 10, 2025  
**Status:** Major improvements implemented and tested ✅

> **Note**: This document tracks resolved issues, completed features, and improvements. For active work or pending items, see the main project documentation.

## Current Status

### Recently Completed (December 10, 2025) ✅

#### 15. Issue #50 - Provide a way to identify operator generated resources ✅ FIXED
- **Issue**: https://github.com/redhat-cop/namespace-configuration-operator/issues/50
- **Status**: ✅ FIXED
- **Problem**: Teams creating their own network policies may get confused with NetworkPolicies injected by the operator. No easy way to identify operator-generated resources.
- **Solution**: Manual specification of labels and annotations in templates
  - Users add identifying labels/annotations to templates (e.g., `app.kubernetes.io/managed-by: namespace-configuration-operator`)
  - Labels and annotations are applied to all created resources
  - Resources can be queried using standard Kubernetes label selectors
- **Key Features**:
  - **Resource Identification**: Resources can be easily identified via labels/annotations
  - **Automatic Cleanup**: Removing namespace labels automatically triggers resource deletion (production-ready)
  - **Automatic Recreation**: Adding namespace labels back automatically recreates resources
  - **No CR Deletion Required**: Resources can be removed from specific namespaces without deleting the entire CR
- **Verification**: Comprehensive test results documented showing:
  - Metadata verification on created resources (ClusterRoleBindings and RoleBindings)
  - Automatic cleanup when namespace labels are removed
  - Automatic recreation when namespace labels are added back
  - Complete lifecycle demonstration
- **Example Template**: Full YAML template example in documentation showing proper metadata specification
- **Status**: ✅ FIXED - Resources can be identified via labels/annotations, and automatic cleanup/recreation works correctly

#### 16. Issue #132 - Status Update Conflict Blocking Subsequent Reconciles ✅
- **Issue**: https://github.com/redhat-cop/namespace-configuration-operator/issues/132
- **Problem**: When status updates failed due to optimistic concurrency conflicts, all following enqueued namespaceconfigs were not processed, blocking the reconciliation queue
- **Root Cause**: `ManageSuccess` function was called directly without retry logic, causing immediate failures on resourceVersion mismatches
- **Solution**: Implemented `ManageSuccessWithRetry` function in `controllers/common/reconciler_helpers.go`
  - Automatic conflict detection using `errors.IsConflict(err)`
  - Re-fetches instance before each retry to get latest resourceVersion
  - Exponential backoff: 5 retries with delays (50ms, 100ms, 200ms, 400ms, 800ms)
  - Applied to all three controllers (GroupConfig, NamespaceConfig, UserConfig)
- **Benefits**: Prevents queue blocking, automatic recovery, better observability, consistent behavior, reduced false positives
- **Files Modified**:
  - `controllers/common/reconciler_helpers.go` - **NEW** - `ManageSuccessWithRetry` function
  - `controllers/groupconfig_controller.go` - Uses `ManageSuccessWithRetry`
  - `controllers/namespaceconfig_controller.go` - Uses `ManageSuccessWithRetry`
  - `controllers/userconfig_controller.go` - Uses `ManageSuccessWithRetry`
- **Status**: ✅ RESOLVED - Optimistic concurrency conflicts now handled automatically with retry logic

#### 17. Code Refactoring: Common Reconciler Helpers
- **Description**: Extracted duplicate retry logic and logging helpers from individual controllers into centralized common package
- **Implementation**: Created `controllers/common/reconciler_helpers.go` with shared functionality
  - `ManageSuccessWithRetry` - Centralized retry logic for all controllers
  - `LogReconcilingStarted` - Centralized logging helper
  - `LogResourcesProcessedSuccessfully` - Centralized logging helper
- **Benefits**: 
  - Single source of truth for retry logic and logging
  - Consistent behavior across all controllers
  - Reduced code duplication (~59 lines removed from each controller)
  - Improved maintainability and testability
- **Files Modified**:
  - `controllers/common/reconciler_helpers.go` - **NEW**
  - `controllers/groupconfig_controller.go` - Refactored (-59 lines)
  - `controllers/namespaceconfig_controller.go` - Refactored (-59 lines)
  - `controllers/userconfig_controller.go` - Refactored (-59 lines)
- **Status**: ✅ COMPLETED - Code duplication eliminated, maintainability improved

#### 18. Documentation: Groups and Bindings Examples
- **New Documentation**: `docs/groups-and-bindings-examples.md` and `openshift-rbac-automation/docs/groups-and-bindings-examples.md`
- **Content**: Comprehensive documentation providing:
  - Group naming patterns (cluster-level and namespace-level)
  - Example commands to view and inspect groups
  - ClusterRoleBindings and RoleBindings examples
  - Common queries for counting, finding, and verifying bindings
  - Real-world operator log examples with explanations
  - Log level configuration guidance (corrected to use Subscription, not Deployment)
  - Troubleshooting commands
- **Status**: ✅ COMPLETED - Practical documentation for operators and administrators

#### 19. Documentation Fix: Log Level Configuration Guidance
- **Issue**: Incorrect guidance on setting `ZAP_LOG_LEVEL` and `ZAP_DEVEL` directly on Deployment
- **Fix**: Updated documentation to correctly explain configuration via OLM Subscription resource
  - For OLM-managed deployments: Configure via `Subscription.spec.config.env`
  - For local development: Set environment variables when running `./run-go.sh`
- **Files Updated**: `docs/groups-and-bindings-examples.md` (both repositories)
- **Status**: ✅ COMPLETED - Documentation now reflects correct configuration method

### Previously Completed (December 8-9, 2025) ✅

#### 9. Enhanced Template Filtering with AND/OR Logic (Extended)
- **Comprehensive AND/OR Logic Support**: Extended template filtering to all controllers (GroupConfig, NamespaceConfig, UserConfig)
- **AND Logic**: When template uses `{{- if and`, ALL patterns must match (not just one)
- **OR Logic**: When template uses `{{- if` or `{{- else if`, ANY pattern match is sufficient
- **Comprehensive Test Coverage**: 
  - Added extensive unit tests for AND/OR logic in all three controllers
  - Test cases cover multiple scenarios: hasSuffix patterns, contains patterns, mixed patterns
  - Real-world test examples in `examples/test-and-logic/`
- **Status**: ✅ COMPLETED - Templates with AND/OR conditions now work correctly across all controllers

#### 10. Unrecognized Conditional Logic Detection
- **Improved Detection**: Enhanced detection of unrecognized template conditionals (eq, hasPrefix, ne, etc.)
- **Fallback Behavior**: When unrecognized conditionals are detected, templates apply to all resources (relying on template rendering to handle logic)
- **Debug Logging**: Added V(2) level logging for unrecognized conditional detection
- **Test Coverage**: Comprehensive tests for unrecognized conditionals in `controllers/unrecognized_conditionals_test.go`
- **Status**: ✅ COMPLETED - Better handling of templates with unsupported conditional functions

#### 11. Issue #194 - Field Removal with Value 0 Investigation
- **Problem Identified**: Fields with value "0" not being removed when template conditionals change from true to false
- **Root Cause Analysis**: Bug identified in `operator-utils` dependency (not in this operator)
  - Issue is in `UpdateLockedResources` method of `lockedresourcecontroller.EnforcingReconciler`
  - Comparison/patch logic doesn't produce removals for fields present in actual but missing in expected when value is "0"
- **Documentation**: Comprehensive documentation added in `examples/test-and-logic/`:
  - `ISSUE-194-ROOT-CAUSE-SUMMARY.md` - Root cause analysis
  - `ISSUE-194-FIX-IMPLEMENTATION.md` - Fix implementation details (forked operator-utils)
  - `ISSUE-194-VERIFICATION-GUIDE.md` - Verification and testing guide
  - Test resources: `test-issue-194-field-removal-namespaceconfig.yaml`
- **Workaround**: Using forked operator-utils with fix: `github.com/ephico2real2/operator-utils@fix-issue-194-field-removal-zero-value`
- **Status**: ✅ ROOT CAUSE IDENTIFIED - Fix requires operator-utils dependency update

#### 12. Template Filtering Extended to All Controllers
- **NamespaceConfig Controller**: Added template filtering with AND/OR logic support
- **UserConfig Controller**: Added template filtering with AND/OR logic support
- **Consistent Implementation**: All three controllers now have the same template filtering capabilities
- **Test Coverage**: Comprehensive unit tests added for NamespaceConfig and UserConfig controllers
- **Status**: ✅ COMPLETED - Template filtering now works consistently across all controllers

#### 13. Documentation Consolidation
- **Issue #194 Documentation**: Consolidated multiple documentation files into three main documents
- **Test Examples**: Enhanced `examples/test-and-logic/README.md` with comprehensive test scenarios
- **Test Results**: Added test result documentation for AND/OR logic and unrecognized conditionals
- **Status**: ✅ COMPLETED - Documentation organized and comprehensive

#### 14. Local Utilities Updates
- **Updated Scripts**: Enhanced local utility scripts with latest improvements
- **Status**: ✅ COMPLETED

### Previously Completed (December 7, 2025) ✅

#### 1. Build and Run Scripts
- **build.sh**: Wrapper script that automatically sets VERSION, COMMIT, and BUILD_DATE via ldflags
  - Eliminates need to manually specify build parameters
  - Supports environment variable overrides
  - Works with any go build arguments
- **run-go.sh**: Script to build and run operator locally with log configuration
  - Supports --log-level, --dev, --skip-build, --stop options
  - Automatically stops existing operator before starting
  - Auto-builds if binary missing even with --skip-build
- **BUILD-RUN.md**: Comprehensive documentation for both scripts

#### 2. Version Information System
- **internal/version package**: Version management with automatic detection
  - GetVersion(): Detects from git describe or ldflags
  - GetCommitHash(): Gets commit hash from git or ldflags
  - GetBuildDate(): Gets build date from ldflags or current time
  - PrintStartupBanner(): Displays formatted startup banner
- **Startup Banner**: Operator now displays version, commit, and build date on startup
- **Build System Integration**: Dockerfile and Makefiles updated to pass version info

#### 3. Controller Predicate Fix (Issue 3)
- **ResourceGenerationOrFinalizerOrDeletionTimestampChangedPredicate**: New predicate in controllers/common/common.go
  - Handles deletion timestamp changes in addition to generation and finalizer changes
  - Fixes resources stuck in deletion by triggering reconciliation
- **All Controllers Updated**: namespaceconfig, groupconfig, userconfig controllers now use new predicate
- **Status**: ✅ COMPLETED - Resources no longer get stuck in deletion

#### 4. Log Level Configuration (Issue #134) ✅
- **Issue**: https://github.com/redhat-cop/namespace-configuration-operator/issues/134
- **Problem**: Operator creating lots of Info logs sent to ELK, need to set log level to Error
- **Solution**: 
  - **Environment Variable Support**: ZAP_LOG_LEVEL and ZAP_DEVEL support in main.go
  - **Kyverno Policy**: operator-log-level-config.yaml for OLM-managed deployments (persists across updates)
  - **Log Level Options**: Supports "error", "info", "debug", or numeric levels (0-10)
  - **To set Error level**: Update Kyverno policy `ZAP_LOG_LEVEL` value to "error"
- **Documentation**: docs/LOG_LEVEL_CONFIGURATION.md with OLM-compatible methods
- **Default Configuration**: config/manager/manager.yaml with production defaults
- **Template Support**: env-operator-log-level-config.yaml.tpl for environment substitution
- **Status**: ✅ RESOLVED - Log level can now be set to "error" via Kyverno policy or environment variable

#### 5. Template Filtering AND Logic Fix (Bug 3) - Initial Implementation
- **isTemplateApplicableToGroup**: Updated to correctly handle AND conditions
- **Logic Fix**: When template uses `{{- if and`, ALL patterns must match (not just one)
- **Debug Logging**: Added V(2) logging for template filtering verification
- **Status**: ✅ COMPLETED - Templates with AND conditions now work correctly
- **Note**: This was the initial GroupConfig-only implementation. See item #9 for extended implementation across all controllers.

#### 6. Kyverno Policies and Utilities
- **Image Replacement Policies**: Docker Hub and internal registry redirection
- **Policy Templates**: env-*.yaml.tpl files for environment variable substitution
- **generate-policies.sh**: Utility to generate policies from templates
- **create-dockerhub-secret.sh**: Simple utility to create Docker Hub secrets
- **monitor-operator-logs.sh**: Enhanced log monitoring with filtering
- **Documentation**: Comprehensive README files for all utilities

#### 7. Build System Improvements
- **Dockerfile**: Added ARG support for VERSION, COMMIT, BUILD_DATE
- **PodmanMakefile**: 
  - Automatic version detection and passing
  - Fixed EXTERNAL_USER variable expansion
  - Replaced hardcoded credentials with placeholders
  - Made test dependency optional via SKIP_TESTS
  - Updated CONTROLLER_TOOLS_VERSION to v0.19.0
- **Makefile**: Updated build target with automatic version info

#### 8. Documentation Updates
- **BUILD-RUN.md**: Complete documentation for build and run scripts
- **docs/LOG_LEVEL_CONFIGURATION.md**: Log level configuration guide
- **kyverno-policies/README.md**: Policy documentation with customization guide
- **kyverno-policies/README-TEMPLATES.md**: Template usage instructions
- **local-utilities/README.md**: Utility scripts documentation

### Previously Completed ✅

#### Issue 1 - GroupConfig "Object is Null" Fix
- Dynamic template filtering implemented
- Pattern extraction for hasSuffix and contains
- Unit tests created and passing
- ✅ Already implemented and working in production

#### Issue 2 - Finalizer Domain Qualification
- All controllers updated with domain-qualified finalizers
- No more warnings in logs
- ✅ Already implemented and working in production

## Commits Created

### Recent Commits (December 8-9, 2025)
1. **c352ea5** - Update local utilities
2. **1157ec1** - docs(issue-194): keep only 3 consolidated docs; remove superseded 194 markdown files
3. **eecf6de** - docs(issue-194): add appendix explaining pseudo-version derivation
4. **3e40ed7** - docs(issue-194): add pr-194.md (PR body) under examples/test-and-logic
5. **98c37f4** - docs(issue-194): consolidate docs + add real-time verification; wire operator-utils fix
6. **c309030** - fix: improve detection of unrecognized template conditionals
7. **97392ef** - feat: improve template filtering with AND/OR logic and add comprehensive tests
8. **de1c07a** - docs: add comprehensive test examples and documentation for AND/OR logic
9. **6d3e659** - test: add comprehensive test cases for AND and OR logic
10. **00d21e0** - feat: implement AND logic in template filtering for GroupConfig

### Earlier Commits (December 7, 2025)
11. **4da76c7** - Add build.sh and run-go.sh scripts for simplified operator development
12. **7b2c29e** - Add startup banner with version information
13. **359537d** - Fix controller reconciliation for resources stuck in deletion
14. **96e6362** - Add log level configuration documentation and defaults
15. **07658ec** - Add Kyverno policies and local development utilities
16. **88434fa** - Update build system to support automatic version information
17. **2a52a85** - Update .gitignore to ignore generated Helm chart artifacts
18. **d4852fe** - Update generated code and CRDs

## Files Created/Modified

### New Files
- `build.sh` - Build wrapper script
- `run-go.sh` - Run script with options
- `BUILD-RUN.md` - Build and run documentation
- `internal/version/version.go` - Version management package
- `controllers/common/common.go` - Common utilities and predicates
- `controllers/common/reconciler_helpers.go` - **NEW (December 10, 2025)** - Common reconciler helper functions (ManageSuccessWithRetry, logging helpers)
- `docs/LOG_LEVEL_CONFIGURATION.md` - Log level configuration guide
- `docs/groups-and-bindings-examples.md` - **NEW (December 10, 2025)** - Groups and bindings examples documentation
- `kyverno-policies/` - Kyverno policy files and templates
- `local-utilities/` - Development utility scripts
- `controllers/unrecognized_conditionals_test.go` - Tests for unrecognized conditional detection
- `controllers/namespaceconfig_controller_test.go` - Comprehensive tests for NamespaceConfig template filtering
- `controllers/userconfig_controller_test.go` - Comprehensive tests for UserConfig template filtering
- `examples/test-and-logic/` - Comprehensive test examples and documentation:
  - `README.md` - Test documentation
  - `test-and-logic-groupconfig.yaml` - AND logic test
  - `test-or-logic-groupconfig.yaml` - OR logic test
  - `test-unrecognized-conditionals-groupconfig.yaml` - Unrecognized conditionals test
  - `test-issue-194-field-removal-namespaceconfig.yaml` - Issue #194 test
  - `ISSUE-194-ROOT-CAUSE-SUMMARY.md` - Root cause analysis
  - `ISSUE-194-FIX-IMPLEMENTATION.md` - Fix implementation details
  - `ISSUE-194-VERIFICATION-GUIDE.md` - Verification guide
  - Various explanation and results markdown files

### Modified Files
- `main.go` - Added startup banner and log level configuration
- `controllers/groupconfig_controller.go` - Template filtering AND/OR logic, unrecognized conditional detection, refactored to use common reconciler helpers (December 10, 2025)
- `controllers/namespaceconfig_controller.go` - New predicate, template filtering with AND/OR logic, unrecognized conditional detection, refactored to use common reconciler helpers (December 10, 2025)
- `controllers/userconfig_controller.go` - New predicate, template filtering with AND/OR logic, unrecognized conditional detection, refactored to use common reconciler helpers (December 10, 2025)
- `docs/FEATURES_AND_ISSUES_RESOLUTION.md` - **UPDATED (December 10, 2025)** - Added issue #50 and issue #132 documentation, updated with recent work
- `Dockerfile` - Version info and log level defaults
- `PodmanMakefile` - Version detection and build improvements
- `Makefile` - Version detection in build target
- `config/manager/manager.yaml` - Log level defaults
- `.gitignore` - Restored charts/ pattern
- `go.mod` - Updated to use forked operator-utils with issue #194 fix

## Testing Status

### Build Scripts ✅
- All build.sh options tested and working
- All run-go.sh options tested and working
- Version info correctly embedded in binaries
- Auto-stop functionality working

### Controllers ✅
- Deletion handling fixed and tested
- Template filtering AND/OR logic fixed and extended to all controllers
- Unrecognized conditional detection implemented
- All predicates working correctly
- Comprehensive test coverage for all three controllers

### Log Level ✅
- Environment variables working
- Documentation complete
- Kyverno policy tested

## Next Steps

### Immediate
1. **Issue #194 Fix**: 
   - Wait for operator-utils to merge fix for issue #194, OR
   - Continue using forked operator-utils until upstream fix is available
   - Monitor upstream operator-utils repository for fix merge
2. **Test in Cluster**: Deploy updated operator to test cluster with all recent improvements
3. **Verify Template Filtering**: Test AND/OR logic and unrecognized conditional detection in production
4. **Verify Version Banner**: Confirm startup banner displays in cluster logs

### Follow-up
1. **Monitor Production**: Watch for any issues with new template filtering improvements
2. **Update Documentation**: Keep documentation current as needed
3. **Consider Additional Features**: Based on production feedback
4. **Upstream Contribution**: Consider contributing issue #194 fix to operator-utils upstream

## Key Success Metrics

- ✅ Build scripts simplify development workflow
- ✅ Version information visible in startup banner
- ✅ Resources no longer stuck in deletion
- ✅ Log level configurable via OLM-compatible methods
- ✅ Template filtering correctly handles AND/OR conditions across all controllers
- ✅ Unrecognized conditional detection prevents template filtering errors
- ✅ Comprehensive test coverage for all template filtering scenarios
- ✅ Issue #50 resolved - Resources can be identified via labels/annotations, automatic cleanup/recreation works
- ✅ Issue #50 resolved - Resources can be identified via labels/annotations, automatic cleanup/recreation works
- ✅ Issue #194 root cause identified (operator-utils dependency)
- ✅ Issue #132 resolved - Optimistic concurrency conflicts handled automatically with retry logic
- ✅ Code refactoring eliminates duplication and improves maintainability
- ✅ All utilities documented and tested
- ✅ Build system automatically detects version info
- ✅ Documentation consolidated and comprehensive
- ✅ Groups and bindings examples documentation provides practical guidance

## Known Issues

### Issue #194 - Field Removal with Value 0
- **Status**: Root cause identified in operator-utils dependency
- **Impact**: Fields with value "0" not removed when template conditionals change
- **Workaround**: Using forked operator-utils with fix: `github.com/ephico2real2/operator-utils@fix-issue-194-field-removal-zero-value`
- **Resolution**: Waiting for upstream operator-utils fix or continuing with forked version
- **Documentation**: See `examples/test-and-logic/ISSUE-194-*.md` files for details
