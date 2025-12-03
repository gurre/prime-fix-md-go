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
	"fmt"
	"log"

	"prime-fix-md-go/constants"
)

func (a *FixApp) displayHelp() {
	fmt.Print(`Commands:
  --- Market Data ---
  md <symbol> [flags...]        - Market data request
  unsubscribe <symbol|reqId>    - Stop subscription(s)
  status                        - Show active subscriptions

  --- Order Entry ---
  order <buy|sell> <symbol> <qty> [price] [flags...]  - Submit new order
  cancel <clOrdId|orderId>      - Cancel an order
  replace <clOrdId> [--qty Q] [--price P]  - Modify an order
  ordstatus <clOrdId|orderId>   - Request order status
  orders                        - List tracked orders

  --- RFQ (Request for Quote) ---
  rfq <buy|sell> <symbol> <qty> - Request a quote
  accept <quoteId|quoteReqId>   - Accept a received quote
  quotes                        - List received quotes

  --- General ---
  help                          - Show this help message
  version, exit

Market Data Flags:
  --snapshot / --subscribe      - Request type
  --depth N                     - Order book depth (0=full, 1=L1, N=LN)
  --trades                      - Trade data
  --o, --c, --h, --l, --v       - OHLCV data

Order Flags:
  --type <market|limit|stop>    - Order type
  --tif <gtc|ioc|fok|gtd>       - Time in force
  --strategy <L|M|T|V|SL>       - Target strategy
  --postonly                    - Post-only (maker)
  --cash                        - Qty in quote currency

Examples:
  md BTC-USD --snapshot --trades          - Recent trades
  md BTC-USD --subscribe --depth 10       - Live L10 book
  order buy BTC-USD 0.01 50000            - Limit buy 0.01 BTC at $50k
  order sell ETH-USD 1.5 --type market    - Market sell 1.5 ETH
  rfq buy BTC-USD 1.0                     - Request buy quote for 1 BTC
  cancel ord_123                          - Cancel order
`)
}

func (a *FixApp) displaySnapshotTrades(trades []Trade, symbol string) {
	log.Printf("\n📋 Market Data Snapshot for %s:", symbol)

	// Group entries by type
	byType := make(map[string][]Trade)
	for _, trade := range trades {
		entryType := trade.EntryType
		if entryType == "" {
			entryType = "2" // Default to Trade if not specified
		}
		byType[entryType] = append(byType[entryType], trade)
	}

	// Display each type separately
	for entryType, entries := range byType {
		typeName := getMdEntryTypeName(entryType)
		log.Printf("\n🔹 %s Entries (%d):", typeName, len(entries))

		if entryType == constants.MdEntryTypeBid || entryType == constants.MdEntryTypeOffer {
			// Display bid/offer book format
			fmt.Printf("┌─────┬───────────────┬────────────────┬───────────────┬──────────┐\n")
			fmt.Printf("│ Pos │ Price         │ Size           │ Time          │ Type     │\n")
			fmt.Printf("├─────┼───────────────┼────────────────┼───────────────┼──────────┤\n")

			for _, entry := range entries {
				pos := entry.Position
				if pos == "" {
					pos = "-"
				}
				fmt.Printf("│ %-3s │ %-13s │ %-14s │ %-13s │ %-8s │\n",
					pos, entry.Price, entry.Size, entry.Time, typeName)
			}
			fmt.Printf("└─────┴───────────────┴────────────────┴───────────────┴──────────┘\n")

		} else if entryType == constants.MdEntryTypeTrade {
			// Display trade format
			fmt.Printf("┌─────┬───────────────┬────────────────┬───────────────┬───────────┐\n")
			fmt.Printf("│ #   │ Price         │ Size           │ Time          │ Aggressor │\n")
			fmt.Printf("├─────┼───────────────┼────────────────┼───────────────┼───────────┤\n")

			for i, entry := range entries {
				aggressor := entry.Aggressor
				if aggressor == "" {
					aggressor = "-"
				}
				fmt.Printf("│ %-3d │ %-13s │ %-14s │ %-13s │ %-9s │\n",
					i+1, entry.Price, entry.Size, entry.Time, aggressor)
			}
			fmt.Printf("└─────┴───────────────┴────────────────┴───────────────┴───────────┘\n")

		} else {
			// Display OHLC/Volume format (no size column - not relevant for these data types)
			fmt.Printf("┌─────┬───────────────┬───────────────┐\n")
			fmt.Printf("│ #   │ Value         │ Time          │\n")
			fmt.Printf("├─────┼───────────────┼───────────────┤\n")

			for i, entry := range entries {
				value := entry.Price
				if entryType == constants.MdEntryTypeVolume {
					value = entry.Size // For volume, the "size" field contains the volume
				}

				fmt.Printf("│ %-3d │ %-13s │ %-13s │\n",
					i+1, value, entry.Time)
			}
			fmt.Printf("└─────┴───────────────┴───────────────┘\n")
		}
	}

	log.Printf("\nTotal Entries Displayed: %d", len(trades))
}

