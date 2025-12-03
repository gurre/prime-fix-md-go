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

// Package fixclient provides FIX protocol message parsing for market data.
//
// HOT PATH [3]: This file contains the core parsing logic for FIX messages.
// All functions here are in the critical performance path.
//
// Parsing Strategy:
// We use raw string parsing instead of quickfix's structured field access because:
// 1. quickfix.Message.GetGroup() has significant overhead for repeating groups
// 2. Direct string search is faster for our specific field extraction pattern
// 3. We know the exact tags we need (269, 270, 271, 273, 290, 2446)
//
// Performance Characteristics:
// - findEntryBoundaries: O(m) where m = message length, 1 allocation
// - parseTradeFromSegment: O(s) where s = segment length, 0 allocations
// - Total parsing: O(n*s) where n = entries, s = avg segment length
package fixclient

import (
	"strconv"
	"strings"
	"time"

	"prime-fix-md-go/constants"
	"prime-fix-md-go/utils"

	"github.com/quickfixgo/quickfix"
)

// extractTrades is the entry point for parsing trades from a FIX message.
// HOT PATH [3]: Delegates to extractTradesImproved for the actual parsing.
func (a *FixApp) extractTrades(msg *quickfix.Message, symbol, mdReqId string, isSnapshot bool, seqNum string) []Trade {
	return a.extractTradesImproved(msg, symbol, mdReqId, isSnapshot, seqNum)
}

// extractTradesImproved parses all MD entries from a FIX market data message.
// HOT PATH [3]: Main parsing logic - converts raw FIX to Trade structs.
//
// Algorithm:
//  1. Convert message to raw string (msg.String() - single allocation)
//  2. Find all entry boundaries (positions of "269=" tags)
//  3. Extract each segment and parse fields using single-pass parser
//
// Performance: O(n*m) where n=entries, m=avg segment length
// Allocations: 2 (boundary slice + trades slice, both pre-sized)
func (a *FixApp) extractTradesImproved(msg *quickfix.Message, symbol, mdReqId string, isSnapshot bool, seqNum string) []Trade {
	// HOT PATH: msg.String() creates a single string from FIX fields
	rawMsg := msg.String()

	// Early exit if no entries - avoids unnecessary parsing
	noMdEntriesStr := utils.GetString(msg, constants.TagNoMdEntries)
	if noMdEntriesStr == "" || noMdEntriesStr == "0" {
		return nil
	}

	// HOT PATH [3a]: Find all "269=" positions in one pass
	entryStarts := a.findEntryBoundaries(rawMsg)
	if len(entryStarts) == 0 {
		return nil
	}

	// HOT PATH: Pre-allocate trades slice with exact capacity
	// Eliminates slice growth allocations during append
	trades := make([]Trade, 0, len(entryStarts))

	// HOT PATH: Single time.Now() call for entire batch
	// Avoids syscall overhead of calling per-entry
	now := time.Now()

	msgLen := len(rawMsg)
	for i, startPos := range entryStarts {
		endPos := a.getEntryEndPos(entryStarts, i, msgLen)
		// HOT PATH: Substring is O(1) - no allocation, just new slice header
		entrySegment := rawMsg[startPos:endPos]

		// HOT PATH [3b]: Parse individual entry using single-pass parser
		trade := a.parseTradeFromSegmentFast(entrySegment, symbol, mdReqId, isSnapshot, seqNum, i, now)
		trades = append(trades, trade)
	}

	return trades
}

// findEntryBoundaries locates all MD entry start positions in a raw FIX message.
// HOT PATH [3a]: Called once per message to find all "269=" (MdEntryType) tags.
//
// The "269=" tag marks the start of each repeating group entry in FIX market data.
// We find all positions to define segment boundaries for individual parsing.
//
// Performance: O(m) where m = message length (two passes: Count + Index loop)
// Allocations: 1 (pre-sized slice based on strings.Count)
//
// Trade-off: strings.Count adds ~50% time but eliminates slice growth allocations.
// For 100 entries, this reduces allocations from 8 to 1.
func (a *FixApp) findEntryBoundaries(rawMsg string) []int {
	// HOT PATH: strings.Count is assembly-optimized for pattern counting
	// This pre-scan enables exact capacity allocation below
	count := strings.Count(rawMsg, "269=")
	if count == 0 {
		return nil
	}

	// HOT PATH: Single allocation with exact capacity - no slice growth
	entryStarts := make([]int, 0, count)
	searchFrom := 0
	for {
		// HOT PATH: strings.Index uses assembly-optimized search
		// Substring creates new slice header but shares backing array (no alloc)
		pos := strings.Index(rawMsg[searchFrom:], "269=")
		if pos == -1 {
			break
		}
		entryStarts = append(entryStarts, searchFrom+pos)
		searchFrom += pos + 4 // Skip past "269=" to find next occurrence
	}
	return entryStarts
}

// getEntryEndPos returns the end position for an entry segment.
// HOT PATH: Simple index lookup, O(1), no allocations.
func (a *FixApp) getEntryEndPos(entryStarts []int, currentIndex, msgLen int) int {
	if currentIndex < len(entryStarts)-1 {
		return entryStarts[currentIndex+1]
	}
	return msgLen
}

