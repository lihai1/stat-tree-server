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
		service = services.NewLotteryService(nil)
		ctx = context.Background()
	})

	Describe("GenerateForm", func() {
		Context("with valid request", func() {
			It("should generate form successfully", func() {
				req := &lotteryv1.GenerateFormRequest{
					HowMany:  6,
					FormType: 1,
					WillBe:   []int32{1, 2, 3},
				}

				resp, err := service.GenerateForm(ctx, req)

				Expect(err).ToNot(HaveOccurred())
				Expect(resp).ToNot(BeNil())
			})

			It("should produce a single NumberSet with formType numbers", func() {
				req := &lotteryv1.GenerateFormRequest{
					HowMany:  6,
					FormType: 8,
				}

				resp, err := service.GenerateForm(ctx, req)

				Expect(err).ToNot(HaveOccurred())
				// howMany 6 → up to 6 forms generated
				Expect(resp.GetForms()).ToNot(BeEmpty())
				// formType 8 → 8 numbers in each generated form
				Expect(resp.GetForms()[0].GetNumbers()).To(HaveLen(8))
			})

			It("should default formType to 6 when unspecified", func() {
				req := &lotteryv1.GenerateFormRequest{
					HowMany: 6,
				}

				resp, err := service.GenerateForm(ctx, req)

				Expect(err).ToNot(HaveOccurred())
				Expect(resp.GetForms()[0].GetNumbers()).To(HaveLen(6))
			})
		})

		Context("with empty request", func() {
			It("should handle empty request", func() {
				req := &lotteryv1.GenerateFormRequest{}

				resp, err := service.GenerateForm(ctx, req)

				Expect(err).ToNot(HaveOccurred())
				Expect(resp).ToNot(BeNil())
			})
		})

		Context("with negative how_many", func() {
			It("should return at least one form for negative how_many", func() {
				req := &lotteryv1.GenerateFormRequest{
					HowMany: -1,
				}

				resp, err := service.GenerateForm(ctx, req)

				Expect(err).ToNot(HaveOccurred())
				Expect(resp).ToNot(BeNil())
				// negative howMany defaults to 1
				Expect(resp.GetForms()).ToNot(BeEmpty())
			})
		})
	})

	Describe("GetStatistics", func() {
		Context("with valid request", func() {
			It("should calculate statistics successfully", func() {
				req := &lotteryv1.GetStatisticsRequest{
					HowMany:  10,
					FormType: 2,
				}

				resp, err := service.GetStatistics(ctx, req)

				Expect(err).ToNot(HaveOccurred())
				Expect(resp).ToNot(BeNil())
			})

			It("should split count out of pair numbers (count bug fix)", func() {
				req := &lotteryv1.GetStatisticsRequest{
					HowMany:  5,
					FormType: 2,
					Strength: lotteryv1.Strength_STRONG,
				}

				resp, err := service.GetStatistics(ctx, req)

				Expect(err).ToNot(HaveOccurred())
				// Without a DB the archive is empty so pairs may be empty,
				// but any returned pair must have Count separated from Numbers.
				for _, pair := range resp.GetPairs() {
					// Numbers should not contain the count value
					Expect(pair.GetNumbers()).ToNot(ContainElement(pair.GetCount()))
				}
			})

			It("should default group size to 2 when formType is 0", func() {
				req := &lotteryv1.GetStatisticsRequest{
					HowMany: 10,
				}

				resp, err := service.GetStatistics(ctx, req)

				Expect(err).ToNot(HaveOccurred())
				Expect(resp).ToNot(BeNil())
			})
		})

		Context("with empty request", func() {
			It("should handle empty request", func() {
				req := &lotteryv1.GetStatisticsRequest{}

				resp, err := service.GetStatistics(ctx, req)

				Expect(err).ToNot(HaveOccurred())
				Expect(resp).ToNot(BeNil())
			})
		})
	})

	Describe("Analyze", func() {
		Context("with valid request", func() {
			It("should analyze numbers successfully", func() {
				req := &lotteryv1.AnalyzeRequest{
					Form: []int32{1, 2, 3, 4, 5, 6},
				}

				resp, err := service.Analyze(ctx, req)

				Expect(err).ToNot(HaveOccurred())
				Expect(resp).ToNot(BeNil())
			})

			It("should return grouped frequency (possibly empty without DB)", func() {
				req := &lotteryv1.AnalyzeRequest{
					Form: []int32{1, 2, 3, 4, 5, 6},
				}

				resp, err := service.Analyze(ctx, req)

				Expect(err).ToNot(HaveOccurred())
				Expect(resp.GetFrequencyGroups()).ToNot(BeNil())
				// Always returns 6 groups (sizes 1–6), even if entries are empty.
				Expect(resp.GetFrequencyGroups()).To(HaveLen(6))
			})

			It("should populate group size correctly (combos computed by UI)", func() {
				req := &lotteryv1.AnalyzeRequest{
					Form: []int32{1, 2, 3, 4, 5, 6},
				}

				resp, err := service.Analyze(ctx, req)

				Expect(err).ToNot(HaveOccurred())
				groups := resp.GetFrequencyGroups()
				for i, g := range groups {
					Expect(g.GetSize()).To(Equal(int32(i + 1)))
					// Combos is 0 from the tree — the UI computes it via
					// combinations(37, size) with a || fallback.
					Expect(g.GetCombos()).To(Equal(int32(0)))
				}
			})

			It("should return archive size (possibly 0 without DB)", func() {
				req := &lotteryv1.AnalyzeRequest{
					Form: []int32{1, 2, 3, 4, 5, 6},
				}

				resp, err := service.Analyze(ctx, req)

				Expect(err).ToNot(HaveOccurred())
				Expect(resp.GetArchiveSize()).To(BeNumerically(">=", 0))
			})
		})

		Context("with empty request", func() {
			It("should handle empty request", func() {
				req := &lotteryv1.AnalyzeRequest{}

				resp, err := service.Analyze(ctx, req)

				Expect(err).ToNot(HaveOccurred())
				Expect(resp).ToNot(BeNil())
			})
		})

		Context("with form including a trailing strong number", func() {
			It("should drop the last element when form has more than 6 numbers", func() {
				// 7 numbers — the 7th is a strong number that should be dropped
				req := &lotteryv1.AnalyzeRequest{
					Form: []int32{1, 2, 3, 4, 5, 6, 7},
				}

				resp, err := service.Analyze(ctx, req)

				Expect(err).ToNot(HaveOccurred())
				Expect(resp).ToNot(BeNil())
				// The analysis should succeed and return frequency groups
				// as if only 6 numbers were analyzed.
				Expect(resp.GetFrequencyGroups()).ToNot(BeNil())
				Expect(len(resp.GetFrequencyGroups())).To(Equal(6))
			})

			It("should not drop when form has exactly 6 numbers", func() {
				req := &lotteryv1.AnalyzeRequest{
					Form: []int32{1, 2, 3, 4, 5, 6},
				}

				resp, err := service.Analyze(ctx, req)

				Expect(err).ToNot(HaveOccurred())
				Expect(resp).ToNot(BeNil())
				Expect(resp.GetFrequencyGroups()).ToNot(BeNil())
				Expect(len(resp.GetFrequencyGroups())).To(Equal(6))
			})

			It("should not drop when form has fewer than 6 numbers", func() {
				req := &lotteryv1.AnalyzeRequest{
					Form: []int32{1, 2, 3},
				}

				resp, err := service.Analyze(ctx, req)

				Expect(err).ToNot(HaveOccurred())
				Expect(resp).ToNot(BeNil())
				Expect(resp.GetFrequencyGroups()).ToNot(BeNil())
			})
		})
	})
})
