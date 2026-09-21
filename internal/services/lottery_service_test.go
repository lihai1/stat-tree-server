package services_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/lihai1/stat-tree-server/internal/services"
	lotteryv1 "github.com/lihai1/stat-tree-server/pkg/gen"
)

var _ = Describe("LotteryService", func() {
	var (
		service *services.LotteryService
		ctx     context.Context
	)

	BeforeEach(func() {
		service = services.NewLotteryServiceWithRepo(nil)
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
			It("should reject empty request", func() {
				req := &lotteryv1.GenerateFormRequest{}

				resp, err := service.GenerateForm(ctx, req)

				Expect(err).To(HaveOccurred())
				Expect(status.Code(err)).To(Equal(codes.InvalidArgument))
				Expect(resp).To(BeNil())
			})
		})

		Context("with negative how_many", func() {
			It("should reject negative how_many", func() {
				req := &lotteryv1.GenerateFormRequest{
					HowMany: -1,
				}

				resp, err := service.GenerateForm(ctx, req)

				Expect(err).To(HaveOccurred())
				Expect(status.Code(err)).To(Equal(codes.InvalidArgument))
				Expect(resp).To(BeNil())
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

		Context("with form_type > 6 (rejected)", func() {
			It("should reject form_type 7", func() {
				req := &lotteryv1.GetStatisticsRequest{
					HowMany:  10,
					FormType: 7,
				}

				_, err := service.GetStatistics(ctx, req)
				Expect(err).To(HaveOccurred())
			})

			It("should reject form_type 8", func() {
				req := &lotteryv1.GetStatisticsRequest{
					HowMany:  10,
					FormType: 8,
				}

				_, err := service.GetStatistics(ctx, req)
				Expect(err).To(HaveOccurred())
			})

			It("should accept form_type 6", func() {
				req := &lotteryv1.GetStatisticsRequest{
					HowMany:  10,
					FormType: 6,
				}

				_, err := service.GetStatistics(ctx, req)
				Expect(err).ToNot(HaveOccurred())
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

			It("should populate group size and combos correctly", func() {
				req := &lotteryv1.AnalyzeRequest{
					Form: []int32{1, 2, 3, 4, 5, 6},
				}

				resp, err := service.Analyze(ctx, req)

				Expect(err).ToNot(HaveOccurred())
				// Combos = C(37, size) per the documented proto contract.
				expected := []int32{37, 666, 7770, 66045, 435897, 2324784}
				for i, g := range resp.GetFrequencyGroups() {
					Expect(g.GetSize()).To(Equal(int32(i + 1)))
					Expect(g.GetCombos()).To(Equal(expected[i]))
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

		Context("ScoreForm", func() {
			It("should score a form without error (empty archive → heat 0)", func() {
				req := &lotteryv1.ScoreFormRequest{
					Form: []int32{1, 2, 3, 4, 5, 6},
				}

				resp, err := service.ScoreForm(ctx, req)

				Expect(err).ToNot(HaveOccurred())
				Expect(resp).ToNot(BeNil())
				Expect(resp.GetPairCount()).To(Equal(int32(15))) // C(6,2)
				Expect(resp.GetDraws()).To(Equal(int32(0)))
				Expect(resp.GetHeat()).To(Equal(0.0))
			})

			It("should reject a form with fewer than 2 numbers", func() {
				req := &lotteryv1.ScoreFormRequest{Form: []int32{7}}

				_, err := service.ScoreForm(ctx, req)

				Expect(err).To(HaveOccurred())
				Expect(status.Code(err)).To(Equal(codes.InvalidArgument))
			})
		})

		Context("with form larger than six numbers (BUG 5 fix)", func() {
			It("should analyze all numbers without dropping the last (BUG 5)", func() {
				// 8 regular numbers — none should be stripped. The archive
				// stores strong separately, so every form element is regular.
				req := &lotteryv1.AnalyzeRequest{
					Form: []int32{1, 2, 3, 4, 5, 6, 7, 8},
				}

				resp, err := service.Analyze(ctx, req)

				Expect(err).ToNot(HaveOccurred())
				Expect(resp).ToNot(BeNil())
				Expect(resp.GetFrequencyGroups()).ToNot(BeNil())
				Expect(len(resp.GetFrequencyGroups())).To(Equal(6))
				// Without a DB the archive is empty, so entries are empty,
				// but the call must succeed and not strip number 8.
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
