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

// Package fixclient provides order management and tracking for FIX order entry.
//
// OrderStore maintains the state of all orders submitted through the FIX session,
// tracking their lifecycle from submission through fill or cancellation.
package fixclient

import (
	"sync"
	"time"
)

// Order represents an order's current state as tracked by the client.
// Fields are ordered for optimal memory alignment.
type Order struct {
	// Time fields (24 bytes each)
	CreatedAt      time.Time `json:"createdAt"`
	UpdatedAt      time.Time `json:"updatedAt"`
	ValidUntilTime time.Time `json:"validUntilTime,omitempty"`

	// String fields (16 bytes each)
	ClOrdID        string `json:"clOrdId"`        // Client order ID
	OrderID        string `json:"orderId"`        // Exchange-assigned order ID
	Symbol         string `json:"symbol"`         // Product pair (e.g., BTC-USD)
	Side           string `json:"side"`           // "1" buy, "2" sell
	OrdType        string `json:"ordType"`        // Order type
	TargetStrategy string `json:"targetStrategy"` // L, M, T, V, SL, R
	TimeInForce    string `json:"timeInForce"`    // 1, 3, 4, 6
	OrdStatus      string `json:"ordStatus"`      // Current status
	ExecType       string `json:"execType"`       // Last execution type

	// Price/Quantity fields
	OrderQty     string `json:"orderQty"`     // Original order quantity
	CashOrderQty string `json:"cashOrderQty"` // If in quote units
	Price        string `json:"price"`        // Limit price
	StopPx       string `json:"stopPx"`       // Stop price
	AvgPx        string `json:"avgPx"`        // Average fill price
	CumQty       string `json:"cumQty"`       // Cumulative filled quantity
	LeavesQty    string `json:"leavesQty"`    // Remaining quantity

	// Fill information
	LastPx     string `json:"lastPx"`     // Last fill price
	LastShares string `json:"lastShares"` // Last fill quantity
	ExecID     string `json:"execId"`     // Last execution ID

	// Fee information
	Commission string `json:"commission"` // Total commission
	FilledAmt  string `json:"filledAmt"`  // Cumulative quote currency impact
	NetAvgPx   string `json:"netAvgPx"`   // Net average price with fees

	// Rejection/Error info
	OrdRejReason string `json:"ordRejReason,omitempty"` // If rejected
	Text         string `json:"text,omitempty"`         // Error text

	// Account info
	Account string `json:"account"` // Portfolio ID
}

// Quote represents a received quote from the RFQ process.
type Quote struct {
	// Time fields
	ReceivedAt     time.Time `json:"receivedAt"`
	ValidUntilTime time.Time `json:"validUntilTime"`

	// Identifiers
	QuoteID    string `json:"quoteId"`
	QuoteReqID string `json:"quoteReqId"`
	Account    string `json:"account"`
	Symbol     string `json:"symbol"`

	// Quote values (only one set populated based on side)
	BidPx     string `json:"bidPx,omitempty"`     // For sells
	BidSize   string `json:"bidSize,omitempty"`   // For sells
	OfferPx   string `json:"offerPx,omitempty"`   // For buys
	OfferSize string `json:"offerSize,omitempty"` // For buys
}

// ExecutionReport represents a parsed Execution Report (8) message.
type ExecutionReport struct {
	// Identifiers
	ClOrdID string `json:"clOrdId"`
	OrderID string `json:"orderId"`
	ExecID  string `json:"execId"`
	Account string `json:"account"`
	Symbol  string `json:"symbol"`

	// Status
	OrdStatus string `json:"ordStatus"`
	ExecType  string `json:"execType"`
	Side      string `json:"side"`
	OrdType   string `json:"ordType"`

	// Quantities
	OrderQty     string `json:"orderQty"`
	CumQty       string `json:"cumQty"`
	LeavesQty    string `json:"leavesQty"`
	CashOrderQty string `json:"cashOrderQty,omitempty"`

	// Prices
	Price      string `json:"price,omitempty"`
	AvgPx      string `json:"avgPx,omitempty"`
	LastPx     string `json:"lastPx,omitempty"`
	LastShares string `json:"lastShares,omitempty"`

	// Fees
	Commission string `json:"commission,omitempty"`
	FilledAmt  string `json:"filledAmt,omitempty"`
	NetAvgPx   string `json:"netAvgPx,omitempty"`

	// Error info
	OrdRejReason string `json:"ordRejReason,omitempty"`
	Text         string `json:"text,omitempty"`

	// Timing
	EffectiveTime string `json:"effectiveTime,omitempty"`
}

