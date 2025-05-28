package repositories

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"trading/models" // Importar los modelos
)

// TradeRepository handles database operations for Orders, Trades, and BotState.
type TradeRepository struct {
	db *sql.DB
}

// NewTradeRepository creates and returns a new TradeRepository.
func NewTradeRepository(db *sql.DB) *TradeRepository {
	return &TradeRepository{db: db}
}

// --- Order Operations ---

// CreateOrder inserts a new Order into the database.
func (r *TradeRepository) CreateOrder(ctx context.Context, order *models.Order) error {
	query := `
		INSERT INTO orders (binance_id, symbol, type, price, quantity, quote_qty, status, is_test, placed_at, last_updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING id;
	`
	err := r.db.QueryRowContext(
		ctx,
		query,
		order.BinanceID,
		order.Symbol,
		order.Type,
		order.Price,
		order.Quantity,
		order.QuoteQty,
		order.Status,
		order.IsTest,
		order.PlacedAt,
		order.LastUpdatedAt,
	).Scan(&order.ID) // Populate the internal ID back into the struct

	if err != nil {
		return fmt.Errorf("failed to create order in DB: %w", err)
	}
	return nil
}

// UpdateOrder updates an existing Order in the database.
func (r *TradeRepository) UpdateOrder(ctx context.Context, order *models.Order) error {
	query := `
		UPDATE orders
		SET status = $1, executed_at = $2, last_updated_at = $3
		WHERE binance_id = $4;
	`
	res, err := r.db.ExecContext(
		ctx,
		query,
		order.Status,
		order.ExecutedAt, // Will be NULL if not executed
		order.LastUpdatedAt,
		order.BinanceID,
	)
	if err != nil {
		return fmt.Errorf("failed to update order %d in DB: %w", order.BinanceID, err)
	}
	rowsAffected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected for order update %d: %w", order.BinanceID, err)
	}
	if rowsAffected == 0 {
		return fmt.Errorf("order with binance_id %d not found for update", order.BinanceID)
	}
	return nil
}

// GetOrderByBinanceID fetches an Order by its BinanceID.
func (r *TradeRepository) GetOrderByBinanceID(ctx context.Context, binanceID int64) (*models.Order, error) {
	order := &models.Order{}
	query := `
		SELECT id, binance_id, symbol, type, price, quantity, quote_qty, status, is_test, placed_at, executed_at, last_updated_at
		FROM orders
		WHERE binance_id = $1;
	`
	// Use sql.NullTime for nullable fields
	var executedAt sql.NullTime

	err := r.db.QueryRowContext(ctx, query, binanceID).Scan(
		&order.ID,
		&order.BinanceID,
		&order.Symbol,
		&order.Type,
		&order.Price,
		&order.Quantity,
		&order.QuoteQty,
		&order.Status,
		&order.IsTest,
		&order.PlacedAt,
		&executedAt,
		&order.LastUpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("order with binance_id %d not found", binanceID)
		}
		return nil, fmt.Errorf("failed to get order by binance_id %d: %w", binanceID, err)
	}

	if executedAt.Valid {
		order.ExecutedAt = &executedAt.Time
	}

	return order, nil
}

// --- Trade Operations ---

// CreateTrade inserts a new Trade into the database.
func (r *TradeRepository) CreateTrade(ctx context.Context, trade *models.Trade) error {
	query := `
		INSERT INTO trades (buy_order_id, sell_order_id, symbol, buy_price, buy_quantity, sell_price_target, actual_sell_price, status, profit_usdt, opened_at, closed_at, last_status_update)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		RETURNING id;
	`
	var sellOrderID sql.NullInt64
	if trade.SellOrderID != nil {
		sellOrderID.Int64 = *trade.SellOrderID
		sellOrderID.Valid = true
	}

	var actualSellPrice sql.NullFloat64
	if trade.ActualSellPrice != nil {
		actualSellPrice.Float64 = *trade.ActualSellPrice
		actualSellPrice.Valid = true
	}

	var profitUSDT sql.NullFloat64
	if trade.ProfitUSDT != nil {
		profitUSDT.Float64 = *trade.ProfitUSDT
		profitUSDT.Valid = true
	}

	var closedAt sql.NullTime
	if trade.ClosedAt != nil {
		closedAt.Time = *trade.ClosedAt
		closedAt.Valid = true
	}

	err := r.db.QueryRowContext(
		ctx,
		query,
		trade.BuyOrderID,
		sellOrderID,
		trade.Symbol,
		trade.BuyPrice,
		trade.BuyQuantity,
		trade.SellPriceTarget,
		actualSellPrice,
		trade.Status,
		profitUSDT,
		trade.OpenedAt,
		closedAt,
		trade.LastStatusUpdate,
	).Scan(&trade.ID)

	if err != nil {
		return fmt.Errorf("failed to create trade in DB: %w", err)
	}
	return nil
}

