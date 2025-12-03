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

// Benchmarks for OrderStore operations.
// These benchmarks measure performance of order tracking and state management.
// Run with: go test -bench=OrderStore -benchmem ./fixclient/
package fixclient

import (
	"fmt"
	"sync"
	"testing"
)

// generateTestOrders creates a slice of test orders for benchmarking.
func generateTestOrders(count int) []*Order {
	orders := make([]*Order, count)
	for i := 0; i < count; i++ {
		orders[i] = &Order{
			ClOrdID:   fmt.Sprintf("order-%d", i),
			OrderID:   fmt.Sprintf("cb-order-%d", i),
			Symbol:    "BTC-USD",
			Side:      "1",
			OrdType:   "2",
			OrdStatus: "0",
			OrderQty:  fmt.Sprintf("%.4f", 0.01+float64(i)*0.001),
			Price:     fmt.Sprintf("%.2f", 50000.00+float64(i)*10),
			CumQty:    "0",
			LeavesQty: fmt.Sprintf("%.4f", 0.01+float64(i)*0.001),
			Account:   "portfolio-123",
		}
	}
	return orders
}

// BenchmarkOrderStore_AddOrder measures the performance of adding orders.
func BenchmarkOrderStore_AddOrder(b *testing.B) {
	benchCases := []struct {
		name      string
		prefillN  int
	}{
		{"EmptyStore", 0},
		{"10Orders", 10},
		{"100Orders", 100},
		{"1000Orders", 1000},
	}

	for _, bc := range benchCases {
		b.Run(bc.name, func(b *testing.B) {
			store := NewOrderStore()

			// Prefill
			for i := 0; i < bc.prefillN; i++ {
				store.AddOrder(&Order{ClOrdID: fmt.Sprintf("prefill-%d", i)})
			}

			order := &Order{
				ClOrdID:   "bench-order",
				Symbol:    "BTC-USD",
				Side:      "1",
				OrdType:   "2",
				OrderQty:  "0.01",
				Price:     "50000",
				OrdStatus: "A",
			}

			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				store.AddOrder(order)
			}
		})
	}
}

// BenchmarkOrderStore_GetOrder measures lookup by ClOrdID.
func BenchmarkOrderStore_GetOrder(b *testing.B) {
	benchCases := []struct {
		name   string
		orders int
	}{
		{"10Orders", 10},
		{"100Orders", 100},
		{"1000Orders", 1000},
		{"10000Orders", 10000},
	}

	for _, bc := range benchCases {
		b.Run(bc.name, func(b *testing.B) {
			store := NewOrderStore()

			for i := 0; i < bc.orders; i++ {
				store.AddOrder(&Order{
					ClOrdID: fmt.Sprintf("order-%d", i),
					Symbol:  "BTC-USD",
				})
			}

			// Lookup middle element for fair comparison
			targetID := fmt.Sprintf("order-%d", bc.orders/2)

			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_ = store.GetOrder(targetID)
			}
		})
	}
}

// BenchmarkOrderStore_GetOrderByOrderID measures O(n) linear scan lookup.
func BenchmarkOrderStore_GetOrderByOrderID(b *testing.B) {
	benchCases := []struct {
		name   string
		orders int
	}{
		{"10Orders", 10},
		{"100Orders", 100},
		{"1000Orders", 1000},
	}

	for _, bc := range benchCases {
		b.Run(bc.name, func(b *testing.B) {
			store := NewOrderStore()

			for i := 0; i < bc.orders; i++ {
				store.AddOrder(&Order{
					ClOrdID: fmt.Sprintf("order-%d", i),
					OrderID: fmt.Sprintf("cb-order-%d", i),
					Symbol:  "BTC-USD",
				})
			}

			// Lookup middle element (worst case is last)
			targetID := fmt.Sprintf("cb-order-%d", bc.orders/2)

			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_ = store.GetOrderByOrderID(targetID)
			}
		})
	}
}

// BenchmarkOrderStore_UpdateFromExecReport measures execution report processing.
func BenchmarkOrderStore_UpdateFromExecReport(b *testing.B) {
	benchCases := []struct {
		name       string
		preExists  bool
	}{
		{"NewOrder", false},
		{"ExistingOrder", true},
	}

	for _, bc := range benchCases {
		b.Run(bc.name, func(b *testing.B) {
			store := NewOrderStore()

			if bc.preExists {
				store.AddOrder(&Order{
					ClOrdID:   "order-1",
					Symbol:    "BTC-USD",
					OrdStatus: "A",
				})
			}

			er := &ExecutionReport{
				ClOrdID:   "order-1",
				OrderID:   "cb-order-1",
				Symbol:    "BTC-USD",
				Side:      "1",
				OrdType:   "2",
				OrdStatus: "0",
				ExecType:  "0",
				OrderQty:  "0.01",
				CumQty:    "0",
				LeavesQty: "0.01",
			}

			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				store.UpdateOrderFromExecReport(er)
			}
		})
	}
}

// BenchmarkOrderStore_GetAllOrders measures bulk copy cost.
func BenchmarkOrderStore_GetAllOrders(b *testing.B) {
	benchCases := []struct {
		name   string
		orders int
	}{
		{"10Orders", 10},
		{"100Orders", 100},
		{"500Orders", 500},
		{"1000Orders", 1000},
	}

	for _, bc := range benchCases {
		b.Run(bc.name, func(b *testing.B) {
			store := NewOrderStore()

			for i := 0; i < bc.orders; i++ {
				store.AddOrder(&Order{
					ClOrdID:   fmt.Sprintf("order-%d", i),
					Symbol:    "BTC-USD",
					Side:      "1",
					OrdType:   "2",
					OrdStatus: "0",
					OrderQty:  "0.01",
					Price:     "50000",
				})
			}

			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_ = store.GetAllOrders()
			}
		})
	}
}

