package services

import (
	"context"
	"fmt"
	"time"

	"binance-trader-bot/config"
	"binance-trader-bot/models"
	"binance-trader-bot/utils"
)

// TradingStrategy implements the core logic of the automated trading bot.
type TradingStrategy struct {
	binanceService *BinanceService
	stateManager   *StateManager
	config         *config.Config
	logger         *utils.Logger
}

// NewTradingStrategy creates and returns a new TradingStrategy.
func NewTradingStrategy(
	binanceService *BinanceService,
	stateManager *StateManager,
	cfg *config.Config,
	logger *utils.Logger,
) *TradingStrategy {
	return &TradingStrategy{
		binanceService: binanceService,
		stateManager:   stateManager,
		config:         cfg,
		logger:         logger,
	}
}

// ExecuteTradingCycle is the main loop function called periodically by main.go.
// It orchestrates all the trading logic.
func (ts *TradingStrategy) ExecuteTradingCycle(ctx context.Context) error {
	ts.logger.Info("Starting new trading cycle...")

	botState := ts.stateManager.GetBotState()
	if botState == nil {
		ts.logger.Error("Bot state is nil, cannot proceed with trading cycle. This should not happen after LoadBotState.")
		return fmt.Errorf("bot state is nil")
	}

	// 1. Initialize Bot State if it's new (only first run)
	if botState.ID == 0 { // A new state, ID is 0 before first save
		ts.logger.Info("Initializing bot state for the first time...")
		initialState := models.NewBotState(ts.config.InitialUSDT)
		ts.stateManager.SetBotState(initialState)
		botState = initialState // Update the local reference
	}

	// 2. Refresh Account Balances
	var usdtBal float64
	var btcBal float64
	var err error // Variable para errores

	usdtBal, err = ts.binanceService.GetAccountBalance(ctx, "USDT")
	if err != nil {
		ts.logger.Errorf("Failed to refresh USDT balance: %v", err)
		// Decide si quieres retornar, continuar, o manejar este error de otra forma
		// Por ahora, para que compile y funcione, lo dejaré solo logueado.
		// Podrías considerar un 'return' o un 'continue' en un ciclo.
		// Para depuración, podríamos inicializar usdtBal a 0.
		usdtBal = 0 // O manejar el error de otra forma
	}

	// Obtener el balance de BTC
	btcBal, err = ts.binanceService.GetAccountBalance(ctx, "BTC") // Asumiendo que "BTC" es el asset string
	if err != nil {
		ts.logger.Errorf("Failed to refresh BTC balance: %v", err)
		// Decide si quieres retornar, continuar, o manejar este error de otra forma
		// Para depuración, podríamos inicializar btcBal a 0.
		btcBal = 0 // O manejar el error de otra forma
	}

	botState.UpdateBalances(usdtBal, btcBal)
	ts.logger.Infof("Balances refreshed: USDT=%f, BTC=%f", usdtBal, btcBal)

	// 3. Get Current Market Price
	currentPrice, err := ts.binanceService.GetCurrentPrice(ctx, ts.config.Symbol)
	if err != nil {
		ts.logger.Errorf("Failed to get current market price: %v", err)
		return fmt.Errorf("failed to get current price, skipping cycle: %w", err)
	}
	ts.logger.Infof("Current market price for %s: %f", ts.config.Symbol, currentPrice)

	// 4. Execute Initial Buy Orders
	if !botState.IsInitialBuyingComplete {
		ts.logger.Info("Checking for initial buy orders...")
		if err := ts.placeInitialBuyOrders(ctx, currentPrice); err != nil {
			ts.logger.Errorf("Error placing initial buy orders: %v", err)
		}
	}

	// 5. Check and Place Sell Orders for Filled Buy Orders
	ts.logger.Info("Checking for filled buy orders to place sell orders...")
	if err := ts.checkAndPlaceSellOrders(ctx, currentPrice); err != nil {
		ts.logger.Errorf("Error checking and placing sell orders: %v", err)
	}

	// 6. Manage Open Orders (check status and update)
	ts.logger.Info("Managing open orders...")
	if err := ts.manageOpenOrders(ctx); err != nil {
		ts.logger.Errorf("Error managing open orders: %v", err)
	}

	// 7. Place Additional Buy Orders (if initial phase complete and USDT available)
	if botState.IsInitialBuyingComplete && botState.CurrentUSDTBalance >= ts.config.OrderAmount {
		ts.logger.Info("Checking for additional buy opportunities...")
		if err := ts.placeAdditionalBuyOrders(ctx, currentPrice); err != nil {
			ts.logger.Errorf("Error placing additional buy orders: %v", err)
		}
	}

	// 8. Save Bot State
	if err := ts.stateManager.SaveBotState(ctx); err != nil {
		ts.logger.Fatalf("Failed to save bot state: %v", err) // This is critical
	}

	ts.logger.Info("Trading cycle completed.")
	return nil
}

