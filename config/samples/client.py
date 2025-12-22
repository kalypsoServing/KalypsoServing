#!/usr/bin/env python3
"""
Triton Inference Client Example for KalypsoTritonServer

This script demonstrates how to send inference requests to a deployed
KalypsoTritonServer using the tritonclient library.

Prerequisites:
    pip install "tritonclient[all]" numpy

Usage:
    # First, port-forward to your Triton server
    kubectl port-forward svc/add-sub-server-svc -n kalypso-system 8000:8000 8001:8001

    # Then run this script
    python client.py --protocol http --host localhost --port 8000
    python client.py --protocol grpc --host localhost --port 8001
"""

import argparse
import sys
import numpy as np

# HTTP client
try:
    import tritonclient.http as httpclient
    HTTP_AVAILABLE = True
except ImportError:
    HTTP_AVAILABLE = False

# gRPC client
try:
    import tritonclient.grpc as grpcclient
    GRPC_AVAILABLE = True
except ImportError:
    GRPC_AVAILABLE = False


def create_http_client(host: str, port: int):
    """Create HTTP Triton client."""
    if not HTTP_AVAILABLE:
        raise RuntimeError("tritonclient[http] is not installed. Run: pip install tritonclient[http]")
    
    url = f"{host}:{port}"
    client = httpclient.InferenceServerClient(url=url)
    return client


def create_grpc_client(host: str, port: int):
    """Create gRPC Triton client."""
    if not GRPC_AVAILABLE:
        raise RuntimeError("tritonclient[grpc] is not installed. Run: pip install tritonclient[grpc]")
    
    url = f"{host}:{port}"
    client = grpcclient.InferenceServerClient(url=url)
    return client


def infer_http(client, model_name: str, input0: np.ndarray, input1: np.ndarray):
    """Perform inference using HTTP protocol."""
    # Create input tensors
    inputs = [
        httpclient.InferInput("INPUT0", input0.shape, "FP32"),
        httpclient.InferInput("INPUT1", input1.shape, "FP32"),
    ]
    inputs[0].set_data_from_numpy(input0)
    inputs[1].set_data_from_numpy(input1)

    # Define output tensors
    outputs = [
        httpclient.InferRequestedOutput("OUTPUT0"),
        httpclient.InferRequestedOutput("OUTPUT1"),
    ]

    # Perform inference
    result = client.infer(model_name=model_name, inputs=inputs, outputs=outputs)

    # Get output tensors
    output0 = result.as_numpy("OUTPUT0")
    output1 = result.as_numpy("OUTPUT1")

    return output0, output1


def infer_grpc(client, model_name: str, input0: np.ndarray, input1: np.ndarray):
    """Perform inference using gRPC protocol."""
    # Create input tensors
    inputs = [
        grpcclient.InferInput("INPUT0", input0.shape, "FP32"),
        grpcclient.InferInput("INPUT1", input1.shape, "FP32"),
    ]
    inputs[0].set_data_from_numpy(input0)
    inputs[1].set_data_from_numpy(input1)

    # Define output tensors
    outputs = [
        grpcclient.InferRequestedOutput("OUTPUT0"),
        grpcclient.InferRequestedOutput("OUTPUT1"),
    ]

    # Perform inference
    result = client.infer(model_name=model_name, inputs=inputs, outputs=outputs)

    # Get output tensors
    output0 = result.as_numpy("OUTPUT0")
    output1 = result.as_numpy("OUTPUT1")

    return output0, output1


def check_server_health(client, protocol: str) -> bool:
    """Check if the server is live and ready."""
    try:
        is_live = client.is_server_live()
        is_ready = client.is_server_ready()
        print(f"Server Live: {is_live}")
        print(f"Server Ready: {is_ready}")
        return is_live and is_ready
    except Exception as e:
        print(f"Failed to check server health: {e}")
        return False


def check_model_ready(client, model_name: str, protocol: str) -> bool:
    """Check if the model is ready for inference."""
    try:
        is_ready = client.is_model_ready(model_name)
        print(f"Model '{model_name}' Ready: {is_ready}")
        return is_ready
    except Exception as e:
        print(f"Failed to check model status: {e}")
        return False


