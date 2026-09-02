package lotterytree_test

import (
	"fmt"
	"sort"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	. "github.com/lihai1/stat-tree-server/internal/lottery-tree"
	"github.com/lihai1/stat-tree-server/internal/models"
	lotteryv1 "github.com/lihai1/stat-tree-server/pkg/gen"
)

var _ = Describe("LoTree", func() {
	Context("NewLoTree", func() {
		It("should create a new tree with correct lottery size", func() {
			tree := NewLoTree(37)
			Expect(tree).NotTo(BeNil())
			Expect(tree.Lottery).To(Equal(37))
			Expect(tree.Anchor).NotTo(BeNil())
		})
	})

	Context("Build", func() {
		It("should build tree from data", func() {
			tree := NewLoTree(37)
			data := [][]int{
				{1, 2, 3, 4, 5, 6},
				{7, 8, 9, 10, 11, 12},
			}
			tree.Build(data, 6)
			Expect(tree.BuildCalls).To(Equal(2))
		})
	})

	Context("Build depth and metadata", func() {
		It("should record BuildCalls equal to the number of draws", func() {
			tree := NewLoTree(37)
			tree.Build([][]int{{1, 2, 3, 4, 5, 6}, {7, 8, 9, 10, 11, 12}}, 6)
			Expect(tree.BuildCalls).To(Equal(2))
		})
	})

	Context("CollectNodes", func() {
		It("should collect single-number frequencies sorted by count descending", func() {
			tree := NewLoTree(37)
			tree.Build([][]int{
				{1, 2, 3, 4, 5, 6},
				{1, 7, 8, 9, 10, 11},
			}, 6)
			entries := tree.CollectNodes(1, string(Strong))
			Expect(entries).ToNot(BeEmpty())
			// Number 1 appears twice; all others once. Strong mode → 1 first.
			Expect(entries[0].Numbers).To(Equal([]int{1}))
			Expect(entries[0].Count).To(Equal(2))
		})

		It("should sort least-frequent first in weak mode", func() {
			tree := NewLoTree(37)
			tree.Build([][]int{
				{1, 2, 3, 4, 5, 6},
				{1, 7, 8, 9, 10, 11},
			}, 6)
			entries := tree.CollectNodes(1, string(Weak))
			Expect(entries).ToNot(BeEmpty())
			// Weak mode → number 1 (count 2) is last.
			Expect(entries[len(entries)-1].Numbers).To(Equal([]int{1}))
		})

		It("should collect pairs with correct counts", func() {
			tree := NewLoTree(37)
			tree.Build([][]int{
				{1, 2, 3, 4, 5, 6},
				{1, 2, 7, 8, 9, 10},
			}, 6)
			entries := tree.CollectNodes(2, string(Strong))
			// Pair {1,2} appears in both draws.
			Expect(entries[0].Numbers).To(Equal([]int{1, 2}))
			Expect(entries[0].Count).To(Equal(2))
		})

		It("should return nil for invalid size", func() {
			tree := NewLoTree(37)
			tree.Build([][]int{{1, 2, 3, 4, 5, 6}}, 6)
			Expect(tree.CollectNodes(0, string(Strong))).To(BeNil())
			Expect(tree.CollectNodes(-1, string(Strong))).To(BeNil())
		})
	})
})

var _ = Describe("Types", func() {
	Context("Common", func() {
		It("should have correct string values", func() {
			Expect(string(Strong)).To(Equal("strong"))
			Expect(string(Weak)).To(Equal("weak"))
			Expect(string(Random)).To(Equal("random"))
		})
	})

	Context("LotteryType", func() {
		It("should have correct string values", func() {
			Expect(string(Lottery)).To(Equal("lottery"))
			Expect(string(TripleSeven)).To(Equal("777"))
			Expect(string(OneTwoThree)).To(Equal("123"))
		})
	})
})

