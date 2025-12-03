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
	"strings"
	"time"

	"prime-fix-md-go/builder"
	"prime-fix-md-go/constants"
	"prime-fix-md-go/utils"

	"github.com/chzyer/readline"
	"github.com/quickfixgo/quickfix"
)

func Repl(app *FixApp) {
	// Setup readline with command completion
	completer := readline.NewPrefixCompleter(
		// Market data commands
		readline.PcItem("md",
			readline.PcItem("BTC-USD",
				readline.PcItem("--snapshot", readline.PcItem("--trades"), readline.PcItem("--depth")),
				readline.PcItem("--subscribe", readline.PcItem("--trades"), readline.PcItem("--depth")),
			),
			readline.PcItem("ETH-USD",
				readline.PcItem("--snapshot", readline.PcItem("--trades"), readline.PcItem("--depth")),
				readline.PcItem("--subscribe", readline.PcItem("--trades"), readline.PcItem("--depth")),
			),
		),
		readline.PcItem("unsubscribe", readline.PcItem("BTC-USD"), readline.PcItem("ETH-USD")),

		// Order entry commands
		readline.PcItem("order",
			readline.PcItem("buy", readline.PcItem("BTC-USD"), readline.PcItem("ETH-USD")),
			readline.PcItem("sell", readline.PcItem("BTC-USD"), readline.PcItem("ETH-USD")),
		),
		readline.PcItem("cancel"),
		readline.PcItem("replace"),
		readline.PcItem("ordstatus"),
		readline.PcItem("rfq",
			readline.PcItem("buy", readline.PcItem("BTC-USD"), readline.PcItem("ETH-USD")),
			readline.PcItem("sell", readline.PcItem("BTC-USD"), readline.PcItem("ETH-USD")),
		),
		readline.PcItem("accept"),
		readline.PcItem("orders"),
		readline.PcItem("quotes"),

		// General commands
		readline.PcItem("status"),
		readline.PcItem("help"),
		readline.PcItem("version"),
		readline.PcItem("exit"),
	)

	rl, err := readline.NewEx(&readline.Config{
		Prompt:          "FIX-MD> ",
		HistoryFile:     "/tmp/fixmd_history",
		AutoComplete:    completer,
		InterruptPrompt: "^C",
		EOFPrompt:       "exit",
	})
	if err != nil {
		log.Printf("Failed to create readline: %v", err)
		return
	}
	defer rl.Close()

	for {
		if app.ShouldExit() {
			fmt.Println("Exiting due to authentication failures. Please check your credentials.")
			return
		}

		line, err := rl.Readline()
		if err != nil {
			break
		}

		parts := strings.Fields(strings.TrimSpace(line))
		if len(parts) == 0 {
			continue
		}

		cmd := strings.ToLower(parts[0])
		switch cmd {
		// Market data commands
		case "md":
			app.handleDirectMdRequest(parts)
		case "unsubscribe":
			app.handleUnsubscribeRequest(parts)

		// Order entry commands
		case "order":
			app.handleOrderCommand(parts)
		case "cancel":
			app.handleCancelCommand(parts)
		case "replace":
			app.handleReplaceCommand(parts)
		case "ordstatus":
			app.handleOrdStatusCommand(parts)
		case "rfq":
			app.handleRfqCommand(parts)
		case "accept":
			app.handleAcceptQuoteCommand(parts)
		case "orders":
			app.handleOrdersCommand()
		case "quotes":
			app.handleQuotesCommand()

		// General commands
		case "status":
			if !app.handleStatusRequest() {
				return
			}
		case "help":
			app.displayHelp()
		case "version":
			fmt.Println(utils.FullVersion())
		case "exit":
			return
		default:
			fmt.Println("Unknown command. Type 'help' for available commands.")
		}
	}
}

type MdRequestFlags struct {
	subscriptionType string
	marketDepth      string
	entryTypes       []string
}

