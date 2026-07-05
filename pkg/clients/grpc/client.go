package grpc

import (
	"context"
	"fmt"

	"github.com/lihai1/stat-tree-server/pkg/clients"
	lotteryv1 "github.com/lihai1/stat-tree-server/pkg/gen"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// Client implements the LotteryClient interface using gRPC
type Client struct {
	conn   *grpc.ClientConn
	client lotteryv1.LotteryServiceClient
}

// NewClient creates a new gRPC client for the lottery service
func NewClient(address string) (*Client, error) {
	conn, err := grpc.NewClient(address, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("failed to connect to gRPC server: %w", err)
	}

	return &Client{
		conn:   conn,
		client: lotteryv1.NewLotteryServiceClient(conn),
	}, nil
}

// HealthCheck returns the health status of the service
func (c *Client) HealthCheck(ctx context.Context, req *lotteryv1.HealthCheckRequest) (*lotteryv1.HealthCheckResponse, error) {
	return c.client.HealthCheck(ctx, req)
}

// GenerateForm generates lottery number combinations based on input parameters
func (c *Client) GenerateForm(ctx context.Context, req *lotteryv1.GenerateFormRequest) (*lotteryv1.GenerateFormResponse, error) {
	return c.client.GenerateForm(ctx, req)
}

// GetStatistics calculates statistics for number pairs
func (c *Client) GetStatistics(ctx context.Context, req *lotteryv1.GetStatisticsRequest) (*lotteryv1.GetStatisticsResponse, error) {
	return c.client.GetStatistics(ctx, req)
}

// Analyze analyzes user-selected numbers against historical data
func (c *Client) Analyze(ctx context.Context, req *lotteryv1.AnalyzeRequest) (*lotteryv1.AnalyzeResponse, error) {
	return c.client.Analyze(ctx, req)
}

// Close closes the client connection
func (c *Client) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// Ensure Client implements the LotteryClient interface
var _ clients.LotteryClient = (*Client)(nil)
