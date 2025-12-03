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

// Benchmarks for TradeStore operations.
// These benchmarks measure performance of the in-memory trade storage and retrieval.
// Run with: go test -bench=. -benchmem ./fixclient/
package fixclient

import (
	"fmt"
	"strconv"
	"sync"
	"testing"
	"time"
)

// generateTestTrades creates a slice of test trades for benchmarking.
func generateTestTrades(count int, symbol string) []Trade {
	trades := make([]Trade, count)
	now := time.Now()
	for i := 0; i < count; i++ {
		trades[i] = Trade{
			Timestamp:  now,
			Symbol:     symbol,
			Price:      fmt.Sprintf("%.2f", 50000.00+float64(i)*0.01),
			Size:       fmt.Sprintf("%.4f", 1.5+float64(i)*0.001),
			Time:       now.Format(time.RFC3339),
			Aggressor:  "Buy",
			MdReqId:    "req-123",
			IsSnapshot: false,
			IsUpdate:   true,
			EntryType:  "2",
			Position:   "",
			SeqNum:     strconv.Itoa(i + 1),
		}
	}
	return trades
}

// BenchmarkAddTrades measures the performance of adding trades to the store.
// This is called for every incoming market data message.
func BenchmarkAddTrades(b *testing.B) {
	benchCases := []struct {
		name       string
		numTrades  int
		storeSize  int
		prefillPct float64 // percentage of store to prefill
	}{
		{"1Trade_EmptyStore", 1, 10000, 0},
		{"10Trades_EmptyStore", 10, 10000, 0},
		{"50Trades_EmptyStore", 50, 10000, 0},
		{"1Trade_HalfFull", 1, 10000, 0.5},
		{"10Trades_HalfFull", 10, 10000, 0.5},
		{"1Trade_AtCapacity", 1, 10000, 1.0},    // triggers eviction
		{"10Trades_AtCapacity", 10, 10000, 1.0}, // triggers multiple evictions
	}

	for _, bc := range benchCases {
		b.Run(bc.name, func(b *testing.B) {
			store := NewTradeStore(bc.storeSize, "")
			store.AddSubscription("BTC-USD", "1", "req-123")

			// Prefill the store
			prefillCount := int(float64(bc.storeSize) * bc.prefillPct)
			if prefillCount > 0 {
				prefillTrades := generateTestTrades(prefillCount, "BTC-USD")
				store.AddTrades("BTC-USD", prefillTrades, false, "req-123")
			}

			trades := generateTestTrades(bc.numTrades, "BTC-USD")

			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				store.AddTrades("BTC-USD", trades, false, "req-123")
			}
		})
	}
}

// BenchmarkGetRecentTrades measures the performance of retrieving recent trades.
// This is called when displaying trade history or processing requests.
func BenchmarkGetRecentTrades(b *testing.B) {
	benchCases := []struct {
		name      string
		storeSize int
		fillCount int
		limit     int
	}{
		{"Limit10_From100", 10000, 100, 10},
		{"Limit50_From1000", 10000, 1000, 50},
		{"Limit100_From5000", 10000, 5000, 100},
		{"Limit100_From10000", 10000, 10000, 100},
		{"Limit500_From10000", 10000, 10000, 500},
	}

	for _, bc := range benchCases {
		b.Run(bc.name, func(b *testing.B) {
			store := NewTradeStore(bc.storeSize, "")
			store.AddSubscription("BTC-USD", "1", "req-123")

			// Fill the store
			trades := generateTestTrades(bc.fillCount, "BTC-USD")
			store.AddTrades("BTC-USD", trades, false, "req-123")

			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_ = store.GetRecentTrades("BTC-USD", bc.limit)
			}
		})
	}
}

// BenchmarkGetRecentTradesMultiSymbol measures retrieval when store has multiple symbols.
// This tests the filtering overhead when scanning trades.
func BenchmarkGetRecentTradesMultiSymbol(b *testing.B) {
	symbols := []string{"BTC-USD", "ETH-USD", "SOL-USD", "AVAX-USD", "DOGE-USD"}
	storeSize := 10000
	tradesPerSymbol := 2000

	store := NewTradeStore(storeSize, "")
	for i, symbol := range symbols {
		reqId := fmt.Sprintf("req-%d", i)
		store.AddSubscription(symbol, "1", reqId)
		trades := generateTestTrades(tradesPerSymbol, symbol)
		store.AddTrades(symbol, trades, false, reqId)
	}

	benchCases := []struct {
		name   string
		symbol string
		limit  int
	}{
		{"FirstSymbol_Limit50", "BTC-USD", 50},
		{"MiddleSymbol_Limit50", "SOL-USD", 50},
		{"LastSymbol_Limit50", "DOGE-USD", 50},
		{"FirstSymbol_Limit200", "BTC-USD", 200},
	}

	for _, bc := range benchCases {
		b.Run(bc.name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				_ = store.GetRecentTrades(bc.symbol, bc.limit)
			}
		})
	}
}

