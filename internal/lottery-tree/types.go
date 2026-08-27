package lotterytree

// Common represents the strength type for lottery analysis
type Common string

const (
	Strong Common = "strong"
	Weak   Common = "weak"
	Random Common = "random"
)

// LotteryType represents the type of lottery game
type LotteryType string

const (
	Lottery     LotteryType = "lottery"
	TripleSeven LotteryType = "777"
	OneTwoThree LotteryType = "123"
)
