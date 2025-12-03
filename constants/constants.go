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

package constants

import "github.com/quickfixgo/quickfix"

// --- Message Types ---
const (
	// Admin Messages
	MsgTypeLogon            = "A" // Logon
	MsgTypeReject           = "3" // Session-level Reject
	MsgTypeBusinessReject   = "j" // Business Message Reject
	MsgTypeMarketDataReject = "Y" // Market Data Request Reject

	// Market Data Messages
	MsgTypeMarketDataRequest     = "V" // Market Data Request
	MsgTypeMarketDataSnapshot    = "W" // Market Data Snapshot/Full Refresh
	MsgTypeMarketDataIncremental = "X" // Market Data Incremental Refresh

	// Order Entry Messages
	MsgTypeNewOrderSingle       = "D" // New Order Single
	MsgTypeOrderCancelRequest   = "F" // Order Cancel Request
	MsgTypeOrderCancelReplace   = "G" // Order Cancel/Replace Request
	MsgTypeOrderStatusRequest   = "H" // Order Status Request
	MsgTypeExecutionReport      = "8" // Execution Report
	MsgTypeOrderCancelReject    = "9" // Order Cancel Reject
	MsgTypeQuoteRequest         = "R" // Quote Request
	MsgTypeQuote                = "S" // Quote
	MsgTypeQuoteAcknowledgement = "b" // Quote Acknowledgement
)

// --- Protocol Constants ---
const (
	FixTimeFormat     = "20060102-15:04:05.000"
	FixBeginString    = "FIXT.1.1"
	EncryptMethodNone = "0"
	HeartBtInterval   = "30"
	DropCopyFlagYes   = "Y"
	MsgSeqNumInit     = "1"
)

// --- Subscription Request Types ---
const (
	SubscriptionRequestTypeSnapshot    = "0" // Snapshot
	SubscriptionRequestTypeSubscribe   = "1" // Subscribe
	SubscriptionRequestTypeUnsubscribe = "2" // Unsubscribe
)

// --- MD Entry Types ---
const (
	MdEntryTypeBid    = "0" // Bid
	MdEntryTypeOffer  = "1" // Offer/Ask
	MdEntryTypeTrade  = "2" // Trade
	MdEntryTypeOpen   = "4" // Open
	MdEntryTypeClose  = "5" // Close
	MdEntryTypeHigh   = "7" // High
	MdEntryTypeLow    = "8" // Low
	MdEntryTypeVolume = "B" // Volume
)

// --- MD Update Types ---
const (
	MdUpdateTypeFullRefresh = "0" // Full refresh
	MdUpdateTypeIncremental = "1" // Incremental refresh
)

// --- Order Types (Tag 40) ---
const (
	OrdTypeMarket           = "1" // Market
	OrdTypeLimit            = "2" // Limit
	OrdTypeStop             = "3" // Stop
	OrdTypeStopLimit        = "4" // Stop Limit
	OrdTypePreviouslyQuoted = "D" // Previously Quoted (for RFQ)
)

// --- Side (Tag 54) ---
const (
	SideBuy  = "1" // Buy
	SideSell = "2" // Sell
)

// --- Time In Force (Tag 59) ---
const (
	TimeInForceGTC = "1" // Good Till Cancel
	TimeInForceIOC = "3" // Immediate or Cancel
	TimeInForceFOK = "4" // Fill or Kill
	TimeInForceGTD = "6" // Good Till Date
)

// --- Target Strategy (Tag 847) ---
const (
	TargetStrategyLimit     = "L"  // Limit order
	TargetStrategyMarket    = "M"  // Market order
	TargetStrategyTWAP      = "T"  // TWAP order
	TargetStrategyVWAP      = "V"  // VWAP order
	TargetStrategyStopLimit = "SL" // Stop Limit order
	TargetStrategyRFQ       = "R"  // RFQ order
)

// --- Order Status (Tag 39) ---
const (
	OrdStatusNew             = "0" // New
	OrdStatusPartiallyFilled = "1" // Partially Filled
	OrdStatusFilled          = "2" // Filled
	OrdStatusDoneForDay      = "3" // Done for Day
	OrdStatusCanceled        = "4" // Canceled
	OrdStatusReplaced        = "5" // Replaced
	OrdStatusPendingCancel   = "6" // Pending Cancel
	OrdStatusStopped         = "7" // Stopped
	OrdStatusRejected        = "8" // Rejected
	OrdStatusSuspended       = "9" // Suspended
	OrdStatusPendingNew      = "A" // Pending New
	OrdStatusCalculated      = "B" // Calculated
	OrdStatusExpired         = "C" // Expired
	OrdStatusAcceptedBidding = "D" // Accepted for Bidding
	OrdStatusPendingReplace  = "E" // Pending Replace
)