// OrderCancelReject represents a parsed Order Cancel Reject (9) message.
type OrderCancelReject struct {
	ClOrdID          string `json:"clOrdId"`
	OrigClOrdID      string `json:"origClOrdId"`
	OrderID          string `json:"orderId"`
	OrdStatus        string `json:"ordStatus"`
	CxlRejReason     string `json:"cxlRejReason,omitempty"`
	CxlRejResponseTo string `json:"cxlRejResponseTo"` // "1" cancel, "2" replace
	Text             string `json:"text,omitempty"`
}

// SessionReject represents a parsed Reject (3) message.
type SessionReject struct {
	RefSeqNum           string `json:"refSeqNum"`
	RefMsgType          string `json:"refMsgType"`
	RefTagID            string `json:"refTagId,omitempty"`
	SessionRejectReason string `json:"sessionRejectReason,omitempty"`
	Text                string `json:"text,omitempty"`
}

// BusinessReject represents a parsed Business Message Reject (j) message.
type BusinessReject struct {
	RefSeqNum            string `json:"refSeqNum"`
	RefMsgType           string `json:"refMsgType"`
	BusinessRejectReason string `json:"businessRejectReason"`
	Text                 string `json:"text,omitempty"`
}

// QuoteAck represents a parsed Quote Acknowledgement (b) message (rejection).
type QuoteAck struct {
	QuoteID           string `json:"quoteId,omitempty"`
	QuoteReqID        string `json:"quoteReqId"`
	Account           string `json:"account"`
	Symbol            string `json:"symbol"`
	QuoteAckStatus    string `json:"quoteAckStatus"`
	QuoteRejectReason string `json:"quoteRejectReason"`
	Text              string `json:"text,omitempty"`
}

// OrderStore provides thread-safe storage for orders and quotes.
type OrderStore struct {
	mu     sync.RWMutex
	orders map[string]*Order // ClOrdID -> Order
	quotes map[string]*Quote // QuoteReqID -> Quote
}

// NewOrderStore creates a new OrderStore.
func NewOrderStore() *OrderStore {
	return &OrderStore{
		orders: make(map[string]*Order),
		quotes: make(map[string]*Quote),
	}
}

// --- Order Operations ---

// AddOrder adds or updates an order in the store.
func (os *OrderStore) AddOrder(order *Order) {
	os.mu.Lock()
	defer os.mu.Unlock()
	order.UpdatedAt = time.Now()
	if order.CreatedAt.IsZero() {
		order.CreatedAt = order.UpdatedAt
	}
	os.orders[order.ClOrdID] = order
}

// GetOrder retrieves an order by ClOrdID.
func (os *OrderStore) GetOrder(clOrdID string) *Order {
	os.mu.RLock()
	defer os.mu.RUnlock()
	if order, exists := os.orders[clOrdID]; exists {
		copy := *order
		return &copy
	}
	return nil
}

// GetOrderByOrderID retrieves an order by exchange OrderID.
func (os *OrderStore) GetOrderByOrderID(orderID string) *Order {
	os.mu.RLock()
	defer os.mu.RUnlock()
	for _, order := range os.orders {
		if order.OrderID == orderID {
			copy := *order
			return &copy
		}
	}
	return nil
}

