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

package database

import (
	"database/sql"
	"fmt"
	"log"

	_ "github.com/mattn/go-sqlite3"
)

// MarketDataDb provides SQLite storage for market data with prepared statements.
// Prepared statements are initialized once and reused for all batch operations,
// avoiding SQL parsing overhead on each insert.
type MarketDataDb struct {
	db *sql.DB

	// Prepared statements for batch operations - initialized lazily
	stmtTrade     *sql.Stmt
	stmtOrderBook *sql.Stmt
	stmtOHLCV     *sql.Stmt
}

func NewMarketDataDb(dbPath string) (*MarketDataDb, error) {
	db, err := sql.Open("sqlite3", dbPath+"?_journal_mode=WAL&_synchronous=NORMAL&_cache_size=1000")
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %v", err)
	}

	mdb := &MarketDataDb{db: db}
	if err := mdb.initSchema(); err != nil {
		_ = db.Close() // Cleanup on error - return value ignored
		return nil, fmt.Errorf("failed to initialize schema: %v", err)
	}

	// Prepare statements for batch operations - avoids SQL parsing on each insert
	if mdb.stmtTrade, err = db.Prepare(insertTradeQuery); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("failed to prepare trade statement: %v", err)
	}
	if mdb.stmtOrderBook, err = db.Prepare(insertOrderBookQuery); err != nil {
		_ = mdb.stmtTrade.Close()
		_ = db.Close()
		return nil, fmt.Errorf("failed to prepare order book statement: %v", err)
	}
	if mdb.stmtOHLCV, err = db.Prepare(insertOHLCVQuery); err != nil {
		_ = mdb.stmtTrade.Close()
		_ = mdb.stmtOrderBook.Close()
		_ = db.Close()
		return nil, fmt.Errorf("failed to prepare OHLCV statement: %v", err)
	}

	log.Printf("SQLite database initialized at %s", dbPath)
	return mdb, nil
}

func (mdb *MarketDataDb) Close() error {
	// Close prepared statements first - errors ignored as we're shutting down
	if mdb.stmtTrade != nil {
		_ = mdb.stmtTrade.Close()
	}
	if mdb.stmtOrderBook != nil {
		_ = mdb.stmtOrderBook.Close()
	}
	if mdb.stmtOHLCV != nil {
		_ = mdb.stmtOHLCV.Close()
	}
	return mdb.db.Close()
}

// Session management
func (mdb *MarketDataDb) CreateSession(sessionId, symbol, requestType, dataTypes, mdReqId string, depth *int) error {
	_, err := mdb.db.Exec(insertSessionQuery, sessionId, symbol, requestType, dataTypes, depth, mdReqId)
	return err
}

// Trade data storage
func (mdb *MarketDataDb) StoreTrade(symbol, price, size, aggressorSide, tradeTime string, seqNum int, mdReqId string, isSnapshot bool) error {
	_, err := mdb.db.Exec(insertTradeQuery, symbol, price, size, aggressorSide, tradeTime, seqNum, mdReqId, isSnapshot)
	return err
}

// Order book data storage
func (mdb *MarketDataDb) StoreOrderBookEntry(symbol, side, price, size string, position, seqNum int, mdReqId string, isSnapshot bool) error {
	_, err := mdb.db.Exec(insertOrderBookQuery, symbol, side, price, size, position, seqNum, mdReqId, isSnapshot)
	return err
}

// OHLCV data storage
func (mdb *MarketDataDb) StoreOHLCV(symbol, dataType, value, entryTime string, seqNum int, mdReqId string) error {
	_, err := mdb.db.Exec(insertOHLCVQuery, symbol, dataType, value, entryTime, seqNum, mdReqId)
	return err
}

// Batch operations for better performance
func (mdb *MarketDataDb) BeginTransaction() (*sql.Tx, error) {
	return mdb.db.Begin()
}

// StoreTradeBatch inserts a trade using the prepared statement within a transaction.
// Using tx.Stmt() binds the prepared statement to the transaction context.
func (mdb *MarketDataDb) StoreTradeBatch(tx *sql.Tx, symbol, price, size, aggressorSide, tradeTime string, seqNum int, mdReqId string, isSnapshot bool) error {
	_, err := tx.Stmt(mdb.stmtTrade).Exec(symbol, price, size, aggressorSide, tradeTime, seqNum, mdReqId, isSnapshot)
	return err
}

// StoreOrderBookBatch inserts an order book entry using the prepared statement.
func (mdb *MarketDataDb) StoreOrderBookBatch(tx *sql.Tx, symbol, side, price, size string, position, seqNum int, mdReqId string, isSnapshot bool) error {
	_, err := tx.Stmt(mdb.stmtOrderBook).Exec(symbol, side, price, size, position, seqNum, mdReqId, isSnapshot)
	return err
}

// StoreOhlcvBatch inserts an OHLCV entry using the prepared statement.
func (mdb *MarketDataDb) StoreOhlcvBatch(tx *sql.Tx, symbol, dataType, value, entryTime string, seqNum int, mdReqId string) error {
	_, err := tx.Stmt(mdb.stmtOHLCV).Exec(symbol, dataType, value, entryTime, seqNum, mdReqId)
	return err
}
