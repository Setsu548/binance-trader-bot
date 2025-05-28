package models

import "time"

// TradeStatus represents the overall status of a trade (buy + sell).
type TradeStatus string

const (
	TradeStatusOpen     TradeStatus = "OPEN"     // Buy order filled, sell order not yet placed or not filled
	TradeStatusSold     TradeStatus = "SOLD"     // Buy and Sell orders filled
	TradeStatusCanceled TradeStatus = "CANCELED" // Buy order canceled or failed
	TradeStatusError    TradeStatus = "ERROR"    // Trade encountered an irrecoverable error
)

// Trade represents a complete trading operation: a successful buy order
// and its corresponding anticipated or executed sell order.
// This is the core unit the bot tracks for profit/loss.
type Trade struct {
	ID               int64       `json:"id" db:"id"`
	BuyOrderID       int64       `json:"buy_order_id" db:"buy_order_id"`                     // Foreign key to the executed BUY order
	SellOrderID      *int64      `json:"sell_order_id,omitempty" db:"sell_order_id"`         // Foreign key to the associated SELL order (can be null initially)
	Symbol           string      `json:"symbol" db:"symbol"`                                 // Trading pair, e.g., "BTCUSDT"
	BuyPrice         float64     `json:"buy_price" db:"buy_price"`                           // Actual execution price of the buy
	BuyQuantity      float64     `json:"buy_quantity" db:"buy_quantity"`                     // Quantity of base asset bought
	SellPriceTarget  float64     `json:"sell_price_target" db:"sell_price_target"`           // Target price for the sell order
	ActualSellPrice  *float64    `json:"actual_sell_price,omitempty" db:"actual_sell_price"` // Actual execution price of the sell
	Status           TradeStatus `json:"status" db:"status"`                                 // Current status of this trade
	ProfitUSDT       *float64    `json:"profit_usdt,omitempty" db:"profit_usdt"`             // Calculated profit in USDT
	OpenedAt         time.Time   `json:"opened_at" db:"opened_at"`                           // When the buy order was filled
	ClosedAt         *time.Time  `json:"closed_at,omitempty" db:"closed_at"`                 // When the sell order was filled or trade completed
	LastStatusUpdate time.Time   `json:"last_status_update" db:"last_status_update"`         // Timestamp of last status change
}

// NewTrade creates a new Trade instance when a buy order is filled.
func NewTrade(buyOrderID int64, symbol string, buyPrice, buyQuantity, sellPriceTarget float64) *Trade {
	now := time.Now()
	return &Trade{
		BuyOrderID:       buyOrderID,
		Symbol:           symbol,
		BuyPrice:         buyPrice,
		BuyQuantity:      buyQuantity,
		SellPriceTarget:  sellPriceTarget,
		Status:           TradeStatusOpen,
		OpenedAt:         now,
		LastStatusUpdate: now,
	}
}

// MarkAsSold updates the trade status to SOLD and calculates profit.
func (t *Trade) MarkAsSold(actualSellPrice float64) {
	t.Status = TradeStatusSold
	t.ActualSellPrice = &actualSellPrice
	profit := (actualSellPrice - t.BuyPrice) * t.BuyQuantity
	t.ProfitUSDT = &profit
	now := time.Now()
	t.ClosedAt = &now
	t.LastStatusUpdate = now
}

// MarkAsCanceled updates the trade status to CANCELED.
func (t *Trade) MarkAsCanceled() {
	t.Status = TradeStatusCanceled
	now := time.Now()
	t.ClosedAt = &now
	t.LastStatusUpdate = now
}

// SetSellOrder sets the ID for the associated sell order.
func (t *Trade) SetSellOrder(sellOrderID int64) {
	t.SellOrderID = &sellOrderID
}
