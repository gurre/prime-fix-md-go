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
	"strconv"
	"sync"
	"testing"
)

// Tests for TradeStore behavior.
// These tests verify the observable behavior of trade storage including:
// - Adding and retrieving trades
// - Capacity limits and eviction policy
// - Symbol-based filtering
// - Subscription lifecycle management
// - Concurrent access safety

// TestTradeStore_AddedTradesAreRetrievable verifies that trades added to the
// store can be retrieved by symbol. This is the fundamental storage contract.
func TestTradeStore_AddedTradesAreRetrievable(t *testing.T) {
	store := NewTradeStore(100, "")

	trades := []Trade{
		{Price: "50000.00", Size: "1.5"},
		{Price: "50001.00", Size: "2.0"},
	}

	store.AddTrades("BTC-USD", trades, false, "req-123")

	got := store.GetRecentTrades("BTC-USD", 10)

	if len(got) != 2 {
		t.Fatalf("expected 2 trades, got %d", len(got))
	}
}

// TestTradeStore_TradesReturnedInChronologicalOrder verifies that GetRecentTrades
// returns trades with oldest first, newest last - matching insertion order.
func TestTradeStore_TradesReturnedInChronologicalOrder(t *testing.T) {
	store := NewTradeStore(100, "")

	// Add trades with distinct prices to identify order
	trades := []Trade{
		{Price: "1000"},
		{Price: "2000"},
		{Price: "3000"},
	}
	store.AddTrades("BTC-USD", trades, false, "req-123")

	got := store.GetRecentTrades("BTC-USD", 10)

	if len(got) != 3 {
		t.Fatalf("expected 3 trades, got %d", len(got))
	}

	// Verify chronological order: oldest (1000) first, newest (3000) last
	if got[0].Price != "1000" {
		t.Errorf("first trade should be oldest (1000), got %s", got[0].Price)
	}
	if got[2].Price != "3000" {
		t.Errorf("last trade should be newest (3000), got %s", got[2].Price)
	}
}

// TestTradeStore_OldestTradesEvictedAtCapacity verifies the ring buffer eviction
// policy: when capacity is reached, the oldest trades are removed to make room.
func TestTradeStore_OldestTradesEvictedAtCapacity(t *testing.T) {
	capacity := 5
	store := NewTradeStore(capacity, "")

	// Add more trades than capacity
	for i := 0; i < 10; i++ {
		trades := []Trade{{Price: strconv.Itoa(i)}}
		store.AddTrades("BTC-USD", trades, false, "req-123")
	}

	got := store.GetRecentTrades("BTC-USD", 100)

	// Should only have the most recent 5 trades (indices 5-9)
	if len(got) != capacity {
		t.Fatalf("expected %d trades at capacity, got %d", capacity, len(got))
	}

	// Oldest remaining should be index 5, newest should be index 9
	if got[0].Price != "5" {
		t.Errorf("oldest trade should be '5' after eviction, got %s", got[0].Price)
	}
	if got[4].Price != "9" {
		t.Errorf("newest trade should be '9', got %s", got[4].Price)
	}
}

// TestTradeStore_LimitRespectsRequestedCount verifies that GetRecentTrades
// returns at most the requested number of trades.
func TestTradeStore_LimitRespectsRequestedCount(t *testing.T) {
	store := NewTradeStore(100, "")

	// Add 10 trades
	for i := 0; i < 10; i++ {
		trades := []Trade{{Price: strconv.Itoa(i)}}
		store.AddTrades("BTC-USD", trades, false, "req-123")
	}

	// Request only 3
	got := store.GetRecentTrades("BTC-USD", 3)

	if len(got) != 3 {
		t.Fatalf("expected exactly 3 trades with limit=3, got %d", len(got))
	}

	// Should be the 3 most recent (7, 8, 9)
	if got[0].Price != "7" || got[1].Price != "8" || got[2].Price != "9" {
		t.Errorf("expected most recent 3 trades (7,8,9), got %s,%s,%s",
			got[0].Price, got[1].Price, got[2].Price)
	}
}