func (a *FixApp) displayIncrementalTrades(trades []Trade) {
	for _, trade := range trades {
		a.TradeStore.DisplayRealtimeUpdate(trade)
	}
	// Add visual separator after each batch of incremental updates
	if len(trades) > 0 {
		log.Println("────────────────────────────────────────────────")
	}
}

func (a *FixApp) getSubscriptionTypeDesc(subType string) string {
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

func getMarketDataTypeName(msgType string) string {
	switch msgType {
	case constants.MsgTypeMarketDataSnapshot:
		return "Snapshot"
	case constants.MsgTypeMarketDataIncremental:
		return "Incremental"
	default:
		return "Unknown"
	}
}

func getMdEntryTypeName(entryType string) string {
	switch entryType {
	case constants.MdEntryTypeBid:
		return "Bid"
	case constants.MdEntryTypeOffer:
		return "Offer"
	case constants.MdEntryTypeTrade:
		return "Trade"
	case constants.MdEntryTypeOpen:
		return "Open"
	case constants.MdEntryTypeClose:
		return "Close"
	case constants.MdEntryTypeHigh:
		return "High"
	case constants.MdEntryTypeLow:
		return "Low"
	case constants.MdEntryTypeVolume:
		return "Volume"
	default:
		return entryType
	}
}

func getAggressorSideDesc(side string) string {
	switch side {
	case "1":
		return "Buy"
	case "2":
		return "Sell"
	default:
		return side
	}
}

func (a *FixApp) displayMarketDataReject(mdReqId, rejReason, reasonDesc, text string) {
	log.Printf("Market Data Request REJECTED")
	log.Printf("   MdReqId: %s", mdReqId)
	log.Printf("   Reason: %s (%s)", rejReason, reasonDesc)
	if text != "" {
		log.Printf("   Text: %s", text)
	}
}

func (a *FixApp) displayMarketDataRejectHelp(rejReason string) {
	switch rejReason {
	case "0":
		log.Printf("Try a different symbol format (e.g., BTCUSD vs BTC-USD)")
	case "3":
		log.Printf("Check if your account has market data permissions")
	case "5":
		log.Printf("Try MarketDepth=0 (full depth) or MarketDepth=1 (top of book)")
	case "8":
		log.Printf("Try different MdEntryType: 0=Bids, 1=Offers, 2=Trades")
	}
}

func (a *FixApp) displayConnectionSuccess() {
	fmt.Print("Connected! Market data connection established.\n\n")
}

func (a *FixApp) displayMarketDataReceived(msgType, symbol, mdReqId, noMdEntries, seqNum string) {
	log.Printf("Market Data %s for %s (ReqId: %s, Entries: %s, Seq: %s)",
		getMarketDataTypeName(msgType), symbol, mdReqId, noMdEntries, seqNum)
}

// --- Order Entry Display Functions ---

func (a *FixApp) displayExecutionReport(er *ExecutionReport) {
	execTypeDesc := getExecTypeDesc(er.ExecType)
	ordStatusDesc := getOrdStatusDesc(er.OrdStatus)
	sideDesc := getSideDesc(er.Side)

	log.Printf("Execution Report: %s", execTypeDesc)
	log.Printf("   ClOrdID: %s, OrderID: %s", er.ClOrdID, er.OrderID)
	log.Printf("   Symbol: %s, Side: %s, Status: %s", er.Symbol, sideDesc, ordStatusDesc)

	if er.OrderQty != "" {
		log.Printf("   Qty: %s, Filled: %s, Leaves: %s", er.OrderQty, er.CumQty, er.LeavesQty)
	}
	if er.Price != "" {
		log.Printf("   Price: %s", er.Price)
	}
	if er.AvgPx != "" && er.AvgPx != "0" {
		log.Printf("   AvgPx: %s", er.AvgPx)
	}
	if er.LastPx != "" && er.LastShares != "" {
		log.Printf("   Last Fill: %s @ %s", er.LastShares, er.LastPx)
	}
	if er.Commission != "" && er.Commission != "0" {
		log.Printf("   Commission: %s", er.Commission)
	}
	if er.OrdRejReason != "" {
		log.Printf("   Reject Reason: %s (%s)", er.OrdRejReason, getOrdRejReasonDesc(er.OrdRejReason))
	}
	if er.Text != "" {
		log.Printf("   Text: %s", er.Text)
	}
}

func (a *FixApp) displayOrderCancelReject(reject *OrderCancelReject) {
	responseToDesc := "Cancel"
	if reject.CxlRejResponseTo == constants.CxlRejResponseToReplace {
		responseToDesc = "Replace"
	}

	log.Printf("Order %s Rejected", responseToDesc)
	log.Printf("   ClOrdID: %s, OrigClOrdID: %s", reject.ClOrdID, reject.OrigClOrdID)
	log.Printf("   OrderID: %s, Status: %s", reject.OrderID, getOrdStatusDesc(reject.OrdStatus))
	if reject.CxlRejReason != "" {
		log.Printf("   Reason: %s", reject.CxlRejReason)
	}
	if reject.Text != "" {
		log.Printf("   Text: %s", reject.Text)
	}
}

func (a *FixApp) displayQuote(quote *Quote) {
	log.Printf("Quote Received")
	log.Printf("   QuoteID: %s, QuoteReqID: %s", quote.QuoteID, quote.QuoteReqID)
	log.Printf("   Symbol: %s, Account: %s", quote.Symbol, quote.Account)

	if quote.BidPx != "" {
		log.Printf("   Bid: %s @ %s", quote.BidSize, quote.BidPx)
	}
	if quote.OfferPx != "" {
		log.Printf("   Offer: %s @ %s", quote.OfferSize, quote.OfferPx)
	}
	if !quote.ValidUntilTime.IsZero() {
		log.Printf("   Valid Until: %s", quote.ValidUntilTime.Format("15:04:05.000"))
	}
}

func (a *FixApp) displayQuoteAck(ack *QuoteAck) {
	log.Printf("Quote Request Rejected")
	log.Printf("   QuoteReqID: %s, Symbol: %s", ack.QuoteReqID, ack.Symbol)
	log.Printf("   Reason: %s (%s)", ack.QuoteRejectReason, getQuoteRejectReasonDesc(ack.QuoteRejectReason))
	if ack.Text != "" {
		log.Printf("   Text: %s", ack.Text)
	}
}

func (a *FixApp) displaySessionReject(reject *SessionReject) {
	log.Printf("Session Reject (Message Rejected)")
	log.Printf("   RefSeqNum: %s, RefMsgType: %s", reject.RefSeqNum, reject.RefMsgType)
	if reject.RefTagID != "" {
		log.Printf("   RefTagID: %s", reject.RefTagID)
	}
	if reject.SessionRejectReason != "" {
		log.Printf("   Reason: %s (%s)", reject.SessionRejectReason, getSessionRejectReasonDesc(reject.SessionRejectReason))
	}
	if reject.Text != "" {
		log.Printf("   Text: %s", reject.Text)
	}
}

func (a *FixApp) displayBusinessReject(reject *BusinessReject) {
	log.Printf("Business Message Reject")
	log.Printf("   RefSeqNum: %s, RefMsgType: %s", reject.RefSeqNum, reject.RefMsgType)
	log.Printf("   Reason: %s (%s)", reject.BusinessRejectReason, getBusinessRejectReasonDesc(reject.BusinessRejectReason))
	if reject.Text != "" {
		log.Printf("   Text: %s", reject.Text)
	}
}

// --- Order Entry Helper Functions ---

func getExecTypeDesc(execType string) string {
	switch execType {
	case constants.ExecTypeNew:
		return "New Order"
	case constants.ExecTypePartialFill:
		return "Partial Fill"
	case constants.ExecTypeFilled:
		return "Filled"
	case constants.ExecTypeDone:
		return "Done"
	case constants.ExecTypeCanceled:
		return "Canceled"
	case constants.ExecTypePendingCancel:
		return "Pending Cancel"
	case constants.ExecTypeStopped:
		return "Stopped"
	case constants.ExecTypeRejected:
		return "Rejected"
	case constants.ExecTypePendingNew:
		return "Pending New"
	case constants.ExecTypeExpired:
		return "Expired"
	case constants.ExecTypeRestated:
		return "Restated"
	case constants.ExecTypeOrderStatus:
		return "Order Status"
	default:
		return execType
	}
}

func getOrdStatusDesc(status string) string {
	switch status {
	case constants.OrdStatusNew:
		return "New"
	case constants.OrdStatusPartiallyFilled:
		return "Partially Filled"
	case constants.OrdStatusFilled:
		return "Filled"
	case constants.OrdStatusDoneForDay:
		return "Done for Day"
	case constants.OrdStatusCanceled:
		return "Canceled"
	case constants.OrdStatusReplaced:
		return "Replaced"
	case constants.OrdStatusPendingCancel:
		return "Pending Cancel"
	case constants.OrdStatusStopped:
		return "Stopped"
	case constants.OrdStatusRejected:
		return "Rejected"
	case constants.OrdStatusSuspended:
		return "Suspended"
	case constants.OrdStatusPendingNew:
		return "Pending New"
	case constants.OrdStatusCalculated:
		return "Calculated"
	case constants.OrdStatusExpired:
		return "Expired"
	case constants.OrdStatusAcceptedBidding:
		return "Accepted for Bidding"
	case constants.OrdStatusPendingReplace:
		return "Pending Replace"
	default:
		return status
	}
}

func getSideDesc(side string) string {
	switch side {
	case constants.SideBuy:
		return "Buy"
	case constants.SideSell:
		return "Sell"
	default:
		return side
	}
}

func getOrdRejReasonDesc(reason string) string {
	switch reason {
	case constants.OrdRejReasonBrokerOption:
		return "Broker Option"
	case constants.OrdRejReasonUnknownSymbol:
		return "Unknown Symbol"
	case constants.OrdRejReasonExchangeClosed:
		return "Exchange Closed"
	case constants.OrdRejReasonExceedsLimit:
		return "Exceeds Limit"
	case constants.OrdRejReasonTooLate:
		return "Too Late"
	case constants.OrdRejReasonUnknownOrder:
		return "Unknown Order"
	case constants.OrdRejReasonDuplicateOrder:
		return "Duplicate Order"
	case constants.OrdRejReasonOther:
		return "Other"
	default:
		return reason
	}
}

func getQuoteRejectReasonDesc(reason string) string {
	switch reason {
	case constants.QuoteRejectReasonUnknownSymbol:
		return "Unknown Symbol"
	case constants.QuoteRejectReasonExchangeClosed:
		return "Exchange Closed"
	case constants.QuoteRejectReasonExceedsLimit:
		return "Exceeds Limit"
	case constants.QuoteRejectReasonDuplicate:
		return "Duplicate Quote"
	case constants.QuoteRejectReasonInvalidPrice:
		return "Invalid Price"
	case constants.QuoteRejectReasonOther:
		return "Other"
	default:
		return reason
	}
}

func getSessionRejectReasonDesc(reason string) string {
	switch reason {
	case constants.SessionRejectReasonInvalidTag:
		return "Invalid Tag"
	case constants.SessionRejectReasonRequiredTagMissing:
		return "Required Tag Missing"
	case constants.SessionRejectReasonTagNotDefined:
		return "Tag Not Defined"
	case constants.SessionRejectReasonUndefinedTag:
		return "Undefined Tag"
	case constants.SessionRejectReasonTagWithoutValue:
		return "Tag Without Value"
	case constants.SessionRejectReasonValueOutOfRange:
		return "Value Out of Range"
	case constants.SessionRejectReasonIncorrectDataFormat:
		return "Incorrect Data Format"
	case constants.SessionRejectReasonDecryptionProblem:
		return "Decryption Problem"
	case constants.SessionRejectReasonSignatureProblem:
		return "Signature Problem"
	case constants.SessionRejectReasonCompIDProblem:
		return "CompID Problem"
	case constants.SessionRejectReasonSendingTimeAccuracy:
		return "Sending Time Accuracy"
	case constants.SessionRejectReasonInvalidMsgType:
		return "Invalid Msg Type"
	default:
		return reason
	}
}

func getBusinessRejectReasonDesc(reason string) string {
	switch reason {
	case constants.BusinessRejectReasonOther:
		return "Other"
	case constants.BusinessRejectReasonUnknownID:
		return "Unknown ID"
	case constants.BusinessRejectReasonUnknownSecurity:
		return "Unknown Security"
	case constants.BusinessRejectReasonUnsupportedMsgType:
		return "Unsupported Message Type"
	case constants.BusinessRejectReasonApplicationNotAvail:
		return "Application Not Available"
	case constants.BusinessRejectReasonCondRequiredMissing:
		return "Conditionally Required Field Missing"
	case constants.BusinessRejectReasonNotAuthorized:
		return "Not Authorized"
	default:
		return reason
	}
}
