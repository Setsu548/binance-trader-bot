package services

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"binance-trader-bot/models" // Importar los modelos definidos
	"binance-trader-bot/utils"  // Importar el logger

	"github.com/adshao/go-binance/v2" // Cliente de Binance para Spot trading
	"github.com/shopspring/decimal"   // Para manejar floats de forma precisa en cálculos financieros
)

// BinanceService provides an interface for interacting with the Binance API.
type BinanceService struct {
	client  *binance.Client // Changed to *binance.Client
	testnet bool
	logger  *utils.Logger
}

func NewBinanceService(apiKey, secretKey string, useTestnet bool, logger *utils.Logger) *BinanceService {
	var client *binance.Client
	if useTestnet {
		client = binance.NewClient(apiKey, secretKey)
		client.BaseURL = "https://testnet.binance.vision" // Set testnet URL
	} else {
		client = binance.NewClient(apiKey, secretKey)
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
		s.logger.Errorf("No price data returned for %s", symbol)
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

// PlaceLimitOrder places a limit order on Binance.
func (s *BinanceService) PlaceLimitOrder(ctx context.Context, symbol string, orderType models.OrderType, price float64, quantity float64) (*models.Order, error) {
	s.logger.Infof("Attempting to place %s limit order for %f %s at price %f", orderType, quantity, symbol, price)

	// Convert price and quantity to Decimal for precision
	priceDec := decimal.NewFromFloat(price)
	quantityDec := decimal.NewFromFloat(quantity)

	// Retrieve exchange info to get lot size and price filter rules for the symbol
	exchangeInfo, err := s.client.NewExchangeInfoService().Symbol(symbol).Do(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get exchange info for %s: %w", symbol, err)
	}
	if len(exchangeInfo.Symbols) == 0 {
		return nil, fmt.Errorf("exchange info not found for symbol %s", symbol)
	}
	symbolInfo := exchangeInfo.Symbols[0]

	// Apply Filters
	var tickSize, stepSize string
	for _, filter := range symbolInfo.Filters {

		filterType, ok := filter["filterType"].(string)
		if !ok {
			s.logger.Warnf("Filter missing 'filterType' field or not string type: %v", filter)
			continue
		}

		switch filterType {
		case "PRICE_FILTER":
			if ts, ok := filter["tickSize"].(string); ok {
				tickSize = ts
			} else {
				s.logger.Warnf("PRICE_FILTER missing 'tickSize' field or not string type: %v", filter)
			}
		case "LOT_SIZE":
			if ss, ok := filter["stepSize"].(string); ok {
				stepSize = ss
			} else {
				s.logger.Warnf("LOT_SIZE filter missing 'stepSize' field or not string type: %v", filter)
			}
		}
	}

	if tickSize == "" || stepSize == "" {
		return nil, fmt.Errorf("could not find PRICE_FILTER or LOT_SIZE filter for symbol %s", symbol)
	}

	// Calculate decimal places for rounding
	pricePrecision := countDecimalPlaces(tickSize)
	quantityPrecision := countDecimalPlaces(stepSize)

	// --- ESTAS SON LAS LÍNEAS CLAVE QUE DEBEN ESTAR DECLARADAS AQUÍ ---
	// Round price and quantity according to exchange rules
	roundedPrice := priceDec.Round(int32(pricePrecision))
	roundedQuantity := quantityDec.Round(int32(quantityPrecision))
	// --- FIN LÍNEAS CLAVE ---

	// Check if rounded quantity is less than minimum allowed by lot size filter
	lotSizeFilter := symbolInfo.LotSizeFilter()
	if lotSizeFilter == nil {
		return nil, fmt.Errorf("LotSize filter not found for symbol %s", symbol)
	}
	minQtyDec, _ := decimal.NewFromString(lotSizeFilter.MinQuantity)

	if roundedQuantity.LessThan(minQtyDec) {
		s.logger.Warnf("Calculated quantity %s is less than minimum allowed %s for %s. Adjusting to minimum.", roundedQuantity, minQtyDec, symbol)
		roundedQuantity = minQtyDec // Use minimum quantity if calculated is too small
	}

	orderService := s.client.NewCreateOrderService().
		Symbol(symbol).
		Quantity(roundedQuantity.String()). // Use rounded quantity string
		Price(roundedPrice.String()).       // Use rounded price string
		TimeInForce(binance.TimeInForceTypeGTC)

	// Set order type (BUY/SELL)
	switch orderType {
	case models.OrderTypeBuy:
		orderService.Side(binance.SideTypeBuy).Type(binance.OrderTypeLimit)
	case models.OrderTypeSell:
		orderService.Side(binance.SideTypeSell).Type(binance.OrderTypeLimit)
	default:
		return nil, fmt.Errorf("unsupported order type: %s", orderType)
	}

	// Execute the order
	binanceOrder, err := orderService.Do(ctx)
	if err != nil {
		s.logger.Errorf("Failed to place order on Binance: %v", err)
		return nil, fmt.Errorf("failed to place order on Binance: %w", err)
	}

	s.logger.Infof("Order placed successfully on Binance: ID %d, Status: %s", binanceOrder.OrderID, binanceOrder.Status)

	// Convert Binance API response to our internal Order model
	ourOrderType := models.OrderType(binanceOrder.Side)
	if binanceOrder.Type == binance.OrderTypeMarket {
		ourOrderType = models.OrderType(binanceOrder.Type)
	}

	priceF, _ := strconv.ParseFloat(binanceOrder.Price, 64)
	origQtyF, _ := strconv.ParseFloat(binanceOrder.OrigQuantity, 64)
	executedQtyF, _ := strconv.ParseFloat(binanceOrder.ExecutedQuantity, 64)

	quoteQtyF := 0.0
	if executedQtyF > 0 && priceF > 0 {
		quoteQtyF = executedQtyF * priceF
	} else if origQtyF > 0 && priceF > 0 {
		binanceOrderStatus := models.OrderStatus(binanceOrder.Status) // Convertir a nuestro tipo
		if binanceOrderStatus == models.OrderStatusNew || binanceOrderStatus == models.OrderStatusPartiallyFilled {
			quoteQtyF = origQtyF * priceF
		}
	}

	orderStatus := models.OrderStatus(binanceOrder.Status)

	placedAt := time.Unix(0, binanceOrder.TransactTime*int64(time.Millisecond))

	var executedAt *time.Time
	if orderStatus == models.OrderStatusFilled || orderStatus == models.OrderStatusPartiallyFilled {
		// Asumo que si está "Filled" o "PartiallyFilled" en el response de creación,
		// entonces TransactTime puede considerarse el tiempo de la transacción.
		t := time.Unix(0, binanceOrder.TransactTime*int64(time.Millisecond)) // Usar TransactTime
		executedAt = &t
	}

	isTest := s.testnet

	return &models.Order{
		BinanceID:     binanceOrder.OrderID,
		Symbol:        binanceOrder.Symbol,
		Type:          ourOrderType,
		Price:         priceF,
		Quantity:      origQtyF,
		QuoteQty:      quoteQtyF,
		Status:        orderStatus,
		IsTest:        isTest,
		PlacedAt:      placedAt,
		ExecutedAt:    executedAt,
		LastUpdatedAt: placedAt,
	}, nil
}

// GetOrderStatus fetches the status of an order from Binance.
func (s *BinanceService) GetOrderStatus(ctx context.Context, symbol string, binanceOrderID int64) (*models.Order, error) {
	s.logger.Debugf("Fetching status for Binance order ID %d on symbol %s", binanceOrderID, symbol)

	orderRes, err := s.client.NewGetOrderService().
		Symbol(symbol).
		OrderID(binanceOrderID).
		Do(ctx)
	if err != nil {
		s.logger.Errorf("Failed to get order status for ID %d on symbol %s: %v", binanceOrderID, symbol, err)
		return nil, fmt.Errorf("failed to get order status: %w", err)
	}

	priceF, _ := strconv.ParseFloat(orderRes.Price, 64)
	origQtyF, _ := strconv.ParseFloat(orderRes.OrigQuantity, 64)
	executedQtyF, _ := strconv.ParseFloat(orderRes.ExecutedQuantity, 64)

	// --- CORRECCIÓN TAMBIÉN AQUÍ ---
	// For GetOrderService response, it generally has a CumQuote field.
	// If it doesn't, calculate from ExecutedQuantity * Price
	quoteQtyF := 0.0
	if orderRes.CummulativeQuoteQuantity != "" { // Check if the field exists and is not empty
		quoteQtyF, _ = strconv.ParseFloat(orderRes.CummulativeQuoteQuantity, 64)
	} else if executedQtyF > 0 && priceF > 0 {
		quoteQtyF = executedQtyF * priceF
	}

	orderStatus := models.OrderStatus(orderRes.Status)
	placedAt := time.Unix(0, orderRes.Time*int64(time.Millisecond)) // Time of creation
	updatedAt := time.Unix(0, orderRes.UpdateTime*int64(time.Millisecond))

	var executedAt *time.Time
	if orderStatus == models.OrderStatusFilled || orderStatus == models.OrderStatusPartiallyFilled {
		t := time.Unix(0, orderRes.UpdateTime*int64(time.Millisecond))
		executedAt = &t
	}

	isTest := s.testnet

	return &models.Order{
		BinanceID:     orderRes.OrderID,
		Symbol:        orderRes.Symbol,
		Type:          models.OrderType(orderRes.Side),
		Price:         priceF,
		Quantity:      origQtyF,
		QuoteQty:      quoteQtyF, // Use the (corrected) quoteQtyF
		Status:        orderStatus,
		IsTest:        isTest,
		PlacedAt:      placedAt,
		ExecutedAt:    executedAt,
		LastUpdatedAt: updatedAt,
	}, nil
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

// GetAccountBalance fetches the balance of a specific asset from the user's Binance account.
func (s *BinanceService) GetAccountBalance(ctx context.Context, asset string) (float64, error) {
	s.logger.Debugf("Fetching account balance for asset: %s", asset)
	res, err := s.client.NewGetAccountService().Do(ctx)
	if err != nil {
		s.logger.Errorf("Failed to get account info: %v", err)
		return 0, fmt.Errorf("failed to get account info: %w", err)
	}

	for _, balance := range res.Balances {
		if balance.Asset == asset {
			free, err := strconv.ParseFloat(balance.Free, 64)
			if err != nil {
				return 0, fmt.Errorf("failed to parse free balance for %s: %w", asset, err)
			}
			locked, err := strconv.ParseFloat(balance.Locked, 64)
			if err != nil {
				return 0, fmt.Errorf("failed to parse locked balance for %s: %w", asset, err)
			}
			s.logger.Debugf("Balance for %s: Free=%f, Locked=%f", asset, free, locked)
			return free + locked, nil
		}
	}
	s.logger.Warnf("Asset %s not found in account balances.", asset)
	return 0, nil // Return 0 if asset not found, or an error if you prefer
}

// countDecimalPlaces helper function
func countDecimalPlaces(s string) int {
	if !strings.Contains(s, ".") {
		return 0
	}
	return len(s) - strings.Index(s, ".") - 1
}
