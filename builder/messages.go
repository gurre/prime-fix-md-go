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

package builder

import (
	"time"

	"prime-fix-md-go/constants"
	"prime-fix-md-go/utils"

	"github.com/quickfixgo/quickfix"
)

// FieldSetter abstracts setting fields on FIX message components.
type FieldSetter interface {
	SetField(tag quickfix.Tag, field quickfix.FieldValueWriter) *quickfix.FieldMap
}

func setString(fs FieldSetter, tag quickfix.Tag, value string) {
	fs.SetField(tag, quickfix.FIXString(value))
}

// setStringIfNotEmpty sets a field only if the value is non-empty.
func setStringIfNotEmpty(fs FieldSetter, tag quickfix.Tag, value string) {
	if value != "" {
		fs.SetField(tag, quickfix.FIXString(value))
	}
}

// buildHeader sets common header fields for outgoing messages.
func buildHeader(header *quickfix.Header, msgType, senderCompId, targetCompId string) {
	setString(header, constants.TagBeginString, constants.FixBeginString)
	setString(header, constants.TagMsgType, msgType)
	setString(header, constants.TagSenderCompId, senderCompId)
	setString(header, constants.TagTargetCompId, targetCompId)
	setString(header, constants.TagSendingTime, time.Now().UTC().Format(constants.FixTimeFormat))
}

// --- Logon Message ---

func BuildLogon(
	body *quickfix.Body,
	ts, apiKey, apiSecret, passphrase, targetCompId, portfolioId string,
) {
	sig := utils.Sign(ts, constants.MsgTypeLogon, constants.MsgSeqNumInit, apiKey, targetCompId, passphrase, apiSecret)

	setString(body, constants.TagEncryptMethod, constants.EncryptMethodNone)
	setString(body, constants.TagHeartBtInt, constants.HeartBtInterval)

	setString(body, constants.TagPassword, passphrase)
	setString(body, constants.TagAccount, portfolioId)
	setString(body, constants.TagHmac, sig)
	// Per Coinbase Prime FIX API: use Tag 9407 (AccessKey) for API key
	// https://docs.cdp.coinbase.com/prime/fix-api/admin-messages
	setString(body, constants.TagAccessKey, apiKey)
	setString(body, constants.TagDropCopyFlag, constants.DropCopyFlagYes)
}

// --- Market Data Request ---

func BuildMarketDataRequest(
	mdReqId string,
	symbols []string,
	subscriptionRequestType string,
	marketDepth string,
	senderCompId string,
	targetCompId string,
	mdEntryTypes []string,
) *quickfix.Message {
	m := quickfix.NewMessage()
	buildHeader(&m.Header, constants.MsgTypeMarketDataRequest, senderCompId, targetCompId)

	setString(&m.Body, constants.TagMdReqId, mdReqId)
	setString(&m.Body, constants.TagSubscriptionRequestType, subscriptionRequestType)
	setString(&m.Body, constants.TagMarketDepth, marketDepth)

	if subscriptionRequestType == constants.SubscriptionRequestTypeSubscribe {
		setString(&m.Body, constants.TagMdUpdateType, constants.MdUpdateTypeIncremental)
	}

	mdEntryGroup := quickfix.NewRepeatingGroup(
		constants.TagNoMdEntryTypes,
		quickfix.GroupTemplate{quickfix.GroupElement(constants.TagMdEntryType)},
	)

	for _, entryType := range mdEntryTypes {
		setString(mdEntryGroup.Add(), constants.TagMdEntryType, entryType)
	}
	m.Body.SetGroup(mdEntryGroup)

	relatedSymGroup := quickfix.NewRepeatingGroup(
		constants.TagNoRelatedSym,
		quickfix.GroupTemplate{quickfix.GroupElement(constants.TagSymbol)},
	)

	for _, symbol := range symbols {
		setString(relatedSymGroup.Add(), constants.TagSymbol, symbol)
	}
	m.Body.SetGroup(relatedSymGroup)
	return m
}

// --- New Order Single (D) ---

