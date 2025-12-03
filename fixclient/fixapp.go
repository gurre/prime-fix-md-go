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

/*
HOT PATH - Market Data Message Processing Flow

This documents the critical performance path for processing incoming FIX market data.
Each message triggers this sequence; optimizations here have the highest impact.

┌─────────────────────────────────────────────────────────────────────────────┐
│                           NETWORK LAYER                                      │
│                    (quickfix library handles TCP/FIX protocol)               │
└─────────────────────────────────────────────────────────────────────────────┘
                                     │
                                     ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│ [1] FromApp() - fixapp.go:118                                    ENTRY POINT │
│     • Called by quickfix for every application-level message                 │
│     • Type check on MsgType header field (string comparison)                 │
│     • Routes to handleMarketDataMessage() for W/X message types              │
│     • Cost: ~50ns (header extraction + string compare)                       │
└─────────────────────────────────────────────────────────────────────────────┘
                                     │
                                     ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│ [2] handleMarketDataMessage() - fixapp.go:170                    COORDINATOR │
│     • Extracts message metadata (symbol, reqId, seqNum)                      │
│     • Calls extractTrades() for parsing                                      │
│     • Calls TradeStore.AddTrades() for storage                               │
│     • Calls storeTradesToDatabase() for persistence (optional)               │
│     • Cost: ~200ns (field extractions) + downstream costs                    │
└─────────────────────────────────────────────────────────────────────────────┘
                                     │
                                     ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│ [3] extractTrades() → extractTradesImproved() - parser.go:30-54     PARSER  │
│     • Converts quickfix.Message to raw string (msg.String())                 │
│     • Calls findEntryBoundaries() to locate all 269= tags                    │
│     • Iterates entries, calls parseTradeFromSegment() for each               │
│     • Cost: O(n*m) where n=entries, m=avg segment length                     │
│     • Allocations: 1 slice for boundaries + 1 slice for trades               │
└─────────────────────────────────────────────────────────────────────────────┘
                                     │
                    ┌────────────────┴────────────────┐
                    ▼                                 ▼
┌──────────────────────────────────┐  ┌──────────────────────────────────────┐
│ [3a] findEntryBoundaries()       │  │ [3b] parseTradeFromSegment()         │
│      parser.go:56-73             │  │      parser.go:83-119                │
│ • strings.Count for pre-alloc    │  │ • Extracts 6 fields per entry        │
│ • strings.Index loop to find all │  │ • Each field: strings.Index O(m)     │
│   "269=" occurrences             │  │ • Zero allocations (returns struct)  │
│ • Cost: O(m) where m=msg length  │  │ • Cost: ~150-200ns per entry         │
│ • Allocations: 1 (pre-sized)     │  │ • Allocations: 0                     │
└──────────────────────────────────┘  └──────────────────────────────────────┘
                                     │
                                     ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│ [4] TradeStore.AddTrades() - tradestore.go:70-101                   STORAGE │
│     • Acquires write lock (sync.RWMutex)                                     │
│     • Updates subscription metadata                                          │
│     • Ring buffer insertion: O(1) per trade, zero allocations                │
│     • Cost: ~70ns per trade (dominated by mutex)                             │
│     • Allocations: 0 (ring buffer pre-allocated)                             │
└─────────────────────────────────────────────────────────────────────────────┘
                                     │
                                     ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│ [5] storeTradesToDatabase() - storage.go (OPTIONAL)              PERSISTENCE │
│     • SQLite transaction with batch inserts                                  │
│     • Cost: ~1-10ms depending on batch size and disk                         │
│     • Can be made async to not block hot path                                │
└─────────────────────────────────────────────────────────────────────────────┘

PERFORMANCE CHARACTERISTICS (Apple M4 Pro benchmarks):
┌────────────────────────────────┬───────────┬────────────┬─────────────────┐
│ Operation                      │ Time      │ Allocs     │ Memory          │
├────────────────────────────────┼───────────┼────────────┼─────────────────┤
│ Parse 10 entries               │ 3.3µs     │ 1          │ 80B             │
│ Parse 100 entries              │ 33µs      │ 1          │ 896B            │
│ Store 10 trades (ring buffer)  │ 700ns     │ 0          │ 0B              │
│ Retrieve 100 trades            │ 2.8µs     │ 1          │ 18KB            │
└────────────────────────────────┴───────────┴────────────┴─────────────────┘

OPTIMIZATION NOTES:
• Ring buffer eliminates allocation on eviction (was: slice copy per trade)
• Pre-allocated boundary slice eliminates grow allocations (was: 8 allocs)
• GetRecentTrades uses two-pass to avoid O(n²) prepend (was: 999 allocs)
• Struct fields ordered for memory alignment (time.Time first, bools last)
*/

