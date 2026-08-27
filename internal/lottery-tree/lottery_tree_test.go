package lotterytree_test

import (
	"fmt"
	"sort"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	. "github.com/lihai1/stat-tree-server/internal/lottery-tree"
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

	Context("InTree", func() {
		It("should not crash when checking for forms", func() {
			tree := NewLoTree(37)
			data := [][]int{
				{1, 2, 3, 4, 5, 6},
			}
			tree.Build(data, 6)
			_ = tree.InTree([]int{1, 2, 3, 4, 5, 6})
			_ = tree.InTree([]int{1, 2, 3, 4, 5, 7})
		})
	})
})

var _ = Describe("LotteryList", func() {
	Context("NewLotteryList", func() {
		It("should create an empty list", func() {
			list := NewLotteryList()
			Expect(list.Size).To(Equal(0))
		})
	})

	Context("Add", func() {
		It("should add forms to the list", func() {
			list := NewLotteryList()
			form1 := []int{1, 2, 3}
			list.Add(form1)
			Expect(list.Size).To(Equal(1))

			form2 := []int{4, 5, 6}
			list.Add(form2)
			Expect(list.Size).To(Equal(2))
		})

		It("should not add duplicates", func() {
			list := NewLotteryList()
			form1 := []int{1, 2, 3}
			list.Add(form1)
			list.Add(form1)
			Expect(list.Size).To(Equal(1))
		})
	})

	Context("Iteration", func() {
		It("should iterate through all forms", func() {
			list := NewLotteryList()
			form1 := []int{1, 2, 3}
			form2 := []int{4, 5, 6}
			list.Add(form1)
			list.Add(form2)

			list.ToHeadList()
			count := 0
			for list.HasMoreForms() {
				form := list.NextForm()
				Expect(form).NotTo(BeNil())
				count++
			}
			Expect(count).To(Equal(2))
		})
	})
})

var _ = Describe("LotteryArray", func() {
	Context("Sort", func() {
		It("should sort array in ascending order", func() {
			la := NewLotteryArray("test", Lottery)
			arr := []int{6, 5, 4, 3, 2, 1}
			sorted := la.Sort(arr)

			for i := 0; i < len(sorted)-1; i++ {
				Expect(sorted[i]).To(BeNumerically("<=", sorted[i+1]))
			}
		})
	})

	Context("CutArrayTo", func() {
		It("should cut array to specified size", func() {
			la := NewLotteryArray("test", Lottery)
			arr := []int{1, 2, 3, 4, 5, 6}
			cut := la.CutArrayTo(arr, 3)

			Expect(len(cut)).To(Equal(3))
			for i := 0; i < 3; i++ {
				Expect(cut[i]).To(Equal(arr[i]))
			}
		})
	})

	Context("HowManyNumbersInLeft", func() {
		It("should count matching numbers", func() {
			la := NewLotteryArray("test", Lottery)
			test := []int{1, 2, 3, 4, 5, 6}
			gotNums := []int{2, 4, 6}

			count := la.HowManyNumbersInLeft(test, gotNums, 6)
			Expect(count).To(Equal(3))
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

var _ = Describe("AnalyzeForm", func() {
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

	Context("with a non-empty archive", func() {
		var la *LotteryArray

		BeforeEach(func() {
			la = NewLotteryArray("", Lottery)
			la.Archive = [][]int{
				{1, 2, 3, 4, 5, 6},
				{1, 2, 3, 7, 8, 9},
			}
		})

		It("should return an AnalyzeResponse with 6 groups and archive size", func() {
			resp := la.AnalyzeForm([]int{1, 2, 3})
			Expect(resp).NotTo(BeNil())
			Expect(resp.GetArchiveSize()).To(Equal(int32(2)))
			groups := resp.GetFrequencyGroups()
			Expect(groups).To(HaveLen(6))
			for i, g := range groups {
				Expect(int(g.GetSize())).To(Equal(i + 1))
			}
		})

		It("should group size-1 results with correct counts", func() {
			resp := la.AnalyzeForm([]int{1, 2, 3})
			// {1}, {2}, {3} — each appears in both draws
			entries := resp.GetFrequencyGroups()[0].GetEntries()
			Expect(entries).To(HaveLen(3))
			Expect(entryKeys(entries)).To(Equal([]string{"[1]", "[2]", "[3]"}))
			for _, e := range entries {
				Expect(e.GetCount()).To(Equal(int32(2)), "size-1 count for %v", e.GetNumbers())
			}
		})

		It("should group size-2 results with correct counts", func() {
			resp := la.AnalyzeForm([]int{1, 2, 3})
			// {1,2}, {1,3}, {2,3} — each appears in both draws
			entries := resp.GetFrequencyGroups()[1].GetEntries()
			Expect(entries).To(HaveLen(3))
			Expect(entryKeys(entries)).To(Equal([]string{"[1 2]", "[1 3]", "[2 3]"}))
			for _, e := range entries {
				Expect(e.GetCount()).To(Equal(int32(2)), "size-2 count for %v", e.GetNumbers())
			}
		})

		It("should group size-3 results with correct counts", func() {
			resp := la.AnalyzeForm([]int{1, 2, 3})
			// {1,2,3} — appears in both draws
			entries := resp.GetFrequencyGroups()[2].GetEntries()
			Expect(entries).To(HaveLen(1))
			Expect(entries[0].GetNumbers()).To(Equal([]int32{1, 2, 3}))
			Expect(entries[0].GetCount()).To(Equal(int32(2)))
		})

		It("should not produce duplicate subsets within any group", func() {
			resp := la.AnalyzeForm([]int{1, 2, 3, 4, 5, 6})
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
			resp := la.AnalyzeForm([]int{1, 2, 3, 4, 5, 6})
			for _, g := range resp.GetFrequencyGroups() {
				for _, e := range g.GetEntries() {
					Expect(len(e.GetNumbers())).To(Equal(int(g.GetSize())),
						"size %d entry has %d numbers: %v",
						g.GetSize(), len(e.GetNumbers()), e.GetNumbers())
				}
			}
		})

		It("should sort entries by count descending within each group", func() {
			// Archive where {1} appears twice but {2} and {3} appear once
			la := NewLotteryArray("", Lottery)
			la.Archive = [][]int{
				{1, 2, 3, 4, 5, 6},
				{1, 7, 8, 9, 10, 11},
			}
			resp := la.AnalyzeForm([]int{1, 2, 3})
			entries := resp.GetFrequencyGroups()[0].GetEntries() // size 1
			Expect(entries).To(HaveLen(3))
			// {1} has count 2, should be first; {2} and {3} have count 1
			Expect(entries[0].GetNumbers()).To(Equal([]int32{1}))
			Expect(entries[0].GetCount()).To(Equal(int32(2)))
		})
	})

	Context("with an empty archive", func() {
		It("should return 6 empty groups and archive size 0", func() {
			la := NewLotteryArray("", Lottery)
			resp := la.AnalyzeForm([]int{1, 2, 3})
			Expect(resp.GetArchiveSize()).To(Equal(int32(0)))
			groups := resp.GetFrequencyGroups()
			Expect(groups).To(HaveLen(6))
			for _, g := range groups {
				Expect(g.GetEntries()).To(BeEmpty())
			}
		})
	})
})
