package services

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"trading/models" // Importar los modelos definidos
	"trading/utils"  // Importar el logger

	"github.com/adshao/go-binance/v2/spot" // Cliente de Binance para Spot trading
	"github.com/shopspring/decimal"        // Para manejar floats de forma precisa en cálculos financieros
)

// BinanceService provides an interface for interacting with the Binance API.
type BinanceService struct {
	client  *spot.Client
	testnet bool
	logger  *utils.Logger
}

// NewBinanceService creates and returns a new BinanceService.
func NewBinanceService(apiKey, secretKey string, useTestnet bool, logger *utils.Logger) *BinanceService {
	var client *spot.Client
	if useTestnet {
		client = spot.NewClient(apiKey, secretKey)
		client.BaseURL = "https://testnet.binance.vision" // Binance Testnet base URL for spot
	} else {
		client = spot.NewClient(apiKey, secretKey)
	}

	return &BinanceService{
		client:  client,
		testnet: useTestnet,
		logger:  logger,
	}
}

// GetCurrentPrice fetches the current market price for a given symbol.
func (s *BinanceService) GetCurrentPrice(ctx context.Context, symbol string) (float64, error) {
	s.logger.Debugf("Fetching current price for %s...", symbol)
	res, err := s.client.NewListPricesService().Symbol(symbol).Do(ctx)
	if err != nil {
		s.logger.Errorf("Failed to get current price for %s: %v", symbol, err)
		return 0, fmt.Errorf("failed to get current price: %w", err)
	}
	if len(res) == 0 {
		s.logger.Error("No price data returned for %s", symbol)
		return 0, fmt.Errorf("no price data returned for %s", symbol)
	}

	price, err := strconv.ParseFloat(res[0].Price, 64)
	if err != nil {
		s.logger.Errorf("Failed to parse price '%s': %v", res[0].Price, err)
		return 0, fmt.Errorf("failed to parse price: %w", err)
	}

	s.logger.Debugf("Current price for %s: %f", symbol, price)
	return price, nil
}

// PlaceLimitOrder places a new limit order (BUY or SELL) on Binance.
// Returns the Binance order ID and an error if any.
func (s *BinanceService) PlaceLimitOrder(
	ctx context.Context,
	symbol string,
	orderType models.OrderType,
	price float64,
	quantity float64,
) (*models.Order, error) {
	s.logger.Infof("Attempting to place %s limit order for %f %s at price %f", orderType, quantity, symbol, price)

	// Use decimal for precision in calculations to avoid floating point issues
	priceDec := decimal.NewFromFloat(price)
	quantityDec := decimal.NewFromFloat(quantity)

	// Get exchange info to determine precision rules
	exchangeInfo, err := s.client.NewExchangeInfoService().Symbol(symbol).Do(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get exchange info for %s: %w", symbol, err)
	}

	var pricePrecision, quantityPrecision int
	foundSymbol := false
	for _, symInfo := range exchangeInfo.Symbols {
		if symInfo.Symbol == symbol {
			foundSymbol = true
			for _, filter := range symInfo.Filters {
				if filter.FilterType == "PRICE_FILTER" {
					if tickSize, ok := filter.MetaData["tickSize"].(string); ok {
						pricePrecision = countDecimalPlaces(tickSize)
					}
				}
				if filter.FilterType == "LOT_SIZE" {
					if stepSize, ok := filter.MetaData["stepSize"].(string); ok {
						quantityPrecision = countDecimalPlaces(stepSize)
					}
				}
			}
			break
		}
	}
	if !foundSymbol {
		return nil, fmt.Errorf("symbol %s not found in exchange info", symbol)
	}

	// Format price and quantity according to Binance's precision rules
	formattedPrice := priceDec.Round(int32(pricePrecision)).String()
	formattedQuantity := quantityDec.Round(int32(quantityPrecision)).String()

	newOrderService := s.client.NewCreateOrderService().
		Symbol(symbol).
		Side(string(orderType)). // Convert models.OrderType to string
		Type(spot.OrderTypeLimit).
		TimeInForce(spot.TimeInForceGTC). // Good Till Cancelled
		Quantity(formattedQuantity).
		Price(formattedPrice)

	res, err := newOrderService.Do(ctx)
	if err != nil {
		s.logger.Errorf("Failed to place %s order for %s: %v", orderType, symbol, err)
		return nil, fmt.Errorf("failed to place order: %w", err)
	}

	orderID, err := strconv.ParseInt(fmt.Sprintf("%v", res.OrderID), 10, 64) // Convert OrderID to int64
	if err != nil {
		s.logger.Errorf("Failed to parse Binance OrderID %v: %v", res.OrderID, err)
		return nil, fmt.Errorf("failed to parse Binance OrderID: %w", err)
	}

	// Calculate quote quantity if it's a BUY order, for consistency in model
	quoteQty := 0.0
	if orderType == models.OrderTypeBuy {
		// Use the originally requested quote quantity for the model before Binance rounds
		quoteQty = quantity * price
	} else {
		// For SELL, it's the quantity sold * price
		quoteQty = quantity * price
	}

	// Convert Binance status string to models.OrderStatus
	binanceOrderStatus := models.OrderStatus(res.Status)

	s.logger.Infof("Successfully placed %s order for %s. Binance ID: %d, Status: %s, Price: %s, Qty: %s",
		orderType, symbol, orderID, binanceOrderStatus, formattedPrice, formattedQuantity)

	return models.NewOrder(
		orderID,
		symbol,
		orderType,
		price,    // Store the original requested price
		quantity, // Store the original requested quantity
		quoteQty,
		binanceOrderStatus,
		s.testnet,
	), nil
}

