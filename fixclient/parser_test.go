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
	"testing"
)

// Tests for FIX message parsing behavior.
// These tests verify that market data entries are correctly extracted from FIX messages,
// ensuring the parser handles real-world message formats and edge cases.

// TestExtractTrades_SingleTradeEntry verifies that a single trade entry is correctly
// parsed with all fields populated. This is the most common case for incremental updates.
func TestExtractTrades_SingleTradeEntry(t *testing.T) {
	t.Helper()
	app := &FixApp{TradeStore: NewTradeStore(100, "")}

	// FIX segment with trade entry (type=2) including aggressor side
	segment := "269=2\x01270=50000.00\x01271=1.5000\x01273=20250101-12:00:00\x012446=1\x01"

	trades := parseSegmentToTrades(t, app, segment, "BTC-USD", "req-123", false)

	if len(trades) != 1 {
		t.Fatalf("expected 1 trade, got %d", len(trades))
	}

	trade := trades[0]
	assertTradeFields(t, trade, expectedTrade{
		entryType: "2",
		price:     "50000.00",
		size:      "1.5000",
		time:      "20250101-12:00:00",
		aggressor: "Buy",
		symbol:    "BTC-USD",
	})
}

// TestExtractTrades_BidOfferEntries verifies that bid (type=0) and offer (type=1)
// entries are correctly parsed with position information for order book data.
func TestExtractTrades_BidOfferEntries(t *testing.T) {
	app := &FixApp{TradeStore: NewTradeStore(100, "")}

	tests := []struct {
		name      string
		segment   string
		wantType  string
		wantPrice string
		wantPos   string
	}{
		{
			name:      "bid entry with position",
			segment:   "269=0\x01270=49999.00\x01271=2.5000\x01273=20250101-12:00:00\x01290=1\x01",
			wantType:  "0",
			wantPrice: "49999.00",
			wantPos:   "1",
		},
		{
			name:      "offer entry with position",
			segment:   "269=1\x01270=50001.00\x01271=3.0000\x01273=20250101-12:00:00\x01290=5\x01",
			wantType:  "1",
			wantPrice: "50001.00",
			wantPos:   "5",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			trades := parseSegmentToTrades(t, app, tt.segment, "BTC-USD", "req-123", false)

			if len(trades) != 1 {
				t.Fatalf("expected 1 trade, got %d", len(trades))
			}

			trade := trades[0]
			if trade.EntryType != tt.wantType {
				t.Errorf("entry type: got %q, want %q", trade.EntryType, tt.wantType)
			}
			if trade.Price != tt.wantPrice {
				t.Errorf("price: got %q, want %q", trade.Price, tt.wantPrice)
			}
			if trade.Position != tt.wantPos {
				t.Errorf("position: got %q, want %q", trade.Position, tt.wantPos)
			}
		})
	}
}

// TestExtractTrades_OHLCVEntries verifies that OHLCV candle data entries
// (open, close, high, low, volume) are correctly identified by entry type.
func TestExtractTrades_OHLCVEntries(t *testing.T) {
	app := &FixApp{TradeStore: NewTradeStore(100, "")}

	tests := []struct {
		name     string
		segment  string
		wantType string
	}{
		{"open price", "269=4\x01270=49500.00\x01273=20250101-00:00:00\x01", "4"},
		{"close price", "269=5\x01270=50500.00\x01273=20250101-23:59:59\x01", "5"},
		{"high price", "269=7\x01270=51000.00\x01273=20250101-14:30:00\x01", "7"},
		{"low price", "269=8\x01270=48000.00\x01273=20250101-03:15:00\x01", "8"},
		{"volume", "269=B\x01271=12345.67\x01273=20250101-00:00:00\x01", "B"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			trades := parseSegmentToTrades(t, app, tt.segment, "ETH-USD", "req-456", true)

			if len(trades) != 1 {
				t.Fatalf("expected 1 trade, got %d", len(trades))
			}

			if trades[0].EntryType != tt.wantType {
				t.Errorf("entry type: got %q, want %q", trades[0].EntryType, tt.wantType)
			}
		})
	}
}