// UpdateTrade updates an existing Trade in the database.
func (r *TradeRepository) UpdateTrade(ctx context.Context, trade *models.Trade) error {
	query := `
		UPDATE trades
		SET sell_order_id = $1, actual_sell_price = $2, status = $3, profit_usdt = $4, closed_at = $5, last_status_update = $6
		WHERE id = $7;
	`
	var sellOrderID sql.NullInt64
	if trade.SellOrderID != nil {
		sellOrderID.Int64 = *trade.SellOrderID
		sellOrderID.Valid = true
	}

	var actualSellPrice sql.NullFloat64
	if trade.ActualSellPrice != nil {
		actualSellPrice.Float64 = *trade.ActualSellPrice
		actualSellPrice.Valid = true
	}

	var profitUSDT sql.NullFloat64
	if trade.ProfitUSDT != nil {
		profitUSDT.Float64 = *trade.ProfitUSDT
		profitUSDT.Valid = true
	}

	var closedAt sql.NullTime
	if trade.ClosedAt != nil {
		closedAt.Time = *trade.ClosedAt
		closedAt.Valid = true
	}

	res, err := r.db.ExecContext(
		ctx,
		query,
		sellOrderID,
		actualSellPrice,
		trade.Status,
		profitUSDT,
		closedAt,
		trade.LastStatusUpdate,
		trade.ID,
	)
	if err != nil {
		return fmt.Errorf("failed to update trade %d in DB: %w", trade.ID, err)
	}
	rowsAffected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected for trade update %d: %w", trade.ID, err)
	}
	if rowsAffected == 0 {
		return fmt.Errorf("trade with ID %d not found for update", trade.ID)
	}
	return nil
}

// GetTradesByStatus fetches all Trades with a specific status.
func (r *TradeRepository) GetTradesByStatus(ctx context.Context, status models.TradeStatus) ([]*models.Trade, error) {
	query := `
		SELECT id, buy_order_id, sell_order_id, symbol, buy_price, buy_quantity, sell_price_target, actual_sell_price, status, profit_usdt, opened_at, closed_at, last_status_update
		FROM trades
		WHERE status = $1;
	`
	rows, err := r.db.QueryContext(ctx, query, status)
	if err != nil {
		return nil, fmt.Errorf("failed to get trades by status '%s': %w", status, err)
	}
	defer rows.Close()

	var trades []*models.Trade
	for rows.Next() {
		trade := &models.Trade{}
		var sellOrderID sql.NullInt64
		var actualSellPrice sql.NullFloat64
		var profitUSDT sql.NullFloat64
		var closedAt sql.NullTime

		err := rows.Scan(
			&trade.ID,
			&trade.BuyOrderID,
			&sellOrderID,
			&trade.Symbol,
			&trade.BuyPrice,
			&trade.BuyQuantity,
			&trade.SellPriceTarget,
			&actualSellPrice,
			&trade.Status,
			&profitUSDT,
			&trade.OpenedAt,
			&closedAt,
			&trade.LastStatusUpdate,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan trade row: %w", err)
		}

		if sellOrderID.Valid {
			trade.SellOrderID = &sellOrderID.Int64
		}
		if actualSellPrice.Valid {
			trade.ActualSellPrice = &actualSellPrice.Float64
		}
		if profitUSDT.Valid {
			trade.ProfitUSDT = &profitUSDT.Float64
		}
		if closedAt.Valid {
			trade.ClosedAt = &closedAt.Time
		}

		trades = append(trades, trade)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating over trade rows: %w", err)
	}

	return trades, nil
}

// --- BotState Operations ---

