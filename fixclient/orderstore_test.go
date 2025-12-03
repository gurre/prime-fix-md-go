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

package fixclient

import (
	"sync"
	"testing"
	"time"
)

// TestOrderStore_AddAndGetOrder verifies that orders can be added and retrieved by ClOrdID.
// This is the fundamental operation for tracking order lifecycle.
func TestOrderStore_AddAndGetOrder(t *testing.T) {
	store := NewOrderStore()

	order := &Order{
		ClOrdID:  "order-1",
		Symbol:   "BTC-USD",
		Side:     "1",
		OrdType:  "2",
		OrderQty: "0.01",
		Price:    "50000",
	}

	store.AddOrder(order)

	retrieved := store.GetOrder("order-1")
	if retrieved == nil {
		t.Fatal("expected to retrieve order")
	}

	if retrieved.Symbol != "BTC-USD" {
		t.Errorf("expected Symbol=BTC-USD, got %s", retrieved.Symbol)
	}
	if retrieved.Price != "50000" {
		t.Errorf("expected Price=50000, got %s", retrieved.Price)
	}
}

// TestOrderStore_GetOrder_ReturnsDefensiveCopy verifies that GetOrder returns a copy,
// not the original reference, to prevent external mutation of internal state.
func TestOrderStore_GetOrder_ReturnsDefensiveCopy(t *testing.T) {
	store := NewOrderStore()
	store.AddOrder(&Order{ClOrdID: "order-1", Symbol: "BTC-USD"})

	retrieved := store.GetOrder("order-1")
	retrieved.Symbol = "MODIFIED"

	original := store.GetOrder("order-1")
	if original.Symbol == "MODIFIED" {
		t.Error("GetOrder should return a defensive copy, but original was modified")
	}
}

// TestOrderStore_GetOrder_NotFound verifies nil return for non-existent orders.
func TestOrderStore_GetOrder_NotFound(t *testing.T) {
	store := NewOrderStore()

	if store.GetOrder("nonexistent") != nil {
		t.Error("expected nil for non-existent order")
	}
}

// TestOrderStore_GetOrderByOrderID verifies lookup by exchange-assigned OrderID.
func TestOrderStore_GetOrderByOrderID(t *testing.T) {
	store := NewOrderStore()
	store.AddOrder(&Order{
		ClOrdID: "client-order-1",
		OrderID: "exchange-order-abc",
		Symbol:  "ETH-USD",
	})

	retrieved := store.GetOrderByOrderID("exchange-order-abc")
	if retrieved == nil {
		t.Fatal("expected to retrieve order by OrderID")
	}

	if retrieved.ClOrdID != "client-order-1" {
		t.Errorf("expected ClOrdID=client-order-1, got %s", retrieved.ClOrdID)
	}
}

// TestOrderStore_UpdateOrderFromExecReport verifies that execution reports
// properly update order state, including partial fills and status changes.
func TestOrderStore_UpdateOrderFromExecReport(t *testing.T) {
	store := NewOrderStore()
	store.AddOrder(&Order{
		ClOrdID:   "order-1",
		Symbol:    "BTC-USD",
		OrdStatus: "A", // PendingNew
	})

	// Simulate order acknowledgement
	er := &ExecutionReport{
		ClOrdID:   "order-1",
		OrderID:   "cb-12345",
		ExecType:  "0", // New
		OrdStatus: "0", // New
		Symbol:    "BTC-USD",
		OrderQty:  "0.01",
		CumQty:    "0",
		LeavesQty: "0.01",
	}
	store.UpdateOrderFromExecReport(er)

	order := store.GetOrder("order-1")
	if order.OrdStatus != "0" {
		t.Errorf("expected OrdStatus=0, got %s", order.OrdStatus)
	}
	if order.OrderID != "cb-12345" {
		t.Errorf("expected OrderID=cb-12345, got %s", order.OrderID)
	}

	// Simulate partial fill
	er2 := &ExecutionReport{
		ClOrdID:    "order-1",
		OrderID:    "cb-12345",
		ExecType:   "1", // PartialFill
		OrdStatus:  "1", // PartiallyFilled
		Symbol:     "BTC-USD",
		OrderQty:   "0.01",
		CumQty:     "0.005",
		LeavesQty:  "0.005",
		LastPx:     "50100",
		LastShares: "0.005",
		AvgPx:      "50100",
	}
	store.UpdateOrderFromExecReport(er2)

	order = store.GetOrder("order-1")
	if order.OrdStatus != "1" {
		t.Errorf("expected OrdStatus=1, got %s", order.OrdStatus)
	}
	if order.CumQty != "0.005" {
		t.Errorf("expected CumQty=0.005, got %s", order.CumQty)
	}
}

// TestOrderStore_UpdateOrderFromExecReport_CreatesIfNotExists verifies that
// execution reports for unknown orders create new order entries.
func TestOrderStore_UpdateOrderFromExecReport_CreatesIfNotExists(t *testing.T) {
	store := NewOrderStore()

	er := &ExecutionReport{
		ClOrdID:   "new-order",
		OrderID:   "cb-99999",
		Symbol:    "SOL-USD",
		OrdStatus: "0",
	}
	store.UpdateOrderFromExecReport(er)

	order := store.GetOrder("new-order")
	if order == nil {
		t.Fatal("expected order to be created from execution report")
	}
	if order.Symbol != "SOL-USD" {
		t.Errorf("expected Symbol=SOL-USD, got %s", order.Symbol)
	}
}

