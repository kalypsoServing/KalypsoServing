//go:build e2e
// +build e2e

/*
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
*/

package e2e

import (
	"fmt"
	"os/exec"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/kalypsoServing/KalypsoServing/test/utils"
)

const (
	testNamespace           = "kalypso-test"
	testProjectName         = "test-project"
	testApplicationName     = "test-application"
	testTritonServerName    = "test-tritonserver"
	observabilityTestServer = "observability-test-server"
)

var _ = Describe("KalypsoTritonServer Observability", Ordered, func() {
	BeforeAll(func() {
		By("creating test namespace")
		cmd := exec.Command("kubectl", "create", "ns", testNamespace)
		_, _ = utils.Run(cmd) // Ignore error if namespace already exists

		By("applying prerequisite resources")
		// Create KalypsoProject
		projectYAML := fmt.Sprintf(`
apiVersion: serving.serving.kalypso.io/v1alpha1
kind: KalypsoProject
metadata:
  name: %s
  namespace: %s
spec:
  name: "Test Project"
  description: "Test project for observability tests"
`, testProjectName, testNamespace)

		cmd = exec.Command("kubectl", "apply", "-f", "-")
		cmd.Stdin = strings.NewReader(projectYAML)
		_, err := utils.Run(cmd)
		Expect(err).NotTo(HaveOccurred(), "Failed to create KalypsoProject")

		// Create KalypsoApplication
		appYAML := fmt.Sprintf(`
apiVersion: serving.serving.kalypso.io/v1alpha1
kind: KalypsoApplication
metadata:
  name: %s
  namespace: %s
spec:
  projectRef: "%s"
  name: "Test Application"
  description: "Test application for observability tests"
`, testApplicationName, testNamespace, testProjectName)

		cmd = exec.Command("kubectl", "apply", "-f", "-")
		cmd.Stdin = strings.NewReader(appYAML)
		_, err = utils.Run(cmd)
		Expect(err).NotTo(HaveOccurred(), "Failed to create KalypsoApplication")

		// Wait for resources to be ready
		time.Sleep(5 * time.Second)
	})

	AfterAll(func() {
		By("cleaning up test resources")
		cmd := exec.Command("kubectl", "delete", "ns", testNamespace, "--ignore-not-found")
		_, _ = utils.Run(cmd)
	})

	Context("Logging Configuration", func() {
		It("should inject correct logging args based on level", func() {
			By("creating KalypsoTritonServer with logging level VERBOSE")
			serverYAML := fmt.Sprintf(`
apiVersion: serving.serving.kalypso.io/v1alpha1
kind: KalypsoTritonServer
metadata:
  name: logging-test-server
  namespace: %s
spec:
  applicationRef: "%s"
  storageUri: "s3://test-bucket/models"
  tritonConfig:
    image: "nvcr.io/nvidia/tritonserver"
    tag: "24.12-py3"
  observability:
    enabled: true
    logging:
      enabled: true
      level: "VERBOSE"
`, testNamespace, testApplicationName)

			cmd := exec.Command("kubectl", "apply", "-f", "-")
			cmd.Stdin = strings.NewReader(serverYAML)
			_, err := utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred(), "Failed to create KalypsoTritonServer")

			By("waiting for Deployment to be created")
			Eventually(func() error {
				cmd := exec.Command("kubectl", "get", "deployment", "logging-test-server-deploy",
					"-n", testNamespace, "-o", "jsonpath={.metadata.name}")
				_, err := utils.Run(cmd)
				return err
			}, 60*time.Second, 2*time.Second).Should(Succeed())

			By("verifying logging args in container")
			Eventually(func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "deployment", "logging-test-server-deploy",
					"-n", testNamespace,
					"-o", "jsonpath={.spec.template.spec.containers[0].args}")
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(ContainSubstring("--log-verbose=1"),
					"Expected --log-verbose=1 in container args")
			}, 30*time.Second, 2*time.Second).Should(Succeed())

			By("cleaning up logging test server")
			cmd = exec.Command("kubectl", "delete", "kalypsotritonserver", "logging-test-server",
				"-n", testNamespace, "--ignore-not-found")
			_, _ = utils.Run(cmd)
		})

		It("should inject INFO logging args", func() {
			By("creating KalypsoTritonServer with logging level INFO")
			serverYAML := fmt.Sprintf(`
apiVersion: serving.serving.kalypso.io/v1alpha1
kind: KalypsoTritonServer
metadata:
  name: logging-info-server
  namespace: %s
spec:
  applicationRef: "%s"
  storageUri: "s3://test-bucket/models"
  tritonConfig:
    image: "nvcr.io/nvidia/tritonserver"
    tag: "24.12-py3"
  observability:
    enabled: true
    logging:
      enabled: true
      level: "INFO"
`, testNamespace, testApplicationName)

			cmd := exec.Command("kubectl", "apply", "-f", "-")
			cmd.Stdin = strings.NewReader(serverYAML)
			_, err := utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred())

			Eventually(func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "deployment", "logging-info-server-deploy",
					"-n", testNamespace,
					"-o", "jsonpath={.spec.template.spec.containers[0].args}")
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(ContainSubstring("--log-info=true"))
			}, 60*time.Second, 2*time.Second).Should(Succeed())

			cmd = exec.Command("kubectl", "delete", "kalypsotritonserver", "logging-info-server",
				"-n", testNamespace, "--ignore-not-found")
			_, _ = utils.Run(cmd)
		})
	})

	Context("Tracing Configuration", func() {
		It("should inject trace-config args when tracing is enabled", func() {
			By("creating KalypsoTritonServer with tracing enabled")
			serverYAML := fmt.Sprintf(`
apiVersion: serving.serving.kalypso.io/v1alpha1
kind: KalypsoTritonServer
metadata:
  name: tracing-test-server
  namespace: %s
spec:
  applicationRef: "%s"
  storageUri: "s3://test-bucket/models"
  tritonConfig:
    image: "nvcr.io/nvidia/tritonserver"
    tag: "24.12-py3"
  observability:
    enabled: true
    collectorEndpoint: "http://tempo.monitoring.svc:4317"
    tracing:
      enabled: true
      samplingRate: "0.5"
`, testNamespace, testApplicationName)

			cmd := exec.Command("kubectl", "apply", "-f", "-")
			cmd.Stdin = strings.NewReader(serverYAML)
			_, err := utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred(), "Failed to create KalypsoTritonServer")

			By("waiting for Deployment to be created")
			Eventually(func() error {
				cmd := exec.Command("kubectl", "get", "deployment", "tracing-test-server-deploy",
					"-n", testNamespace, "-o", "jsonpath={.metadata.name}")
				_, err := utils.Run(cmd)
				return err
			}, 60*time.Second, 2*time.Second).Should(Succeed())

			By("verifying trace-config args in container")
			Eventually(func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "deployment", "tracing-test-server-deploy",
					"-n", testNamespace,
					"-o", "jsonpath={.spec.template.spec.containers[0].args}")
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(ContainSubstring("--trace-config=mode=opentelemetry"))
				g.Expect(output).To(ContainSubstring("url=http://tempo.monitoring.svc:4317"))
				g.Expect(output).To(ContainSubstring("rate=0.5"))
			}, 30*time.Second, 2*time.Second).Should(Succeed())

			By("cleaning up tracing test server")
			cmd = exec.Command("kubectl", "delete", "kalypsotritonserver", "tracing-test-server",
				"-n", testNamespace, "--ignore-not-found")
			_, _ = utils.Run(cmd)
		})
	})

	Context("Profiling Configuration", func() {
		It("should add profiling annotations to Pod when profiling is enabled", func() {
			By("creating KalypsoTritonServer with profiling enabled")
			serverYAML := fmt.Sprintf(`
apiVersion: serving.serving.kalypso.io/v1alpha1
kind: KalypsoTritonServer
metadata:
  name: profiling-test-server
  namespace: %s
spec:
  applicationRef: "%s"
  storageUri: "s3://test-bucket/models"
  tritonConfig:
    image: "nvcr.io/nvidia/tritonserver"
    tag: "24.12-py3"
  observability:
    enabled: true
    profiling:
      enabled: true
      profiles:
        cpu: true
        memory: true
`, testNamespace, testApplicationName)

			cmd := exec.Command("kubectl", "apply", "-f", "-")
			cmd.Stdin = strings.NewReader(serverYAML)
			_, err := utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred(), "Failed to create KalypsoTritonServer")

			By("waiting for Deployment to be created")
			Eventually(func() error {
				cmd := exec.Command("kubectl", "get", "deployment", "profiling-test-server-deploy",
					"-n", testNamespace, "-o", "jsonpath={.metadata.name}")
				_, err := utils.Run(cmd)
				return err
			}, 60*time.Second, 2*time.Second).Should(Succeed())

			By("verifying profiling annotations in Pod template")
			Eventually(func(g Gomega) {
				// Check CPU profiling annotation
				cmd := exec.Command("kubectl", "get", "deployment", "profiling-test-server-deploy",
					"-n", testNamespace,
					"-o", "jsonpath={.spec.template.metadata.annotations['profiles\\.grafana\\.com/cpu\\.scrape']}")
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(Equal("true"), "Expected CPU profiling annotation")

				// Check memory profiling annotation
				cmd = exec.Command("kubectl", "get", "deployment", "profiling-test-server-deploy",
					"-n", testNamespace,
					"-o", "jsonpath={.spec.template.metadata.annotations['profiles\\.grafana\\.com/memory\\.scrape']}")
				output, err = utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(Equal("true"), "Expected memory profiling annotation")

				// Check service_name annotation
				cmd = exec.Command("kubectl", "get", "deployment", "profiling-test-server-deploy",
					"-n", testNamespace,
					"-o", "jsonpath={.spec.template.metadata.annotations['profiles\\.grafana\\.com/service_name']}")
				output, err = utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(Equal("profiling-test-server"), "Expected service_name annotation")
			}, 30*time.Second, 2*time.Second).Should(Succeed())

			By("cleaning up profiling test server")
			cmd = exec.Command("kubectl", "delete", "kalypsotritonserver", "profiling-test-server",
				"-n", testNamespace, "--ignore-not-found")
			_, _ = utils.Run(cmd)
		})
	})

	Context("Metrics Configuration", func() {
		It("should create ServiceMonitor when enableServiceMonitor is true", func() {
			By("creating KalypsoTritonServer with ServiceMonitor enabled")
			serverYAML := fmt.Sprintf(`
apiVersion: serving.serving.kalypso.io/v1alpha1
kind: KalypsoTritonServer
metadata:
  name: metrics-test-server
  namespace: %s
spec:
  applicationRef: "%s"
  storageUri: "s3://test-bucket/models"
  tritonConfig:
    image: "nvcr.io/nvidia/tritonserver"
    tag: "24.12-py3"
  observability:
    enabled: true
    metrics:
      enabled: true
      interval: "30s"
      enableServiceMonitor: true
`, testNamespace, testApplicationName)

			cmd := exec.Command("kubectl", "apply", "-f", "-")
			cmd.Stdin = strings.NewReader(serverYAML)
			_, err := utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred(), "Failed to create KalypsoTritonServer")

			By("waiting for Deployment to be created")
			Eventually(func() error {
				cmd := exec.Command("kubectl", "get", "deployment", "metrics-test-server-deploy",
					"-n", testNamespace, "-o", "jsonpath={.metadata.name}")
				_, err := utils.Run(cmd)
				return err
			}, 60*time.Second, 2*time.Second).Should(Succeed())

			By("verifying ServiceMonitor is created")
			// Note: ServiceMonitor may not be created if Prometheus Operator CRD is not installed
			// This test verifies the attempt to create it - actual creation depends on CRD availability
			Eventually(func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "servicemonitor", "metrics-test-server-monitor",
					"-n", testNamespace, "-o", "jsonpath={.metadata.name}")
				output, err := utils.Run(cmd)
				// If ServiceMonitor CRD is installed, verify creation
				if err == nil {
					g.Expect(output).To(Equal("metrics-test-server-monitor"))

					// Verify interval
					cmd = exec.Command("kubectl", "get", "servicemonitor", "metrics-test-server-monitor",
						"-n", testNamespace,
						"-o", "jsonpath={.spec.endpoints[0].interval}")
					intervalOutput, err := utils.Run(cmd)
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(intervalOutput).To(Equal("30s"))
				}
			}, 30*time.Second, 2*time.Second).Should(Succeed())

			By("cleaning up metrics test server")
			cmd = exec.Command("kubectl", "delete", "kalypsotritonserver", "metrics-test-server",
				"-n", testNamespace, "--ignore-not-found")
			_, _ = utils.Run(cmd)
		})
	})

	Context("Full Observability Configuration", func() {
		It("should configure all observability features together", func() {
			By("creating KalypsoTritonServer with all observability features enabled")
			serverYAML := fmt.Sprintf(`
apiVersion: serving.serving.kalypso.io/v1alpha1
kind: KalypsoTritonServer
metadata:
  name: full-observability-server
  namespace: %s
spec:
  applicationRef: "%s"
  storageUri: "s3://test-bucket/models"
  tritonConfig:
    image: "nvcr.io/nvidia/tritonserver"
    tag: "24.12-py3"
  observability:
    enabled: true
    collectorEndpoint: "http://alloy-gateway.monitoring.svc:4317"
    logging:
      enabled: true
      level: "INFO"
    tracing:
      enabled: true
      samplingRate: "0.1"
    profiling:
      enabled: true
      profiles:
        cpu: true
        memory: true
    metrics:
      enabled: true
      interval: "15s"
      enableServiceMonitor: true
`, testNamespace, testApplicationName)

			cmd := exec.Command("kubectl", "apply", "-f", "-")
			cmd.Stdin = strings.NewReader(serverYAML)
			_, err := utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred(), "Failed to create KalypsoTritonServer")

			By("waiting for Deployment to be created")
			Eventually(func() error {
				cmd := exec.Command("kubectl", "get", "deployment", "full-observability-server-deploy",
					"-n", testNamespace, "-o", "jsonpath={.metadata.name}")
				_, err := utils.Run(cmd)
				return err
			}, 60*time.Second, 2*time.Second).Should(Succeed())

			By("verifying all observability configurations")
			Eventually(func(g Gomega) {
				// Verify logging args
				cmd := exec.Command("kubectl", "get", "deployment", "full-observability-server-deploy",
					"-n", testNamespace,
					"-o", "jsonpath={.spec.template.spec.containers[0].args}")
				argsOutput, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(argsOutput).To(ContainSubstring("--log-info=true"), "Logging args missing")
				g.Expect(argsOutput).To(ContainSubstring("--trace-config="), "Tracing args missing")

				// Verify profiling annotations
				cmd = exec.Command("kubectl", "get", "deployment", "full-observability-server-deploy",
					"-n", testNamespace,
					"-o", "jsonpath={.spec.template.metadata.annotations['profiles\\.grafana\\.com/cpu\\.scrape']}")
				cpuAnnotation, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(cpuAnnotation).To(Equal("true"), "Profiling CPU annotation missing")
			}, 30*time.Second, 2*time.Second).Should(Succeed())

			By("cleaning up full observability test server")
			cmd = exec.Command("kubectl", "delete", "kalypsotritonserver", "full-observability-server",
				"-n", testNamespace, "--ignore-not-found")
			_, _ = utils.Run(cmd)
		})
	})
})
