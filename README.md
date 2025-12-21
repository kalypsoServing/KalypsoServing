# KalypsoServing

KalypsoServing is a Kubernetes operator for managing ML model serving infrastructure, built on top of NVIDIA Triton Inference Server.

## Overview

KalypsoServing provides three Custom Resource Definitions (CRDs) to manage the lifecycle of ML model serving:

- **KalypsoProject**: Manages logical namespaces for ML projects with environment isolation (dev, stage, prod)
- **KalypsoApplication**: Defines logical service units that group related Triton servers
- **KalypsoTritonServer**: Deploys and manages NVIDIA Triton Inference Servers

### Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                        KalypsoProject                           │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐             │
│  │   dev-ns    │  │  stage-ns   │  │   prod-ns   │             │
│  │ (Namespace) │  │ (Namespace) │  │ (Namespace) │             │
│  └─────────────┘  └─────────────┘  └─────────────┘             │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│                     KalypsoApplication                          │
│  ┌─────────────────────────────────────────────────────────┐   │
│  │  - projectRef: sample-project                            │   │
│  │  - storage: aws-s3-credentials                           │   │
│  │  - gatewayEndpoint: istio-gateway/...                    │   │
│  └─────────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│                    KalypsoTritonServer                          │
│  ┌──────────────────┐  ┌──────────────────┐                    │
│  │   Deployment     │  │     Service      │                    │
│  │ (tritonserver)   │  │ (http/grpc/metrics)                   │
│  └──────────────────┘  └──────────────────┘                    │
└─────────────────────────────────────────────────────────────────┘
```

## QuickStart

### Prerequisites

- Kubernetes cluster (kind, minikube, or cloud provider)
- kubectl v1.11.3+
- Go v1.22+ (for development)

### 1. Install CRDs

```sh
# Clone the repository
git clone https://github.com/kalypsoServing/KalypsoServing.git
cd KalypsoServing

# Install CRDs
make install

# Create kalypso-system namespace
kubectl create namespace kalypso-system
```

### 2. Run the Controller

```sh
# Run the controller locally (for development)
make run
```

### 3. Create a KalypsoProject

```yaml
# config/samples/serving_v1alpha1_kalypsoproject.yaml
apiVersion: serving.serving.kalypso.io/v1alpha1
kind: KalypsoProject
metadata:
  name: sample-project
  namespace: kalypso-system
spec:
  displayName: "GenAI Model Serving Demo"
  owner: "team-ai-research"
  environments:
    dev:
      namespace: sample-project-dev
      description: "Development environment"
      resourceQuota:
        limits:
          nvidia.com/gpu: "1"
    stage:
      namespace: sample-project-stage
      description: "Staging environment"
    prod:
      namespace: sample-project-prod
      description: "Production environment"
      resourceQuota:
        limits:
          nvidia.com/gpu: "4"
```

```sh
kubectl apply -f config/samples/serving_v1alpha1_kalypsoproject.yaml
```

### 4. Create a KalypsoApplication

```yaml
# config/samples/serving_v1alpha1_kalypsoapplication.yaml
apiVersion: serving.serving.kalypso.io/v1alpha1
kind: KalypsoApplication
metadata:
  name: recommendation-application
  namespace: kalypso-system
spec:
  projectRef: "sample-project"
  description: "Product recommendation model serving application"
  storage:
    secretName: "aws-s3-credentials"
    region: "ap-northeast-2"
```

```sh
kubectl apply -f config/samples/serving_v1alpha1_kalypsoapplication.yaml
```

### 5. Create a KalypsoTritonServer

```yaml
# config/samples/serving_v1alpha1_kalypsotritonserver.yaml
apiVersion: serving.serving.kalypso.io/v1alpha1
kind: KalypsoTritonServer
metadata:
  name: recommendation-v1
  namespace: kalypso-system