// GetOrderStatus fetches the current status of an order by its Binance ID.
func (s *BinanceService) GetOrderStatus(ctx context.Context, symbol string, binanceOrderID int64) (*models.Order, error) {
	s.logger.Debugf("Checking status for order ID %d for symbol %s...", binanceOrderID, symbol)

	order, err := s.client.NewGetOrderService().Symbol(symbol).OrderID(binanceOrderID).Do(ctx)
	if err != nil {
		s.logger.Errorf("Failed to get status for order ID %d (%s): %v", binanceOrderID, symbol, err)
		return nil, fmt.Errorf("failed to get order status: %w", err)
	}

	price, _ := strconv.ParseFloat(order.Price, 64)
	origQty, _ := strconv.ParseFloat(order.OrigQuantity, 64)
	executedQty, _ := strconv.ParseFloat(order.ExecutedQuantity, 64)
	cummulativeQuoteQty, _ := strconv.ParseFloat(order.CummulativeQuoteQty, 64)

	// If order is FILLED or PARTIALLY_FILLED, the actual executed price might differ slightly from limit price
	// For simplicity, we'll use the original price for now, but in a real bot,
	// you'd typically use `price` from the response for filled orders or weighted average.
	actualExecutedPrice := price
	if order.Status == string(models.OrderStatusFilled) || order.Status == string(models.OrderStatusPartiallyFilled) {
		if executedQty > 0 {
			// For filled orders, use average price if available or cummulativeQuoteQty / executedQty
			if cummulativeQuoteQty > 0 {
				actualExecutedPrice = cummulativeQuoteQty / executedQty
			}
		}
	}

	retrievedOrder := models.NewOrder(
		order.OrderID,
		order.Symbol,
		models.OrderType(order.Side),
		actualExecutedPrice, // Use executed price for filled/partially filled
		origQty,
		cummulativeQuoteQty, // This is the total quote asset spent/received
		models.OrderStatus(order.Status),
		s.testnet,
	)
	if order.UpdateTime != 0 {
		updateTime := time.Unix(order.UpdateTime/1000, (order.UpdateTime%1000)*int64(time.Millisecond))
		retrievedOrder.LastUpdatedAt = updateTime
		if order.Status == string(models.OrderStatusFilled) || order.Status == string(models.OrderStatusPartiallyFilled) {
			retrievedOrder.ExecutedAt = &updateTime
		}
	}

	s.logger.Debugf("Order ID %d status: %s, Price: %f, Original Qty: %f, Executed Qty: %f",
		binanceOrderID, retrievedOrder.Status, retrievedOrder.Price, retrievedOrder.Quantity, executedQty)

	return retrievedOrder, nil
}

