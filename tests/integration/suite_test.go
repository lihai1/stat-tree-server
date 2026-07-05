package integration_test

import (
	"github.com/lihai1/stat-tree-server/internal/config"
	"github.com/lihai1/stat-tree-server/internal/startup"
	"testing"
	"time"

	grpcclient "github.com/lihai1/stat-tree-server/pkg/clients/grpc"
	httpclient "github.com/lihai1/stat-tree-server/pkg/clients/http"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestIntegration(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Integration Suite")
}

var (
	server     *startup.Server
	grpcClient *grpcclient.Client
	httpClient *httpclient.Client
	testConfig *config.Config
)

var _ = BeforeSuite(func() {

	// Load test configuration
	cfg, err := config.Load()
	Expect(err).NotTo(HaveOccurred())
	testConfig = cfg

	// Start the server
	server, err = startup.NewServer(testConfig)
	Expect(err).NotTo(HaveOccurred())

	err = server.Start()
	Expect(err).NotTo(HaveOccurred())

	// Wait for servers to be ready
	time.Sleep(2 * time.Second)

	// Create gRPC client
	grpcClient, err = grpcclient.NewClient("localhost:" + testConfig.Server.GRPCPort)
	Expect(err).NotTo(HaveOccurred())

	// Create HTTP client
	httpClient = httpclient.NewClient("http://localhost:" + testConfig.Server.GatewayPort)
})

var _ = AfterSuite(func() {
	// Close clients
	if grpcClient != nil {
		grpcClient.Close()
	}
	if httpClient != nil {
		httpClient.Close()
	}

	// Stop server (ignore shutdown errors during tests)
	if server != nil {
		_ = server.Stop()
	}
})
