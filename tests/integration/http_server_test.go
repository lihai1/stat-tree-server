package integration_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	lotteryv1 "github.com/lihai1/stat-tree-server/pkg/gen"
)

var _ = Describe("HTTP Server Integration Tests", func() {
	Describe("GenerateForm", func() {
		It("should generate lottery numbers successfully", func() {
			ctx := context.Background()
			req := &lotteryv1.GenerateFormRequest{
				Numbers:      []int32{1, 2, 3, 4, 5, 6},
				Exclude:      []int32{7, 8, 9},
				Count:        6,
				AnalysisType: "standard",
			}

			resp, err := httpClient.GenerateForm(ctx, req)
			Expect(err).NotTo(HaveOccurred())
			Expect(resp).NotTo(BeNil())
			Expect(resp.Success).To(BeTrue())
			Expect(resp.Numbers).NotTo(BeEmpty())
			Expect(len(resp.Numbers)).To(BeNumerically("==", 6))
		})

		It("should handle invalid count", func() {
			ctx := context.Background()
			req := &lotteryv1.GenerateFormRequest{
				Count: -1,
			}

			resp, err := httpClient.GenerateForm(ctx, req)
			Expect(err).NotTo(HaveOccurred())
			Expect(resp).NotTo(BeNil())
			Expect(resp.Success).To(BeFalse())
			Expect(resp.Message).To(ContainSubstring("Count must be non-negative"))
		})
	})

	Describe("GetStatistics", func() {
		It("should calculate statistics successfully", func() {
			ctx := context.Background()
			req := &lotteryv1.GetStatisticsRequest{
				Numbers:      []int32{1, 2, 3, 4, 5, 6},
				AnalysisType: "standard",
			}

			resp, err := httpClient.GetStatistics(ctx, req)
			Expect(err).NotTo(HaveOccurred())
			Expect(resp).NotTo(BeNil())
			Expect(resp.Success).To(BeTrue())
			Expect(resp.Frequency).NotTo(BeNil())
		})
	})

	Describe("Analyze", func() {
		It("should analyze numbers successfully", func() {
			ctx := context.Background()
			req := &lotteryv1.AnalyzeRequest{
				UserNumbers: []int32{1, 2, 3, 4, 5, 6},
				Historical:  []int32{1, 2, 3, 4, 5, 6, 7, 8, 9, 10},
			}

			resp, err := httpClient.Analyze(ctx, req)
			Expect(err).NotTo(HaveOccurred())
			Expect(resp).NotTo(BeNil())
			Expect(resp.Success).To(BeTrue())
			Expect(resp.Frequency).NotTo(BeNil())
			Expect(resp.Pairs).NotTo(BeNil())
			Expect(resp.Patterns).NotTo(BeEmpty())
		})
	})
})
