package integration_test

import (
	"context"
	"io"
	"net/http"
	"strings"

	lotteryv1 "github.com/lihai1/stat-tree-server/pkg/gen"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// rawRequest performs a plain HTTP request against the running gateway with
// no Authorization header, returning the status code and response body. This
// is used to exercise the auth middleware end-to-end through the real server.
func rawRequest(method, path string, body string) (int, string) {
	baseURL := "http://localhost:" + testConfig.Server.GatewayPort
	var bodyReader io.Reader
	if body != "" {
		bodyReader = strings.NewReader(body)
	}
	req, err := http.NewRequestWithContext(context.Background(), method, baseURL+path, bodyReader)
	Expect(err).NotTo(HaveOccurred())
	req.Header.Del("Authorization")
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := http.DefaultClient.Do(req)
	Expect(err).NotTo(HaveOccurred())
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	Expect(err).NotTo(HaveOccurred())
	return resp.StatusCode, string(respBody)
}

var _ = Describe("Auth Middleware Integration Tests", func() {
	Describe("protected endpoints without auth", func() {
		It("should return 401 for POST /api/generate/form without Authorization", func() {
			if !testConfig.Auth.Enabled {
				Skip("AUTH_ENABLED=false — auth middleware is disabled")
			}
			status, _ := rawRequest(
				http.MethodPost,
				"/api/generate/form",
				`{"how_many":6,"form_type":1,"will_be":[1,2,3,4,5,6]}`,
			)
			Expect(status).To(Equal(http.StatusUnauthorized))
		})
	})

	Describe("open endpoints without auth", func() {
		It("should return 200 for GET /health without Authorization", func() {
			status, body := rawRequest(http.MethodGet, "/health", "")
			Expect(status).To(Equal(http.StatusOK))
			Expect(body).To(ContainSubstring("healthy"))
		})
	})

	Describe("gRPC endpoint without auth metadata", func() {
		It("should return Unauthenticated error for GenerateForm without authorization metadata", func() {
			if !testConfig.Auth.Enabled {
				Skip("AUTH_ENABLED=false — gRPC auth interceptor is disabled")
			}
			ctx := context.Background()
			req := &lotteryv1.GenerateFormRequest{
				HowMany:  6,
				FormType: 1,
				WillBe:   []int32{1, 2, 3, 4, 5, 6},
			}
			_, err := grpcClient.GenerateForm(ctx, req)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("Unauthenticated"))
		})
	})
})
