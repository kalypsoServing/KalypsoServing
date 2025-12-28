# KalypsoServing Context

## Project Overview

**KalypsoServing** is a Kubernetes operator designed to manage Machine Learning (ML) model serving infrastructure, leveraging NVIDIA Triton Inference Server. It simplifies the deployment and lifecycle management of ML models on Kubernetes.

### Key Components (CRDs)

*   **KalypsoProject**: Defines a logical workspace for ML projects, providing environment isolation (e.g., dev, stage, prod) and resource quotas.
*   **KalypsoApplication**: Represents a logical grouping of serving units (Triton servers) within a project. It handles shared configurations like storage credentials.
*   **KalypsoTritonServer**: The core workload resource that deploys and manages instances of NVIDIA Triton Inference Server. It supports configuration for:
    *   Model storage (S3/GCS).
    *   Compute resources (GPU/CPU/Memory).
    *   Networking (HTTP/gRPC/Metrics ports).
    *   Observability (Logging, Tracing, Profiling, Metrics).
    *   Backend types (Python, TensorFlow, PyTorch, etc.).

### Architecture

The operator follows the standard Kubernetes controller pattern.
*   **API**: defined in `api/v1alpha1/`.
*   **Controllers**: Logic resides in `internal/controller/`.
*   **Configuration**: Kustomize manifests in `config/`.

## Building and Running

The project uses a `Makefile` for standard operations.

### Prerequisites

*   Go 1.24.0+
*   Docker (or compatible container tool)
*   Kubernetes cluster (Kind, Minikube, or remote)
*   `kubectl`

### Key Commands

*   **Install Dependencies:**
    ```bash
    make setup-envtest
    ```
*   **Build Binary:**
    ```bash
    make build
    ```
*   **Run Controller Locally:**
    ```bash
    make run
    ```
    *Note: Ensure you have a kubeconfig pointing to a valid cluster.*

*   **Install CRDs to Cluster:**
    ```bash
    make install
    ```

*   **Uninstall CRDs:**
    ```bash
    make uninstall
    ```

*   **Build Docker Image:**
    ```bash
    make docker-build IMG=<your-image-tag>
    ```

*   **Deploy to Cluster:**
    ```bash
    make deploy IMG=<your-image-tag>
    ```

## Testing

The project uses **Ginkgo** and **Gomega** for testing.

*   **Unit & Integration Tests:**
    Runs using `envtest` (simulated API server).
    ```bash
    make test
    ```

*   **End-to-End (E2E) Tests:**
    Runs against a real cluster (defaults to creating a Kind cluster named `kalypsoserving-test-e2e`).
    ```bash
    make test-e2e
    ```

## Development Conventions

*   **Framework**: Built with [Kubebuilder](https://book.kubebuilder.io/).
*   **Code Style**: Standard Go conventions. Run `make fmt` and `make vet` before committing.
*   **Linting**: Uses `golangci-lint`. Run `make lint`.
*   **Directory Structure**:
    *   `api/`: CRD definitions (Go structs).
    *   `internal/controller/`: Reconcile logic.
    *   `config/`: Kustomize configuration for CRDs, RBAC, and Manager.
    *   `hack/`: Scripts and boilerplate.
*   **Observability**: The `KalypsoTritonServer` CRD has extensive support for observability stacks (Loki, Tempo, Pyroscope, Prometheus), which should be considered when modifying the server spec.