func (a *FixApp) handleDirectMdRequest(parts []string) {
	if len(parts) < 2 {
		fmt.Print(`Usage: md <symbol1> [symbol2 symbol3 ...] [flags...]

Subscription Flags:
  --snapshot              - Snapshot only
  --subscribe             - Snapshot + live updates
  --unsubscribe           - Stop updates

Depth Flag:
  --depth N               - Market depth (0=full, 1=top, N=best N levels)
                            Automatically includes both bids and offers

Entry Type Flags:
  --trades                - Executed trades
  --o                     - Opening price
  --c                     - Closing price
  --h                     - High price
  --l                     - Low price
  --v                     - Trading volume

Examples:
  md BTC-USD --snapshot --trades
  md BTC-USD ETH-USD --snapshot --depth 1
  md BTC-USD ETH-USD SOL-USD --subscribe --depth 10
  md ETH-USD --snapshot --o --c --h --l --v
  md BTC-USD --unsubscribe
`)
		return
	}

	// Parse symbols and flags
	var symbols []string
	var flagStart int

	// Find where flags start (first argument starting with --)
	for i, part := range parts[1:] {
		if strings.HasPrefix(part, "--") {
			flagStart = i + 1 // offset since we skipped parts[0]
			break
		}
		symbols = append(symbols, strings.ToUpper(part))
	}

	// If no flags found, all remaining parts are symbols
	if flagStart == 0 {
		flagStart = len(parts)
	}

	// Parse flags from flagStart onwards
	var flagArgs []string
	if flagStart < len(parts) {
		flagArgs = parts[flagStart:]
	}

	flags := a.parseMdFlags(flagArgs)

	// Validate we have a subscription type
	if flags.subscriptionType == "" {
		fmt.Println("Error: Must specify subscription type (--snapshot, --subscribe, or --unsubscribe)")
		return
	}

	// For unsubscribe, we don't need depth or entry types
	if flags.subscriptionType == constants.SubscriptionRequestTypeUnsubscribe {
		for _, symbol := range symbols {
			a.sendUnsubscribeBySymbol(symbol)
		}
		return
	}

	// Default depth to full if not specified
	if flags.marketDepth == "" {
		flags.marketDepth = "0"
	}

	// Default entry types based on context
	if len(flags.entryTypes) == 0 {
		// If depth is specified, default to bids and offers (order book data)
		if flags.marketDepth != "" {
			flags.entryTypes = []string{constants.MdEntryTypeBid, constants.MdEntryTypeOffer}
		} else {
			// Otherwise default to trades
			flags.entryTypes = []string{constants.MdEntryTypeTrade}
		}
	}

	// Determine description
	description := "Snapshot"
	if flags.subscriptionType == constants.SubscriptionRequestTypeSubscribe {
		description = "Live Subscription"
	}

	a.sendMarketDataRequestWithOptions(symbols, flags.subscriptionType, flags.marketDepth, flags.entryTypes, description)
}

func (a *FixApp) parseMdFlags(args []string) MdRequestFlags {
	flags := MdRequestFlags{
		entryTypes: []string{},
	}

	for i := 0; i < len(args); i++ {
		arg := args[i]

		switch arg {
		// Subscription type flags
		case "--snapshot":
			flags.subscriptionType = constants.SubscriptionRequestTypeSnapshot
		case "--subscribe":
			flags.subscriptionType = constants.SubscriptionRequestTypeSubscribe
		case "--unsubscribe":
			flags.subscriptionType = constants.SubscriptionRequestTypeUnsubscribe

		// Depth flag (requires next argument)
		case "--depth":
			if i+1 < len(args) {
				i++
				flags.marketDepth = args[i]
			}

		case "--trades":
			flags.entryTypes = append(flags.entryTypes, constants.MdEntryTypeTrade)
		case "--o":
			flags.entryTypes = append(flags.entryTypes, constants.MdEntryTypeOpen)
		case "--c":
			flags.entryTypes = append(flags.entryTypes, constants.MdEntryTypeClose)
		case "--h":
			flags.entryTypes = append(flags.entryTypes, constants.MdEntryTypeHigh)
		case "--l":
			flags.entryTypes = append(flags.entryTypes, constants.MdEntryTypeLow)
		case "--v":
			flags.entryTypes = append(flags.entryTypes, constants.MdEntryTypeVolume)
		}
	}

	return flags
}

