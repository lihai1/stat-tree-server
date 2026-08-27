package scraper

import (
	"encoding/csv"
	"fmt"
	"io"
	"log"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/lihai1/stat-tree-server/internal/models"
	"golang.org/x/net/html"
)

const (
	paisDownloadURL = "http://www.pais.co.il/Lotto/lotto_resultsDownload.aspx"
	// paisDrawURL is the per-draw results page. It renders an HTML prize
	// table with aria-label attributes for each tier (winners + per-winner
	// prize amount) plus the advertised first/second prize amounts.
	paisDrawURL = "https://www.pais.co.il/lotto/currentlotto.aspx?lotteryId=%d"
)

// browserHeaders are sent with HTML page requests. The pais.co.il ASP.NET
// backend returns a server-error page for requests without a real
// User-Agent (the CSV download endpoint is unaffected).
var browserHeaders = map[string]string{
	"User-Agent":      "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
	"Accept":          "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8",
	"Accept-Language": "he-IL,he;q=0.9,en;q=0.8",
	"Referer":         "https://www.pais.co.il/Lotto/Pages/default.aspx",
}

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
	log.Printf("PaisScraper: fetching lottery CSV from %s", paisDownloadURL)
	resp, err := ps.client.Get(paisDownloadURL)
	if err != nil {
		log.Printf("PaisScraper: CSV fetch failed: %v", err)
		return nil, fmt.Errorf("failed to fetch data: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("PaisScraper: CSV fetch returned unexpected status %d", resp.StatusCode)
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	log.Printf("PaisScraper: parsing CSV response")
	return ps.parseCSV(resp.Body)
}

// FetchPrizeAmounts scrapes the per-draw prize table from the pais.co.il
// individual draw page and returns the 8-tier prize amounts (ILS per winner)
// in canonical order:
//
//	[0] 6+strong, [1] 6, [2] 5+strong, [3] 5,
//	[4] 4+strong, [5] 4, [6] 3+strong, [7] 3
//
// For tiers 1-2 (6+strong, 6) with 0 winners, the advertised first/second
// prize amounts are used instead of the 0 ₪ per-winner value, so the
// simulation reflects the prize pool that would have been paid out.
//
// Returns an error if the page is unreachable or the prize table cannot
// be parsed. The caller should treat this as "no data for this draw" and
// fall back to default estimates.
func (ps *PaisScraper) FetchPrizeAmounts(drawNumber int) ([]float64, error) {
	url := fmt.Sprintf(paisDrawURL, drawNumber)
	log.Printf("PaisScraper: fetching prize table for draw %d from %s", drawNumber, url)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		log.Printf("PaisScraper: failed to build request for draw %d: %v", drawNumber, err)
		return nil, fmt.Errorf("prize scraper: failed to build request: %w", err)
	}
	for k, v := range browserHeaders {
		req.Header.Set(k, v)
	}

	resp, err := ps.client.Do(req)
	if err != nil {
		log.Printf("PaisScraper: HTTP request failed for draw %d: %v", drawNumber, err)
		return nil, fmt.Errorf("prize scraper: request failed for draw %d: %w", drawNumber, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("PaisScraper: draw %d returned unexpected status %d", drawNumber, resp.StatusCode)
		return nil, fmt.Errorf("prize scraper: unexpected status %d for draw %d", resp.StatusCode, drawNumber)
	}

	log.Printf("PaisScraper: parsing prize HTML for draw %d", drawNumber)
	amounts, err := parsePrizeHTML(resp.Body, drawNumber)
	if err != nil {
		log.Printf("PaisScraper: HTML parse failed for draw %d: %v", drawNumber, err)
		return nil, err
	}

	log.Printf("PaisScraper: draw %d prize amounts parsed: %v", drawNumber, amounts)
	return amounts, nil
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

// ── Prize table HTML parsing ───────────────────────────────────────

// tierLabelRE matches aria-label values like "רמת פרס 6 + חזק" or "רמת פרס 5".
var tierLabelRE = regexp.MustCompile(`רמת פרס (\d+)( \+ חזק)?`)

// winnersRE matches aria-label values like "מספר זוכים 13" or "מספר זוכים 10,848".
var winnersRE = regexp.MustCompile(`מספר זוכים ([\d,]+)`)

// prizeRE matches aria-label values like "סכום זכייה 5,945 ₪" or "סכום זכייה 0 ₪".
var prizeRE = regexp.MustCompile(`סכום זכייה ([\d,]+)`)

// firstPrizeRE matches the advertised first prize text:
// "סכום הפרס הראשון בהגרלה זו עמד על 8,000,000 ₪"
var firstPrizeRE = regexp.MustCompile(`הפרס הראשון בהגרלה זו עמד על.*?([\d,]+)\s*₪`)

// secondPrizeRE matches the advertised second prize text.
var secondPrizeRE = regexp.MustCompile(`הפרס השני בהגרלה זו עמד על.*?([\d,]+)\s*₪`)

// parsePrizeHTML parses the pais.co.il individual-draw HTML page and
// returns the 8-tier regular-lotto prize amounts (ILS per winner).
func parsePrizeHTML(r io.Reader, drawNumber int) ([]float64, error) {
	doc, err := html.Parse(r)
	if err != nil {
		log.Printf("parsePrizeHTML: HTML parse error for draw %d: %v", drawNumber, err)
		return nil, fmt.Errorf("prize scraper: failed to parse HTML for draw %d: %w", drawNumber, err)
	}

	// Collect all aria-label values in document order.
	var ariaLabels []string
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode {
			for _, attr := range n.Attr {
				if attr.Key == "aria-label" {
					ariaLabels = append(ariaLabels, attr.Val)
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)

	log.Printf("parsePrizeHTML: draw %d — collected %d aria-labels", drawNumber, len(ariaLabels))

	// Also collect text content for the advertised first/second prize.
	pageText := textContent(doc)
	firstPrize := parseAmount(firstPrizeRE, pageText)
	secondPrize := parseAmount(secondPrizeRE, pageText)

	log.Printf("parsePrizeHTML: draw %d — advertised firstPrize=%.0f, secondPrize=%.0f",
		drawNumber, firstPrize, secondPrize)

	// Walk the aria-labels in order and group them into tier triplets:
	// (tier label, winners, prize amount). The regular lotto table
	// appears first, followed by the double lotto table. We only
	// parse the first 8 tiers (regular lotto).
	tierAmounts := make([]float64, 8)
	tierWinners := make([]int, 8)
	tiersFound := 0

	i := 0
	for i < len(ariaLabels) && tiersFound < 8 {
		label := ariaLabels[i]

		// Tier label: "רמת פרס X" or "רמת פרס X + חזק"
		if m := tierLabelRE.FindStringSubmatch(label); m != nil {
			tierIdx := tierLabelToIndex(m[1], m[2] != "")
			if tierIdx < 0 {
				i++
				continue
			}

			// Look ahead for the next "מספר זוכים" and "סכום זכייה".
			winners := -1
			amount := -1.0
			for j := i + 1; j < len(ariaLabels) && j <= i+4; j++ {
				if winners < 0 {
					if wm := winnersRE.FindStringSubmatch(ariaLabels[j]); wm != nil {
						winners = parseHebrewNumber(wm[1])
					}
				}
				if amount < 0 {
					if pm := prizeRE.FindStringSubmatch(ariaLabels[j]); pm != nil {
						amount = float64(parseHebrewNumber(pm[1]))
					}
				}
			}

			if winners >= 0 && amount >= 0 {
				tierWinners[tierIdx] = winners
				tierAmounts[tierIdx] = amount
				tiersFound++
			}
		}
		i++
	}

	if tiersFound < 8 {
		log.Printf("parsePrizeHTML: draw %d — only found %d/8 tiers, aborting", drawNumber, tiersFound)
		return nil, fmt.Errorf("prize scraper: only found %d/8 tiers for draw %d", tiersFound, drawNumber)
	}

	// For tiers 1-2 (6+strong, 6) with 0 winners, the per-winner prize
	// is 0 ₪ because nobody won. Use the advertised first/second prize
	// instead so the simulation reflects the actual prize pool.
	if tierWinners[0] == 0 && firstPrize > 0 {
		log.Printf("parsePrizeHTML: draw %d — tier 1 (6+strong) had 0 winners, using advertised firstPrize=%.0f",
			drawNumber, firstPrize)
		tierAmounts[0] = firstPrize
	}
	if tierWinners[1] == 0 && secondPrize > 0 {
		log.Printf("parsePrizeHTML: draw %d — tier 2 (6) had 0 winners, using advertised secondPrize=%.0f",
			drawNumber, secondPrize)
		tierAmounts[1] = secondPrize
	}

	log.Printf("parsePrizeHTML: draw %d — successfully parsed 8 tiers: winners=%v, amounts=%v",
		drawNumber, tierWinners, tierAmounts)
	return tierAmounts, nil
}

// tierLabelToIndex maps a tier label's number + strong flag to the
// canonical 0-7 index. Returns -1 for unrecognized tiers.
//
//	"6" + strong → 0 (6+strong)
//	"6"          → 1 (6)
//	"5" + strong → 2 (5+strong)
//	"5"          → 3 (5)
//	"4" + strong → 4 (4+strong)
//	"4"          → 5 (4)
//	"3" + strong → 6 (3+strong)
//	"3"          → 7 (3)
func tierLabelToIndex(numStr string, strong bool) int {
	n, err := strconv.Atoi(numStr)
	if err != nil {
		return -1
	}
	switch n {
	case 6:
		if strong {
			return 0
		}
		return 1
	case 5:
		if strong {
			return 2
		}
		return 3
	case 4:
		if strong {
			return 4
		}
		return 5
	case 3:
		if strong {
			return 6
		}
		return 7
	default:
		return -1
	}
}

// parseHebrewNumber converts a Hebrew-formatted number string like
// "10,848" or "5,945" to an int (strips thousands separators).
func parseHebrewNumber(s string) int {
	s = strings.ReplaceAll(s, ",", "")
	s = strings.TrimSpace(s)
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0
	}
	return n
}

// parseAmount applies a regex to text and returns the first matched
// number (with thousands separators removed), or 0 if no match.
func parseAmount(re *regexp.Regexp, text string) float64 {
	m := re.FindStringSubmatch(text)
	if m == nil {
		return 0
	}
	return float64(parseHebrewNumber(m[1]))
}

// textContent extracts all visible text from an HTML node tree.
func textContent(n *html.Node) string {
	if n.Type == html.TextNode {
		return n.Data
	}
	var sb strings.Builder
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		sb.WriteString(textContent(c))
	}
	return sb.String()
}
