package lotterytree

// LoNode represents a node in the lottery tree
type LoNode struct {
	Piece []int
	Prev  *LoNode
	Next  []*LoNode
	Num   int
	Count int
}

// NewLoNode creates a new lottery node
func NewLoNode(n int, lottery int) *LoNode {
	node := &LoNode{
		Num:   n,
		Count: 1,
		Prev:  nil,
	}

	nextSize := lottery - n
	if lottery == 3 {
		nextSize = lottery
	}

	node.Next = make([]*LoNode, nextSize)
	return node
}

// NewLoNodeWithParent creates a new node with a parent
func NewLoNodeWithParent(parent *LoNode, n int, lottery int) *LoNode {
	node := &LoNode{
		Num:   n,
		Count: 1,
		Prev:  parent,
	}

	nextSize := lottery - n
	if lottery == 3 {
		nextSize = lottery
	}

	node.Next = make([]*LoNode, nextSize)
	return node
}

// Build builds the tree structure from a form
func (n *LoNode) Build(form []int, current int, howMany int) {
	if howMany == 0 {
		return
	}

	for i := current; i < len(form); i++ {
		treeIndex := form[i] - n.Num - 1
		if treeIndex >= 0 && treeIndex < len(n.Next) {
			if n.Next[treeIndex] == nil {
				n.Next[treeIndex] = NewLoNodeWithParent(n, form[i], len(n.Next)+n.Num)
			} else {
				n.Next[treeIndex].Count++
			}
			n.Next[treeIndex].Build(form, i+1, howMany-1)
		}
	}
}