// parseTradeFromSegmentFast extracts trade fields using single-pass parsing.
// HOT PATH [3b]: Called once per entry - optimized inner loop.
//
// This is the optimized version that parses all fields in a single pass through
// the segment, instead of calling extractSingleFieldValue 6 times.
//
// Performance: ~50-80ns per entry (3-4x faster than multi-pass)
// Allocations: 0 (returns struct by value, strings are substrings)
func (a *FixApp) parseTradeFromSegmentFast(segment, symbol, mdReqId string, isSnapshot bool, seqNum string, entryIndex int, timestamp time.Time) Trade {
	trade := Trade{
		Timestamp:  timestamp,
		Symbol:     symbol,
		MdReqId:    mdReqId,
		IsSnapshot: isSnapshot,
		IsUpdate:   !isSnapshot,
		SeqNum:     seqNum,
	}

	// Single-pass parsing: iterate through segment once, extract all fields
	// FIX format: TAG=VALUE\x01TAG=VALUE\x01...
	pos := 0
	segLen := len(segment)

	for pos < segLen {
		// Find the '=' separator for tag
		eqPos := strings.IndexByte(segment[pos:], '=')
		if eqPos == -1 {
			break
		}
		eqPos += pos

		// Extract tag number
		tag := segment[pos:eqPos]

		// Find the SOH delimiter for value end
		valueStart := eqPos + 1
		sohPos := strings.IndexByte(segment[valueStart:], '\x01')
		var value string
		var nextPos int
		if sohPos == -1 {
			// Last field in segment
			value = segment[valueStart:]
			nextPos = segLen
		} else {
			value = segment[valueStart : valueStart+sohPos]
			nextPos = valueStart + sohPos + 1
		}

		// Match tag and assign value - ordered by frequency
		switch tag {
		case "269": // MdEntryType - always present
			trade.EntryType = value
		case "270": // MdEntryPx - usually present
			trade.Price = value
		case "271": // MdEntrySize - usually present
			trade.Size = value
		case "273": // MdEntryTime - usually present
			trade.Time = value
		case "290": // MdEntryPositionNo - optional
			trade.Position = value
		case "2446": // AggressorSide - optional, only for trades
			trade.Aggressor = getAggressorSideDesc(value)
		}
		// Skip unknown tags silently

		pos = nextPos
	}

	// Set default position for bids/offers if not provided
	if trade.Position == "" && (trade.EntryType == "0" || trade.EntryType == "1") {
		trade.Position = strconv.Itoa(entryIndex + 1)
	}

	return trade
}

// parseTradeFromSegment extracts trade fields from a single FIX entry segment.
// DEPRECATED: Use parseTradeFromSegmentFast for better performance.
// Kept for reference and comparison benchmarks.
//
// Performance: ~150-200ns per entry (calls extractSingleFieldValue 6 times)
// Allocations: 0 (returns struct by value, strings are substrings)
func (a *FixApp) parseTradeFromSegment(segment, symbol, mdReqId string, isSnapshot bool, seqNum string, entryIndex int) Trade {
	trade := Trade{
		Timestamp:  time.Now(),
		Symbol:     symbol,
		MdReqId:    mdReqId,
		IsSnapshot: isSnapshot,
		IsUpdate:   !isSnapshot,
		SeqNum:     seqNum,
	}

	if entryType := extractSingleFieldValue(segment, "269="); entryType != "" {
		trade.EntryType = entryType
	}
	if price := extractSingleFieldValue(segment, "270="); price != "" {
		trade.Price = price
	}
	if size := extractSingleFieldValue(segment, "271="); size != "" {
		trade.Size = size
	}
	if timeVal := extractSingleFieldValue(segment, "273="); timeVal != "" {
		trade.Time = timeVal
	}

	if position := extractSingleFieldValue(segment, "290="); position != "" {
		trade.Position = position
	} else {
		if trade.EntryType == "0" || trade.EntryType == "1" {
			trade.Position = strconv.Itoa(entryIndex + 1)
		}
	}

	if aggressor := extractSingleFieldValue(segment, "2446="); aggressor != "" {
		trade.Aggressor = getAggressorSideDesc(aggressor)
	}

	return trade
}

// extractSingleFieldValue extracts a field value from a FIX segment by tag prefix.
// HOT PATH: Called 6 times per entry - the most frequently called function.
//
// FIX format: "TAG=VALUE\x01" where \x01 (SOH) is the field delimiter.
// Example: "270=50000.00\x01" extracts "50000.00" for prefix "270="
//
// Performance: O(n) where n = segment length (single strings.Index call)
// Allocations: 0 (returns substring sharing backing array)
//
// Note: Substring in Go is O(1) and shares the backing array with the original
// string, so no memory is copied. The returned string is valid as long as
// the original rawMsg is not garbage collected.
func extractSingleFieldValue(fixSegment, tagPrefix string) string {
	// HOT PATH: strings.Index uses assembly-optimized Boyer-Moore variant
	start := strings.Index(fixSegment, tagPrefix)
	if start == -1 {
		return ""
	}

	start += len(tagPrefix)
	// HOT PATH: Search for SOH delimiter from value start position
	end := strings.Index(fixSegment[start:], "\x01") // FIX field delimiter (SOH)
	if end == -1 {
		return fixSegment[start:] // Last field in segment
	}

	// HOT PATH: Substring operation - O(1), no allocation
	return fixSegment[start : start+end]
}