// GetBotState fetches the single bot state row from the database.
func (r *TradeRepository) GetBotState(ctx context.Context) (*models.BotState, error) {
	state := &models.BotState{}
	query := `
		SELECT
			id,
			initial_usdt_investment,
			current_usdt_balance,
			current_btc_balance,
			total_usdt_invested,
			total_usdt_profit,
			initial_buy_orders_placed_count,
			last_initial_buy_order_placed_at,
			is_initial_buying_complete,
			last_bot_run_timestamp,
			created_at,
			updated_at
		FROM bot_states
		WHERE id = 1; -- We assume only one row with ID = 1
	`
	var lastInitialBuyOrderPlacedAt sql.NullTime

	err := r.db.QueryRowContext(ctx, query).Scan(
		&state.ID,
		&state.InitialUSDTInvestment,
		&state.CurrentUSDTBalance,
		&state.CurrentBTCBalance,
		&state.TotalUSDTInvested,
		&state.TotalUSDTProfit,
		&state.InitialBuyOrdersPlacedCount,
		&lastInitialBuyOrderPlacedAt,
		&state.IsInitialBuyingComplete,
		&state.LastBotRunTimestamp,
		&state.CreatedAt,
		&state.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("bot state not found (ID=1). Run migrations to initialize it")
		}
		return nil, fmt.Errorf("failed to get bot state: %w", err)
	}

	if lastInitialBuyOrderPlacedAt.Valid {
		state.LastInitialBuyOrderPlacedAt = &lastInitialBuyOrderPlacedAt.Time
	}

	return state, nil
}

// SaveBotState updates the existing bot state row in the database.
// This function performs an UPSERT (UPDATE if exists, INSERT if not),
// leveraging the `ON CONFLICT` clause in PostgreSQL for the bot_states table
// (which is already in the migration).
func (r *TradeRepository) SaveBotState(ctx context.Context, state *models.BotState) error {
	query := `
		INSERT INTO bot_states (
			id,
			initial_usdt_investment,
			current_usdt_balance,
			current_btc_balance,
			total_usdt_invested,
			total_usdt_profit,
			initial_buy_orders_placed_count,
			last_initial_buy_order_placed_at,
			is_initial_buying_complete,
			last_bot_run_timestamp,
			created_at,
			updated_at
		) VALUES (
			1, $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11
		)
		ON CONFLICT (id) DO UPDATE SET
			initial_usdt_investment = EXCLUDED.initial_usdt_investment,
			current_usdt_balance = EXCLUDED.current_usdt_balance,
			current_btc_balance = EXCLUDED.current_btc_balance,
			total_usdt_invested = EXCLUDED.total_usdt_invested,
			total_usdt_profit = EXCLUDED.total_usdt_profit,
			initial_buy_orders_placed_count = EXCLUDED.initial_buy_orders_placed_count,
			last_initial_buy_order_placed_at = EXCLUDED.last_initial_buy_order_placed_at,
			is_initial_buying_complete = EXCLUDED.is_initial_buying_complete,
			last_bot_run_timestamp = EXCLUDED.last_bot_run_timestamp,
			updated_at = EXCLUDED.updated_at;
	`
	var lastInitialBuyOrderPlacedAt sql.NullTime
	if state.LastInitialBuyOrderPlacedAt != nil {
		lastInitialBuyOrderPlacedAt.Time = *state.LastInitialBuyOrderPlacedAt
		lastInitialBuyOrderPlacedAt.Valid = true
	}

	// For the initial insert (if state.CreatedAt is zero), set it to NOW()
	// For updates, use the existing state.CreatedAt
	// However, the `ON CONFLICT` clause ensures `created_at` is only set once by the `INSERT`.
	// The `updated_at` should always be set to `time.Now()` before calling this.

	_, err := r.db.ExecContext(
		ctx,
		query,
		state.InitialUSDTInvestment,
		state.CurrentUSDTBalance,
		state.CurrentBTCBalance,
		state.TotalUSDTInvested,
		state.TotalUSDTProfit,
		state.InitialBuyOrdersPlacedCount,
		lastInitialBuyOrderPlacedAt,
		state.IsInitialBuyingComplete,
		state.LastBotRunTimestamp,
		state.CreatedAt, // Use the existing CreatedAt
		time.Now(),      // Always update UpdatedAt on save
	)
	if err != nil {
		return fmt.Errorf("failed to save bot state in DB: %w", err)
	}
	return nil
}