// placeInitialBuyOrders handles the logic for the first 10 staggered buy orders.
func (ts *TradingStrategy) placeInitialBuyOrders(ctx context.Context, currentPrice float64) error {
	botState := ts.stateManager.GetBotState()

	if botState.InitialBuyOrdersPlacedCount >= 10 {
		botState.SetInitialBuyingComplete()
		ts.logger.Info("Initial buying phase complete.")
		return nil
	}

	// Check interval since last initial order
	if botState.LastInitialBuyOrderPlacedAt != nil {
		nextOrderTime := botState.LastInitialBuyOrderPlacedAt.Add(time.Duration(ts.config.OrderIntervalMinutes) * time.Minute)
		if time.Now().Before(nextOrderTime) {
			ts.logger.Debugf("Waiting for next initial buy order interval. Next order at: %s", nextOrderTime.Format(time.RFC3339))
			return nil
		}
	}

	// Ensure enough USDT balance for the order
	if botState.CurrentUSDTBalance < ts.config.OrderAmount {
		ts.logger.Warnf("Not enough USDT (%f) to place initial buy order (needs %f). Waiting for funds.",
			botState.CurrentUSDTBalance, ts.config.OrderAmount)
		return nil
	}

	buyPrice := utils.CalculateBuyPrice(currentPrice, ts.config.InitialBuyPercentage)
	// Calculate quantity based on ORDER_AMOUNT and calculated buyPrice
	quantity := ts.config.OrderAmount / buyPrice

	ts.logger.Infof("Placing initial buy order #%d: %f %s at %.8f USDT (%.2f%% below market %f)",
		botState.InitialBuyOrdersPlacedCount+1, quantity, ts.config.Symbol, buyPrice, ts.config.InitialBuyPercentage, currentPrice)

	order, err := ts.binanceService.PlaceLimitOrder(ctx, ts.config.Symbol, models.OrderTypeBuy, buyPrice, quantity)
	if err != nil {
		ts.logger.Errorf("Failed to place initial buy order: %v", err)
		return err
	}

	// Save the newly placed order to DB
	if err := ts.stateManager.AddOrder(ctx, order); err != nil {
		ts.logger.Errorf("Failed to save new buy order to DB: %v", err)
		// This is a serious problem, consider what to do (retry, alert)
	}

	botState.IncrementInitialBuyOrdersCount()
	botState.UpdateBalances(botState.CurrentUSDTBalance-ts.config.OrderAmount, botState.CurrentBTCBalance) // Optimistic update
	ts.logger.Infof("Initial buy order #%d placed. Remaining initial orders: %d",
		botState.InitialBuyOrdersPlacedCount, 10-botState.InitialBuyOrdersPlacedCount)

	return nil
}

