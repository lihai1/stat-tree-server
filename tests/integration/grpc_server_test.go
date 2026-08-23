package integration_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	lotteryv1 "github.com/lihai1/stat-tree-server/pkg/gen"
)

var _ = Describe("gRPC Server Integration Tests", func() {
	Describe("HealthCheck", func() {
		It("should return healthy status", func() {
			ctx := context.Background()
			req := &lotteryv1.HealthCheckRequest{}

			resp, err := grpcClient.HealthCheck(ctx, req)
			Expect(err).NotTo(HaveOccurred())
			Expect(resp).NotTo(BeNil())
			Expect(resp.Status).To(Equal("healthy"))
			Expect(resp.Version).To(Equal("1.0.0"))
		})
	})

	Describe("GenerateForm", func() {
		It("should generate lottery numbers successfully", func() {
			ctx := context.Background()
			req := &lotteryv1.GenerateFormRequest{
				HowMany:  6,
				FormType: 1,
				WillBe:   []int32{1, 2, 3, 4, 5, 6},
			}

			resp, err := grpcClient.GenerateForm(ctx, req)
			Expect(err).NotTo(HaveOccurred())
			Expect(resp).NotTo(BeNil())
			Expect(resp.GetForms()).NotTo(BeNil())
		})

		It("should handle invalid how_many", func() {
			ctx := context.Background()
			req := &lotteryv1.GenerateFormRequest{
				HowMany: -1,
			}

			resp, err := grpcClient.GenerateForm(ctx, req)
			Expect(err).NotTo(HaveOccurred())
			Expect(resp).NotTo(BeNil())
			Expect(resp.GetForms()).To(BeEmpty())
		})
	})

	Describe("GetStatistics", func() {
		It("should calculate statistics successfully", func() {
			ctx := context.Background()
			req := &lotteryv1.GetStatisticsRequest{
				HowMany:  10,
				FormType: 2,
			}

			resp, err := grpcClient.GetStatistics(ctx, req)
			Expect(err).NotTo(HaveOccurred())
			Expect(resp).NotTo(BeNil())
			// Pairs slice is always initialized, may be empty without historical data
		})
	})

	Describe("Analyze", func() {
		It("should analyze numbers successfully", func() {
			ctx := context.Background()
			req := &lotteryv1.AnalyzeRequest{
				Form: []int32{1, 2, 3, 4, 5, 6},
			}

			resp, err := grpcClient.Analyze(ctx, req)
			Expect(err).NotTo(HaveOccurred())
			Expect(resp).NotTo(BeNil())
			// Frequency map is always initialized, may be empty without historical data
		})
	})
})