spec:
  applicationRef: "recommendation-application"
  storageUri: "s3://kalypso-models/recommendation/v1/"
  tritonConfig:
    image: "nvcr.io/nvidia/tritonserver"
    tag: "24.12-py3"
    backendType: "python"
  replicas: 2
  resources:
    requests:
      cpu: "500m"
      memory: "1Gi"
    limits:
      cpu: "1000m"
      memory: "2Gi"
  networking:
    httpPort: 8000
    grpcPort: 8001
    metricsPort: 8002
```

```sh
kubectl apply -f config/samples/serving_v1alpha1_kalypsotritonserver.yaml
```

### 6. Verify Deployment

```sh
# Check KalypsoProject status
kubectl get kalypsoproject -n kalypso-system
# NAME             PHASE   OWNER              AGE
# sample-project   Ready   team-ai-research   1m

# Check created namespaces
kubectl get ns -l kalypso-serving.io/project=sample-project
# NAME                   STATUS   AGE
# sample-project-dev     Active   1m
# sample-project-prod    Active   1m
# sample-project-stage   Active   1m

# Check KalypsoApplication status
kubectl get kalypsoapplication -n kalypso-system
# NAME                         PROJECT          PHASE   MODELS   AGE
# recommendation-application   sample-project   Ready   1        1m

# Check KalypsoTritonServer status
kubectl get kalypsotritonserver -n kalypso-system
# NAME               APPLICATION                  PHASE     REPLICAS   AVAILABLE   AGE
# recommendation-v1  recommendation-application   Pending   2          0           1m

# Check Deployment and Service
kubectl get deployment,svc -n kalypso-system -l kalypso-serving.io/tritonserver=recommendation-v1
```

### 7. Cleanup

```sh
# Delete all resources
kubectl delete kalypsotritonserver --all -n kalypso-system
kubectl delete kalypsoapplication --all -n kalypso-system
kubectl delete kalypsoproject --all -n kalypso-system

# Delete namespace
kubectl delete namespace kalypso-system

# Uninstall CRDs
make uninstall
```

## Getting Started (Production Deployment)

### Prerequisites
- go version v1.22+
- docker version 17.03+
- kubectl version v1.11.3+
- Access to a Kubernetes v1.11.3+ cluster

### Deploy to Cluster

**Build and push your image to the location specified by `IMG`:**

```sh
make docker-build docker-push IMG=<some-registry>/kalypsoserving:tag
```

**Install the CRDs into the cluster:**

```sh
make install
```

**Deploy the Manager to the cluster with the image specified by `IMG`:**

```sh
make deploy IMG=<some-registry>/kalypsoserving:tag
```

> **NOTE**: If you encounter RBAC errors, you may need to grant yourself cluster-admin
privileges or be logged in as admin.

**Create instances of your solution**

```sh
kubectl apply -k config/samples/
```

### To Uninstall

**Delete the instances (CRs) from the cluster:**

```sh
kubectl delete -k config/samples/
```

**Delete the APIs(CRDs) from the cluster:**

```sh
make uninstall
```

**UnDeploy the controller from the cluster:**

```sh
make undeploy
```

## CRD Reference

### KalypsoProject

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `spec.displayName` | string | No | Human-readable project name |
| `spec.owner` | string | No | Team or user owning the project |
| `spec.environments` | map | No | Environment-specific configurations |
| `spec.modelRegistry` | object | No | Model registry settings |

### KalypsoApplication

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `spec.projectRef` | string | Yes | Reference to parent KalypsoProject |
| `spec.description` | string | No | Application description |
| `spec.source` | object | No | Git repository configuration |
| `spec.storage` | object | No | Storage/secret configuration |

### KalypsoTritonServer

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `spec.applicationRef` | string | Yes | Reference to parent KalypsoApplication |
| `spec.storageUri` | string | Yes | S3/GCS path to model repository |
| `spec.tritonConfig` | object | Yes | Triton server configuration |
| `spec.replicas` | int | No | Number of replicas (default: 1) |
| `spec.resources` | object | No | K8s resource requests/limits |
| `spec.networking` | object | No | Service port configuration |

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

**NOTE:** Run `make help` for more information on all potential `make` targets

More information can be found via the [Kubebuilder Documentation](https://book.kubebuilder.io/introduction.html)

## License

Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
