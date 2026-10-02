package report

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"slices"
	"time"

	"log-analyzer/internal/logparse"
)

// Summary accumulates the statistics for summary mode. It keeps only
// counters and the earliest and latest timestamps, so memory use does not
// grow with the number of entries.
type Summary struct {
	total          int
	first, last    time.Time
	countsByStatus map[int]int
}

// NewSummary returns an empty Summary.
func NewSummary() *Summary {
	return &Summary{countsByStatus: map[int]int{}}
}

// Add records one entry.
func (s *Summary) Add(e logparse.Entry) {
	if s.total == 0 || e.Time.Before(s.first) {
		s.first = e.Time
	}
	if s.total == 0 || e.Time.After(s.last) {
		s.last = e.Time
	}
	s.total++
	s.countsByStatus[e.Status]++
}

// RequestsPerMinute returns total / max(span in minutes, 1), rounded to two
// decimals.
func (s *Summary) RequestsPerMinute() float64 {
	if s.total == 0 {
		return 0
	}
	minutes := max(s.last.Sub(s.first).Minutes(), 1)
	return math.Round(float64(s.total)/minutes*100) / 100
}

// classCounts holds the counts of one status class (e.g. 5xx).
type classCounts struct {
	class int // 1-5
	count int
	codes []int // ascending
}

// classes groups the status counts by class, in ascending order.
func (s *Summary) classes() []classCounts {
	codes := make([]int, 0, len(s.countsByStatus))
	for code := range s.countsByStatus {
		codes = append(codes, code)
	}
	slices.Sort(codes)

	var out []classCounts
	for _, code := range codes {
		class := code / 100
		if len(out) == 0 || out[len(out)-1].class != class {
			out = append(out, classCounts{class: class})
		}
		c := &out[len(out)-1]
		c.count += s.countsByStatus[code]
		c.codes = append(c.codes, code)
	}
	return out
}

func (s *Summary) percent(count int) float64 {
	return float64(count) * 100 / float64(s.total)
}

// WriteText writes the human-readable report.
func (s *Summary) WriteText(w io.Writer) error {
	bw := bufio.NewWriter(w)
	fmt.Fprintf(bw, "%-21s%d\n", "Total requests:", s.total)
	if s.total > 0 {
		fmt.Fprintf(bw, "%-21s%s – %s\n", "Time range:",
			s.first.Format(time.RFC3339), s.last.Format(time.RFC3339))
	}
	fmt.Fprintf(bw, "%-21s%.2f\n", "Requests per minute:", s.RequestsPerMinute())

	if s.total > 0 {
		fmt.Fprintf(bw, "\nStatus codes:\n")
		for _, c := range s.classes() {
			fmt.Fprintf(bw, "%-7s%5d %5.1f%%\n", fmt.Sprintf("  %dxx", c.class), c.count, s.percent(c.count))
			for _, code := range c.codes {
				n := s.countsByStatus[code]
				fmt.Fprintf(bw, "%-7s%5d %5.1f%%\n", fmt.Sprintf("    %d", code), n, s.percent(n))
			}
		}
	}
	return bw.Flush()
}

type summaryJSON struct {
	Total             int                  `json:"total"`
	TimeRange         *timeRangeJSON       `json:"time_range"`
	RequestsPerMinute float64              `json:"requests_per_minute"`
	Status            map[string]classJSON `json:"status"`
}

type timeRangeJSON struct {
	Start string `json:"start"`
	End   string `json:"end"`
}

type classJSON struct {
	Count   int            `json:"count"`
	Percent float64        `json:"percent"`
	Codes   map[string]int `json:"codes"`
}

// WriteJSON writes the report as an indented JSON object.
func (s *Summary) WriteJSON(w io.Writer) error {
	out := summaryJSON{
		Total:             s.total,
		RequestsPerMinute: s.RequestsPerMinute(),
		Status:            map[string]classJSON{},
	}
	if s.total > 0 {
		out.TimeRange = &timeRangeJSON{
			Start: s.first.Format(time.RFC3339),
			End:   s.last.Format(time.RFC3339),
		}
	}
	for _, c := range s.classes() {
		codes := map[string]int{}
		for _, code := range c.codes {
			codes[fmt.Sprint(code)] = s.countsByStatus[code]
		}
		out.Status[fmt.Sprintf("%dxx", c.class)] = classJSON{
			Count:   c.count,
			Percent: math.Round(s.percent(c.count)*10) / 10,
			Codes:   codes,
		}
	}

	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}