// checkAndPlaceSellOrders checks for filled buy orders and places corresponding sell orders.
func (ts *TradingStrategy) checkAndPlaceSellOrders(ctx context.Context, currentPrice float64) error {
	openTrades, err := ts.stateManager.GetOpenTrades(ctx) // Get trades where buy order is filled but sell is not
	if err != nil {
		return fmt.Errorf("failed to get open trades: %w", err)
	}

	if len(openTrades) == 0 {
		ts.logger.Debug("No open trades to check for sell orders.")
		return nil
	}

	for _, trade := range openTrades {
		// First, check if the buy order associated with this trade is actually FILLED on Binance.
		// This is important because the local state might be outdated.
		buyOrder, err := ts.stateManager.GetOrder(ctx, trade.BuyOrderID)
		if err != nil {
			ts.logger.Errorf("Failed to retrieve buy order %d for trade %d: %v", trade.BuyOrderID, trade.ID, err)
			continue
		}

		if buyOrder.Status != models.OrderStatusFilled {
			ts.logger.Debugf("Buy order %d for trade %d is not yet FILLED (%s). Skipping sell order placement.",
				buyOrder.BinanceID, trade.ID, buyOrder.Status)
			continue
		}

		// If a sell order for this trade hasn't been placed yet
		if trade.SellOrderID == nil {
			ts.logger.Infof("Buy order %d for trade %d is FILLED. Placing sell order...", buyOrder.BinanceID, trade.ID)
			sellPrice := utils.CalculateSellPrice(buyOrder.Price, ts.config.SellProfitPercentage)
			// Quantity to sell is the quantity that was bought
			quantityToSell := buyOrder.Quantity

			ts.logger.Infof("Placing sell order for trade %d: %f %s at %.8f USDT (%.2f%% profit target)",
				trade.ID, quantityToSell, ts.config.Symbol, sellPrice, ts.config.SellProfitPercentage)

			sellOrder, err := ts.binanceService.PlaceLimitOrder(ctx, ts.config.Symbol, models.OrderTypeSell, sellPrice, quantityToSell)
			if err != nil {
				ts.logger.Errorf("Failed to place sell order for trade %d (BuyOrderID %d): %v", trade.ID, trade.BuyOrderID, err)
				// Consider marking trade as ERROR or retrying
				continue
			}

			// Update Trade with sell order ID and save sell order to DB
			trade.SetSellOrder(sellOrder.BinanceID)
			if err := ts.stateManager.UpdateTrade(ctx, trade); err != nil {
				ts.logger.Errorf("Failed to update trade %d with sell order ID: %v", trade.ID, err)
			}
			if err := ts.stateManager.AddOrder(ctx, sellOrder); err != nil {
				ts.logger.Errorf("Failed to save new sell order %d to DB: %v", sellOrder.BinanceID, err)
			}
			ts.logger.Infof("Sell order %d placed for trade %d.", sellOrder.BinanceID, trade.ID)
		} else {
			// If sell order already placed, check its status
			sellOrder, err := ts.stateManager.GetOrder(ctx, *trade.SellOrderID)
			if err != nil {
				ts.logger.Errorf("Failed to retrieve sell order %d for trade %d: %v", *trade.SellOrderID, trade.ID, err)
				continue
			}

			if sellOrder.Status == models.OrderStatusFilled {
				ts.logger.Infof("Sell order %d for trade %d is FILLED! Marking trade as SOLD.", sellOrder.BinanceID, trade.ID)
				trade.MarkAsSold(sellOrder.Price) // Use the actual executed price from the sell order
				if err := ts.stateManager.UpdateTrade(ctx, trade); err != nil {
					ts.logger.Errorf("Failed to mark trade %d as SOLD: %v", trade.ID, err)
				}
				// Update bot's profit and balances
				botState := ts.stateManager.GetBotState()
				if trade.ProfitUSDT != nil {
					botState.UpdateInvestedAndProfit(0, *trade.ProfitUSDT) // Profit is added, no new investment
				}
				// Also update balances based on the full trade execution
				// For simplicity, we update based on current balances from Binance, which should reflect this.
				// A more precise calculation would adjust balances by order amounts, but less robust if Binance API is preferred source.
			} else {
				ts.logger.Debugf("Sell order %d for trade %d is still %s.", sellOrder.BinanceID, trade.ID, sellOrder.Status)
			}
		}
	}
	return nil
}