var _ = Describe("LotteryArchive", func() {
	// Helper: collect numbers from a group's entries into a sorted slice of
	// string keys for easy comparison.
	entryKeys := func(entries []*lotteryv1.FrequencyEntry) []string {
		keys := make([]string, 0, len(entries))
		for _, e := range entries {
			sorted := make([]int32, len(e.GetNumbers()))
			copy(sorted, e.GetNumbers())
			sort.Slice(sorted, func(a, b int) bool { return sorted[a] < sorted[b] })
			keys = append(keys, fmt.Sprintf("%v", sorted))
		}
		sort.Strings(keys)
		return keys
	}

	draws := func(nums [][]int) []models.LotteryResult {
		out := make([]models.LotteryResult, 0, len(nums))
		for i, n := range nums {
			strong := 0
			row := n
			// Allow a trailing strong in the test input.
			if len(n) > 6 {
				strong = n[len(n)-1]
				row = n[:6]
			}
			out = append(out, models.LotteryResult{
				DrawNumber: i + 1,
				DrawDate:   time.Date(2020, 1, i+1, 0, 0, 0, 0, time.UTC),
				Numbers:    row,
				Strong:     strong,
			})
		}
		return out
	}

	Context("NewLotteryArchive", func() {
		It("should build trees and winning-combo set from draws", func() {
			arch := NewLotteryArchive(Lottery, time.Time{}, time.Time{},
				draws([][]int{{1, 2, 3, 4, 5, 6}, {1, 2, 3, 7, 8, 9}}))
			Expect(arch).NotTo(BeNil())
			Expect(arch.Size()).To(Equal(2))
			Expect(arch.Tree).NotTo(BeNil())
			Expect(arch.StrongTree).NotTo(BeNil())
			Expect(arch.WinningCombos).To(HaveLen(2))
		})

		It("should handle an empty draw set", func() {
			arch := NewLotteryArchive(Lottery, time.Time{}, time.Time{}, nil)
			Expect(arch.Size()).To(Equal(0))
			Expect(arch.WinningCombos).To(BeEmpty())
		})
	})

	Context("RankNumbers", func() {
		It("should rank most frequent first in strong mode", func() {
			arch := NewLotteryArchive(Lottery, time.Time{}, time.Time{},
				draws([][]int{{1, 2, 3, 4, 5, 6}, {1, 7, 8, 9, 10, 11}}))
			ranked := arch.RankNumbers(string(Strong))
			Expect(ranked[0]).To(Equal(1)) // 1 appears twice
		})

		It("should rank least frequent first in weak mode", func() {
			arch := NewLotteryArchive(Lottery, time.Time{}, time.Time{},
				draws([][]int{{1, 2, 3, 4, 5, 6}, {1, 7, 8, 9, 10, 11}}))
			ranked := arch.RankNumbers(string(Weak))
			// 1 has count 2, so it should be last in weak mode.
			Expect(ranked[len(ranked)-1]).To(Equal(1))
		})
	})

	Context("TopStrong", func() {
		It("should return the most frequent strong number in strong mode", func() {
			arch := NewLotteryArchive(Lottery, time.Time{}, time.Time{},
				draws([][]int{{1, 2, 3, 4, 5, 6, 5}, {1, 2, 3, 7, 8, 9, 3}}))
			// Strong 5 appears once, strong 3 appears once. TopStrong(strong)
			// returns one of them; both have count 1 so either is valid.
			top := arch.TopStrong(string(Strong))
			Expect(top).To(BeNumerically(">", 0))
		})

		It("should return 0 when no strong numbers are in range", func() {
			arch := NewLotteryArchive(Lottery, time.Time{}, time.Time{},
				draws([][]int{{1, 2, 3, 4, 5, 6, 9}}))
			// Strong 9 is outside 1..7, so TopStrong returns 0.
			Expect(arch.TopStrong(string(Strong))).To(Equal(0))
		})
	})

	Context("HasWon", func() {
		It("should detect a winning combination", func() {
			arch := NewLotteryArchive(Lottery, time.Time{}, time.Time{},
				draws([][]int{{1, 2, 3, 4, 5, 6}}))
			Expect(arch.HasWon([]int{6, 5, 4, 3, 2, 1})).To(BeTrue()) // order-independent
		})

		It("should reject a non-winning combination", func() {
			arch := NewLotteryArchive(Lottery, time.Time{}, time.Time{},
				draws([][]int{{1, 2, 3, 4, 5, 6}}))
			Expect(arch.HasWon([]int{1, 2, 3, 4, 5, 7})).To(BeFalse())
		})
	})

	Context("AnalyzeForm", func() {
		var arch *LotteryArchive

		BeforeEach(func() {
			arch = NewLotteryArchive(Lottery, time.Time{}, time.Time{},
				draws([][]int{{1, 2, 3, 4, 5, 6}, {1, 2, 3, 7, 8, 9}}))
		})

		It("should return an AnalyzeResponse with 6 groups and archive size", func() {
			resp := arch.AnalyzeForm([]int{1, 2, 3})
			Expect(resp).NotTo(BeNil())
			Expect(resp.GetArchiveSize()).To(Equal(int32(2)))
			groups := resp.GetFrequencyGroups()
			Expect(groups).To(HaveLen(6))
			for i, g := range groups {
				Expect(int(g.GetSize())).To(Equal(i + 1))
			}
		})

		It("should group size-1 results with correct counts", func() {
			resp := arch.AnalyzeForm([]int{1, 2, 3})
			entries := resp.GetFrequencyGroups()[0].GetEntries()
			Expect(entries).To(HaveLen(3))
			Expect(entryKeys(entries)).To(Equal([]string{"[1]", "[2]", "[3]"}))
			for _, e := range entries {
				Expect(e.GetCount()).To(Equal(int32(2)), "size-1 count for %v", e.GetNumbers())
			}
		})

		It("should group size-2 results with correct counts", func() {
			resp := arch.AnalyzeForm([]int{1, 2, 3})
			entries := resp.GetFrequencyGroups()[1].GetEntries()
			Expect(entries).To(HaveLen(3))
			Expect(entryKeys(entries)).To(Equal([]string{"[1 2]", "[1 3]", "[2 3]"}))
			for _, e := range entries {
				Expect(e.GetCount()).To(Equal(int32(2)), "size-2 count for %v", e.GetNumbers())
			}
		})

		It("should group size-3 results with correct counts", func() {
			resp := arch.AnalyzeForm([]int{1, 2, 3})
			entries := resp.GetFrequencyGroups()[2].GetEntries()
			Expect(entries).To(HaveLen(1))
			Expect(entries[0].GetNumbers()).To(Equal([]int32{1, 2, 3}))
			Expect(entries[0].GetCount()).To(Equal(int32(2)))
		})

		It("should not produce duplicate subsets within any group", func() {
			resp := arch.AnalyzeForm([]int{1, 2, 3, 4, 5, 6})
			for _, g := range resp.GetFrequencyGroups() {
				seen := map[string]bool{}
				for _, e := range g.GetEntries() {
					key := fmt.Sprintf("%v", e.GetNumbers())
					Expect(seen[key]).To(BeFalse(),
						"duplicate subset at size %d: %v", g.GetSize(), e.GetNumbers())
					seen[key] = true
				}
			}
		})

		It("should produce entries where Numbers length matches the group size", func() {
			resp := arch.AnalyzeForm([]int{1, 2, 3, 4, 5, 6})
			for _, g := range resp.GetFrequencyGroups() {
				for _, e := range g.GetEntries() {
					Expect(len(e.GetNumbers())).To(Equal(int(g.GetSize())),
						"size %d entry has %d numbers: %v",
						g.GetSize(), len(e.GetNumbers()), e.GetNumbers())
				}
			}
		})

		It("should sort entries by count descending within each group", func() {
			a := NewLotteryArchive(Lottery, time.Time{}, time.Time{},
				draws([][]int{{1, 2, 3, 4, 5, 6}, {1, 7, 8, 9, 10, 11}}))
			resp := a.AnalyzeForm([]int{1, 2, 3})
			entries := resp.GetFrequencyGroups()[0].GetEntries()
			Expect(entries).To(HaveLen(3))
			Expect(entries[0].GetNumbers()).To(Equal([]int32{1}))
			Expect(entries[0].GetCount()).To(Equal(int32(2)))
		})

		It("should analyze all 8 regular numbers without dropping the 8th (BUG 5)", func() {
			a := NewLotteryArchive(Lottery, time.Time{}, time.Time{},
				draws([][]int{{1, 2, 3, 4, 5, 6}, {1, 2, 3, 7, 8, 9}}))
			resp := a.AnalyzeForm([]int{1, 2, 3, 4, 5, 6, 7, 8})
			// Size-1 group must contain all 8 numbers, including 8.
			entries := resp.GetFrequencyGroups()[0].GetEntries()
			nums := entryKeys(entries)
			Expect(nums).To(ContainElement("[8]"))
			Expect(len(nums)).To(Equal(8))
		})
	})

	Context("with an empty archive", func() {
		It("should return 6 empty groups and archive size 0", func() {
			arch := NewLotteryArchive(Lottery, time.Time{}, time.Time{}, nil)
			resp := arch.AnalyzeForm([]int{1, 2, 3})
			Expect(resp.GetArchiveSize()).To(Equal(int32(0)))
			groups := resp.GetFrequencyGroups()
			Expect(groups).To(HaveLen(6))
			for _, g := range groups {
				Expect(g.GetEntries()).To(BeEmpty())
			}
		})
	})
})