// NewOrderParams contains parameters for creating a new order.
type NewOrderParams struct {
	Account        string // Portfolio ID (required)
	ClOrdID        string // Client order ID (required)
	Symbol         string // Product pair e.g. BTC-USD (required)
	Side           string // "1" buy, "2" sell (required)
	OrdType        string // Order type (required)
	TargetStrategy string // L, M, T, V, SL, R (required)
	TimeInForce    string // 1, 3, 4, 6 (required)
	OrderQty       string // Size in base units (conditional)
	CashOrderQty   string // Size in quote units (conditional)
	Price          string // Limit price (conditional)
	StopPx         string // Stop price for stop orders (conditional)
	ExpireTime     string // For GTD/TWAP/VWAP (conditional)
	EffectiveTime  string // Start time for TWAP/VWAP (conditional)
	MaxShow        string // Display size (optional)
	ExecInst       string // "A" for post-only (conditional)
	PartRate       string // Participation rate for TWAP/VWAP (conditional)
	QuoteID        string // For RFQ orders (conditional)
	IsRaiseExact   string // Y/N for raise exact orders (optional)
}

// BuildNewOrderSingle creates a New Order Single (D) message.
//
// Example - Market order:
//
//	params := NewOrderParams{
//	    Account: "portfolio-123", ClOrdID: "order-1", Symbol: "BTC-USD",
//	    Side: constants.SideBuy, OrdType: constants.OrdTypeMarket,
//	    TargetStrategy: constants.TargetStrategyMarket,
//	    TimeInForce: constants.TimeInForceIOC, OrderQty: "0.01",
//	}
//	msg := BuildNewOrderSingle(params, senderCompId, targetCompId)
func BuildNewOrderSingle(params NewOrderParams, senderCompId, targetCompId string) *quickfix.Message {
	m := quickfix.NewMessage()
	buildHeader(&m.Header, constants.MsgTypeNewOrderSingle, senderCompId, targetCompId)

	// Required fields
	setString(&m.Body, constants.TagAccount, params.Account)
	setString(&m.Body, constants.TagClOrdID, params.ClOrdID)
	setString(&m.Body, constants.TagSymbol, params.Symbol)
	setString(&m.Body, constants.TagSide, params.Side)
	setString(&m.Body, constants.TagOrdType, params.OrdType)
	setString(&m.Body, constants.TagTargetStrategy, params.TargetStrategy)
	setString(&m.Body, constants.TagTimeInForce, params.TimeInForce)
	setString(&m.Body, constants.TagTransactTime, time.Now().UTC().Format(constants.FixTimeFormat))

	// Conditional fields
	setStringIfNotEmpty(&m.Body, constants.TagOrderQty, params.OrderQty)
	setStringIfNotEmpty(&m.Body, constants.TagCashOrderQty, params.CashOrderQty)
	setStringIfNotEmpty(&m.Body, constants.TagPrice, params.Price)
	setStringIfNotEmpty(&m.Body, constants.TagStopPx, params.StopPx)
	setStringIfNotEmpty(&m.Body, constants.TagExpireTime, params.ExpireTime)
	setStringIfNotEmpty(&m.Body, constants.TagEffectiveTime, params.EffectiveTime)
	setStringIfNotEmpty(&m.Body, constants.TagMaxShow, params.MaxShow)
	setStringIfNotEmpty(&m.Body, constants.TagExecInst, params.ExecInst)
	setStringIfNotEmpty(&m.Body, constants.TagParticipationRate, params.PartRate)
	setStringIfNotEmpty(&m.Body, constants.TagQuoteID, params.QuoteID)
	setStringIfNotEmpty(&m.Body, constants.TagIsRaiseExact, params.IsRaiseExact)

	return m
}

// --- Order Cancel Request (F) ---

// CancelOrderParams contains parameters for canceling an order.
type CancelOrderParams struct {
	Account      string // Portfolio ID (required)
	ClOrdID      string // Cancel request ID (required)
	OrigClOrdID  string // Original order's ClOrdID (required)
	OrderID      string // Coinbase order ID (required)
	Symbol       string // Product pair (required)
	Side         string // "1" buy, "2" sell (required)
	OrderQty     string // Original order quantity (conditional)
	CashOrderQty string // If originally in quote units (conditional)
}

