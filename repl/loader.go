package repl

import (
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"strconv"

	"github.com/MoroZvlg/tascript/object"
)

var candleHeader = []string{"open", "high", "low", "close", "volume"}

func loadCandlesCSV(path string) (*object.CandleSeries, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	r := csv.NewReader(f)
	header, err := r.Read()
	if err != nil {
		return nil, fmt.Errorf("read header: %w", err)
	}
	if len(header) != len(candleHeader) {
		return nil, fmt.Errorf("expected header %v, got %v", candleHeader, header)
	}
	for i, want := range candleHeader {
		if header[i] != want {
			return nil, fmt.Errorf("expected header %v, got %v", candleHeader, header)
		}
	}

	var candles []object.Candle
	row := 1
	for {
		rec, err := r.Read()
		if err == io.EOF {
			break
		}
		row++
		if err != nil {
			return nil, fmt.Errorf("row %d: %w", row, err)
		}
		if len(rec) != len(candleHeader) {
			return nil, fmt.Errorf("row %d: expected %d fields, got %d", row, len(candleHeader), len(rec))
		}
		vals := make([]float64, len(candleHeader))
		for i, s := range rec {
			v, err := strconv.ParseFloat(s, 64)
			if err != nil {
				return nil, fmt.Errorf("row %d col %s: %w", row, candleHeader[i], err)
			}
			vals[i] = v
		}
		candles = append(candles, object.Candle{
			Open:   vals[0],
			High:   vals[1],
			Low:    vals[2],
			Close:  vals[3],
			Volume: vals[4],
		})
	}

	return &object.CandleSeries{Value: candles}, nil
}