func (a *FixApp) handleUnsubscribeRequest(parts []string) {
	if len(parts) < 2 {
		fmt.Print(`Usage: unsubscribe <symbol|reqId>
Examples: 
  unsubscribe BTC-USD           - Cancel ALL BTC-USD subscriptions
  unsubscribe md_1234567890     - Cancel specific subscription by reqId
  unsubscribe --reqid md_123    - Cancel specific subscription (explicit)
`)
		return
	}

	// Handle --reqid flag for explicit reqId targeting
	if len(parts) >= 3 && parts[1] == "--reqid" {
		a.sendUnsubscribeByReqId(parts[2])
		return
	}

	input := parts[1]

	// Auto-detect: if input looks like reqId, treat as reqId; otherwise as symbol
	if strings.HasPrefix(input, "md_") {
		a.sendUnsubscribeByReqId(input)
	} else {
		symbol := strings.ToUpper(input)
		a.sendUnsubscribeBySymbol(symbol)
	}
}

func (a *FixApp) handleStatusRequest() bool {
	if a.ShouldExit() {
		fmt.Println("Exiting due to authentication failures. Please check your credentials.")
		return false
	}

	fmt.Printf("Session: %s ", a.SessionId)
	if a.SessionId.String() != "" {
		fmt.Println("(Connected)")
	} else {
		fmt.Println("(Disconnected)")
	}

	subscriptionsBySymbol := a.TradeStore.GetSubscriptionsBySymbol()
	if len(subscriptionsBySymbol) == 0 {
		fmt.Println("No active subscriptions")
		return true
	}

	fmt.Print(`
Active Subscriptions:
┌─────────────┬──────────────────┬─────────────┬─────────────┬──────────────┬──────────────────┐
│ Symbol      │ Type             │ Status      │ Updates     │ Last Update  │ ReqId            │
├─────────────┼──────────────────┼─────────────┼─────────────┼──────────────┼──────────────────┤
`)

	for symbol, subs := range subscriptionsBySymbol {
		for i, sub := range subs {
			status := "Active"
			if !sub.Active {
				status = "Inactive"
			}

			lastUpdate := "Never"
			if !sub.LastUpdate.IsZero() {
				lastUpdate = sub.LastUpdate.Format("15:04:05")
			}

			// Show symbol only on first line for multiple subscriptions
			displaySymbol := symbol
			if i > 0 {
				displaySymbol = ""
			}

			// Truncate reqId for display
			shortReqId := sub.MdReqId
			if len(shortReqId) > 16 {
				shortReqId = "..." + shortReqId[len(shortReqId)-13:]
			}

			fmt.Printf("│ %-11s │ %-16s │ %-11s │ %-11d │ %-12s │ %-16s │\n",
				displaySymbol, a.getSubscriptionTypeDesc(sub.SubscriptionType), status, sub.TotalUpdates, lastUpdate, shortReqId)
		}
	}

	fmt.Println("└─────────────┴──────────────────┴─────────────┴─────────────┴──────────────┴──────────────────┘")

	return true
}

// --- Order Entry Command Handlers ---