// TestTradeStore_SymbolFilteringReturnsOnlyMatchingTrades verifies that
// GetRecentTrades filters by symbol and doesn't return trades from other symbols.
func TestTradeStore_SymbolFilteringReturnsOnlyMatchingTrades(t *testing.T) {
	store := NewTradeStore(100, "")

	store.AddTrades("BTC-USD", []Trade{{Price: "50000"}}, false, "req-1")
	store.AddTrades("ETH-USD", []Trade{{Price: "3000"}}, false, "req-2")
	store.AddTrades("BTC-USD", []Trade{{Price: "50001"}}, false, "req-3")
	store.AddTrades("SOL-USD", []Trade{{Price: "100"}}, false, "req-4")

	btcTrades := store.GetRecentTrades("BTC-USD", 100)
	ethTrades := store.GetRecentTrades("ETH-USD", 100)
	solTrades := store.GetRecentTrades("SOL-USD", 100)

	if len(btcTrades) != 2 {
		t.Errorf("expected 2 BTC trades, got %d", len(btcTrades))
	}
	if len(ethTrades) != 1 {
		t.Errorf("expected 1 ETH trade, got %d", len(ethTrades))
	}
	if len(solTrades) != 1 {
		t.Errorf("expected 1 SOL trade, got %d", len(solTrades))
	}

	// Verify no cross-contamination
	for _, trade := range btcTrades {
		if trade.Symbol != "BTC-USD" {
			t.Errorf("BTC trades should only contain BTC-USD, got %s", trade.Symbol)
		}
	}
}

// TestTradeStore_GetAllTradesReturnsChronologicalCopy verifies that GetAllTrades
// returns all trades in chronological order as a defensive copy.
func TestTradeStore_GetAllTradesReturnsChronologicalCopy(t *testing.T) {
	store := NewTradeStore(100, "")

	store.AddTrades("BTC-USD", []Trade{{Price: "100"}}, false, "req-1")
	store.AddTrades("ETH-USD", []Trade{{Price: "200"}}, false, "req-2")
	store.AddTrades("BTC-USD", []Trade{{Price: "300"}}, false, "req-3")

	got := store.GetAllTrades()

	if len(got) != 3 {
		t.Fatalf("expected 3 trades, got %d", len(got))
	}

	// Verify chronological order
	if got[0].Price != "100" || got[1].Price != "200" || got[2].Price != "300" {
		t.Errorf("expected chronological order 100,200,300, got %s,%s,%s",
			got[0].Price, got[1].Price, got[2].Price)
	}
}

// TestTradeStore_EmptyStoreReturnsNil verifies that empty stores return nil
// rather than empty slices, matching Go idioms.
func TestTradeStore_EmptyStoreReturnsNil(t *testing.T) {
	store := NewTradeStore(100, "")

	recent := store.GetRecentTrades("BTC-USD", 10)
	all := store.GetAllTrades()

	if recent != nil {
		t.Error("expected nil for GetRecentTrades on empty store")
	}
	if all != nil {
		t.Error("expected nil for GetAllTrades on empty store")
	}
}

// TestTradeStore_NonExistentSymbolReturnsNil verifies that querying for a
// symbol with no trades returns nil.
func TestTradeStore_NonExistentSymbolReturnsNil(t *testing.T) {
	store := NewTradeStore(100, "")

	store.AddTrades("BTC-USD", []Trade{{Price: "50000"}}, false, "req-1")

	got := store.GetRecentTrades("NONEXISTENT", 10)

	if got != nil {
		t.Errorf("expected nil for nonexistent symbol, got %d trades", len(got))
	}
}

// TestTradeStore_FieldsPopulatedOnAdd verifies that AddTrades correctly
// populates the symbol, snapshot flag, and mdReqId on stored trades.
func TestTradeStore_FieldsPopulatedOnAdd(t *testing.T) {
	store := NewTradeStore(100, "")

	// Add a minimal trade - AddTrades should populate metadata fields
	store.AddTrades("ETH-USD", []Trade{{Price: "3000"}}, true, "req-snapshot")

	got := store.GetRecentTrades("ETH-USD", 1)

	if len(got) != 1 {
		t.Fatalf("expected 1 trade, got %d", len(got))
	}

	trade := got[0]
	if trade.Symbol != "ETH-USD" {
		t.Errorf("expected symbol ETH-USD, got %s", trade.Symbol)
	}
	if !trade.IsSnapshot {
		t.Error("expected IsSnapshot=true for snapshot trade")
	}
	if trade.IsUpdate {
		t.Error("expected IsUpdate=false for snapshot trade")
	}
	if trade.MdReqId != "req-snapshot" {
		t.Errorf("expected MdReqId 'req-snapshot', got %s", trade.MdReqId)
	}
}

// --- Subscription Tests ---

