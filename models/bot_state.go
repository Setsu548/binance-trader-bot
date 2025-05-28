package models

import "time"

// BotState represents the overall persistent state of the bot.
// This struct holds critical information that needs to be saved
// and loaded to ensure the bot can resume operations correctly
// after a restart, and keep track of its progress.
type BotState struct {
	ID                          int64      `json:"id" db:"id"`
	InitialUSDTInvestment       float64    `json:"initial_usdt_investment" db:"initial_usdt_investment"`
	CurrentUSDTBalance          float64    `json:"current_usdt_balance" db:"current_usdt_balance"`
	CurrentBTCBalance           float64    `json:"current_btc_balance" db:"current_btc_balance"` // Track actual BTC balance
	TotalUSDTInvested           float64    `json:"total_usdt_invested" db:"total_usdt_invested"`
	TotalUSDTProfit             float64    `json:"total_usdt_profit" db:"total_usdt_profit"`
	InitialBuyOrdersPlacedCount int        `json:"initial_buy_orders_placed_count" db:"initial_buy_orders_placed_count"`
	LastInitialBuyOrderPlacedAt *time.Time `json:"last_initial_buy_order_placed_at,omitempty" db:"last_initial_buy_order_placed_at"`
	IsInitialBuyingComplete     bool       `json:"is_initial_buying_complete" db:"is_initial_buying_complete"`
	LastBotRunTimestamp         time.Time  `json:"last_bot_run_timestamp" db:"last_bot_run_timestamp"`
	// You might want to store specific order IDs that are currently open
	// This would likely be a slice of IDs or a more complex structure,
	// potentially requiring a separate table or JSONB column if using PostgreSQL.
	// For simplicity, we might query open orders directly from Binance via service,
	// but for persistent tracking, storing their IDs here or in a dedicated table is better.
	// For now, let's keep it simple and assume `Trade` objects track open orders.

	// Added to ensure we only have one state entry and track the update time
	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

// NewBotState creates a new BotState instance with initial values.
func NewBotState(initialUSDT float64) *BotState {
	now := time.Now()
	return &BotState{
		InitialUSDTInvestment:       initialUSDT,
		CurrentUSDTBalance:          initialUSDT, // Start with initial investment as current balance
		CurrentBTCBalance:           0.0,
		TotalUSDTInvested:           0.0,
		TotalUSDTProfit:             0.0,
		InitialBuyOrdersPlacedCount: 0,
		IsInitialBuyingComplete:     false,
		LastBotRunTimestamp:         now,
		CreatedAt:                   now,
		UpdatedAt:                   now,
	}
}

// UpdateBalances updates the bot's USDT and BTC balances.
func (bs *BotState) UpdateBalances(usdt, btc float64) {
	bs.CurrentUSDTBalance = usdt
	bs.CurrentBTCBalance = btc
	bs.UpdatedAt = time.Now()
}

// IncrementInitialBuyOrdersCount increments the counter and updates timestamp.
func (bs *BotState) IncrementInitialBuyOrdersCount() {
	bs.InitialBuyOrdersPlacedCount++
	now := time.Now()
	bs.LastInitialBuyOrderPlacedAt = &now
	if bs.InitialBuyOrdersPlacedCount >= 10 { // Assuming 10 initial orders
		bs.IsInitialBuyingComplete = true
	}
	bs.UpdatedAt = now
}

// UpdateInvestedAndProfit updates the total invested and profit.
func (bs *BotState) UpdateInvestedAndProfit(usdtInvested, usdtProfit float64) {
	bs.TotalUSDTInvested += usdtInvested
	bs.TotalUSDTProfit += usdtProfit
	bs.UpdatedAt = time.Now()
}

// SetInitialBuyingComplete marks the initial buying phase as complete.
func (bs *BotState) SetInitialBuyingComplete() {
	bs.IsInitialBuyingComplete = true
	bs.UpdatedAt = time.Now()
}

// UpdateLastBotRunTimestamp updates the timestamp of the last bot run.
func (bs *BotState) UpdateLastBotRunTimestamp() {
	bs.LastBotRunTimestamp = time.Now()
	bs.UpdatedAt = time.Now()
}
