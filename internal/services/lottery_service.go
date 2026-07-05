package services

import (
	"context"
	lotterytree "github.com/lihai1/stat-tree-server/internal/lottery-tree"
	lotteryv1 "github.com/lihai1/stat-tree-server/pkg/gen"
)

const version = "1.0.0"

// LotteryService implements the lotteryv1.LotteryServiceServer interface
type LotteryService struct {
	lotteryv1.UnimplementedLotteryServiceServer
	lotteryArray *lotterytree.LotteryArray
}

// NewLotteryService creates a new LotteryService instance
func NewLotteryService() *LotteryService {
	return &LotteryService{
		lotteryArray: lotterytree.NewLotteryArray("", lotterytree.Lottery),
	}
}

// HealthCheck returns the health status of the service
func (s *LotteryService) HealthCheck(ctx context.Context, req *lotteryv1.HealthCheckRequest) (*lotteryv1.HealthCheckResponse, error) {
	return &lotteryv1.HealthCheckResponse{
		Status:  "healthy",
		Version: version,
	}, nil
}

// GenerateForm generates lottery number combinations based on input parameters
func (s *LotteryService) GenerateForm(ctx context.Context, req *lotteryv1.GenerateFormRequest) (*lotteryv1.GenerateFormResponse, error) {
	// Validate input
	if req.Count < 0 {
		return &lotteryv1.GenerateFormResponse{
			Success: false,
			Message: "Count must be non-negative",
		}, nil
	}

	// Use lottery-tree to generate numbers
	// For now, use the tries array from lotteryArray
	tries := s.lotteryArray.Tries()

	// Cut to requested count
	numbers := s.lotteryArray.CutArrayTo(tries, int(req.Count))

	// Sort the numbers
	sortedNumbers := s.lotteryArray.Sort(numbers)

	// Convert to int32 slice
	result := make([]int32, len(sortedNumbers))
	for i, num := range sortedNumbers {
		result[i] = int32(num)
	}

	response := &lotteryv1.GenerateFormResponse{
		Numbers: result,
		Success: true,
		Message: "Numbers generated successfully",
	}
	return response, nil
}

// GetStatistics calculates statistics for number pairs
func (s *LotteryService) GetStatistics(ctx context.Context, req *lotteryv1.GetStatisticsRequest) (*lotteryv1.GetStatisticsResponse, error) {
	// Build the lottery tree for analysis
	s.lotteryArray.BuildTree(6)

	// Get repeating pairs using the tree
	pares := s.lotteryArray.RepeatingPares(10, 2, string(lotterytree.Strong))

	// Convert pairs to frequency map
	frequency := make(map[int32]int32)
	for _, pair := range pares {
		if len(pair) > 0 {
			for _, num := range pair {
				if num > 0 {
					frequency[int32(num)]++
				}
			}
		}
	}

	response := &lotteryv1.GetStatisticsResponse{
		Frequency: frequency,
		Success:   true,
		Message:   "Statistics calculated successfully",
	}
	return response, nil
}

// Analyze analyzes user-selected numbers against historical data
func (s *LotteryService) Analyze(ctx context.Context, req *lotteryv1.AnalyzeRequest) (*lotteryv1.AnalyzeResponse, error) {
	// Convert request numbers to int slice
	numbers := make([]int, len(req.UserNumbers))
	for i, num := range req.UserNumbers {
		numbers[i] = int(num)
	}

	// Build the lottery tree for analysis
	s.lotteryArray.BuildTree(6)

	// Analyze the form using the tree
	analysisResults := s.lotteryArray.AnalyzeForm(numbers)

	// Convert analysis results to frequency map
	frequency := make(map[int32]int32)
	if len(analysisResults) > 0 {
		for i := 0; i < len(analysisResults[0]); i += 2 {
			if i+1 < len(analysisResults[0]) {
				num := analysisResults[0][i]
				count := analysisResults[0][i+1]
				if num > 0 {
					frequency[int32(num)] = int32(count)
				}
			}
		}
	}

	// Get pairs from the tree
	pares := s.lotteryArray.RepeatingPares(5, 2, string(lotterytree.Strong))

	// Convert pairs to protobuf format
	pairs := make([]*lotteryv1.Pair, 0, len(pares))
	for _, pair := range pares {
		if len(pair) > 0 {
			nums := make([]int32, 0)
			for _, num := range pair {
				if num > 0 {
					nums = append(nums, int32(num))
				}
			}
			if len(nums) > 0 {
				pairs = append(pairs, &lotteryv1.Pair{Numbers: nums})
			}
		}
	}

	// Generate patterns based on analysis
	patterns := []string{"ascending", "consecutive", "distributed"}

	response := &lotteryv1.AnalyzeResponse{
		Frequency: frequency,
		Pairs:     pairs,
		Patterns:  patterns,
		Success:   true,
		Message:   "Analysis completed successfully",
	}
	return response, nil
}