// TestSubscription_AddAndRemoveByReqId verifies the subscription lifecycle:
// add, verify active, remove by reqId, verify removed.
func TestSubscription_AddAndRemoveByReqId(t *testing.T) {
	store := NewTradeStore(100, "")

	store.AddSubscription("BTC-USD", "1", "req-123")

	subs := store.GetSubscriptionStatus()
	if len(subs) != 1 {
		t.Fatalf("expected 1 subscription after add, got %d", len(subs))
	}

	if subs["req-123"] == nil {
		t.Fatal("expected subscription with reqId 'req-123'")
	}
	if subs["req-123"].Symbol != "BTC-USD" {
		t.Errorf("expected symbol BTC-USD, got %s", subs["req-123"].Symbol)
	}

	store.RemoveSubscriptionByReqId("req-123")

	subs = store.GetSubscriptionStatus()
	if len(subs) != 0 {
		t.Fatalf("expected 0 subscriptions after removal, got %d", len(subs))
	}
}

// TestSubscription_RemoveBySymbolRemovesAllMatching verifies that
// RemoveSubscription removes all subscriptions for a symbol.
func TestSubscription_RemoveBySymbolRemovesAllMatching(t *testing.T) {
	store := NewTradeStore(100, "")

	// Add multiple subscriptions for the same symbol
	store.AddSubscription("BTC-USD", "1", "req-1")
	store.AddSubscription("BTC-USD", "1", "req-2")
	store.AddSubscription("ETH-USD", "1", "req-3")

	store.RemoveSubscription("BTC-USD")

	subs := store.GetSubscriptionStatus()
	if len(subs) != 1 {
		t.Fatalf("expected 1 subscription (ETH-USD), got %d", len(subs))
	}
	if subs["req-3"] == nil {
		t.Error("ETH-USD subscription should remain")
	}
}

// TestSubscription_SnapshotReceivedFlagUpdated verifies that adding snapshot
// trades updates the subscription's SnapshotReceived flag.
func TestSubscription_SnapshotReceivedFlagUpdated(t *testing.T) {
	store := NewTradeStore(100, "")

	store.AddSubscription("BTC-USD", "1", "req-123")

	// Add non-snapshot trade first
	store.AddTrades("BTC-USD", []Trade{{Price: "50000"}}, false, "req-123")

	subs := store.GetSubscriptionStatus()
	if subs["req-123"].SnapshotReceived {
		t.Error("SnapshotReceived should be false after incremental update")
	}

	// Add snapshot trade
	store.AddTrades("BTC-USD", []Trade{{Price: "50001"}}, true, "req-123")

	subs = store.GetSubscriptionStatus()
	if !subs["req-123"].SnapshotReceived {
		t.Error("SnapshotReceived should be true after snapshot")
	}
}

// TestSubscription_TotalUpdatesTracked verifies that the subscription tracks
// the total count of trades received.
func TestSubscription_TotalUpdatesTracked(t *testing.T) {
	store := NewTradeStore(100, "")

	store.AddSubscription("BTC-USD", "1", "req-123")

	// Add batches of trades
	store.AddTrades("BTC-USD", []Trade{{}, {}, {}}, false, "req-123") // 3 trades
	store.AddTrades("BTC-USD", []Trade{{}, {}}, false, "req-123")     // 2 more trades

	subs := store.GetSubscriptionStatus()
	if subs["req-123"].TotalUpdates != 5 {
		t.Errorf("expected TotalUpdates=5, got %d", subs["req-123"].TotalUpdates)
	}
}

// TestSubscription_GetBySymbol verifies that subscriptions can be grouped
// and retrieved by symbol.
func TestSubscription_GetBySymbol(t *testing.T) {
	store := NewTradeStore(100, "")

	store.AddSubscription("BTC-USD", "1", "req-1")
	store.AddSubscription("BTC-USD", "0", "req-2")
	store.AddSubscription("ETH-USD", "1", "req-3")

	bySymbol := store.GetSubscriptionsBySymbol()

	if len(bySymbol["BTC-USD"]) != 2 {
		t.Errorf("expected 2 BTC-USD subscriptions, got %d", len(bySymbol["BTC-USD"]))
	}
	if len(bySymbol["ETH-USD"]) != 1 {
		t.Errorf("expected 1 ETH-USD subscription, got %d", len(bySymbol["ETH-USD"]))
	}
}

// --- Concurrent Access Tests ---

