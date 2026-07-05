package lotterytree_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	. "github.com/lihai1/stat-tree-server/internal/lottery-tree"
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
