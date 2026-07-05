package lotterytree

// LoTree represents the lottery tree structure
type LoTree struct {
	Anchor     *LoNode
	Node       *LoNode
	Size       int
	Lottery    int
	Pares      [][]int
	BuildCalls int
}

// NewLoTree creates a new lottery tree
func NewLoTree(lottery int) *LoTree {
	return &LoTree{
		Lottery: lottery,
		Anchor:  NewLoNode(0, lottery),
	}
}

// Build builds the tree from data
func (t *LoTree) Build(data [][]int, howMany int) *LoTree {
	for i := 0; i < len(data); i++ {
		t.Anchor.Build(data[i], 0, howMany)
	}
	t.BuildCalls = len(data)
	return &LoTree{Anchor: t.Anchor}
}

// InTree checks if data exists in the tree
func (t *LoTree) InTree(data []int) bool {
	return t.Anchor.InTree(data, 0, t.Anchor, len(data))
}

// NodesInTree counts total nodes in the tree
func (t *LoTree) NodesInTree() int {
	return t.nodesInTree(t.Anchor)
}

func (t *LoTree) nodesInTree(current *LoNode) int {
	if current == nil {
		return 0
	}
	sum := 1
	for i := 0; i < len(current.Next); i++ {
		sum += t.nodesInTree(current.Next[i])
	}
	return sum
}

// NodesInLevel counts nodes at a specific level
func (t *LoTree) NodesInLevel(level int) int {
	return t.nodesInLevel(t.Anchor, level)
}

func (t *LoTree) nodesInLevel(current *LoNode, level int) int {
	if current == nil {
		return 0
	}
	if level == 0 {
		return 1
	}
	sum := 0
	for i := 0; i < len(current.Next); i++ {
		sum += t.nodesInLevel(current.Next[i], level-1)
	}
	return sum
}

// BestPares finds the best pairs based on strength
func (t *LoTree) BestPares(sumOfPares int, howMany int, weakStrong string) [][]int {
	t.Pares = make([][]int, sumOfPares)
	for i := range t.Pares {
		t.Pares[i] = make([]int, howMany+1)
	}
	
	newPares := make([][]int, sumOfPares)
	for i := range newPares {
		newPares[i] = make([]int, howMany+1)
	}
	
	for j := 0; j < sumOfPares; j++ {
		if t.Pares[j][0] == 0 {
			ans := t.bestPares(t.Anchor, howMany, weakStrong)
			if ans != nil {
				newPares[j][howMany] = ans.Count
				for i := howMany - 1; i >= 0 && ans != nil; i-- {
					t.Pares[j][i] = ans.Num
					newPares[j][i] = ans.Num
					ans = ans.Prev
				}
			}
		}
	}
	return newPares
}

func (t *LoTree) bestPares(current *LoNode, howMany int, weakStrong string) *LoNode {
	if howMany == 0 || current == nil {
		return current
	}
	
	ans := &LoNode{Num: 0, Count: -1}
	if weakStrong == "weak" {
		ans.Count = 2147483647 // Max int
	}
	
	for i := current.Num; i < t.Lottery && i < len(current.Next)+current.Num; i++ {
		treeIndex := i - current.Num
		if treeIndex >= 0 && treeIndex < len(current.Next) {
			pos := t.bestPares(current.Next[treeIndex], howMany-1, weakStrong)
			if pos != nil && !t.exist(pos) {
				if weakStrong == "strong" && ans.Count < pos.Count {
					ans = pos
				} else if weakStrong == "weak" && ans.Count > pos.Count {
					ans = pos
				}
			}
		}
	}
	
	if ans.Num == 0 {
		return nil
	}
	return ans
}

func (t *LoTree) exist(current *LoNode) bool {
	if t.Lottery < 10 {
		return false
	}
	
	for j := 0; j < len(t.Pares); j++ {
		test := current
		flag := true
		for i := len(t.Pares[0]) - 2; i >= 0 && flag; i-- {
			if t.Pares[j][i] == test.Num {
				flag = true
			} else {
				flag = false
				i = -1
			}
			if test != nil {
				test = test.Prev
			}
		}
		if flag {
			return true
		}
	}
	return false
}
