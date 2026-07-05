package services_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lihai1/stat-tree-server/internal/services"
	lotteryv1 "github.com/lihai1/stat-tree-server/pkg/gen"
)

var _ = Describe("LotteryService", func() {
	var (
		service *services.LotteryService
		ctx     context.Context
	)

	BeforeEach(func() {
		service = services.NewLotteryService()
		ctx = context.Background()
	})

	Describe("GenerateForm", func() {
		Context("with valid request", func() {
			It("should generate form successfully", func() {
				req := &lotteryv1.GenerateFormRequest{
					Numbers:      []int32{1, 2, 3},
					Count:        6,
					AnalysisType: "frequency",
				}

				resp, err := service.GenerateForm(ctx, req)

				Expect(err).ToNot(HaveOccurred())
				Expect(resp).ToNot(BeNil())
				Expect(resp.Success).To(BeTrue())
			})
		})

		Context("with empty request", func() {
			It("should handle empty request", func() {
				req := &lotteryv1.GenerateFormRequest{}

				resp, err := service.GenerateForm(ctx, req)

				Expect(err).ToNot(HaveOccurred())
				Expect(resp).ToNot(BeNil())
				Expect(resp.Success).To(BeTrue())
			})
		})

		Context("with negative count", func() {
			It("should return error for negative count", func() {
				req := &lotteryv1.GenerateFormRequest{
					Count: -1,
				}

				resp, err := service.GenerateForm(ctx, req)

				Expect(err).ToNot(HaveOccurred())
				Expect(resp).ToNot(BeNil())
				Expect(resp.Success).To(BeFalse())
				Expect(resp.Message).To(Equal("Count must be non-negative"))
			})
		})
	})

	Describe("GetStatistics", func() {
		Context("with valid request", func() {
			It("should calculate statistics successfully", func() {
				req := &lotteryv1.GetStatisticsRequest{
					Numbers:      []int32{1, 2, 3, 4, 5},
					AnalysisType: "pairs",
				}

				resp, err := service.GetStatistics(ctx, req)

				Expect(err).ToNot(HaveOccurred())
				Expect(resp).ToNot(BeNil())
				Expect(resp.Success).To(BeTrue())
			})
		})

		Context("with empty request", func() {
			It("should handle empty request", func() {
				req := &lotteryv1.GetStatisticsRequest{}

				resp, err := service.GetStatistics(ctx, req)

				Expect(err).ToNot(HaveOccurred())
				Expect(resp).ToNot(BeNil())
				Expect(resp.Success).To(BeTrue())
			})
		})
	})

	Describe("Analyze", func() {
		Context("with valid request", func() {
			It("should analyze numbers successfully", func() {
				req := &lotteryv1.AnalyzeRequest{
					UserNumbers: []int32{1, 2, 3, 4, 5, 6},
					Historical:  []int32{7, 8, 9, 10, 11, 12},
				}

				resp, err := service.Analyze(ctx, req)

				Expect(err).ToNot(HaveOccurred())
				Expect(resp).ToNot(BeNil())
				Expect(resp.Success).To(BeTrue())
			})
		})

		Context("with empty request", func() {
			It("should handle empty request", func() {
				req := &lotteryv1.AnalyzeRequest{}

				resp, err := service.Analyze(ctx, req)

				Expect(err).ToNot(HaveOccurred())
				Expect(resp).ToNot(BeNil())
				Expect(resp.Success).To(BeTrue())
			})
		})
	})
})