def get_model_metadata(client, model_name: str, protocol: str):
    """Get model metadata."""
    try:
        metadata = client.get_model_metadata(model_name)
        print(f"\nModel Metadata for '{model_name}':")
        if protocol == "http":
            print(f"  Name: {metadata.get('name')}")
            print(f"  Versions: {metadata.get('versions')}")
            print(f"  Inputs: {metadata.get('inputs')}")
            print(f"  Outputs: {metadata.get('outputs')}")
        else:  # grpc
            print(f"  Name: {metadata.name}")
            print(f"  Versions: {metadata.versions}")
            print(f"  Inputs: {[(inp.name, inp.datatype, inp.shape) for inp in metadata.inputs]}")
            print(f"  Outputs: {[(out.name, out.datatype, out.shape) for out in metadata.outputs]}")
        return metadata
    except Exception as e:
        print(f"Failed to get model metadata: {e}")
        return None


def main():
    parser = argparse.ArgumentParser(description="Triton Inference Client for KalypsoTritonServer")
    parser.add_argument(
        "--protocol",
        type=str,
        default="http",
        choices=["http", "grpc"],
        help="Protocol to use for inference (default: http)"
    )
    parser.add_argument(
        "--host",
        type=str,
        default="localhost",
        help="Triton server host (default: localhost)"
    )
    parser.add_argument(
        "--port",
        type=int,
        default=None,
        help="Triton server port (default: 8000 for http, 8001 for grpc)"
    )
    parser.add_argument(
        "--model",
        type=str,
        default="add_sub",
        help="Model name to use for inference (default: add_sub)"
    )
    parser.add_argument(
        "--batch-size",
        type=int,
        default=1,
        help="Batch size for inference (default: 1)"
    )
    args = parser.parse_args()

    # Set default port based on protocol
    if args.port is None:
        args.port = 8000 if args.protocol == "http" else 8001

    print("=" * 60)
    print("KalypsoTritonServer Inference Client")
    print("=" * 60)
    print(f"Protocol: {args.protocol.upper()}")
    print(f"Server: {args.host}:{args.port}")
    print(f"Model: {args.model}")
    print(f"Batch Size: {args.batch_size}")
    print("=" * 60)

    # Create client
    try:
        if args.protocol == "http":
            client = create_http_client(args.host, args.port)
        else:
            client = create_grpc_client(args.host, args.port)
    except Exception as e:
        print(f"Failed to create client: {e}")
        sys.exit(1)

    # Check server health
    print("\n[1] Checking server health...")
    if not check_server_health(client, args.protocol):
        print("Server is not ready. Exiting.")
        sys.exit(1)

    # Check model status
    print(f"\n[2] Checking model '{args.model}' status...")
    if not check_model_ready(client, args.model, args.protocol):
        print(f"Model '{args.model}' is not ready. Exiting.")
        sys.exit(1)

    # Get model metadata
    print(f"\n[3] Getting model metadata...")
    get_model_metadata(client, args.model, args.protocol)

    # Prepare input data
    print(f"\n[4] Preparing input data...")
    # Create sample input: batch_size x 4 float32 arrays
    input0 = np.array([[1.0, 2.0, 3.0, 4.0]] * args.batch_size, dtype=np.float32)
    input1 = np.array([[4.0, 3.0, 2.0, 1.0]] * args.batch_size, dtype=np.float32)
    
    print(f"INPUT0 shape: {input0.shape}")
    print(f"INPUT0 data: {input0[0].tolist()}")
    print(f"INPUT1 shape: {input1.shape}")
    print(f"INPUT1 data: {input1[0].tolist()}")

    # Perform inference
    print(f"\n[5] Performing inference...")
    try:
        if args.protocol == "http":
            output0, output1 = infer_http(client, args.model, input0, input1)
        else:
            output0, output1 = infer_grpc(client, args.model, input0, input1)
        
        print("\n" + "=" * 60)
        print("INFERENCE RESULTS")
        print("=" * 60)
        print(f"INPUT0:  {input0[0].tolist()}")
        print(f"INPUT1:  {input1[0].tolist()}")
        print("-" * 60)
        print(f"OUTPUT0 (INPUT0 + INPUT1): {output0[0].tolist()}")
        print(f"OUTPUT1 (INPUT0 - INPUT1): {output1[0].tolist()}")
        print("=" * 60)
        
        # Verify results
        expected_add = input0[0] + input1[0]
        expected_sub = input0[0] - input1[0]
        
        if np.allclose(output0[0], expected_add) and np.allclose(output1[0], expected_sub):
            print("\n✅ Inference successful! Results are correct.")
        else:
            print("\n⚠️  Inference completed but results don't match expected values.")
            print(f"Expected OUTPUT0: {expected_add.tolist()}")
            print(f"Expected OUTPUT1: {expected_sub.tolist()}")
            
    except Exception as e:
        print(f"\n❌ Inference failed: {e}")
        sys.exit(1)


if __name__ == "__main__":
    main()