// TestExtractTrades_AggressorSideMapping verifies that aggressor side values
// are correctly mapped to human-readable labels (Buy/Sell).
func TestExtractTrades_AggressorSideMapping(t *testing.T) {
	app := &FixApp{TradeStore: NewTradeStore(100, "")}

	tests := []struct {
		name          string
		aggressorCode string
		wantLabel     string
	}{
		{"buy aggressor", "1", "Buy"},
		{"sell aggressor", "2", "Sell"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			segment := "269=2\x01270=50000.00\x01271=1.0\x012446=" + tt.aggressorCode + "\x01"
			trades := parseSegmentToTrades(t, app, segment, "BTC-USD", "req-123", false)

			if trades[0].Aggressor != tt.wantLabel {
				t.Errorf("aggressor: got %q, want %q", trades[0].Aggressor, tt.wantLabel)
			}
		})
	}
}

// TestExtractTrades_MultipleEntriesInMessage verifies that messages containing
// multiple MD entries are fully parsed with all entries returned.
func TestExtractTrades_MultipleEntriesInMessage(t *testing.T) {
	app := &FixApp{TradeStore: NewTradeStore(100, "")}

	// Message with 3 entries: bid, offer, trade
	rawMsg := buildFIXMessage(3, []string{
		"269=0\x01270=49999.00\x01271=1.0\x01290=1\x01",
		"269=1\x01270=50001.00\x01271=2.0\x01290=1\x01",
		"269=2\x01270=50000.00\x01271=0.5\x012446=1\x01",
	})

	boundaries := app.findEntryBoundaries(rawMsg)

	if len(boundaries) != 3 {
		t.Fatalf("expected 3 entry boundaries, got %d", len(boundaries))
	}
}

// TestExtractTrades_SnapshotVsUpdate verifies that snapshot and update flags
// are correctly propagated to parsed trades.
func TestExtractTrades_SnapshotVsUpdate(t *testing.T) {
	app := &FixApp{TradeStore: NewTradeStore(100, "")}
	segment := "269=2\x01270=50000.00\x01271=1.0\x01"

	t.Run("snapshot trade", func(t *testing.T) {
		trades := parseSegmentToTrades(t, app, segment, "BTC-USD", "req-123", true)
		if !trades[0].IsSnapshot {
			t.Error("expected IsSnapshot=true for snapshot trade")
		}
		if trades[0].IsUpdate {
			t.Error("expected IsUpdate=false for snapshot trade")
		}
	})

	t.Run("update trade", func(t *testing.T) {
		trades := parseSegmentToTrades(t, app, segment, "BTC-USD", "req-123", false)
		if trades[0].IsSnapshot {
			t.Error("expected IsSnapshot=false for update trade")
		}
		if !trades[0].IsUpdate {
			t.Error("expected IsUpdate=true for update trade")
		}
	})
}

// TestExtractTrades_MissingOptionalFields verifies that parsing succeeds when
// optional fields (aggressor, position) are absent from the message.
func TestExtractTrades_MissingOptionalFields(t *testing.T) {
	app := &FixApp{TradeStore: NewTradeStore(100, "")}

	// Trade without aggressor side (tag 2446)
	segment := "269=2\x01270=50000.00\x01271=1.0\x01273=20250101-12:00:00\x01"
	trades := parseSegmentToTrades(t, app, segment, "BTC-USD", "req-123", false)

	if trades[0].Aggressor != "" {
		t.Errorf("expected empty aggressor when not present, got %q", trades[0].Aggressor)
	}
}

// TestExtractTrades_BidOfferDefaultPosition verifies that bid/offer entries
// without explicit position get a default position based on entry index.
func TestExtractTrades_BidOfferDefaultPosition(t *testing.T) {
	app := &FixApp{TradeStore: NewTradeStore(100, "")}

	// Bid without position tag (290)
	segment := "269=0\x01270=49999.00\x01271=1.0\x01"
	trades := parseSegmentToTrades(t, app, segment, "BTC-USD", "req-123", false)

	// First entry should get position "1" as default
	if trades[0].Position != "1" {
		t.Errorf("expected default position '1', got %q", trades[0].Position)
	}
}

