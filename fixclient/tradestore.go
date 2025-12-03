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

// Package fixclient provides in-memory trade storage with a ring buffer implementation.
//
// HOT PATH [4]: TradeStore is the in-memory storage layer for market data.
// AddTrades is called for every incoming message and must be fast.
//
// Ring Buffer Design:
// We use a circular buffer (ring buffer) instead of a growing slice because:
// 1. Fixed memory footprint - no unbounded growth
// 2. O(1) insertion - no slice copying when buffer is full
// 3. Zero allocations on eviction - just overwrites oldest entry
// 4. Cache-friendly - contiguous memory access pattern
//
// Concurrency Model:
// - Single writer (FIX message handler goroutine)
// - Multiple readers (status display, database persistence)
// - sync.RWMutex for read-write locking
//
// Performance Characteristics:
// - AddTrades: O(n) where n = trades to add, ~70ns per trade
// - GetRecentTrades: O(m) where m = trades in buffer, 1 allocation
// - GetAllTrades: O(m), 1 allocation for copy
package fixclient

import (
	"log"
	"sync"
	"time"
)

// Trade represents a single market data entry from a FIX message.
// Fields are ordered for optimal memory alignment:
// - time.Time (24 bytes) first
// - strings (16 bytes each) next
// - bools (1 byte each) last to minimize padding
type Trade struct {
	Timestamp  time.Time `json:"timestamp"`
	Symbol     string    `json:"symbol"`
	Price      string    `json:"price"`
	Size       string    `json:"size"`
	Time       string    `json:"time"`
	Aggressor  string    `json:"aggressor"`
	MdReqId    string    `json:"mdReqId"`
	EntryType  string    `json:"entryType"` // MdEntryType (0=Bid, 1=Offer, 2=Trade, 4=Open, 5=Close, 7=High, 8=Low, B=Volume)
	Position   string    `json:"position"`  // Position in book (for bids/offers)
	SeqNum     string    `json:"seqNum"`    // FIX MsgSeqNum for ordering
	IsSnapshot bool      `json:"isSnapshot"`
	IsUpdate   bool      `json:"isUpdate"`
}

// TradeStore provides thread-safe in-memory storage for market data trades.
// HOT PATH [4]: This is the primary storage layer hit by every market data message.
//
// Ring Buffer Layout:
//
//	┌───────────────────────────────────────────────────────────────┐
//	│  trades[0]  │  trades[1]  │  ...  │  trades[maxSize-1]       │
//	└───────────────────────────────────────────────────────────────┘
//	       ↑                              ↑
//	      head                    (head + count - 1) % maxSize = tail
//	   (oldest)                        (newest)
//
// When count < maxSize: buffer is filling, head stays at 0
// When count == maxSize: buffer is full, head advances on each insert (overwrites oldest)
type TradeStore struct {
	mu            sync.RWMutex             // Read-write lock for concurrent access
	trades        []Trade                  // Ring buffer - pre-allocated to maxSize
	head          int                      // Index of oldest element (ring buffer read position)
	count         int                      // Number of valid elements in buffer (0 to maxSize)
	subscriptions map[string]*Subscription // reqId -> subscription metadata
	updateCount   int64                    // Total trades ever added (for metrics)
	maxSize       int                      // Maximum buffer capacity
}

// Subscription tracks an active market data subscription.
// Fields are ordered for optimal memory alignment.
type Subscription struct {
	LastUpdate       time.Time // 24 bytes
	TotalUpdates     int64     // 8 bytes
	Symbol           string    // 16 bytes
	SubscriptionType string    // 16 bytes - "0"=snapshot, "1"=subscribe, "2"=unsubscribe
	MdReqId          string    // 16 bytes
	Active           bool      // 1 byte
	SnapshotReceived bool      // 1 byte
}

// NewTradeStore creates a new TradeStore with pre-allocated ring buffer.
// The buffer is allocated once at creation and never grows or shrinks.
//
// Example:
//
//	store := NewTradeStore(10000, "") // 10K trade capacity
//	store.AddTrades("BTC-USD", trades, false, "req-123")
func NewTradeStore(maxSize int, persistenceFile string) *TradeStore {
	return &TradeStore{
		trades:        make([]Trade, maxSize), // HOT PATH: Pre-allocate to avoid runtime growth
		head:          0,
		count:         0,
		subscriptions: make(map[string]*Subscription),
		maxSize:       maxSize,
	}
}

