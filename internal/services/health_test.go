package services_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lihai1/stat-tree-server/internal/services"
)

var _ = Describe("HealthCheck", func() {
	It("should return healthy status", func() {
		service := services.NewLotteryService(nil)
		resp, err := service.HealthCheck(context.Background(), nil)

		Expect(err).To(BeNil())
		Expect(resp.Status).To(Equal("healthy"))
		Expect(resp.Version).To(Equal("1.0.0"))
	})
})