// handleOrderCommand processes new order requests.
// Usage: order <buy|sell> <symbol> <qty> [price] [--type <type>] [--tif <tif>] [--strategy <strategy>]
func (a *FixApp) handleOrderCommand(parts []string) {
	if len(parts) < 4 {
		fmt.Print(`Usage: order <buy|sell> <symbol> <qty> [price] [flags...]

Order Flags:
  --type <type>           - Order type: market, limit, stop, stoplimit (default: limit if price given)
  --tif <tif>             - Time in force: gtc, ioc, fok, gtd (default: gtc)
  --strategy <strategy>   - Target strategy: L (limit), M (market), T (TWAP), V (VWAP), SL (stop-limit)
  --stop <price>          - Stop price (for stop/stoplimit orders)
  --postonly              - Post-only order (maker only)
  --cash                  - Qty is in quote currency (cash order)

Examples:
  order buy BTC-USD 0.01 50000               - Limit buy 0.01 BTC at $50,000
  order sell ETH-USD 1.5 --type market       - Market sell 1.5 ETH
  order buy BTC-USD 0.1 --cash 5000          - Buy $5,000 worth of BTC (cash order)
  order sell BTC-USD 0.5 48000 --tif ioc     - IOC limit sell
  order buy ETH-USD 2 --strategy T           - TWAP buy 2 ETH
`)
		return
	}

	// Parse side
	side := strings.ToLower(parts[1])
	var sideCode string
	switch side {
	case "buy":
		sideCode = constants.SideBuy
	case "sell":
		sideCode = constants.SideSell
	default:
		fmt.Println("Error: Side must be 'buy' or 'sell'")
		return
	}

	symbol := strings.ToUpper(parts[2])
	qty := parts[3]

	// Parse optional flags
	var price, stopPx, ordType, tif, strategy string
	var isCashOrder, postOnly bool

	for i := 4; i < len(parts); i++ {
		switch parts[i] {
		case "--type":
			if i+1 < len(parts) {
				i++
				ordType = parseOrdType(parts[i])
			}
		case "--tif":
			if i+1 < len(parts) {
				i++
				tif = parseTif(parts[i])
			}
		case "--strategy":
			if i+1 < len(parts) {
				i++
				strategy = strings.ToUpper(parts[i])
			}
		case "--stop":
			if i+1 < len(parts) {
				i++
				stopPx = parts[i]
			}
		case "--postonly":
			postOnly = true
		case "--cash":
			isCashOrder = true
		default:
			// If it doesn't start with --, it might be the price
			if !strings.HasPrefix(parts[i], "--") && price == "" {
				price = parts[i]
			}
		}
	}

	// Default order type based on presence of price
	if ordType == "" {
		if price != "" {
			ordType = constants.OrdTypeLimit
		} else {
			ordType = constants.OrdTypeMarket
		}
	}

	// Default TIF
	if tif == "" {
		tif = constants.TimeInForceGTC
	}

	// Generate ClOrdID
	clOrdID := fmt.Sprintf("ord_%d", time.Now().UnixNano())

	params := builder.NewOrderParams{
		ClOrdID:        clOrdID,
		Account:        a.Config.PortfolioId,
		Symbol:         symbol,
		Side:           sideCode,
		OrdType:        ordType,
		TimeInForce:    tif,
		TargetStrategy: strategy,
	}

	// PostOnly uses ExecInst = "A" per Coinbase Prime FIX API
	if postOnly {
		params.ExecInst = constants.ExecInstPostOnly
	}

	if isCashOrder {
		params.CashOrderQty = qty
	} else {
		params.OrderQty = qty
	}

	if price != "" {
		params.Price = price
	}
	if stopPx != "" {
		params.StopPx = stopPx
	}

	// Build and send message
	msg := builder.BuildNewOrderSingle(params, a.Config.SenderCompId, a.Config.TargetCompId)

	if err := quickfix.SendToTarget(msg, a.SessionId); err != nil {
		log.Printf("Error sending order: %v", err)
		return
	}

	// Track order locally
	order := &Order{
		ClOrdID:        clOrdID,
		Symbol:         symbol,
		Side:           sideCode,
		OrdType:        ordType,
		OrderQty:       qty,
		Price:          price,
		TargetStrategy: strategy,
		TimeInForce:    tif,
		OrdStatus:      constants.OrdStatusPendingNew,
		Account:        a.Config.PortfolioId,
	}
	if isCashOrder {
		order.CashOrderQty = qty
		order.OrderQty = ""
	}
	a.OrderStore.AddOrder(order)

	log.Printf("Order submitted: %s %s %s @ %s (ClOrdID: %s)", side, qty, symbol, price, clOrdID)
}

