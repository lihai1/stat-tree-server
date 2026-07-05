package http

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/lihai1/stat-tree-server/pkg/clients"
	lotteryv1 "github.com/lihai1/stat-tree-server/pkg/gen"
)

// Client implements the LotteryClient interface using HTTP
type Client struct {
	baseURL    string
	httpClient *http.Client
}

// NewClient creates a new HTTP client for the lottery service
func NewClient(baseURL string) *Client {
	return &Client{
		baseURL:    baseURL,
		httpClient: &http.Client{},
	}
}

// HealthCheck returns the health status of the service
func (c *Client) HealthCheck(ctx context.Context, req *lotteryv1.HealthCheckRequest) (*lotteryv1.HealthCheckResponse, error) {
	var resp lotteryv1.HealthCheckResponse
	err := c.doGetRequest(ctx, "/health", &resp)
	if err != nil {
		return nil, err
	}
	return &resp, nil
}

// GenerateForm generates lottery number combinations based on input parameters
func (c *Client) GenerateForm(ctx context.Context, req *lotteryv1.GenerateFormRequest) (*lotteryv1.GenerateFormResponse, error) {
	var resp lotteryv1.GenerateFormResponse
	err := c.doRequest(ctx, "/api/generate/form", req, &resp)
	if err != nil {
		return nil, err
	}
	return &resp, nil
}

// GetStatistics calculates statistics for number pairs
func (c *Client) GetStatistics(ctx context.Context, req *lotteryv1.GetStatisticsRequest) (*lotteryv1.GetStatisticsResponse, error) {
	var resp lotteryv1.GetStatisticsResponse
	err := c.doRequest(ctx, "/api/generate/pares", req, &resp)
	if err != nil {
		return nil, err
	}
	return &resp, nil
}

// Analyze analyzes user-selected numbers against historical data
func (c *Client) Analyze(ctx context.Context, req *lotteryv1.AnalyzeRequest) (*lotteryv1.AnalyzeResponse, error) {
	var resp lotteryv1.AnalyzeResponse
	err := c.doRequest(ctx, "/api/generate/analyze", req, &resp)
	if err != nil {
		return nil, err
	}
	return &resp, nil
}

// Close closes the client connection (no-op for HTTP client)
func (c *Client) Close() error {
	return nil
}

// doRequest performs an HTTP request to the lottery service
func (c *Client) doRequest(ctx context.Context, path string, req interface{}, resp interface{}) error {
	body, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	url := c.baseURL + path
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(body))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")

	httpResp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("failed to perform request: %w", err)
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(httpResp.Body)
		return fmt.Errorf("request failed with status %d: %s", httpResp.StatusCode, string(body))
	}

	respBody, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	if err := json.Unmarshal(respBody, resp); err != nil {
		return fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return nil
}

// doGetRequest performs an HTTP GET request to the lottery service
func (c *Client) doGetRequest(ctx context.Context, path string, resp interface{}) error {
	url := c.baseURL + path
	httpReq, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	httpResp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("failed to perform request: %w", err)
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(httpResp.Body)
		return fmt.Errorf("request failed with status %d: %s", httpResp.StatusCode, string(body))
	}

	respBody, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	if err := json.Unmarshal(respBody, resp); err != nil {
		return fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return nil
}

// Ensure Client implements the LotteryClient interface
var _ clients.LotteryClient = (*Client)(nil)