// BuildOrderCancelRequest creates an Order Cancel Request (F) message.
//
// Example:
//
//	params := CancelOrderParams{
//	    Account: "portfolio-123", ClOrdID: "cancel-1", OrigClOrdID: "order-1",
//	    OrderID: "cb-order-id", Symbol: "BTC-USD", Side: constants.SideBuy,
//	    OrderQty: "0.01",
//	}
//	msg := BuildOrderCancelRequest(params, senderCompId, targetCompId)
func BuildOrderCancelRequest(params CancelOrderParams, senderCompId, targetCompId string) *quickfix.Message {
	m := quickfix.NewMessage()
	buildHeader(&m.Header, constants.MsgTypeOrderCancelRequest, senderCompId, targetCompId)

	setString(&m.Body, constants.TagAccount, params.Account)
	setString(&m.Body, constants.TagClOrdID, params.ClOrdID)
	setString(&m.Body, constants.TagOrigClOrdID, params.OrigClOrdID)
	setString(&m.Body, constants.TagOrderID, params.OrderID)
	setString(&m.Body, constants.TagSymbol, params.Symbol)
	setString(&m.Body, constants.TagSide, params.Side)
	setString(&m.Body, constants.TagTransactTime, time.Now().UTC().Format(constants.FixTimeFormat))

	setStringIfNotEmpty(&m.Body, constants.TagOrderQty, params.OrderQty)
	setStringIfNotEmpty(&m.Body, constants.TagCashOrderQty, params.CashOrderQty)

	return m
}

// --- Order Cancel/Replace Request (G) ---

// ReplaceOrderParams contains parameters for modifying an order.
type ReplaceOrderParams struct {
	Account      string // Portfolio ID (required)
	ClOrdID      string // New request ID (required, must differ from OrigClOrdID)
	OrigClOrdID  string // Original order's ClOrdID (required)
	OrderID      string // Coinbase order ID (required)
	Symbol       string // Product pair (required)
	Side         string // Must match original (required)
	OrdType      string // Must match original (required)
	OrderQty     string // Total intended quantity including filled (conditional)
	CashOrderQty string // If originally in quote units (conditional)
	Price        string // New limit price (required)
	StopPx       string // New stop price for stop-limit (conditional)
	ExpireTime   string // New expiration (conditional)
	MaxShow      string // New display size (conditional)
}

// BuildOrderCancelReplaceRequest creates an Order Cancel/Replace Request (G) message.
//
// Example:
//
//	params := ReplaceOrderParams{
//	    Account: "portfolio-123", ClOrdID: "replace-1", OrigClOrdID: "order-1",
//	    OrderID: "cb-order-id", Symbol: "BTC-USD", Side: constants.SideBuy,
//	    OrdType: constants.OrdTypeLimit, OrderQty: "0.02", Price: "50000.00",
//	}
//	msg := BuildOrderCancelReplaceRequest(params, senderCompId, targetCompId)
func BuildOrderCancelReplaceRequest(params ReplaceOrderParams, senderCompId, targetCompId string) *quickfix.Message {
	m := quickfix.NewMessage()
	buildHeader(&m.Header, constants.MsgTypeOrderCancelReplace, senderCompId, targetCompId)

	setString(&m.Body, constants.TagAccount, params.Account)
	setString(&m.Body, constants.TagClOrdID, params.ClOrdID)
	setString(&m.Body, constants.TagOrigClOrdID, params.OrigClOrdID)
	setString(&m.Body, constants.TagOrderID, params.OrderID)
	setString(&m.Body, constants.TagSymbol, params.Symbol)
	setString(&m.Body, constants.TagSide, params.Side)
	setString(&m.Body, constants.TagOrdType, params.OrdType)
	setString(&m.Body, constants.TagHandlInst, constants.HandlInstAutomatedNoIntervention)
	setString(&m.Body, constants.TagTransactTime, time.Now().UTC().Format(constants.FixTimeFormat))
	setString(&m.Body, constants.TagPrice, params.Price)

	setStringIfNotEmpty(&m.Body, constants.TagOrderQty, params.OrderQty)
	setStringIfNotEmpty(&m.Body, constants.TagCashOrderQty, params.CashOrderQty)
	setStringIfNotEmpty(&m.Body, constants.TagStopPx, params.StopPx)
	setStringIfNotEmpty(&m.Body, constants.TagExpireTime, params.ExpireTime)
	setStringIfNotEmpty(&m.Body, constants.TagMaxShow, params.MaxShow)

	return m
}

// --- Order Status Request (H) ---

