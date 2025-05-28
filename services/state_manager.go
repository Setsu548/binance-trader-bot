package services

import (
	"context"
	"fmt"

	"binance-trader-bot/models" // Importar los modelos
	"binance-trader-bot/repositories"
	"binance-trader-bot/utils" // Importar el logger
)

// StateManager handles the persistence and retrieval of the bot's state.
type StateManager struct {
	tradeRepo *repositories.TradeRepository // We'll manage trades and bot state via this
	logger    *utils.Logger
	botState  *models.BotState // In-memory representation of the bot's state
}

// NewStateManager creates and returns a new StateManager.
func NewStateManager(tradeRepo *repositories.TradeRepository, logger *utils.Logger) *StateManager {
	return &StateManager{
		tradeRepo: tradeRepo,
		logger:    logger,
	}
}

// LoadBotState attempts to load the bot's state from the database.
// If no state is found, it initializes a new one.
func (sm *StateManager) LoadBotState(ctx context.Context) error {
	sm.logger.Info("Attempting to load bot state from database...")

	state, err := sm.tradeRepo.GetBotState(ctx) // Assuming GetBotState exists in TradeRepository
	if err != nil {
		sm.logger.Warnf("No existing bot state found or error retrieving: %v. Initializing new state.", err)
		// This initial state should reflect the config.InitialUSDT
		// We'll set this when NewBotState is called by trading_strategy based on config.
		// For now, setting it to a default placeholder.
		sm.botState = models.NewBotState(0.0) // Will be properly initialized by trading_strategy
		return nil                            // No error if state simply doesn't exist, it will be created later
	}

	sm.botState = state
	sm.logger.Infof("Bot state loaded successfully (InitialUSDTInvestment: %f, InitialBuyOrdersPlaced: %d, IsInitialBuyingComplete: %t)",
		sm.botState.InitialUSDTInvestment, sm.botState.InitialBuyOrdersPlacedCount, sm.botState.IsInitialBuyingComplete)
	return nil
}

// SaveBotState saves the current in-memory bot state to the database.
func (sm *StateManager) SaveBotState(ctx context.Context) error {
	if sm.botState == nil {
		return fmt.Errorf("cannot save nil bot state")
	}
	sm.botState.UpdateLastBotRunTimestamp() // Update timestamp before saving

	sm.logger.Debug("Saving bot state to database...")
	err := sm.tradeRepo.SaveBotState(ctx, sm.botState) // Assuming SaveBotState exists in TradeRepository
	if err != nil {
		return fmt.Errorf("failed to save bot state: %w", err)
	}
	sm.logger.Debug("Bot state saved.")
	return nil
}

// GetBotState returns the current in-memory bot state.
func (sm *StateManager) GetBotState() *models.BotState {
	return sm.botState
}

// SetBotState allows external components (like TradingStrategy) to set the initial state.
func (sm *StateManager) SetBotState(state *models.BotState) {
	sm.botState = state
}

// AddOrder adds a new order to the database.
func (sm *StateManager) AddOrder(ctx context.Context, order *models.Order) error {
	return sm.tradeRepo.CreateOrder(ctx, order) // Assuming CreateOrder exists
}

// UpdateOrder updates an existing order in the database.
func (sm *StateManager) UpdateOrder(ctx context.Context, order *models.Order) error {
	return sm.tradeRepo.UpdateOrder(ctx, order) // Assuming UpdateOrder exists
}

// GetOrder fetches an order by its internal ID or Binance ID.
func (sm *StateManager) GetOrder(ctx context.Context, binanceID int64) (*models.Order, error) {
	return sm.tradeRepo.GetOrderByBinanceID(ctx, binanceID) // Assuming GetOrderByBinanceID exists
}

// AddTrade adds a new trade to the database.
func (sm *StateManager) AddTrade(ctx context.Context, trade *models.Trade) error {
	return sm.tradeRepo.CreateTrade(ctx, trade) // Assuming CreateTrade exists
}

// UpdateTrade updates an existing trade in the database.
func (sm *StateManager) UpdateTrade(ctx context.Context, trade *models.Trade) error {
	return sm.tradeRepo.UpdateTrade(ctx, trade) // Assuming UpdateTrade exists
}

// GetOpenTrades fetches all trades that are currently in 'OPEN' status.
func (sm *StateManager) GetOpenTrades(ctx context.Context) ([]*models.Trade, error) {
	return sm.tradeRepo.GetTradesByStatus(ctx, models.TradeStatusOpen) // Assuming GetTradesByStatus exists
}