// UpdateOrderFromExecReport updates an order based on an execution report.
func (os *OrderStore) UpdateOrderFromExecReport(er *ExecutionReport) {
	os.mu.Lock()
	defer os.mu.Unlock()

	order, exists := os.orders[er.ClOrdID]
	if !exists {
		// Create new order from execution report
		order = &Order{
			ClOrdID:   er.ClOrdID,
			CreatedAt: time.Now(),
		}
		os.orders[er.ClOrdID] = order
	}

	order.UpdatedAt = time.Now()
	order.OrderID = er.OrderID
	order.Symbol = er.Symbol
	order.Side = er.Side
	order.OrdType = er.OrdType
	order.OrdStatus = er.OrdStatus
	order.ExecType = er.ExecType
	order.Account = er.Account

	if er.OrderQty != "" {
		order.OrderQty = er.OrderQty
	}
	if er.CashOrderQty != "" {
		order.CashOrderQty = er.CashOrderQty
	}
	if er.Price != "" {
		order.Price = er.Price
	}
	if er.AvgPx != "" {
		order.AvgPx = er.AvgPx
	}
	if er.CumQty != "" {
		order.CumQty = er.CumQty
	}
	if er.LeavesQty != "" {
		order.LeavesQty = er.LeavesQty
	}
	if er.LastPx != "" {
		order.LastPx = er.LastPx
	}
	if er.LastShares != "" {
		order.LastShares = er.LastShares
	}
	if er.ExecID != "" {
		order.ExecID = er.ExecID
	}
	if er.Commission != "" {
		order.Commission = er.Commission
	}
	if er.FilledAmt != "" {
		order.FilledAmt = er.FilledAmt
	}
	if er.NetAvgPx != "" {
		order.NetAvgPx = er.NetAvgPx
	}
	if er.OrdRejReason != "" {
		order.OrdRejReason = er.OrdRejReason
	}
	if er.Text != "" {
		order.Text = er.Text
	}
}

// GetAllOrders returns a copy of all orders.
func (os *OrderStore) GetAllOrders() []*Order {
	os.mu.RLock()
	defer os.mu.RUnlock()

	result := make([]*Order, 0, len(os.orders))
	for _, order := range os.orders {
		copy := *order
		result = append(result, &copy)
	}
	return result
}

// GetOpenOrders returns orders that are still open (not filled, canceled, or rejected).
func (os *OrderStore) GetOpenOrders() []*Order {
	os.mu.RLock()
	defer os.mu.RUnlock()

	result := make([]*Order, 0)
	for _, order := range os.orders {
		if isOpenStatus(order.OrdStatus) {
			copy := *order
			result = append(result, &copy)
		}
	}
	return result
}

// RemoveOrder removes an order from the store.
func (os *OrderStore) RemoveOrder(clOrdID string) {
	os.mu.Lock()
	defer os.mu.Unlock()
	delete(os.orders, clOrdID)
}

// --- Quote Operations ---

// AddQuote adds or updates a quote in the store.
func (os *OrderStore) AddQuote(quote *Quote) {
	os.mu.Lock()
	defer os.mu.Unlock()
	quote.ReceivedAt = time.Now()
	os.quotes[quote.QuoteReqID] = quote
}

// GetQuote retrieves a quote by QuoteReqID.
func (os *OrderStore) GetQuote(quoteReqID string) *Quote {
	os.mu.RLock()
	defer os.mu.RUnlock()
	if quote, exists := os.quotes[quoteReqID]; exists {
		copy := *quote
		return &copy
	}
	return nil
}

// GetQuoteByQuoteID retrieves a quote by QuoteID.
func (os *OrderStore) GetQuoteByQuoteID(quoteID string) *Quote {
	os.mu.RLock()
	defer os.mu.RUnlock()
	for _, quote := range os.quotes {
		if quote.QuoteID == quoteID {
			copy := *quote
			return &copy
		}
	}
	return nil
}

// RemoveQuote removes a quote from the store.
func (os *OrderStore) RemoveQuote(quoteReqID string) {
	os.mu.Lock()
	defer os.mu.Unlock()
	delete(os.quotes, quoteReqID)
}

// GetAllQuotes returns a copy of all quotes.
func (os *OrderStore) GetAllQuotes() []*Quote {
	os.mu.RLock()
	defer os.mu.RUnlock()

	result := make([]*Quote, 0, len(os.quotes))
	for _, quote := range os.quotes {
		copy := *quote
		result = append(result, &copy)
	}
	return result
}

// --- Helper Functions ---

// isOpenStatus returns true if the order status indicates an open order.
func isOpenStatus(status string) bool {
	switch status {
	case "0", "1", "6", "9", "A", "E": // New, PartiallyFilled, PendingCancel, Suspended, PendingNew, PendingReplace
		return true
	default:
		return false
	}
}