// BenchmarkGetAllTrades measures the cost of copying all trades from the store.
func BenchmarkGetAllTrades(b *testing.B) {
	benchCases := []struct {
		name      string
		fillCount int
	}{
		{"100Trades", 100},
		{"1000Trades", 1000},
		{"5000Trades", 5000},
		{"10000Trades", 10000},
	}

	for _, bc := range benchCases {
		b.Run(bc.name, func(b *testing.B) {
			store := NewTradeStore(bc.fillCount, "")
			store.AddSubscription("BTC-USD", "1", "req-123")

			trades := generateTestTrades(bc.fillCount, "BTC-USD")
			store.AddTrades("BTC-USD", trades, false, "req-123")

			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_ = store.GetAllTrades()
			}
		})
	}
}

// BenchmarkGetSubscriptionStatus measures the cost of copying subscription state.
func BenchmarkGetSubscriptionStatus(b *testing.B) {
	benchCases := []struct {
		name             string
		numSubscriptions int
	}{
		{"1Subscription", 1},
		{"10Subscriptions", 10},
		{"50Subscriptions", 50},
		{"100Subscriptions", 100},
	}

	for _, bc := range benchCases {
		b.Run(bc.name, func(b *testing.B) {
			store := NewTradeStore(10000, "")

			for i := 0; i < bc.numSubscriptions; i++ {
				symbol := fmt.Sprintf("SYMBOL%d-USD", i)
				reqId := fmt.Sprintf("req-%d", i)
				store.AddSubscription(symbol, "1", reqId)
			}

			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_ = store.GetSubscriptionStatus()
			}
		})
	}
}

// BenchmarkConcurrentReadWrite measures performance under concurrent access patterns.
// This simulates real-world usage where writes (from FIX messages) and reads
// (from status/display) happen concurrently.
func BenchmarkConcurrentReadWrite(b *testing.B) {
	benchCases := []struct {
		name       string
		numWriters int
		numReaders int
	}{
		{"1Writer_1Reader", 1, 1},
		{"1Writer_4Readers", 1, 4},
		{"4Writers_4Readers", 4, 4},
		{"1Writer_10Readers", 1, 10},
	}

	for _, bc := range benchCases {
		b.Run(bc.name, func(b *testing.B) {
			store := NewTradeStore(10000, "")
			store.AddSubscription("BTC-USD", "1", "req-123")

			// Prefill with some data
			prefill := generateTestTrades(1000, "BTC-USD")
			store.AddTrades("BTC-USD", prefill, false, "req-123")

			trades := generateTestTrades(10, "BTC-USD")

			b.ReportAllocs()
			b.ResetTimer()

			var wg sync.WaitGroup
			iterations := b.N / (bc.numWriters + bc.numReaders)
			if iterations < 1 {
				iterations = 1
			}

			// Start writers
			for w := 0; w < bc.numWriters; w++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					for i := 0; i < iterations; i++ {
						store.AddTrades("BTC-USD", trades, false, "req-123")
					}
				}()
			}

			// Start readers
			for r := 0; r < bc.numReaders; r++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					for i := 0; i < iterations; i++ {
						_ = store.GetRecentTrades("BTC-USD", 50)
					}
				}()
			}

			wg.Wait()
		})
	}
}

// BenchmarkTradeStructSize reports the memory size of the Trade struct.
// Useful for understanding memory alignment and padding.
func BenchmarkTradeStructSize(b *testing.B) {
	// This benchmark is mainly for reporting allocation patterns
	b.Run("SingleTradeAllocation", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			t := Trade{
				Timestamp:  time.Now(),
				Symbol:     "BTC-USD",
				Price:      "50000.00",
				Size:       "1.5000",
				Time:       "2025-01-01T12:00:00Z",
				Aggressor:  "Buy",
				MdReqId:    "req-123",
				IsSnapshot: false,
				IsUpdate:   true,
				EntryType:  "2",
				Position:   "1",
				SeqNum:     "12345",
			}
			_ = t
		}
	})

	b.Run("SliceOf100Trades", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			trades := make([]Trade, 100)
			for j := range trades {
				trades[j] = Trade{
					Symbol: "BTC-USD",
					Price:  "50000.00",
				}
			}
			_ = trades
		}
	})
}

// BenchmarkCircularBufferOverhead measures the cost of the current
// slice-based eviction vs. ideal performance.
func BenchmarkCircularBufferOverhead(b *testing.B) {
	// Benchmark the current eviction approach
	b.Run("CurrentSliceEviction", func(b *testing.B) {
		store := NewTradeStore(1000, "")
		store.AddSubscription("BTC-USD", "1", "req-123")

		// Fill to capacity
		prefill := generateTestTrades(1000, "BTC-USD")
		store.AddTrades("BTC-USD", prefill, false, "req-123")

		singleTrade := generateTestTrades(1, "BTC-USD")

		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			store.AddTrades("BTC-USD", singleTrade, false, "req-123")
		}
	})
}
