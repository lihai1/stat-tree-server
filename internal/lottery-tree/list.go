package lotterytree

// LotteryList represents a linked list of lottery forms
type LotteryList struct {
	Anchor *LoNode
	Node   *LoNode
	Size   int
}

// NewLotteryList creates a new lottery list
func NewLotteryList() *LotteryList {
	return &LotteryList{
		Size:   0,
		Anchor: nil,
	}
}

// ToHeadList resets the node pointer to the anchor
func (l *LotteryList) ToHeadList() {
	l.Node = l.Anchor
}

// NextForm returns the next form in the list
func (l *LotteryList) NextForm() []int {
	if l.Node != nil {
		ans := l.Node.Piece
		if len(l.Node.Next) > 0 {
			l.Node = l.Node.Next[0]
		} else {
			l.Node = nil
		}
		return ans
	}
	return nil
}

// HasMoreForms checks if there are more forms in the list
func (l *LotteryList) HasMoreForms() bool {
	return l.Node != nil
}

// Add adds a form to the list if it doesn't already exist
func (l *LotteryList) Add(form []int) {
	if l.Size > 0 {
		l.Node = l.Anchor
		var prev *LoNode
		for l.Node != nil && !l.equals(l.Node.Piece, form) {
			prev = l.Node
			if len(l.Node.Next) > 0 {
				l.Node = l.Node.Next[0]
			} else {
				l.Node = nil
			}
		}
		if l.Node == nil && prev != nil {
			prev.Next = []*LoNode{NewLoNode(0, 1)}
			prev.Next[0].Piece = form
			l.Size++
		}
	} else {
		l.Anchor = NewLoNode(0, 1)
		l.Anchor.Piece = form
		l.Size = 1
	}
}

// equals checks if two int slices are equal
func (l *LotteryList) equals(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := 0; i < len(a); i++ {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// Pieces returns all forms in the list
func (l *LotteryList) Pieces() [][]int {
	l.Node = l.Anchor
	ans := make([][]int, l.Size)
	for i := 0; i < l.Size; i++ {
		ans[i] = l.Node.Piece
		if len(l.Node.Next) > 0 {
			l.Node = l.Node.Next[0]
		}
	}
	return ans
}
