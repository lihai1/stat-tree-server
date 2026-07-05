package scraper

import (
	"encoding/csv"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/lihai1/stat-tree-server/internal/models"
)

const (
	paisDownloadURL = "http://www.pais.co.il/Lotto/lotto_resultsDownload.aspx"
)

type PaisScraper struct {
	client *http.Client
}

func NewPaisScraper() *PaisScraper {
	return &PaisScraper{
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// FetchLotteryData fetches lottery data from pais.co.il
func (ps *PaisScraper) FetchLotteryData() ([]models.LotteryResult, error) {
	resp, err := ps.client.Get(paisDownloadURL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch data: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	return ps.parseCSV(resp.Body)
}

// parseCSV parses the CSV data from pais.co.il
func (ps *PaisScraper) parseCSV(reader io.Reader) ([]models.LotteryResult, error) {
	csvReader := csv.NewReader(reader)
	csvReader.Comma = ','

	var results []models.LotteryResult

	// Skip header row if present
	_, err := csvReader.Read()
	if err != nil && err != io.EOF {
		return nil, fmt.Errorf("failed to read header: %w", err)
	}

	lineNum := 1
	for {
		record, err := csvReader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to read line %d: %w", lineNum, err)
		}

		result, err := ps.parseRecord(record)
		if err != nil {
			log.Printf("Warning: failed to parse line %d: %v", lineNum, err)
			lineNum++
			continue
		}

		results = append(results, result)
		lineNum++
	}

	return results, nil
}

// parseRecord parses a single CSV record into a LotteryResult
func (ps *PaisScraper) parseRecord(record []string) (models.LotteryResult, error) {
	if len(record) < 8 {
		return models.LotteryResult{}, fmt.Errorf("invalid record length: %d", len(record))
	}

	// Parse draw number
	drawNumber, err := strconv.Atoi(strings.TrimSpace(record[0]))
	if err != nil {
		return models.LotteryResult{}, fmt.Errorf("invalid draw number: %w", err)
	}

	// Parse draw date (format: DD/MM/YYYY)
	drawDate, err := time.Parse("02/01/2006", strings.TrimSpace(record[1]))
	if err != nil {
		return models.LotteryResult{}, fmt.Errorf("invalid draw date: %w", err)
	}

	// Parse numbers (columns 2-7 contain the 6 numbers)
	numbers := make([]int, 6)
	for i := 0; i < 6; i++ {
		num, err := strconv.Atoi(strings.TrimSpace(record[i+2]))
		if err != nil {
			return models.LotteryResult{}, fmt.Errorf("invalid number at position %d: %w", i, err)
		}
		numbers[i] = num
	}

	// Parse strong number (column 8)
	strong, err := strconv.Atoi(strings.TrimSpace(record[8]))
	if err != nil {
		return models.LotteryResult{}, fmt.Errorf("invalid strong number: %w", err)
	}

	now := time.Now()

	return models.LotteryResult{
		DrawNumber:  drawNumber,
		DrawDate:    drawDate,
		Numbers:     numbers,
		Strong:      strong,
		LotteryType: "lotto",
		CreatedAt:   now,
		UpdatedAt:   now,
	}, nil
}
