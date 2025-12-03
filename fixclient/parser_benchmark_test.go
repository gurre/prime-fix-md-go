/**
 * Copyright 2025-present Coinbase Global, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *  http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

// Benchmarks for FIX message parsing functions.
// These benchmarks measure performance of the critical hot path in market data processing.
// Run with: go test -bench=. -benchmem ./fixclient/
package fixclient

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

// generateFIXMessage creates a realistic FIX market data message with the specified number of entries.
// This simulates actual market data snapshots received from Coinbase Prime.
func generateFIXMessage(numEntries int) string {
	var b strings.Builder
	// FIX header
	b.WriteString("8=FIX.4.4\x019=1000\x0135=W\x0149=COINBASE\x0156=CLIENT\x0134=12345\x01")
	b.WriteString("52=20250101-12:00:00.123\x0155=BTC-USD\x01262=req-12345\x01")
	b.WriteString(fmt.Sprintf("268=%d\x01", numEntries))

	// Generate entries with alternating bids, offers, and trades
	for i := 0; i < numEntries; i++ {
		entryType := i % 3 // 0=Bid, 1=Offer, 2=Trade
		price := 50000.00 + float64(i)*0.01
		size := 1.5 + float64(i)*0.1

		b.WriteString(fmt.Sprintf("269=%d\x01", entryType))
		b.WriteString(fmt.Sprintf("270=%.2f\x01", price))
		b.WriteString(fmt.Sprintf("271=%.4f\x01", size))
		b.WriteString("273=20250101-12:00:00\x01")

		if entryType == 2 { // Trade entry has aggressor
			b.WriteString("2446=1\x01")
		}
		if entryType == 0 || entryType == 1 { // Bid/Offer has position
			b.WriteString(fmt.Sprintf("290=%d\x01", (i/3)+1))
		}
	}

	// FIX trailer
	b.WriteString("10=123\x01")
	return b.String()
}

// BenchmarkFindEntryBoundaries measures the performance of locating MD entry boundaries
// in raw FIX messages. This is called once per message to find all entry start positions.
func BenchmarkFindEntryBoundaries(b *testing.B) {
	app := &FixApp{TradeStore: NewTradeStore(1000, "")}

	benchCases := []struct {
		name       string
		numEntries int
	}{
		{"1Entry", 1},
		{"5Entries", 5},
		{"10Entries", 10},
		{"20Entries", 20},
		{"50Entries", 50},
		{"100Entries", 100},
	}

	for _, bc := range benchCases {
		rawMsg := generateFIXMessage(bc.numEntries)
		b.Run(bc.name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				_ = app.findEntryBoundaries(rawMsg)
			}
		})
	}
}

// BenchmarkExtractSingleFieldValue measures the performance of extracting a single
// FIX field value from a message segment. Called multiple times per entry.
func BenchmarkExtractSingleFieldValue(b *testing.B) {
	// Typical FIX entry segment
	segment := "269=2\x01270=50000.00\x01271=1.5000\x01273=20250101-12:00:00\x012446=1\x01290=5\x01"

	benchCases := []struct {
		name      string
		tagPrefix string
	}{
		{"FirstField_269", "269="},
		{"MiddleField_271", "271="},
		{"LastField_290", "290="},
		{"MissingField_999", "999="},
	}

	for _, bc := range benchCases {
		b.Run(bc.name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				_ = extractSingleFieldValue(segment, bc.tagPrefix)
			}
		})
	}
}

// BenchmarkParseTradeFromSegment measures the performance of parsing a complete
// trade entry from a FIX message segment using the original multi-pass approach.
func BenchmarkParseTradeFromSegment(b *testing.B) {
	app := &FixApp{TradeStore: NewTradeStore(1000, "")}

	benchCases := []struct {
		name    string
		segment string
	}{
		{
			"TradeEntry",
			"269=2\x01270=50000.00\x01271=1.5000\x01273=20250101-12:00:00\x012446=1\x01",
		},
		{
			"BidEntry",
			"269=0\x01270=49999.00\x01271=2.5000\x01273=20250101-12:00:00\x01290=1\x01",
		},
		{
			"OfferEntry",
			"269=1\x01270=50001.00\x01271=3.0000\x01273=20250101-12:00:00\x01290=1\x01",
		},
		{
			"OHLCVEntry",
			"269=4\x01270=49500.00\x01273=20250101-12:00:00\x01",
		},
	}

	for _, bc := range benchCases {
		b.Run(bc.name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				_ = app.parseTradeFromSegment(bc.segment, "BTC-USD", "req-123", false, "12345", 0)
			}
		})
	}
}

// BenchmarkParseTradeFromSegmentFast measures the optimized single-pass parser.
// Compare with BenchmarkParseTradeFromSegment to see the improvement.
func BenchmarkParseTradeFromSegmentFast(b *testing.B) {
	app := &FixApp{TradeStore: NewTradeStore(1000, "")}
	now := time.Now()

	benchCases := []struct {
		name    string
		segment string
	}{
		{
			"TradeEntry",
			"269=2\x01270=50000.00\x01271=1.5000\x01273=20250101-12:00:00\x012446=1\x01",
		},
		{
			"BidEntry",
			"269=0\x01270=49999.00\x01271=2.5000\x01273=20250101-12:00:00\x01290=1\x01",
		},
		{
			"OfferEntry",
			"269=1\x01270=50001.00\x01271=3.0000\x01273=20250101-12:00:00\x01290=1\x01",
		},
		{
			"OHLCVEntry",
			"269=4\x01270=49500.00\x01273=20250101-12:00:00\x01",
		},
	}

	for _, bc := range benchCases {
		b.Run(bc.name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				_ = app.parseTradeFromSegmentFast(bc.segment, "BTC-USD", "req-123", false, "12345", 0, now)
			}
		})
	}
}

// BenchmarkExtractTradesOld measures end-to-end parsing with the OLD multi-pass parser.
// Kept for comparison with BenchmarkExtractTradesFast.
func BenchmarkExtractTradesOld(b *testing.B) {
	app := &FixApp{TradeStore: NewTradeStore(1000, "")}

	benchCases := []struct {
		name       string
		numEntries int
	}{
		{"1Entry", 1},
		{"5Entries", 5},
		{"10Entries", 10},
		{"20Entries", 20},
		{"50Entries", 50},
		{"100Entries", 100},
	}

	for _, bc := range benchCases {
		rawMsg := generateFIXMessage(bc.numEntries)
		b.Run(bc.name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				boundaries := app.findEntryBoundaries(rawMsg)
				for j, startPos := range boundaries {
					endPos := app.getEntryEndPos(boundaries, j, len(rawMsg))
					segment := rawMsg[startPos:endPos]
					_ = app.parseTradeFromSegment(segment, "BTC-USD", "req-123", false, "12345", j)
				}
			}
		})
	}
}

// BenchmarkExtractTradesFast measures end-to-end parsing with the NEW single-pass parser.
// This represents the current production code path.
func BenchmarkExtractTradesFast(b *testing.B) {
	app := &FixApp{TradeStore: NewTradeStore(1000, "")}

	benchCases := []struct {
		name       string
		numEntries int
	}{
		{"1Entry", 1},
		{"5Entries", 5},
		{"10Entries", 10},
		{"20Entries", 20},
		{"50Entries", 50},
		{"100Entries", 100},
	}

	for _, bc := range benchCases {
		rawMsg := generateFIXMessage(bc.numEntries)
		b.Run(bc.name, func(b *testing.B) {
			now := time.Now()
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				boundaries := app.findEntryBoundaries(rawMsg)
				for j, startPos := range boundaries {
					endPos := app.getEntryEndPos(boundaries, j, len(rawMsg))
					segment := rawMsg[startPos:endPos]
					_ = app.parseTradeFromSegmentFast(segment, "BTC-USD", "req-123", false, "12345", j, now)
				}
			}
		})
	}
}

// BenchmarkStringOperations measures the cost of common string operations
// used in FIX parsing to identify optimization opportunities.
func BenchmarkStringOperations(b *testing.B) {
	rawMsg := generateFIXMessage(20)
	segment := "269=2\x01270=50000.00\x01271=1.5000\x01273=20250101-12:00:00\x012446=1\x01"

	b.Run("StringsIndex", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			_ = strings.Index(rawMsg, "269=")
		}
	})

	b.Run("StringsIndexFromOffset", func(b *testing.B) {
		b.ReportAllocs()
		offset := 100
		for i := 0; i < b.N; i++ {
			_ = strings.Index(rawMsg[offset:], "269=")
		}
	})

	b.Run("Substring", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			_ = segment[4:14]
		}
	})
}