// AddTrades inserts trades into the ring buffer.
// HOT PATH [4]: Called for every incoming market data message.
//
// Ring buffer insertion is O(1) per trade with zero allocations:
//  1. Calculate write position: (head + count) % maxSize
//  2. Write trade to that position (overwrites if buffer full)
//  3. If buffer was full, advance head to discard oldest
//
// Performance: ~70ns per trade (dominated by mutex acquisition)
// Allocations: 0 (ring buffer pre-allocated, struct copied by value)
//
// Concurrency: Holds write lock for duration of insertion.
// Consider batching for high-throughput scenarios.
func (ts *TradeStore) AddTrades(symbol string, trades []Trade, isSnapshot bool, mdReqId string) {
	// HOT PATH: Acquire write lock - this is the main contention point
	ts.mu.Lock()
	defer ts.mu.Unlock()

	// Update subscription metadata if exists
	if sub, exists := ts.subscriptions[mdReqId]; exists {
		sub.LastUpdate = time.Now()
		sub.TotalUpdates += int64(len(trades))
		if isSnapshot {
			sub.SnapshotReceived = true
		}
	}

	// HOT PATH: Single time.Now() call for all trades in batch
	// Avoids syscall overhead of calling time.Now() per trade
	now := time.Now()
	for _, trade := range trades {
		// HOT PATH: Struct field assignment - all stack operations
		trade.Timestamp = now
		trade.Symbol = symbol
		trade.MdReqId = mdReqId
		trade.IsSnapshot = isSnapshot
		trade.IsUpdate = !isSnapshot

		// HOT PATH: Ring buffer insertion - O(1), zero allocations
		// writeIdx cycles through 0, 1, 2, ..., maxSize-1, 0, 1, ...
		writeIdx := (ts.head + ts.count) % ts.maxSize
		ts.trades[writeIdx] = trade // Direct array assignment, no slice append

		if ts.count < ts.maxSize {
			// Buffer not yet full - just increment count
			ts.count++
		} else {
			// Buffer full - advance head to overwrite oldest entry
			// This is where we "evict" the oldest trade
			ts.head = (ts.head + 1) % ts.maxSize
		}
		ts.updateCount++
	}
}

// GetRecentTrades retrieves the most recent trades for a symbol.
// Returns trades in chronological order (oldest first, newest last).
//
// Algorithm (two-pass to avoid O(n²) prepend):
//  1. First pass: count matching trades from newest to oldest
//  2. Pre-allocate result slice with exact capacity
//  3. Second pass: fill slice from end to start (places in chronological order)
//
// Performance: O(m) where m = trades in buffer, worst case scans entire buffer
// Allocations: 1 (result slice with exact capacity)
//
// Previous implementation used prepend: append([]Trade{t}, result...)
// This caused O(n²) allocations - 999 allocs for 500 trades!
// Current implementation: single allocation regardless of result size.
//
// Example:
//
//	trades := store.GetRecentTrades("BTC-USD", 100) // Last 100 BTC trades
func (ts *TradeStore) GetRecentTrades(symbol string, limit int) []Trade {
	// Read lock allows concurrent readers
	ts.mu.RLock()
	defer ts.mu.RUnlock()

	if ts.count == 0 {
		return nil
	}

	// First pass: count matching trades (iterate from newest to oldest)
	// We iterate backwards from tail to find the N most recent matches
	matchCount := 0
	for i := 0; i < ts.count && matchCount < limit; i++ {
		// Ring buffer index calculation: newest is at (head + count - 1) % maxSize
		// Going backwards: subtract i from that position
		idx := (ts.head + ts.count - 1 - i) % ts.maxSize
		if ts.trades[idx].Symbol == symbol {
			matchCount++
		}
	}

	if matchCount == 0 {
		return nil
	}

	// Pre-allocate result slice with exact capacity - single allocation
	recent := make([]Trade, matchCount)

	// Second pass: fill from newest to oldest, but place in chronological order
	// resultIdx starts at end and decrements, so oldest match goes to index 0
	resultIdx := matchCount - 1
	for i := 0; i < ts.count && resultIdx >= 0; i++ {
		idx := (ts.head + ts.count - 1 - i) % ts.maxSize
		if ts.trades[idx].Symbol == symbol {
			recent[resultIdx] = ts.trades[idx]
			resultIdx--
		}
	}

	return recent
}