// handleCancelCommand processes order cancel requests.
// Usage: cancel <clOrdId|orderId>
func (a *FixApp) handleCancelCommand(parts []string) {
	if len(parts) < 2 {
		fmt.Print(`Usage: cancel <clOrdId|orderId>

Examples:
  cancel ord_1234567890     - Cancel order by client order ID
  cancel 123-456-789        - Cancel order by exchange order ID
`)
		return
	}

	identifier := parts[1]

	// Try to find order by ClOrdID first, then by OrderID
	order := a.OrderStore.GetOrder(identifier)
	if order == nil {
		order = a.OrderStore.GetOrderByOrderID(identifier)
	}

	if order == nil {
		fmt.Printf("Order not found: %s\n", identifier)
		return
	}

	newClOrdID := fmt.Sprintf("cxl_%d", time.Now().UnixNano())

	params := builder.CancelOrderParams{
		ClOrdID:     newClOrdID,
		OrigClOrdID: order.ClOrdID,
		OrderID:     order.OrderID,
		Account:     a.Config.PortfolioId,
		Symbol:      order.Symbol,
		Side:        order.Side,
		OrderQty:    order.OrderQty,
	}

	msg := builder.BuildOrderCancelRequest(params, a.Config.SenderCompId, a.Config.TargetCompId)

	if err := quickfix.SendToTarget(msg, a.SessionId); err != nil {
		log.Printf("Error sending cancel: %v", err)
		return
	}

	log.Printf("Cancel request sent for order %s (new ClOrdID: %s)", order.ClOrdID, newClOrdID)
}

// handleReplaceCommand processes order cancel/replace requests.
// Usage: replace <clOrdId> [--qty <qty>] [--price <price>]
func (a *FixApp) handleReplaceCommand(parts []string) {
	if len(parts) < 2 {
		fmt.Print(`Usage: replace <clOrdId> [flags...]

Replace Flags:
  --qty <qty>       - New quantity
  --price <price>   - New price

Examples:
  replace ord_123 --price 51000           - Change price to 51000
  replace ord_123 --qty 0.02              - Change quantity to 0.02
  replace ord_123 --qty 0.02 --price 51000  - Change both
`)
		return
	}

	origClOrdID := parts[1]
	order := a.OrderStore.GetOrder(origClOrdID)
	if order == nil {
		fmt.Printf("Order not found: %s\n", origClOrdID)
		return
	}

	// Parse flags
	newQty := order.OrderQty
	newPrice := order.Price

	for i := 2; i < len(parts); i++ {
		switch parts[i] {
		case "--qty":
			if i+1 < len(parts) {
				i++
				newQty = parts[i]
			}
		case "--price":
			if i+1 < len(parts) {
				i++
				newPrice = parts[i]
			}
		}
	}

	newClOrdID := fmt.Sprintf("rep_%d", time.Now().UnixNano())

	params := builder.ReplaceOrderParams{
		ClOrdID:     newClOrdID,
		OrigClOrdID: origClOrdID,
		OrderID:     order.OrderID,
		Account:     a.Config.PortfolioId,
		Symbol:      order.Symbol,
		Side:        order.Side,
		OrdType:     order.OrdType,
		OrderQty:    newQty,
		Price:       newPrice,
	}

	msg := builder.BuildOrderCancelReplaceRequest(params, a.Config.SenderCompId, a.Config.TargetCompId)

	if err := quickfix.SendToTarget(msg, a.SessionId); err != nil {
		log.Printf("Error sending replace: %v", err)
		return
	}

	log.Printf("Replace request sent for order %s -> %s", origClOrdID, newClOrdID)
}

// handleOrdStatusCommand requests status for an order.
// Usage: ordstatus <clOrdId|orderId>
func (a *FixApp) handleOrdStatusCommand(parts []string) {
	if len(parts) < 2 {
		fmt.Print(`Usage: ordstatus <clOrdId|orderId>

Examples:
  ordstatus ord_1234567890  - Get status by client order ID
  ordstatus 123-456-789     - Get status by exchange order ID
`)
		return
	}

	identifier := parts[1]

	order := a.OrderStore.GetOrder(identifier)
	if order == nil {
		order = a.OrderStore.GetOrderByOrderID(identifier)
	}

	var clOrdID, orderID, symbol, side string
	if order != nil {
		clOrdID = order.ClOrdID
		orderID = order.OrderID
		symbol = order.Symbol
		side = order.Side
	} else {
		// If order not in store, use identifier as-is
		if strings.HasPrefix(identifier, "ord_") {
			clOrdID = identifier
		} else {
			orderID = identifier
		}
	}

	msg := builder.BuildOrderStatusRequest(orderID, clOrdID, symbol, side, a.Config.SenderCompId, a.Config.TargetCompId)

	if err := quickfix.SendToTarget(msg, a.SessionId); err != nil {
		log.Printf("Error sending order status request: %v", err)
		return
	}

	log.Printf("Order status request sent for %s", identifier)
}

