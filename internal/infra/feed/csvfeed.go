package feed

import (
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"
	"github.com/felix/papertrading/internal/domain/core"
	"github.com/felix/papertrading/internal/domain/market"
)

type CSVFeed struct {
	path    string
	skip    int
	symbol  core.Symbol
}

type CSVFeedOption func(*CSVFeed)

func WithSkipRows(n int) CSVFeedOption {
	return func(f *CSVFeed) {
		f.skip = n
	}
}

func WithSymbol(symbol core.Symbol) CSVFeedOption {
	return func(f *CSVFeed) {
		f.symbol = symbol
	}
}

func NewCSVFeed(path string, opts ...CSVFeedOption) *CSVFeed {
	f := &CSVFeed{
		path:   path,
		skip:   1,
		symbol: "",
	}
	for _, opt := range opts {
		opt(f)
	}
	return f
}

func (f *CSVFeed) Stream(ctx context.Context, ticks chan<- market.Tick) error {
	file, err := os.Open(f.path)
	if err != nil {
		return fmt.Errorf("open CSV: %w", err)
	}
	defer file.Close()

	reader := csv.NewReader(file)
	reader.TrimLeadingSpace = true

	for i := 0; i < f.skip; i++ {
		if _, err := reader.Read(); err != nil {
			return fmt.Errorf("skip header: %w", err)
		}
	}

	for {
		if err := ctx.Err(); err != nil {
			close(ticks)
			return err
		}

		record, err := reader.Read()
		if err == io.EOF {
			close(ticks)
			return nil
		}
		if err != nil {
			close(ticks)
			return fmt.Errorf("read CSV: %w", err)
		}

		tick, err := parseTick(record, f.symbol)
		if err != nil {
			continue
		}

		select {
		case <-ctx.Done():
			close(ticks)
			return ctx.Err()
		case ticks <- *tick:
		}
	}
}

func parseTick(record []string, defaultSymbol core.Symbol) (*market.Tick, error) {
	if len(record) < 2 {
		return nil, fmt.Errorf("invalid record: %v", record)
	}

	var (
		timestamp time.Time
		symbol    core.Symbol
		price     core.Money
		volume    int64
		err       error
	)

	idx := 0

	if isTimestamp(record[0]) {
		timestamp, err = parseTimestamp(record[0])
		if err != nil {
			return nil, err
		}
		idx++
	} else {
		timestamp = time.Now()
	}

	if idx < len(record) && !isPrice(record[idx]) {
		symbol = core.Symbol(record[idx])
		idx++
	} else {
		symbol = defaultSymbol
	}

	if idx >= len(record) {
		return nil, fmt.Errorf("missing price")
	}
	price, err = core.NewMoney(record[idx])
	if err != nil {
		return nil, err
	}
	idx++

	if idx < len(record) {
		v, err := strconv.ParseInt(record[idx], 10, 64)
		if err != nil {
			volume = 0
		} else {
			volume = v
		}
	}

	if !symbol.Valid() {
		return nil, fmt.Errorf("unknown symbol")
	}

	return &market.Tick{
		Symbol:    symbol,
		Price:     price,
		Volume:    volume,
		Timestamp: timestamp,
	}, nil
}

func isTimestamp(s string) bool {
	s = strings.TrimSpace(s)
	for _, fmt := range []string{
		"2006-01-02 15:04:05",
		"2006-01-02T15:04:05",
		"2006-01-02T15:04:05Z",
		"01/02/2006 15:04",
		"2006-01-02",
	} {
		if _, err := time.Parse(fmt, s); err == nil {
			return true
		}
	}
	return false
}

func parseTimestamp(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	for _, fmt := range []string{
		"2006-01-02 15:04:05",
		"2006-01-02T15:04:05",
		"2006-01-02T15:04:05Z",
		"01/02/2006 15:04",
		"2006-01-02",
	} {
		if t, err := time.Parse(fmt, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("cannot parse timestamp: %s", s)
}

func isPrice(s string) bool {
	_, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	return err == nil
}
