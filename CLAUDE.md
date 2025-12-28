# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

KalypsoServing is a Kubernetes operator for managing ML model serving infrastructure built on NVIDIA Triton Inference Server. It uses Kubebuilder v4.10.1 and follows the standard Kubernetes controller pattern with three core CRDs:

- **KalypsoProject**: Manages environment isolation (dev/stage/prod) with namespace creation and resource quotas
- **KalypsoApplication**: Groups related Triton servers with shared configurations (storage credentials, project references)
- **KalypsoTritonServer**: Deploys and manages NVIDIA Triton Inference Server instances with full observability stack support

## Development Commands

### Build and Run

```bash
# Install dependencies and setup envtest
make setup-envtest

# Build binary
make build

# Run controller locally (requires valid kubeconfig)
make run

# Format and vet code
make fmt vet

# Generate manifests and code (run after API changes)
make manifests generate
```

### Testing

```bash
# Unit and integration tests (uses envtest - simulated API server)
make test

# E2E tests (creates Kind cluster: kalypsoserving-test-e2e)
make test-e2e

# Cleanup E2E cluster
make cleanup-test-e2e
```

E2E tests are tagged with `//go:build e2e` and use Ginkgo/Gomega. Tests expect Kind and build/load the manager image locally. Set `CERT_MANAGER_INSTALL_SKIP=true` to skip CertManager installation.

### Linting

```bash
# Run linter
make lint

# Run linter with auto-fix
make lint-fix

# Verify linter configuration
make lint-config
```

Linting uses golangci-lint v1.62.2 with specific linters enabled: errcheck, gosimple, govet, ineffassign, staticcheck, unused. Test files and generated code (`zz_generated`) are excluded from errcheck.

### Kubernetes Deployment

```bash
# Install CRDs to cluster
make install

# Uninstall CRDs
make uninstall

# Build and push Docker image
make docker-build docker-push IMG=<your-image>

# Deploy controller to cluster
make deploy IMG=<your-image>

# Undeploy controller
make undeploy

# Build installer (consolidated YAML)
make build-installer
```

Default image registry is GitHub Container Registry (ghcr.io/kalypsoserving/kalypsoserving).

### Pre-commit Hooks

The repository uses pre-commit hooks that:
- Prevent commits to `main` branch
- Run `make lint` on Go files
- Run `make manifests` and `make generate` when API files (`api/**/*.go`) change
- Check YAML syntax, merge conflicts, trailing whitespace

## Architecture and Code Organization

### Directory Structure

```
api/v1alpha1/              # CRD type definitions (Go structs)
internal/controller/       # Reconciliation logic for each CRD
config/                    # Kustomize manifests (CRD, RBAC, manager)
  crd/bases/              # Generated CRD YAML files
  rbac/                   # Role definitions
  samples/                # Example CR manifests
cmd/                       # Main entrypoint
test/e2e/                  # End-to-end tests
hack/                      # Scripts and boilerplate
```

### Controller Reconciliation Pattern

All controllers follow the same reconciliation flow:

1. **Fetch resource** - Return if NotFound (resource deleted)
2. **Handle deletion** - Check `DeletionTimestamp`, execute cleanup in `reconcileDelete()`
3. **Add finalizer** - Use controller-specific finalizer names:
   - KalypsoProject: `serving.kalypso.io/finalizer`
   - KalypsoApplication: `serving.kalypso.io/application-finalizer`
   - KalypsoTritonServer: `serving.kalypso.io/tritonserver-finalizer`
4. **Initialize status** - Set initial phase if empty, then requeue
5. **Validate references** - KalypsoApplication validates projectRef, KalypsoTritonServer validates applicationRef
6. **Create/update resources** - Use owner references for garbage collection
7. **Update status** - Reflect current state (Phase, Replicas, Conditions)

### Label Conventions

Standardized labels used across resources:

```go
// Project identification
"kalypso-serving.io/project": "<project-name>"
"kalypso-serving.io/environment": "dev|stage|prod"

// Application identification
"kalypso-serving.io/application": "<application-name>"

// TritonServer identification
"kalypso-serving.io/tritonserver": "<server-name>"

// Managed-by label
"app.kubernetes.io/managed-by": "kalypso-serving"
```

### Resource Ownership

- KalypsoProject creates Namespaces, ResourceQuotas, LimitRanges with owner references
- KalypsoTritonServer creates Deployment, Service, ServiceMonitor with owner references
- Owner references enable automatic garbage collection when parent is deleted

### Status Phases

**KalypsoProject**:
- `Provisioning` - Creating namespaces and resources
- `Ready` - All environments provisioned
- `Failed` - Error during provisioning

**KalypsoApplication**:
- `Pending` - Validating project reference
- `Ready` - Application configured
- `Failed` - Invalid references or configuration

**KalypsoTritonServer**:
- `Pending` - Initial state
- `Running` - Deployment created and running
- `Failed` - Deployment failed or application reference invalid

## Important Implementation Notes

### API Changes Workflow

When modifying CRD structs in `api/v1alpha1/`:
1. Update the Go struct
2. Run `make manifests` to regenerate CRDs in `config/crd/bases/`
3. Run `make generate` to regenerate DeepCopy methods
4. Pre-commit hooks will enforce this automatically

### Observability Support

KalypsoTritonServer has extensive observability configuration:
- Logging (Loki integration via promtail sidecar)
- Tracing (Tempo/Jaeger/Zipkin support)
- Profiling (Pyroscope continuous profiling)
- Metrics (Prometheus via ServiceMonitor)

When modifying TritonServer spec, consider impact on these integrations.

### Testing Best Practices

- Unit tests use envtest (fake API server) - found in `internal/controller/*_test.go`
- E2E tests create real Kind cluster - found in `test/e2e/`
- Use Ginkgo `By()` for test step documentation
- Use `Eventually()` for async operations (deployment readiness, status updates)
- Clean up resources in `AfterEach()` blocks

### RBAC Markers

Controllers use kubebuilder RBAC markers (`+kubebuilder:rbac`) above the Reconcile method. After changing these, run `make manifests` to regenerate RBAC manifests in `config/rbac/`.

### Common Gotchas

- Always check if resource exists before creating (avoid AlreadyExists errors)
- Use `controllerutil.SetControllerReference()` for owner references
- Update Status using `r.Status().Update()`, not `r.Update()`
- Requeue after status updates to ensure fresh object in next reconciliation
- Handle `NotFound` errors gracefully (resource may be deleted between list and get)