// handleRfqCommand sends a quote request (RFQ).
// Usage: rfq <buy|sell> <symbol> <qty>
func (a *FixApp) handleRfqCommand(parts []string) {
	if len(parts) < 4 {
		fmt.Print(`Usage: rfq <buy|sell> <symbol> <qty>

Examples:
  rfq buy BTC-USD 1.0      - Request a quote to buy 1 BTC
  rfq sell ETH-USD 10      - Request a quote to sell 10 ETH
`)
		return
	}

	side := strings.ToLower(parts[1])
	var sideCode string
	switch side {
	case "buy":
		sideCode = constants.SideBuy
	case "sell":
		sideCode = constants.SideSell
	default:
		fmt.Println("Error: Side must be 'buy' or 'sell'")
		return
	}

	symbol := strings.ToUpper(parts[2])
	qty := parts[3]

	quoteReqID := fmt.Sprintf("rfq_%d", time.Now().UnixNano())

	params := builder.QuoteRequestParams{
		QuoteReqID: quoteReqID,
		Account:    a.Config.PortfolioId,
		Symbol:     symbol,
		Side:       sideCode,
		OrderQty:   qty,
	}

	msg := builder.BuildQuoteRequest(params, a.Config.SenderCompId, a.Config.TargetCompId)

	if err := quickfix.SendToTarget(msg, a.SessionId); err != nil {
		log.Printf("Error sending quote request: %v", err)
		return
	}

	log.Printf("Quote request sent: %s %s %s (QuoteReqID: %s)", side, qty, symbol, quoteReqID)
}

// handleAcceptQuoteCommand accepts a received quote.
// Usage: accept <quoteId|quoteReqId>
func (a *FixApp) handleAcceptQuoteCommand(parts []string) {
	if len(parts) < 2 {
		fmt.Print(`Usage: accept <quoteId|quoteReqId>

Examples:
  accept quote_123         - Accept quote by QuoteID
  accept rfq_1234567890    - Accept quote by QuoteReqID
`)
		return
	}

	identifier := parts[1]

	// Find quote
	quote := a.OrderStore.GetQuote(identifier)
	if quote == nil {
		quote = a.OrderStore.GetQuoteByQuoteID(identifier)
	}

	if quote == nil {
		fmt.Printf("Quote not found: %s\n", identifier)
		return
	}

	// Check if quote is still valid
	if !quote.ValidUntilTime.IsZero() && time.Now().After(quote.ValidUntilTime) {
		fmt.Println("Error: Quote has expired")
		return
	}

	// Determine side and price from quote
	var side, price, qty string
	if quote.BidPx != "" {
		// Quote is for a sell (bid = what they'll pay)
		side = constants.SideSell
		price = quote.BidPx
		qty = quote.BidSize
	} else {
		// Quote is for a buy (offer = what they're asking)
		side = constants.SideBuy
		price = quote.OfferPx
		qty = quote.OfferSize
	}

	clOrdID := fmt.Sprintf("acc_%d", time.Now().UnixNano())

	params := builder.AcceptQuoteParams{
		ClOrdID:  clOrdID,
		QuoteID:  quote.QuoteID,
		Account:  quote.Account,
		Symbol:   quote.Symbol,
		Side:     side,
		OrderQty: qty,
		Price:    price,
	}

	msg := builder.BuildAcceptQuote(params, a.Config.SenderCompId, a.Config.TargetCompId)

	if err := quickfix.SendToTarget(msg, a.SessionId); err != nil {
		log.Printf("Error accepting quote: %v", err)
		return
	}

	// Track as order
	order := &Order{
		ClOrdID:   clOrdID,
		Symbol:    quote.Symbol,
		Side:      side,
		OrdType:   constants.OrdTypePreviouslyQuoted,
		OrderQty:  qty,
		Price:     price,
		OrdStatus: constants.OrdStatusPendingNew,
		Account:   quote.Account,
	}
	a.OrderStore.AddOrder(order)

	log.Printf("Quote accepted: %s %s %s @ %s (ClOrdID: %s)", getSideDesc(side), qty, quote.Symbol, price, clOrdID)
}

