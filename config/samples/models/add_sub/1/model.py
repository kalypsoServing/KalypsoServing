import json
import numpy as np
import triton_python_backend_utils as pb_utils


class TritonPythonModel:
    """Simple Add/Sub model for KalypsoServing demo.
    
    This model takes two input tensors (INPUT0, INPUT1) and produces:
    - OUTPUT0: INPUT0 + INPUT1 (element-wise addition)
    - OUTPUT1: INPUT0 - INPUT1 (element-wise subtraction)
    """

    def initialize(self, args):
        """Initialize the model.
        
        Args:
            args: Dictionary containing model configuration
        """
        self.model_config = json.loads(args['model_config'])
        
        # Get output configuration
        output0_config = pb_utils.get_output_config_by_name(
            self.model_config, "OUTPUT0")
        output1_config = pb_utils.get_output_config_by_name(
            self.model_config, "OUTPUT1")

        # Convert Triton types to numpy types
        self.output0_dtype = pb_utils.triton_string_to_numpy(
            output0_config['data_type'])
        self.output1_dtype = pb_utils.triton_string_to_numpy(
            output1_config['data_type'])

    def execute(self, requests):
        """Process inference requests.
        
        Args:
            requests: List of pb_utils.InferenceRequest
            
        Returns:
            List of pb_utils.InferenceResponse
        """
        responses = []

        for request in requests:
            # Get input tensors
            input0 = pb_utils.get_input_tensor_by_name(request, "INPUT0")
            input1 = pb_utils.get_input_tensor_by_name(request, "INPUT1")

            # Convert to numpy arrays
            input0_data = input0.as_numpy()
            input1_data = input1.as_numpy()

            # Perform add and subtract operations
            add_result = (input0_data + input1_data).astype(self.output0_dtype)
            sub_result = (input0_data - input1_data).astype(self.output1_dtype)

            # Create output tensors
            output0_tensor = pb_utils.Tensor("OUTPUT0", add_result)
            output1_tensor = pb_utils.Tensor("OUTPUT1", sub_result)

            # Create inference response
            inference_response = pb_utils.InferenceResponse(
                output_tensors=[output0_tensor, output1_tensor])
            responses.append(inference_response)

        return responses

    def finalize(self):
        """Clean up resources."""
        pass