// --- Execution Type (Tag 150) ---
const (
	ExecTypeNew           = "0" // New Order
	ExecTypePartialFill   = "1" // Partial Fill
	ExecTypeFilled        = "2" // Filled
	ExecTypeDone          = "3" // Done
	ExecTypeCanceled      = "4" // Canceled
	ExecTypePendingCancel = "6" // Pending Cancel
	ExecTypeStopped       = "7" // Stopped
	ExecTypeRejected      = "8" // Rejected
	ExecTypePendingNew    = "A" // Pending New
	ExecTypeExpired       = "C" // Expired
	ExecTypeRestated      = "D" // Restated
	ExecTypeOrderStatus   = "I" // Order Status
)

// --- Order Reject Reason (Tag 103) ---
const (
	OrdRejReasonBrokerOption   = "0"  // Broker option
	OrdRejReasonUnknownSymbol  = "1"  // Unknown symbol
	OrdRejReasonExchangeClosed = "2"  // Exchange closed
	OrdRejReasonExceedsLimit   = "3"  // Order exceeds limit
	OrdRejReasonTooLate        = "4"  // Too late to enter
	OrdRejReasonUnknownOrder   = "5"  // Unknown Order
	OrdRejReasonDuplicateOrder = "6"  // Duplicate Order
	OrdRejReasonOther          = "99" // Other
)

// --- Cancel Reject Response To (Tag 434) ---
const (
	CxlRejResponseToCancel  = "1" // Order Cancel Request (F)
	CxlRejResponseToReplace = "2" // Order Cancel/Replace Request (G)
)

// --- Quote Acknowledgement Status (Tag 297) ---
const (
	QuoteAckStatusRejected = "5" // Rejected
)

// --- Quote Reject Reason (Tag 300) ---
const (
	QuoteRejectReasonUnknownSymbol  = "1"  // Unknown symbol
	QuoteRejectReasonExchangeClosed = "2"  // Exchange closed
	QuoteRejectReasonExceedsLimit   = "3"  // Quote Request exceeds limit
	QuoteRejectReasonDuplicate      = "6"  // Duplicate Quote
	QuoteRejectReasonInvalidPrice   = "8"  // Invalid price
	QuoteRejectReasonOther          = "99" // Other
)

// --- Session Reject Reason (Tag 373) ---
const (
	SessionRejectReasonInvalidTag          = "0"
	SessionRejectReasonRequiredTagMissing  = "1"
	SessionRejectReasonTagNotDefined       = "2"
	SessionRejectReasonUndefinedTag        = "3"
	SessionRejectReasonTagWithoutValue     = "4"
	SessionRejectReasonValueOutOfRange     = "5"
	SessionRejectReasonIncorrectDataFormat = "6"
	SessionRejectReasonDecryptionProblem   = "7"
	SessionRejectReasonSignatureProblem    = "8"
	SessionRejectReasonCompIDProblem       = "9"
	SessionRejectReasonSendingTimeAccuracy = "10"
	SessionRejectReasonInvalidMsgType      = "11"
)

// --- Business Reject Reason (Tag 380) ---
const (
	BusinessRejectReasonOther               = "0"
	BusinessRejectReasonUnknownID           = "1"
	BusinessRejectReasonUnknownSecurity     = "2"
	BusinessRejectReasonUnsupportedMsgType  = "3"
	BusinessRejectReasonApplicationNotAvail = "4"
	BusinessRejectReasonCondRequiredMissing = "5"
	BusinessRejectReasonNotAuthorized       = "6"
)

// --- Execution Instruction (Tag 18) ---
// Per Coinbase Prime FIX API: https://docs.cdp.coinbase.com/prime/fix-api/order-entry-messages
// ExecInst must be "A" for Post Only orders (maker-only).
const (
	ExecInstPostOnly = "A" // Post Only (maker-only order)
)

// --- Handling Instruction (Tag 21) ---
const (
	HandlInstAutomatedNoIntervention = "1"
)

// --- Commission Type (Tag 13) ---
const (
	CommTypeAbsolute = "3" // Absolute (fixed amount)
)

// --- Misc Fee Type (Tag 139) ---
// Per Coinbase Prime FIX API Execution Report:
// https://docs.cdp.coinbase.com/prime/fix-api/order-entry-messages
// MiscFees is a repeating group with Tags 136 (count), 137 (amt), 138 (curr), 139 (type).
const (
	MiscFeeTypeFinancing  = "1" // Financing Fee
	MiscFeeTypeClientComm = "2" // Client Commission
	MiscFeeTypeCESComm    = "3" // CES Commission
	MiscFeeTypeVenueFee   = "4" // Venue Fee
)