// handleOrdersCommand lists all tracked orders.
func (a *FixApp) handleOrdersCommand() {
	orders := a.OrderStore.GetAllOrders()
	if len(orders) == 0 {
		fmt.Println("No orders tracked")
		return
	}

	fmt.Print(`
Orders:
┌──────────────────────┬─────────────┬──────┬───────────────┬───────────────┬───────────────┬─────────────┐
│ ClOrdID              │ Symbol      │ Side │ Qty           │ Price         │ Status        │ Filled      │
├──────────────────────┼─────────────┼──────┼───────────────┼───────────────┼───────────────┼─────────────┤
`)

	for _, order := range orders {
		clOrdID := order.ClOrdID
		if len(clOrdID) > 20 {
			clOrdID = clOrdID[:17] + "..."
		}

		qty := order.OrderQty
		if order.CashOrderQty != "" {
			qty = "$" + order.CashOrderQty
		}

		price := order.Price
		if price == "" {
			price = "MARKET"
		}

		filled := order.CumQty
		if filled == "" {
			filled = "0"
		}

		fmt.Printf("│ %-20s │ %-11s │ %-4s │ %-13s │ %-13s │ %-13s │ %-11s │\n",
			clOrdID,
			order.Symbol,
			getSideDesc(order.Side),
			qty,
			price,
			getOrdStatusDesc(order.OrdStatus),
			filled,
		)
	}

	fmt.Println("└──────────────────────┴─────────────┴──────┴───────────────┴───────────────┴───────────────┴─────────────┘")
}

// handleQuotesCommand lists all received quotes.
func (a *FixApp) handleQuotesCommand() {
	quotes := a.OrderStore.GetAllQuotes()
	if len(quotes) == 0 {
		fmt.Println("No quotes received")
		return
	}

	fmt.Print(`
Quotes:
┌──────────────────────┬─────────────┬───────────────┬───────────────┬───────────────┬──────────────┐
│ QuoteReqID           │ Symbol      │ Bid           │ Offer         │ Valid Until   │ QuoteID      │
├──────────────────────┼─────────────┼───────────────┼───────────────┼───────────────┼──────────────┤
`)

	for _, quote := range quotes {
		quoteReqID := quote.QuoteReqID
		if len(quoteReqID) > 20 {
			quoteReqID = quoteReqID[:17] + "..."
		}

		bid := "-"
		if quote.BidPx != "" {
			bid = fmt.Sprintf("%s@%s", quote.BidSize, quote.BidPx)
		}

		offer := "-"
		if quote.OfferPx != "" {
			offer = fmt.Sprintf("%s@%s", quote.OfferSize, quote.OfferPx)
		}

		validUntil := "-"
		if !quote.ValidUntilTime.IsZero() {
			if time.Now().After(quote.ValidUntilTime) {
				validUntil = "EXPIRED"
			} else {
				validUntil = quote.ValidUntilTime.Format("15:04:05")
			}
		}

		quoteID := quote.QuoteID
		if len(quoteID) > 12 {
			quoteID = quoteID[:9] + "..."
		}

		fmt.Printf("│ %-20s │ %-11s │ %-13s │ %-13s │ %-13s │ %-12s │\n",
			quoteReqID,
			quote.Symbol,
			bid,
			offer,
			validUntil,
			quoteID,
		)
	}

	fmt.Println("└──────────────────────┴─────────────┴───────────────┴───────────────┴───────────────┴──────────────┘")
}

// --- Order Entry Helper Functions ---

func parseOrdType(s string) string {
	switch strings.ToLower(s) {
	case "market", "m":
		return constants.OrdTypeMarket
	case "limit", "l":
		return constants.OrdTypeLimit
	case "stop", "s":
		return constants.OrdTypeStop
	case "stoplimit", "sl":
		return constants.OrdTypeStopLimit
	default:
		return constants.OrdTypeLimit
	}
}

func parseTif(s string) string {
	switch strings.ToLower(s) {
	case "gtc":
		return constants.TimeInForceGTC
	case "ioc":
		return constants.TimeInForceIOC
	case "fok":
		return constants.TimeInForceFOK
	case "gtd":
		return constants.TimeInForceGTD
	default:
		return constants.TimeInForceGTC
	}
}