// BuildOrderStatusRequest creates an Order Status Request (H) message.
//
// Example:
//
//	msg := BuildOrderStatusRequest("cb-order-id", "order-1", "BTC-USD", "1", senderCompId, targetCompId)
func BuildOrderStatusRequest(orderID, clOrdID, symbol, side, senderCompId, targetCompId string) *quickfix.Message {
	m := quickfix.NewMessage()
	buildHeader(&m.Header, constants.MsgTypeOrderStatusRequest, senderCompId, targetCompId)

	setString(&m.Body, constants.TagOrderID, orderID)
	setStringIfNotEmpty(&m.Body, constants.TagClOrdID, clOrdID)
	setStringIfNotEmpty(&m.Body, constants.TagSymbol, symbol)
	setStringIfNotEmpty(&m.Body, constants.TagSide, side)

	return m
}

// --- Quote Request (R) ---

// QuoteRequestParams contains parameters for requesting a quote.
type QuoteRequestParams struct {
	QuoteReqID string // Client-selected identifier (required)
	Account    string // Portfolio ID (required)
	Symbol     string // Product pair (required)
	Side       string // "1" buy, "2" sell (required)
	OrderQty   string // Size in base units (required)
	Price      string // Limit price (required)
}

// BuildQuoteRequest creates a Quote Request (R) message for RFQ.
//
// Example:
//
//	params := QuoteRequestParams{
//	    QuoteReqID: "quote-req-1", Account: "portfolio-123",
//	    Symbol: "BTC-USD", Side: constants.SideBuy,
//	    OrderQty: "1.0", Price: "50000.00",
//	}
//	msg := BuildQuoteRequest(params, senderCompId, targetCompId)
func BuildQuoteRequest(params QuoteRequestParams, senderCompId, targetCompId string) *quickfix.Message {
	m := quickfix.NewMessage()
	buildHeader(&m.Header, constants.MsgTypeQuoteRequest, senderCompId, targetCompId)

	setString(&m.Body, constants.TagQuoteReqID, params.QuoteReqID)
	setString(&m.Body, constants.TagAccount, params.Account)
	setString(&m.Body, constants.TagSymbol, params.Symbol)
	setString(&m.Body, constants.TagSide, params.Side)
	setString(&m.Body, constants.TagOrderQty, params.OrderQty)
	setString(&m.Body, constants.TagOrdType, constants.OrdTypeLimit)
	setString(&m.Body, constants.TagPrice, params.Price)
	setString(&m.Body, constants.TagTimeInForce, constants.TimeInForceFOK)

	return m
}

// --- Accept Quote (New Order Single with QuoteID) ---

// AcceptQuoteParams contains parameters for accepting a quote.
type AcceptQuoteParams struct {
	Account  string // Portfolio ID (required)
	ClOrdID  string // Client order ID (required)
	Symbol   string // Product pair (required)
	Side     string // "1" buy, "2" sell (required)
	QuoteID  string // From Quote message tag 117 (required)
	OrderQty string // Size in base units (required)
	Price    string // From Quote bid/offer price (required)
}

// BuildAcceptQuote creates a New Order Single (D) to accept a Quote.
//
// Example:
//
//	params := AcceptQuoteParams{
//	    Account: "portfolio-123", ClOrdID: "accept-1",
//	    Symbol: "BTC-USD", Side: constants.SideBuy,
//	    QuoteID: "quote-123", OrderQty: "1.0", Price: "50000.00",
//	}
//	msg := BuildAcceptQuote(params, senderCompId, targetCompId)
func BuildAcceptQuote(params AcceptQuoteParams, senderCompId, targetCompId string) *quickfix.Message {
	m := quickfix.NewMessage()
	buildHeader(&m.Header, constants.MsgTypeNewOrderSingle, senderCompId, targetCompId)

	setString(&m.Body, constants.TagAccount, params.Account)
	setString(&m.Body, constants.TagClOrdID, params.ClOrdID)
	setString(&m.Body, constants.TagSymbol, params.Symbol)
	setString(&m.Body, constants.TagSide, params.Side)
	setString(&m.Body, constants.TagOrdType, constants.OrdTypePreviouslyQuoted)
	setString(&m.Body, constants.TagTargetStrategy, constants.TargetStrategyRFQ)
	setString(&m.Body, constants.TagTimeInForce, constants.TimeInForceFOK)
	setString(&m.Body, constants.TagQuoteID, params.QuoteID)
	setString(&m.Body, constants.TagOrderQty, params.OrderQty)
	setString(&m.Body, constants.TagPrice, params.Price)
	setString(&m.Body, constants.TagTransactTime, time.Now().UTC().Format(constants.FixTimeFormat))

	return m
}