// TestExtractTrades_EmptyMessage verifies that empty or invalid messages
// return an empty result without errors.
func TestExtractTrades_EmptyMessage(t *testing.T) {
	app := &FixApp{TradeStore: NewTradeStore(100, "")}

	tests := []struct {
		name    string
		segment string
	}{
		{"empty string", ""},
		{"no entry type tag", "270=50000.00\x01271=1.0\x01"},
		{"whitespace only", "   \t\n"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			boundaries := app.findEntryBoundaries(tt.segment)
			if len(boundaries) != 0 {
				t.Errorf("expected 0 boundaries for %q, got %d", tt.name, len(boundaries))
			}
		})
	}
}

// TestExtractTrades_SymbolPropagation verifies that the symbol parameter
// is correctly assigned to all parsed trades.
func TestExtractTrades_SymbolPropagation(t *testing.T) {
	app := &FixApp{TradeStore: NewTradeStore(100, "")}
	segment := "269=2\x01270=50000.00\x01271=1.0\x01"

	symbols := []string{"BTC-USD", "ETH-USD", "SOL-USD"}
	for _, sym := range symbols {
		trades := parseSegmentToTrades(t, app, segment, sym, "req-123", false)
		if trades[0].Symbol != sym {
			t.Errorf("expected symbol %q, got %q", sym, trades[0].Symbol)
		}
	}
}

// --- Test Helpers ---

type expectedTrade struct {
	entryType string
	price     string
	size      string
	time      string
	aggressor string
	symbol    string
	position  string
}

// parseSegmentToTrades is a test helper that parses a single FIX segment.
func parseSegmentToTrades(t *testing.T, app *FixApp, segment, symbol, mdReqId string, isSnapshot bool) []Trade {
	t.Helper()
	boundaries := app.findEntryBoundaries(segment)
	if len(boundaries) == 0 {
		return nil
	}

	trades := make([]Trade, 0, len(boundaries))
	for i, start := range boundaries {
		end := app.getEntryEndPos(boundaries, i, len(segment))
		trade := app.parseTradeFromSegmentFast(segment[start:end], symbol, mdReqId, isSnapshot, "1", i, app.TradeStore.trades[0].Timestamp)
		trades = append(trades, trade)
	}
	return trades
}

// assertTradeFields verifies trade fields match expected values.
func assertTradeFields(t *testing.T, got Trade, want expectedTrade) {
	t.Helper()
	if want.entryType != "" && got.EntryType != want.entryType {
		t.Errorf("EntryType: got %q, want %q", got.EntryType, want.entryType)
	}
	if want.price != "" && got.Price != want.price {
		t.Errorf("Price: got %q, want %q", got.Price, want.price)
	}
	if want.size != "" && got.Size != want.size {
		t.Errorf("Size: got %q, want %q", got.Size, want.size)
	}
	if want.time != "" && got.Time != want.time {
		t.Errorf("Time: got %q, want %q", got.Time, want.time)
	}
	if want.aggressor != "" && got.Aggressor != want.aggressor {
		t.Errorf("Aggressor: got %q, want %q", got.Aggressor, want.aggressor)
	}
	if want.symbol != "" && got.Symbol != want.symbol {
		t.Errorf("Symbol: got %q, want %q", got.Symbol, want.symbol)
	}
	if want.position != "" && got.Position != want.position {
		t.Errorf("Position: got %q, want %q", got.Position, want.position)
	}
}

// buildFIXMessage constructs a minimal FIX message with the given entries.
func buildFIXMessage(numEntries int, entries []string) string {
	header := "8=FIX.4.4\x019=100\x0135=W\x0149=COINBASE\x0156=CLIENT\x0155=BTC-USD\x01"
	msg := header
	for _, entry := range entries {
		msg += entry
	}
	msg += "10=000\x01"
	return msg
}
