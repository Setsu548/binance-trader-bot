package models

import (
	"time"
)

// OrderType represents the type of a trading order (BUY or SELL).
type OrderType string

const (
	OrderTypeBuy  OrderType = "BUY"
	OrderTypeSell OrderType = "SELL"
)

// OrderStatus represents the current status of a trading order on Binance.
type OrderStatus string

const (
	OrderStatusNew             OrderStatus = "NEW"
	OrderStatusPartiallyFilled OrderStatus = "PARTIALLY_FILLED"
	OrderStatusFilled          OrderStatus = "FILLED"
	OrderStatusCanceled        OrderStatus = "CANCELED"
	OrderStatusPendingCancel   OrderStatus = "PENDING_CANCEL"
	OrderStatusRejected        OrderStatus = "REJECTED"
	OrderStatusExpired         OrderStatus = "EXPIRED"
)

// Order represents a single trading order placed on Binance.
// This model will be used both for orders managed by the bot internally
// and potentially for persisting to the database if needed for detailed logging or recovery.
type Order struct {
	ID        int64       `json:"id" db:"id"`                 // Internal ID for database (if stored)
	BinanceID int64       `json:"binance_id" db:"binance_id"` // Binance's order ID
	Symbol    string      `json:"symbol" db:"symbol"`         // Trading pair, e.g., "BTCUSDT"
	Type      OrderType   `json:"type" db:"type"`             // BUY or SELL
	Price     float64     `json:"price" db:"price"`           // Price at which the order was placed
	Quantity  float64     `json:"quantity" db:"quantity"`     // Quantity of the base asset (e.g., BTC)
	QuoteQty  float64     `json:"quote_qty" db:"quote_qty"`   // Quantity of the quote asset (e.g., USDT)
	Status    OrderStatus `json:"status" db:"status"`         // Current status of the order (NEW, FILLED, etc.)
	IsTest    bool        `json:"is_test" db:"is_test"`       // True if placed on testnet

	// Timestamps
	PlacedAt      time.Time  `json:"placed_at" db:"placed_at"`               // When the order was initially placed by the bot
	ExecutedAt    *time.Time `json:"executed_at,omitempty" db:"executed_at"` // When the order was fully or partially filled
	LastUpdatedAt time.Time  `json:"last_updated_at" db:"last_updated_at"`   // When the order's status was last updated by bot
}

// NewOrder creates a new Order instance with initial values.
// This is a helper constructor function.
func NewOrder(
	binanceID int64,
	symbol string,
	orderType OrderType,
	price float64,
	quantity float64,
	quoteQty float64,
	status OrderStatus,
	isTest bool,
) *Order {
	now := time.Now()
	return &Order{
		BinanceID:     binanceID,
		Symbol:        symbol,
		Type:          orderType,
		Price:         price,
		Quantity:      quantity,
		QuoteQty:      quoteQty,
		Status:        status,
		IsTest:        isTest,
		PlacedAt:      now,
		LastUpdatedAt: now,
	}
}

// UpdateStatus updates the order's status and last updated timestamp.
func (o *Order) UpdateStatus(newStatus OrderStatus) {
	o.Status = newStatus
	o.LastUpdatedAt = time.Now()
	if newStatus == OrderStatusFilled || newStatus == OrderStatusPartiallyFilled {
		now := time.Now()
		o.ExecutedAt = &now // Set executed time if order is filled
	} else if newStatus == OrderStatusCanceled || newStatus == OrderStatusRejected || newStatus == OrderStatusExpired {
		o.ExecutedAt = nil // Reset executed time if cancelled/rejected
	}
}