// --- Standard FIX Tags ---
var (
	TagAccount        = quickfix.Tag(1)
	TagAvgPx          = quickfix.Tag(6)
	TagBeginString    = quickfix.Tag(8)
	TagClOrdID        = quickfix.Tag(11)
	TagCommission     = quickfix.Tag(12)
	TagCommType       = quickfix.Tag(13)
	TagCumQty         = quickfix.Tag(14)
	TagExecID         = quickfix.Tag(17)
	TagExecInst       = quickfix.Tag(18)
	TagHandlInst      = quickfix.Tag(21)
	TagLastMkt        = quickfix.Tag(30)
	TagLastPx         = quickfix.Tag(31)
	TagLastShares     = quickfix.Tag(32)
	TagMsgSeqNum      = quickfix.Tag(34)
	TagMsgType        = quickfix.Tag(35)
	TagOrderID        = quickfix.Tag(37)
	TagOrderQty       = quickfix.Tag(38)
	TagOrdStatus      = quickfix.Tag(39)
	TagOrdType        = quickfix.Tag(40)
	TagOrigClOrdID    = quickfix.Tag(41)
	TagPrice          = quickfix.Tag(44)
	TagRefSeqNum      = quickfix.Tag(45)
	TagSenderCompId   = quickfix.Tag(49)
	TagSenderSubID    = quickfix.Tag(50)
	TagSendingTime    = quickfix.Tag(52)
	TagSide           = quickfix.Tag(54)
	TagSymbol         = quickfix.Tag(55)
	TagText           = quickfix.Tag(58)
	TagTimeInForce    = quickfix.Tag(59)
	TagTransactTime   = quickfix.Tag(60)
	TagTargetCompId   = quickfix.Tag(56)
	TagValidUntilTime = quickfix.Tag(62)
	TagHmac           = quickfix.Tag(96)
	TagEncryptMethod  = quickfix.Tag(98)
	TagStopPx         = quickfix.Tag(99)
	TagOrdRejReason   = quickfix.Tag(103)
	TagCxlRejReason   = quickfix.Tag(102)
	TagHeartBtInt     = quickfix.Tag(108)
	TagQuoteID        = quickfix.Tag(117)
	TagExpireTime     = quickfix.Tag(126)
	TagQuoteReqID     = quickfix.Tag(131)
	TagBidPx          = quickfix.Tag(132)
	TagOfferPx        = quickfix.Tag(133)
	TagBidSize        = quickfix.Tag(134)
	TagOfferSize      = quickfix.Tag(135)
	TagNoMiscFees     = quickfix.Tag(136)
	TagMiscFeeAmt     = quickfix.Tag(137)
	TagMiscFeeCurr    = quickfix.Tag(138)
	TagMiscFeeType    = quickfix.Tag(139)
	TagNoRelatedSym   = quickfix.Tag(146)
	TagExecType       = quickfix.Tag(150)
	TagLeavesQty      = quickfix.Tag(151)
	TagCashOrderQty   = quickfix.Tag(152)
	TagEffectiveTime  = quickfix.Tag(168)
	TagMaxShow        = quickfix.Tag(210)

	// Market Data Tags
	TagMdReqId                 = quickfix.Tag(262)
	TagSubscriptionRequestType = quickfix.Tag(263)
	TagMarketDepth             = quickfix.Tag(264)
	TagMdUpdateType            = quickfix.Tag(265)
	TagNoMdEntryTypes          = quickfix.Tag(267)
	TagNoMdEntries             = quickfix.Tag(268)
	TagMdEntryType             = quickfix.Tag(269)
	TagMdEntryPx               = quickfix.Tag(270)
	TagMdEntrySize             = quickfix.Tag(271)
	TagMdEntryTime             = quickfix.Tag(273)
	TagMdReqRejReason          = quickfix.Tag(281)
	TagMdEntryPositionNo       = quickfix.Tag(290)

	// Quote Tags
	TagQuoteAckStatus    = quickfix.Tag(297)
	TagQuoteRejectReason = quickfix.Tag(300)

	// Reject Tags
	TagRefTagID             = quickfix.Tag(371)
	TagRefMsgType           = quickfix.Tag(372)
	TagSessionRejectReason  = quickfix.Tag(373)
	TagBusinessRejectReason = quickfix.Tag(380)

	// Order Tags
	TagCxlRejResponseTo  = quickfix.Tag(434)
	TagUsername          = quickfix.Tag(553)
	TagPassword          = quickfix.Tag(554)
	TagTargetStrategy    = quickfix.Tag(847)
	TagParticipationRate = quickfix.Tag(849)
	TagDefaultApplVerId  = quickfix.Tag(1137)

	// Coinbase Custom Tags
	TagAggressorSide = quickfix.Tag(2446)
	TagDropCopyFlag  = quickfix.Tag(9406)
	TagAccessKey     = quickfix.Tag(9407)
	TagFilledAmt     = quickfix.Tag(8002)
	TagNetAvgPrice   = quickfix.Tag(8006)
	TagIsRaiseExact  = quickfix.Tag(8999)
)

// --- MD Rejection Reasons ---
const (
	MdReqRejReasonUnknownSymbol              = "0"
	MdReqRejReasonDuplicateMdReqId           = "1"
	MdReqRejReasonInsufficientBandwidth      = "2"
	MdReqRejReasonInsufficientPermission     = "3"
	MdReqRejReasonInvalidSubscriptionReqType = "4"
	MdReqRejReasonInvalidMarketDepth         = "5"
	MdReqRejReasonUnsupportedMdUpdateType    = "6"
	MdReqRejReasonOther                      = "7"
	MdReqRejReasonUnsupportedMdEntryType     = "8"
)