package fixclient

import (
	"log"
	"time"

	"prime-fix-md-go/builder"
	"prime-fix-md-go/constants"
	"prime-fix-md-go/database"
	"prime-fix-md-go/utils"

	"github.com/quickfixgo/quickfix"
)

type Config struct {
	ApiKey       string
	ApiSecret    string
	Passphrase   string
	SenderCompId string
	TargetCompId string
	PortfolioId  string
}

type FixApp struct {
	Config *Config

	SessionId  quickfix.SessionID
	TradeStore *TradeStore
	Db         *database.MarketDataDb

	shouldExit    bool
	lastLogonTime time.Time
}

func NewConfig(apiKey, apiSecret, passphrase, senderCompId, targetCompId, portfolioId string) *Config {
	return &Config{
		ApiKey:       apiKey,
		ApiSecret:    apiSecret,
		Passphrase:   passphrase,
		SenderCompId: senderCompId,
		TargetCompId: targetCompId,
		PortfolioId:  portfolioId,
	}
}

func NewFixApp(config *Config, db *database.MarketDataDb) *FixApp {
	tradeStore := NewTradeStore(10000, "")

	return &FixApp{
		Config:     config,
		TradeStore: tradeStore,
		Db:         db,
		shouldExit: false,
	}
}

func (a *FixApp) OnCreate(sid quickfix.SessionID) {
	a.SessionId = sid
}

func (a *FixApp) OnLogout(sid quickfix.SessionID) {
	log.Println("Logout", sid)

	timeSinceLogon := time.Since(a.lastLogonTime)
	if timeSinceLogon < 5*time.Second || a.lastLogonTime.IsZero() {
		log.Printf("Authentication failed. Exiting to prevent reconnection loop.")
		a.shouldExit = true
	}
}

func (a *FixApp) FromAdmin(_ *quickfix.Message, _ quickfix.SessionID) quickfix.MessageRejectError {
	return nil
}

func (a *FixApp) ToApp(_ *quickfix.Message, _ quickfix.SessionID) error {
	return nil
}

func (a *FixApp) OnLogon(sid quickfix.SessionID) {
	a.SessionId = sid
	a.lastLogonTime = time.Now()
	log.Println("✓ FIX logon", sid)
	a.displayConnectionSuccess()
	a.displayHelp()
}

func (a *FixApp) ToAdmin(msg *quickfix.Message, _ quickfix.SessionID) {
	if t, _ := msg.Header.GetString(constants.TagMsgType); t == constants.MsgTypeLogon {
		ts := time.Now().UTC().Format(constants.FixTimeFormat)
		builder.BuildLogon(
			&msg.Body,
			ts,
			a.Config.ApiKey,
			a.Config.ApiSecret,
			a.Config.Passphrase,
			a.Config.TargetCompId,
			a.Config.PortfolioId,
		)
	}
}