var _ = Describe("FormGenerator", func() {
	// multiDrawArchive builds an archive with two non-overlapping draws so
	// the generator has 12 candidate numbers and two winning combos to avoid.
	multiDrawArchive := func() *LotteryArchive {
		return NewLotteryArchive(Lottery, time.Time{}, time.Time{},
			[]models.LotteryResult{
				{DrawNumber: 1, DrawDate: time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC),
					Numbers: []int{1, 2, 3, 4, 5, 6}, Strong: 5},
				{DrawNumber: 2, DrawDate: time.Date(2020, 1, 2, 0, 0, 0, 0, time.UTC),
					Numbers: []int{7, 8, 9, 10, 11, 12}, Strong: 3},
			})
	}

	Context("Generate", func() {
		It("should produce non-winning forms with the strong number appended", func() {
			arch := multiDrawArchive()
			gen := arch.NewFormGenerator(6, string(Strong))
			forms := gen.Generate(3, nil)
			Expect(forms).ToNot(BeEmpty())
			for _, f := range forms {
				// 6 regular + 1 strong = 7 elements.
				Expect(f).To(HaveLen(7))
				// The strong number is the last element.
				strong := f[len(f)-1]
				Expect(strong).To(BeNumerically(">", 0))
				// The form must not have won before.
				Expect(arch.HasWon(f[:6])).To(BeFalse())
			}
		})

		It("should include willBe numbers in every form", func() {
			arch := multiDrawArchive()
			gen := arch.NewFormGenerator(6, string(Strong))
			forms := gen.Generate(2, []int{1, 2})
			Expect(forms).ToNot(BeEmpty())
			for _, f := range forms {
				regular := f[:6]
				Expect(regular).To(ContainElement(1))
				Expect(regular).To(ContainElement(2))
			}
		})

		It("should not produce duplicate forms", func() {
			arch := multiDrawArchive()
			gen := arch.NewFormGenerator(6, string(Strong))
			forms := gen.Generate(5, nil)
			seen := map[uint64]bool{}
			for _, f := range forms {
				mask := ComboMask(f[:6])
				Expect(seen[mask]).To(BeFalse(), "duplicate form generated")
				seen[mask] = true
			}
		})
	})
})

var _ = Describe("GenerateCombinations", func() {
	It("should generate C(n,k) combinations", func() {
		combos := GenerateCombinations([]int{1, 2, 3, 4}, 2)
		Expect(combos).To(HaveLen(6)) // C(4,2) = 6
	})

	It("should return nil when k > n", func() {
		Expect(GenerateCombinations([]int{1, 2, 3}, 4)).To(BeNil())
	})

	It("should return a single combination when k == n", func() {
		combos := GenerateCombinations([]int{1, 2, 3}, 3)
		Expect(combos).To(HaveLen(1))
		Expect(combos[0]).To(Equal([]int{1, 2, 3}))
	})
})

var _ = Describe("Binomial", func() {
	It("should compute C(n,k) correctly", func() {
		Expect(Binomial(6, 6)).To(Equal(1))
		Expect(Binomial(8, 6)).To(Equal(28))
		Expect(Binomial(10, 6)).To(Equal(210))
		Expect(Binomial(12, 6)).To(Equal(924))
	})

	It("should return 0 for out-of-range k", func() {
		Expect(Binomial(5, 6)).To(Equal(0))
		Expect(Binomial(5, -1)).To(Equal(0))
	})
})
