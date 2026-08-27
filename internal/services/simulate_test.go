package services_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lihai1/stat-tree-server/internal/services"
	lotteryv1 "github.com/lihai1/stat-tree-server/pkg/gen"
)

var _ = Describe("Simulate", func() {
	var (
		service *services.LotteryService
		ctx     context.Context
	)

	BeforeEach(func() {
		service = services.NewLotteryService(nil)
		ctx = context.Background()
	})

	Describe("with no archive (nil repo)", func() {
		It("should return empty results and zero summary", func() {
			req := &lotteryv1.SimulateRequest{
				Form:   []int32{1, 2, 3, 4, 5, 6},
				Strong: 1,
			}

			resp, err := service.Simulate(ctx, req)

			Expect(err).ToNot(HaveOccurred())
			Expect(resp).ToNot(BeNil())
			Expect(resp.GetDraws()).To(BeEmpty())
			Expect(resp.GetSummary().GetTotalDraws()).To(Equal(int32(0)))
			Expect(resp.GetSummary().GetTotalSpent()).To(Equal(0.0))
			Expect(resp.GetSummary().GetTotalWon()).To(Equal(0.0))
		})

		It("should return 8 tier summaries even with no draws", func() {
			req := &lotteryv1.SimulateRequest{
				Form:   []int32{1, 2, 3, 4, 5, 6},
				Strong: 1,
			}

			resp, err := service.Simulate(ctx, req)

			Expect(err).ToNot(HaveOccurred())
			Expect(resp.GetSummary().GetTierSummaries()).To(HaveLen(8))
			Expect(resp.GetSummary().GetTierSummaries()[0].GetLabel()).To(Equal("6+strong"))
			Expect(resp.GetSummary().GetTierSummaries()[7].GetLabel()).To(Equal("3"))
		})
	})

	Describe("validation", func() {
		It("should error when form has fewer than 6 numbers", func() {
			req := &lotteryv1.SimulateRequest{
				Form:   []int32{1, 2, 3},
				Strong: 1,
			}

			_, err := service.Simulate(ctx, req)

			Expect(err).To(HaveOccurred())
		})

		It("should accept exactly 6 numbers", func() {
			req := &lotteryv1.SimulateRequest{
				Form:   []int32{1, 2, 3, 4, 5, 6},
				Strong: 1,
			}

			resp, err := service.Simulate(ctx, req)

			Expect(err).ToNot(HaveOccurred())
			Expect(resp).ToNot(BeNil())
		})

		It("should accept 8 numbers (systematic form)", func() {
			req := &lotteryv1.SimulateRequest{
				Form:   []int32{1, 2, 3, 4, 5, 6, 7, 8},
				Strong: 1,
			}

			resp, err := service.Simulate(ctx, req)

			Expect(err).ToNot(HaveOccurred())
			Expect(resp).ToNot(BeNil())
		})

		It("should accept 12 numbers (systematic form)", func() {
			req := &lotteryv1.SimulateRequest{
				Form:   []int32{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12},
				Strong: 1,
			}

			resp, err := service.Simulate(ctx, req)

			Expect(err).ToNot(HaveOccurred())
			Expect(resp).ToNot(BeNil())
		})
	})

	Describe("prize amount overrides", func() {
		It("should use user-provided prize amounts", func() {
			req := &lotteryv1.SimulateRequest{
				Form:         []int32{1, 2, 3, 4, 5, 6},
				Strong:       1,
				PrizeAmounts: []float64{100, 200, 300, 400, 500, 600, 700, 800},
			}

			resp, err := service.Simulate(ctx, req)

			Expect(err).ToNot(HaveOccurred())
			// Tier summaries should reflect user amounts (even if 0 hits)
			Expect(resp.GetSummary().GetTierSummaries()).To(HaveLen(8))
		})
	})
})