// FromApp is the entry point for all application-level FIX messages.
// HOT PATH [1]: Called by quickfix for every incoming message.
// Performance: ~50ns for type check and routing.
func (a *FixApp) FromApp(msg *quickfix.Message, _ quickfix.SessionID) quickfix.MessageRejectError {
	// HOT PATH: Single string comparison to route market data messages
	if t, _ := msg.Header.GetString(constants.TagMsgType); t == constants.MsgTypeMarketDataSnapshot || t == constants.MsgTypeMarketDataIncremental {
		a.handleMarketDataMessage(msg) // HOT PATH continues
	} else if t == "Y" { // Market Data Request Reject
		a.handleMarketDataReject(msg)
	} else {
		log.Printf("Received application message type %s", t)
	}
	return nil
}

func (a *FixApp) handleMarketDataReject(msg *quickfix.Message) {
	mdReqId := utils.GetString(msg, constants.TagMdReqId)
	rejReason := utils.GetString(msg, constants.TagMdReqRejReason)
	text := utils.GetString(msg, constants.TagText)

	reasonDesc := getMdReqRejReasonDesc(rejReason)

	a.displayMarketDataReject(mdReqId, rejReason, reasonDesc, text)
	a.TradeStore.RemoveSubscriptionByReqId(mdReqId)
	a.displayMarketDataRejectHelp(rejReason)
}

func getMdReqRejReasonDesc(reason string) string {
	switch reason {
	case constants.MdReqRejReasonUnknownSymbol:
		return "Unknown symbol"
	case constants.MdReqRejReasonDuplicateMdReqId:
		return "Duplicate MdReqId"
	case constants.MdReqRejReasonInsufficientBandwidth:
		return "Insufficient bandwidth"
	case constants.MdReqRejReasonInsufficientPermission:
		return "Insufficient permission"
	case constants.MdReqRejReasonInvalidSubscriptionReqType:
		return "Invalid SubscriptionRequestType"
	case constants.MdReqRejReasonInvalidMarketDepth:
		return "Invalid MarketDepth"
	case constants.MdReqRejReasonUnsupportedMdUpdateType:
		return "Unsupported MdUpdateType"
	case constants.MdReqRejReasonOther:
		return "Other"
	case constants.MdReqRejReasonUnsupportedMdEntryType:
		return "Unsupported MdEntryType"
	default:
		return "Unknown reason"
	}
}

func (a *FixApp) ShouldExit() bool {
	return a.shouldExit
}

// handleMarketDataMessage processes market data snapshots and incremental updates.
// HOT PATH [2]: Coordinates parsing, storage, and display of market data.
// Performance: ~200ns for metadata extraction + downstream costs.
func (a *FixApp) handleMarketDataMessage(msg *quickfix.Message) {
	// HOT PATH: Extract message metadata - each GetString is a map lookup
	msgType, _ := msg.Header.GetString(constants.TagMsgType)
	mdReqId := utils.GetString(msg, constants.TagMdReqId)
	symbol := utils.GetString(msg, constants.TagSymbol)
	noMdEntries := utils.GetString(msg, constants.TagNoMdEntries)
	seqNum, _ := msg.Header.GetString(constants.TagMsgSeqNum)

	isSnapshot := msgType == constants.MsgTypeMarketDataSnapshot
	isIncremental := msgType == constants.MsgTypeMarketDataIncremental

	a.displayMarketDataReceived(msgType, symbol, mdReqId, noMdEntries, seqNum)

	// HOT PATH [3]: Parse raw FIX message into Trade structs
	// Cost: O(n*m) where n=entries, m=message length
	trades := a.extractTrades(msg, symbol, mdReqId, isSnapshot, seqNum)

	// HOT PATH [4]: Store in ring buffer - O(1) per trade, zero allocs
	a.TradeStore.AddTrades(symbol, trades, isSnapshot, mdReqId)

	// HOT PATH [5]: Optional persistence - can block if sync
	// Consider making async for high-throughput scenarios
	a.storeTradesToDatabase(trades, seqNum, isSnapshot)

	// Display is not part of hot path critical section
	if isSnapshot {
		a.displaySnapshotTrades(trades, symbol)
	} else if isIncremental {
		a.displayIncrementalTrades(trades)
	}
}