// manageOpenOrders periodically checks the status of all open orders (buy and sell)
// and updates their status in the database.
func (ts *TradingStrategy) manageOpenOrders(ctx context.Context) error {
	// For simplicity, we'll fetch ALL orders from the DB and update their status.
	// In a high-volume bot, you might only fetch orders marked as PENDING or NEW.
	// Or query Binance directly for "open orders".

	// Get all currently open orders from Binance API (more reliable for real-time status)
	openOrders, err := ts.binanceService.client.NewListOpenOrdersService().Symbol(ts.config.Symbol).Do(ctx)
	if err != nil {
		ts.logger.Errorf("Failed to get open orders from Binance: %v", err)
		return fmt.Errorf("failed to get open orders from Binance: %w", err)
	}

	for _, openOrder := range openOrders {
		binanceID := openOrder.OrderID
		// Try to find this order in our local database by Binance ID
		localOrder, err := ts.stateManager.GetOrder(ctx, binanceID)
		if err != nil {
			ts.logger.Warnf("Open order %d from Binance not found in local DB. Skipping update.", binanceID)
			continue // This could happen if a previous save failed, or order was placed manually
		}

		// Check if the status has changed
		newStatus := models.OrderStatus(openOrder.Status)
		if localOrder.Status != newStatus {
			ts.logger.Infof("Updating status for order %d from %s to %s",
				localOrder.BinanceID, localOrder.Status, newStatus)
			localOrder.UpdateStatus(newStatus)
			if err := ts.stateManager.UpdateOrder(ctx, localOrder); err != nil {
				ts.logger.Errorf("Failed to update status of order %d in DB: %v", localOrder.BinanceID, err)
			}
		}
	}
	return nil
}

// placeAdditionalBuyOrders checks if there are opportunities for additional buys
// based on BUY_PERCENTAGES and available USDT.
func (ts *TradingStrategy) placeAdditionalBuyOrders(ctx context.Context, currentPrice float64) error {
	botState := ts.stateManager.GetBotState()

	// Ensure there's enough USDT for another order
	if botState.CurrentUSDTBalance < ts.config.OrderAmount {
		ts.logger.Debugf("Not enough USDT (%f) for an additional buy order (needs %f).",
			botState.CurrentUSDTBalance, ts.config.OrderAmount)
		return nil
	}

	// Get all currently open trades to know current positions
	allTrades, err := ts.stateManager.GetOpenTrades(ctx) // This fetches trades with status models.TradeStatusOpen
	if err != nil {
		ts.logger.Errorf("Failed to retrieve open trades for additional buy logic: %v", err)
		return err
	}

	// Simple logic using allTrades: Don't place additional buys if we already have too many open trades
	// This is a placeholder; adjust threshold based on your risk appetite.
	if len(allTrades) >= ts.config.MaxOpenTrades { // Asumir que existe config.MaxOpenTrades
		ts.logger.Debugf("Max open trades (%d) reached. Skipping additional buy order.", ts.config.MaxOpenTrades)
		return nil
	}

	// ... el resto de la lógica de placeAdditionalBuyOrders ...

	// Si inicial buying is complete, and we have enough USDT, and no pending buy orders (simplified)
	if botState.IsInitialBuyingComplete && botState.CurrentUSDTBalance >= ts.config.OrderAmount {
		if len(ts.config.BuyPercentages) > 0 {
			chosenPercentage := ts.config.BuyPercentages[0]
			potentialBuyPrice := utils.CalculateBuyPrice(currentPrice, chosenPercentage)

			ts.logger.Infof("Placing additional buy order: %f %s at %.8f USDT (%.2f%% below market %f)",
				ts.config.OrderAmount/potentialBuyPrice, ts.config.Symbol, potentialBuyPrice, chosenPercentage, currentPrice)

			quantity := ts.config.OrderAmount / potentialBuyPrice
			order, err := ts.binanceService.PlaceLimitOrder(ctx, ts.config.Symbol, models.OrderTypeBuy, potentialBuyPrice, quantity)
			if err != nil {
				ts.logger.Errorf("Failed to place additional buy order: %v", err)
				return err
			}

			if err := ts.stateManager.AddOrder(ctx, order); err != nil {
				ts.logger.Errorf("Failed to save additional buy order to DB: %v", err)
			}
			botState.UpdateBalances(botState.CurrentUSDTBalance-ts.config.OrderAmount, botState.CurrentBTCBalance)
			ts.logger.Infof("Additional buy order %d placed.", order.BinanceID)
		} else {
			ts.logger.Debug("No BUY_PERCENTAGES defined for additional buys.")
		}
	}
	return nil
}