// TestOrderStore_GetOpenOrders verifies filtering of orders by open status.
func TestOrderStore_GetOpenOrders(t *testing.T) {
	store := NewOrderStore()

	// Add orders with various statuses
	store.AddOrder(&Order{ClOrdID: "pending", OrdStatus: "A"})       // PendingNew - open
	store.AddOrder(&Order{ClOrdID: "new", OrdStatus: "0"})           // New - open
	store.AddOrder(&Order{ClOrdID: "partial", OrdStatus: "1"})       // PartiallyFilled - open
	store.AddOrder(&Order{ClOrdID: "filled", OrdStatus: "2"})        // Filled - closed
	store.AddOrder(&Order{ClOrdID: "canceled", OrdStatus: "4"})      // Canceled - closed
	store.AddOrder(&Order{ClOrdID: "rejected", OrdStatus: "8"})      // Rejected - closed
	store.AddOrder(&Order{ClOrdID: "pendingcxl", OrdStatus: "6"})    // PendingCancel - open
	store.AddOrder(&Order{ClOrdID: "pendingrepl", OrdStatus: "E"})   // PendingReplace - open

	open := store.GetOpenOrders()

	if len(open) != 5 {
		t.Errorf("expected 5 open orders, got %d", len(open))
	}

	// Verify closed orders are not included
	for _, o := range open {
		if o.ClOrdID == "filled" || o.ClOrdID == "canceled" || o.ClOrdID == "rejected" {
			t.Errorf("closed order %s should not be in open orders", o.ClOrdID)
		}
	}
}

// TestOrderStore_RemoveOrder verifies order removal.
func TestOrderStore_RemoveOrder(t *testing.T) {
	store := NewOrderStore()
	store.AddOrder(&Order{ClOrdID: "order-1", Symbol: "BTC-USD"})

	store.RemoveOrder("order-1")

	if store.GetOrder("order-1") != nil {
		t.Error("expected order to be removed")
	}
}

// TestOrderStore_Quote_AddAndGet verifies quote storage and retrieval.
func TestOrderStore_Quote_AddAndGet(t *testing.T) {
	store := NewOrderStore()

	quote := &Quote{
		QuoteID:    "quote-123",
		QuoteReqID: "rfq-1",
		Symbol:     "BTC-USD",
		BidPx:      "49900",
		BidSize:    "1.0",
	}
	store.AddQuote(quote)

	// Get by QuoteReqID
	retrieved := store.GetQuote("rfq-1")
	if retrieved == nil {
		t.Fatal("expected to retrieve quote by QuoteReqID")
	}
	if retrieved.QuoteID != "quote-123" {
		t.Errorf("expected QuoteID=quote-123, got %s", retrieved.QuoteID)
	}

	// Get by QuoteID
	byID := store.GetQuoteByQuoteID("quote-123")
	if byID == nil {
		t.Fatal("expected to retrieve quote by QuoteID")
	}
	if byID.QuoteReqID != "rfq-1" {
		t.Errorf("expected QuoteReqID=rfq-1, got %s", byID.QuoteReqID)
	}
}

// TestOrderStore_Quote_ReturnsDefensiveCopy verifies that quote retrieval
// returns a defensive copy.
func TestOrderStore_Quote_ReturnsDefensiveCopy(t *testing.T) {
	store := NewOrderStore()
	store.AddQuote(&Quote{QuoteReqID: "rfq-1", Symbol: "BTC-USD"})

	retrieved := store.GetQuote("rfq-1")
	retrieved.Symbol = "MODIFIED"

	original := store.GetQuote("rfq-1")
	if original.Symbol == "MODIFIED" {
		t.Error("GetQuote should return a defensive copy")
	}
}

// TestOrderStore_Concurrent verifies thread-safety under concurrent access.
// Multiple goroutines reading and writing should not cause data races.
func TestOrderStore_Concurrent(t *testing.T) {
	store := NewOrderStore()
	var wg sync.WaitGroup

	// Writers
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				store.AddOrder(&Order{
					ClOrdID: "order-concurrent",
					Symbol:  "BTC-USD",
				})
				store.UpdateOrderFromExecReport(&ExecutionReport{
					ClOrdID:   "order-concurrent",
					OrdStatus: "0",
				})
			}
		}(i)
	}

	// Readers
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				store.GetOrder("order-concurrent")
				store.GetAllOrders()
				store.GetOpenOrders()
			}
		}()
	}

	wg.Wait()
}

// TestOrderStore_AddOrder_SetsTimestamps verifies automatic timestamp management.
func TestOrderStore_AddOrder_SetsTimestamps(t *testing.T) {
	store := NewOrderStore()

	before := time.Now()
	store.AddOrder(&Order{ClOrdID: "order-1"})
	after := time.Now()

	order := store.GetOrder("order-1")
	if order.CreatedAt.Before(before) || order.CreatedAt.After(after) {
		t.Error("CreatedAt not set correctly")
	}
	if order.UpdatedAt.Before(before) || order.UpdatedAt.After(after) {
		t.Error("UpdatedAt not set correctly")
	}
}

// TestIsOpenStatus verifies the open status classification logic.
func TestIsOpenStatus(t *testing.T) {
	tests := []struct {
		status string
		open   bool
	}{
		{"0", true},  // New
		{"1", true},  // PartiallyFilled
		{"2", false}, // Filled
		{"3", false}, // DoneForDay
		{"4", false}, // Canceled
		{"5", false}, // Replaced
		{"6", true},  // PendingCancel
		{"7", false}, // Stopped
		{"8", false}, // Rejected
		{"9", true},  // Suspended
		{"A", true},  // PendingNew
		{"B", false}, // Calculated
		{"C", false}, // Expired
		{"D", false}, // AcceptedBidding
		{"E", true},  // PendingReplace
	}

	for _, tt := range tests {
		result := isOpenStatus(tt.status)
		if result != tt.open {
			t.Errorf("isOpenStatus(%s) = %v, want %v", tt.status, result, tt.open)
		}
	}
}