// CancelOrder cancels an open order on Binance.
func (s *BinanceService) CancelOrder(ctx context.Context, symbol string, binanceOrderID int64) error {
	s.logger.Infof("Attempting to cancel order ID %d for symbol %s...", binanceOrderID, symbol)
	_, err := s.client.NewCancelOrderService().Symbol(symbol).OrderID(binanceOrderID).Do(ctx)
	if err != nil {
		s.logger.Errorf("Failed to cancel order ID %d (%s): %v", binanceOrderID, symbol, err)
		return fmt.Errorf("failed to cancel order: %w", err)
	}
	s.logger.Infof("Successfully cancelled order ID %d for symbol %s.", binanceOrderID, symbol)
	return nil
}

// GetAccountBalances fetches the current USDT and BTC balances.
func (s *BinanceService) GetAccountBalances(ctx context.Context, symbol string) (usdtBalance, btcBalance float64, err error) {
	s.logger.Debugf("Fetching account balances...")

	account, err := s.client.NewGetAccountService().Do(ctx)
	if err != nil {
		s.logger.Errorf("Failed to get account info: %v", err)
		return 0, 0, fmt.Errorf("failed to get account info: %w", err)
	}

	baseAsset := symbol[:len(symbol)-4]  // e.g., BTC from BTCUSDT
	quoteAsset := symbol[len(symbol)-4:] // e.g., USDT from BTCUSDT

	for _, asset := range account.Balances {
		if asset.Asset == quoteAsset {
			free, parseErr := strconv.ParseFloat(asset.Free, 64)
			if parseErr != nil {
				s.logger.Errorf("Failed to parse %s free balance '%s': %v", quoteAsset, asset.Free, parseErr)
				return 0, 0, fmt.Errorf("failed to parse %s balance: %w", quoteAsset, parseErr)
			}
			locked, parseErr := strconv.ParseFloat(asset.Locked, 64)
			if parseErr != nil {
				s.logger.Errorf("Failed to parse %s locked balance '%s': %v", quoteAsset, asset.Locked, parseErr)
				return 0, 0, fmt.Errorf("failed to parse %s balance: %w", quoteAsset, parseErr)
			}
			usdtBalance = free + locked // Total available balance
		} else if asset.Asset == baseAsset {
			free, parseErr := strconv.ParseFloat(asset.Free, 64)
			if parseErr != nil {
				s.logger.Errorf("Failed to parse %s free balance '%s': %v", baseAsset, asset.Free, parseErr)
				return 0, 0, fmt.Errorf("failed to parse %s balance: %w", baseAsset, parseErr)
			}
			locked, parseErr := strconv.ParseFloat(asset.Locked, 64)
			if parseErr != nil {
				s.logger.Errorf("Failed to parse %s locked balance '%s': %v", baseAsset, asset.Locked, parseErr)
				return 0, 0, fmt.Errorf("failed to parse %s balance: %w", baseAsset, parseErr)
			}
			btcBalance = free + locked // Total available balance
		}
	}

	s.logger.Debugf("Account Balances - USDT: %f, %s: %f", usdtBalance, baseAsset, btcBalance)
	return usdtBalance, btcBalance, nil
}

// Helper to count decimal places in a string representation of a float (e.g., "0.001" -> 3)
func countDecimalPlaces(s string) int {
	if !strings.Contains(s, ".") {
		return 0
	}
	return len(s) - strings.Index(s, ".") - 1
}