// BenchmarkOrderStore_GetOpenOrders measures filtered retrieval.
func BenchmarkOrderStore_GetOpenOrders(b *testing.B) {
	benchCases := []struct {
		name       string
		orders     int
		openRatio  float64 // percentage of orders that are open
	}{
		{"100Orders_50%Open", 100, 0.5},
		{"100Orders_10%Open", 100, 0.1},
		{"1000Orders_50%Open", 1000, 0.5},
		{"1000Orders_10%Open", 1000, 0.1},
	}

	for _, bc := range benchCases {
		b.Run(bc.name, func(b *testing.B) {
			store := NewOrderStore()

			openCount := int(float64(bc.orders) * bc.openRatio)
			for i := 0; i < bc.orders; i++ {
				status := "2" // Filled (closed)
				if i < openCount {
					status = "0" // New (open)
				}
				store.AddOrder(&Order{
					ClOrdID:   fmt.Sprintf("order-%d", i),
					OrdStatus: status,
				})
			}

			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_ = store.GetOpenOrders()
			}
		})
	}
}

// BenchmarkOrderStore_Quote measures quote operations.
func BenchmarkOrderStore_Quote(b *testing.B) {
	b.Run("AddQuote", func(b *testing.B) {
		store := NewOrderStore()

		quote := &Quote{
			QuoteID:    "quote-123",
			QuoteReqID: "rfq-1",
			Symbol:     "BTC-USD",
			BidPx:      "49900",
			BidSize:    "1.0",
		}

		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			store.AddQuote(quote)
		}
	})

	b.Run("GetQuote", func(b *testing.B) {
		store := NewOrderStore()

		for i := 0; i < 100; i++ {
			store.AddQuote(&Quote{
				QuoteReqID: fmt.Sprintf("rfq-%d", i),
				QuoteID:    fmt.Sprintf("quote-%d", i),
				Symbol:     "BTC-USD",
			})
		}

		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = store.GetQuote("rfq-50")
		}
	})

	b.Run("GetQuoteByQuoteID_LinearScan", func(b *testing.B) {
		store := NewOrderStore()

		for i := 0; i < 100; i++ {
			store.AddQuote(&Quote{
				QuoteReqID: fmt.Sprintf("rfq-%d", i),
				QuoteID:    fmt.Sprintf("quote-%d", i),
				Symbol:     "BTC-USD",
			})
		}

		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = store.GetQuoteByQuoteID("quote-50")
		}
	})
}

// BenchmarkOrderStore_ConcurrentAccess measures thread-safety overhead.
func BenchmarkOrderStore_ConcurrentAccess(b *testing.B) {
	benchCases := []struct {
		name       string
		numWriters int
		numReaders int
	}{
		{"1Writer_1Reader", 1, 1},
		{"1Writer_4Readers", 1, 4},
		{"4Writers_4Readers", 4, 4},
	}

	for _, bc := range benchCases {
		b.Run(bc.name, func(b *testing.B) {
			store := NewOrderStore()

			// Prefill
			for i := 0; i < 100; i++ {
				store.AddOrder(&Order{
					ClOrdID:   fmt.Sprintf("order-%d", i),
					OrdStatus: "0",
				})
			}

			b.ReportAllocs()
			b.ResetTimer()

			var wg sync.WaitGroup
			iterations := b.N / (bc.numWriters + bc.numReaders)
			if iterations < 1 {
				iterations = 1
			}

			// Writers: update from exec reports
			for w := 0; w < bc.numWriters; w++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					er := &ExecutionReport{
						ClOrdID:   "order-50",
						OrdStatus: "1",
						CumQty:    "0.005",
					}
					for i := 0; i < iterations; i++ {
						store.UpdateOrderFromExecReport(er)
					}
				}()
			}

			// Readers
			for r := 0; r < bc.numReaders; r++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					for i := 0; i < iterations; i++ {
						_ = store.GetOrder("order-50")
						_ = store.GetOpenOrders()
					}
				}()
			}

			wg.Wait()
		})
	}
}

// BenchmarkOrderStore_HighFrequencyUpdates simulates rapid execution report updates.
func BenchmarkOrderStore_HighFrequencyUpdates(b *testing.B) {
	store := NewOrderStore()

	// Pre-create orders
	for i := 0; i < 100; i++ {
		store.AddOrder(&Order{
			ClOrdID:   fmt.Sprintf("order-%d", i),
			OrdStatus: "0",
			OrderQty:  "1.0",
			CumQty:    "0",
			LeavesQty: "1.0",
		})
	}

	// Simulate partial fills
	ers := make([]*ExecutionReport, 100)
	for i := 0; i < 100; i++ {
		ers[i] = &ExecutionReport{
			ClOrdID:    fmt.Sprintf("order-%d", i),
			ExecType:   "1", // Partial fill
			OrdStatus:  "1",
			CumQty:     fmt.Sprintf("%.4f", float64(i)*0.01),
			LeavesQty:  fmt.Sprintf("%.4f", 1.0-float64(i)*0.01),
			LastPx:     "50000",
			LastShares: "0.01",
		}
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		store.UpdateOrderFromExecReport(ers[i%100])
	}
}

// BenchmarkIsOpenStatus measures the status check function.
func BenchmarkIsOpenStatus(b *testing.B) {
	statuses := []string{"0", "1", "2", "4", "6", "8", "A", "E"}

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		for _, s := range statuses {
			_ = isOpenStatus(s)
		}
	}
}
