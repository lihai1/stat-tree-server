package clients

import (
	"context"

	lotteryv1 "github.com/lihai1/stat-tree-server/pkg/gen"
)

// LotteryClient defines the common interface for lottery service clients
type LotteryClient interface {
	// HealthCheck returns the health status of the service
	HealthCheck(ctx context.Context, req *lotteryv1.HealthCheckRequest) (*lotteryv1.HealthCheckResponse, error)

	// GenerateForm generates lottery number combinations based on input parameters
	GenerateForm(ctx context.Context, req *lotteryv1.GenerateFormRequest) (*lotteryv1.GenerateFormResponse, error)

	// GetStatistics calculates statistics for number pairs
	GetStatistics(ctx context.Context, req *lotteryv1.GetStatisticsRequest) (*lotteryv1.GetStatisticsResponse, error)

	// Analyze analyzes user-selected numbers against historical data
	Analyze(ctx context.Context, req *lotteryv1.AnalyzeRequest) (*lotteryv1.AnalyzeResponse, error)

	// Close closes the client connection
	Close() error
}