// TestTradeStore_ConcurrentReadWriteSafety verifies that the store handles
// concurrent reads and writes without data races or panics.
func TestTradeStore_ConcurrentReadWriteSafety(t *testing.T) {
	store := NewTradeStore(1000, "")

	var wg sync.WaitGroup
	numWriters := 5
	numReaders := 5
	opsPerGoroutine := 100

	// Writers add trades
	for i := 0; i < numWriters; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < opsPerGoroutine; j++ {
				trades := []Trade{{Price: strconv.Itoa(id*1000 + j)}}
				store.AddTrades("BTC-USD", trades, false, "req-"+strconv.Itoa(id))
			}
		}(i)
	}

	// Readers query trades
	for i := 0; i < numReaders; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < opsPerGoroutine; j++ {
				_ = store.GetRecentTrades("BTC-USD", 10)
				_ = store.GetAllTrades()
			}
		}()
	}

	wg.Wait()

	// Verify some trades were stored (exact count depends on timing)
	trades := store.GetRecentTrades("BTC-USD", 1000)
	if len(trades) == 0 {
		t.Error("expected some trades after concurrent operations")
	}
}

// TestTradeStore_ConcurrentSubscriptionAccess verifies thread-safe subscription
// management under concurrent access.
func TestTradeStore_ConcurrentSubscriptionAccess(t *testing.T) {
	store := NewTradeStore(100, "")

	var wg sync.WaitGroup
	numGoroutines := 10

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			reqId := "req-" + strconv.Itoa(id)

			// Add subscription
			store.AddSubscription("BTC-USD", "1", reqId)

			// Query status multiple times
			for j := 0; j < 10; j++ {
				_ = store.GetSubscriptionStatus()
				_ = store.GetSubscriptionsBySymbol()
			}

			// Remove subscription
			store.RemoveSubscriptionByReqId(reqId)
		}(i)
	}

	wg.Wait()

	// All subscriptions should be removed
	subs := store.GetSubscriptionStatus()
	if len(subs) != 0 {
		t.Errorf("expected 0 subscriptions after cleanup, got %d", len(subs))
	}
}

// --- Edge Cases ---

// TestTradeStore_SingleItemCapacity verifies correct behavior with capacity=1.
func TestTradeStore_SingleItemCapacity(t *testing.T) {
	store := NewTradeStore(1, "")

	store.AddTrades("BTC-USD", []Trade{{Price: "100"}}, false, "req-1")
	store.AddTrades("BTC-USD", []Trade{{Price: "200"}}, false, "req-2")
	store.AddTrades("BTC-USD", []Trade{{Price: "300"}}, false, "req-3")

	got := store.GetRecentTrades("BTC-USD", 10)

	if len(got) != 1 {
		t.Fatalf("expected 1 trade with capacity=1, got %d", len(got))
	}
	if got[0].Price != "300" {
		t.Errorf("expected most recent trade (300), got %s", got[0].Price)
	}
}

// TestTradeStore_WrapAroundEviction verifies ring buffer behavior when
// evictions wrap around the buffer multiple times.
func TestTradeStore_WrapAroundEviction(t *testing.T) {
	capacity := 5
	store := NewTradeStore(capacity, "")

	// Add 3x capacity trades to force multiple wraparounds
	for i := 0; i < capacity*3; i++ {
		trades := []Trade{{Price: strconv.Itoa(i)}}
		store.AddTrades("BTC-USD", trades, false, "req-123")
	}

	got := store.GetRecentTrades("BTC-USD", 100)

	if len(got) != capacity {
		t.Fatalf("expected %d trades, got %d", capacity, len(got))
	}

	// Should have trades 10-14 (last 5 of 0-14)
	for i, trade := range got {
		expected := strconv.Itoa(10 + i)
		if trade.Price != expected {
			t.Errorf("trade[%d]: expected price %s, got %s", i, expected, trade.Price)
		}
	}
}

// TestTradeStore_BatchAddLargerThanCapacity verifies that adding a batch
// larger than capacity keeps only the most recent trades.
func TestTradeStore_BatchAddLargerThanCapacity(t *testing.T) {
	capacity := 3
	store := NewTradeStore(capacity, "")

	// Add batch of 5 trades (larger than capacity of 3)
	trades := []Trade{
		{Price: "100"},
		{Price: "200"},
		{Price: "300"},
		{Price: "400"},
		{Price: "500"},
	}
	store.AddTrades("BTC-USD", trades, false, "req-123")

	got := store.GetRecentTrades("BTC-USD", 10)

	if len(got) != capacity {
		t.Fatalf("expected %d trades, got %d", capacity, len(got))
	}

	// Should keep most recent 3: 300, 400, 500
	if got[0].Price != "300" || got[1].Price != "400" || got[2].Price != "500" {
		t.Errorf("expected 300,400,500, got %s,%s,%s",
			got[0].Price, got[1].Price, got[2].Price)
	}
}