// GetAllTrades returns a copy of all trades in the buffer.
// Trades are returned in chronological order (oldest first).
//
// Performance: O(m) where m = trades in buffer
// Allocations: 1 (result slice)
//
// Note: Returns a defensive copy to prevent callers from modifying internal state.
func (ts *TradeStore) GetAllTrades() []Trade {
	ts.mu.RLock()
	defer ts.mu.RUnlock()

	if ts.count == 0 {
		return nil
	}

	// Single allocation for result
	result := make([]Trade, ts.count)
	// Copy trades in chronological order (oldest to newest)
	// Ring buffer: oldest is at head, iterate through count elements
	for i := 0; i < ts.count; i++ {
		idx := (ts.head + i) % ts.maxSize
		result[i] = ts.trades[idx]
	}
	return result
}

func (ts *TradeStore) AddSubscription(symbol, subscriptionType, mdReqId string) {
	ts.mu.Lock()
	defer ts.mu.Unlock()

	ts.subscriptions[mdReqId] = &Subscription{
		Symbol:           symbol,
		SubscriptionType: subscriptionType,
		MdReqId:          mdReqId,
		Active:           true,
		LastUpdate:       time.Now(),
		TotalUpdates:     0,
		SnapshotReceived: false,
	}

	log.Printf("Added subscription: %s (type=%s, reqId=%s)", symbol, getSubscriptionTypeDesc(subscriptionType), mdReqId)
}

func (ts *TradeStore) RemoveSubscription(symbol string) {
	ts.mu.Lock()
	defer ts.mu.Unlock()

	// Find all subscriptions for this symbol and remove them
	for reqId, sub := range ts.subscriptions {
		if sub.Symbol == symbol {
			delete(ts.subscriptions, reqId)
			log.Printf("Removed subscription: %s (reqId: %s, total updates: %d)", symbol, reqId, sub.TotalUpdates)
		}
	}
}

func (ts *TradeStore) RemoveSubscriptionByReqId(reqId string) {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	if sub, exists := ts.subscriptions[reqId]; exists {
		delete(ts.subscriptions, reqId)
		log.Printf("Removed subscription: %s (ReqId: %s)", sub.Symbol, reqId)
	}
}

func (ts *TradeStore) GetSubscriptionStatus() map[string]*Subscription {
	ts.mu.RLock()
	defer ts.mu.RUnlock()

	result := make(map[string]*Subscription)
	for reqId, v := range ts.subscriptions {
		// Create copy to avoid race conditions
		sub := *v
		result[reqId] = &sub
	}
	return result
}

func (ts *TradeStore) GetSubscriptionsBySymbol() map[string][]*Subscription {
	ts.mu.RLock()
	defer ts.mu.RUnlock()

	result := make(map[string][]*Subscription)
	for _, sub := range ts.subscriptions {
		// Create copy to avoid race conditions
		subCopy := *sub
		result[sub.Symbol] = append(result[sub.Symbol], &subCopy)
	}
	return result
}

func getSubscriptionTypeDesc(subType string) string {
	switch subType {
	case "0":
		return "Snapshot Only"
	case "1":
		return "Snapshot + Updates"
	case "2":
		return "Unsubscribe"
	default:
		return "Unknown"
	}
}

// DisplayRealtimeUpdate shows a single line update for streaming mode
func (ts *TradeStore) DisplayRealtimeUpdate(trade Trade) {
	entryType := trade.EntryType
	if entryType == "" {
		entryType = "2" // Default to Trade
	}

	switch entryType {
	case "0": // Bid
		log.Printf("%s Bid: %s | Size: %s | Pos: %s",
			trade.Symbol, trade.Price, trade.Size, trade.Position)
	case "1": // Offer
		log.Printf("%s Offer: %s | Size: %s | Pos: %s",
			trade.Symbol, trade.Price, trade.Size, trade.Position)
	case "2": // Trade
		aggressor := trade.Aggressor
		if aggressor == "" {
			aggressor = "-"
		}
		log.Printf("%s Trade: %s | Size: %s | Aggressor: %s",
			trade.Symbol, trade.Price, trade.Size, aggressor)
	case "4": // Open
		log.Printf("%s Open: %s", trade.Symbol, trade.Price)
	case "5": // Close
		log.Printf("%s Close: %s", trade.Symbol, trade.Price)
	case "7": // High
		log.Printf("%s High: %s", trade.Symbol, trade.Price)
	case "8": // Low
		log.Printf("%s Low: %s", trade.Symbol, trade.Price)
	case "B": // Volume
		log.Printf("%s Volume: %s", trade.Symbol, trade.Size)
	default: // Unknown
		log.Printf("%s [%s]: %s | Size: %s",
			trade.Symbol, entryType, trade.Price, trade.Size)
	}
}
