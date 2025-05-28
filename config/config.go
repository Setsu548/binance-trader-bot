package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Config holds all the application's configuration parameters.
type Config struct {
	BinanceAPIKey               string
	BinanceSecretKey            string
	UseTestnet                  bool
	DatabaseURL                 string
	Symbol                      string    // e.g., "BTCUSDT"
	InitialUSDT                 float64   // Initial USDT amount for bot to manage
	OrderAmount                 float64   // Amount in USDT to use for each buy order
	OrderIntervalMinutes        int       // Interval in minutes between initial buy orders
	InitialBuyPercentage        float64   // Percentage below current price for initial buys (e.g., 1.0 for 1% below)
	SellProfitPercentage        float64   // Percentage profit target for sell orders (e.g., 2.0 for 2% profit)
	BuyPercentages              []float64 // List of percentages for subsequent "escalonadas" buys
	MaxOpenTrades               int
	TradingCycleIntervalSeconds int
}

// LoadConfig loads configuration from environment variables.
func LoadConfig() (*Config, error) {
	cfg := &Config{}

	cfg.BinanceAPIKey = os.Getenv("BINANCE_API_KEY")
	if cfg.BinanceAPIKey == "" {
		return nil, fmt.Errorf("BINANCE_API_KEY not set")
	}

	cfg.BinanceSecretKey = os.Getenv("BINANCE_SECRET_KEY") // <--- ESTE CAMPO YA SE CARGA AQUÍ
	if cfg.BinanceSecretKey == "" {
		return nil, fmt.Errorf("BINANCE_SECRET_KEY not set")
	}

	useTestnetStr := os.Getenv("USE_TESTNET")
	var err error
	cfg.UseTestnet, err = strconv.ParseBool(useTestnetStr)
	if err != nil {
		fmt.Printf("WARNING: USE_TESTNET not set or invalid ('%s'). Defaulting to false.\n", useTestnetStr)
		cfg.UseTestnet = false
	}

	cfg.DatabaseURL = os.Getenv("DATABASE_URL")
	if cfg.DatabaseURL == "" {
		return nil, fmt.Errorf("DATABASE_URL not set")
	}

	cfg.Symbol = os.Getenv("SYMBOL")
	if cfg.Symbol == "" {
		return nil, fmt.Errorf("SYMBOL not set")
	}

	cfg.InitialUSDT, err = parseFloatEnv("INITIAL_USDT", 100.0)
	if err != nil {
		return nil, err
	}

	cfg.OrderAmount, err = parseFloatEnv("ORDER_AMOUNT", 10.0)
	if err != nil {
		return nil, err
	}

	cfg.OrderIntervalMinutes, err = parseIntEnv("ORDER_INTERVAL_MINUTES", 60)
	if err != nil {
		return nil, err
	}

	cfg.InitialBuyPercentage, err = parseFloatEnv("INITIAL_BUY_PERCENTAGE", 1.0)
	if err != nil {
		return nil, err
	}

	cfg.SellProfitPercentage, err = parseFloatEnv("SELL_PROFIT_PERCENTAGE", 2.0)
	if err != nil {
		return nil, err
	}

	buyPercentagesStr := os.Getenv("BUY_PERCENTAGES")
	if buyPercentagesStr != "" {
		parts := strings.Split(buyPercentagesStr, ",")
		cfg.BuyPercentages = make([]float64, len(parts))
		for i, p := range parts {
			val, parseErr := strconv.ParseFloat(strings.TrimSpace(p), 64)
			if parseErr != nil {
				return nil, fmt.Errorf("invalid value in BUY_PERCENTAGES: '%s' is not a float: %w", p, parseErr)
			}
			cfg.BuyPercentages[i] = val
		}
	} else {
		cfg.BuyPercentages = []float64{}
		fmt.Println("WARNING: BUY_PERCENTAGES not set. No additional buy percentages will be used.")
	}

	// Cargar MaxOpenTrades (NUEVO CAMPO)
	cfg.MaxOpenTrades, err = parseIntEnv("MAX_OPEN_TRADES", 5) // Default a 5 trades abiertos
	if err != nil {
		return nil, err
	}

	cfg.TradingCycleIntervalSeconds, err = parseIntEnv("TRADING_CYCLE_INTERVAL_SECONDS", 300) //
	if err != nil {
		return nil, err
	}

	return cfg, nil
}

// parseIntEnv helper function to parse an integer environment variable with a default.
func parseIntEnv(key string, defaultValue int) (int, error) {
	valStr := os.Getenv(key)
	if valStr == "" {
		return defaultValue, nil
	}
	val, err := strconv.Atoi(valStr)
	if err != nil {
		return 0, fmt.Errorf("environment variable %s ('%s') is not a valid integer: %w", key, valStr, err)
	}
	return val, nil
}

// parseFloatEnv helper function to parse a float environment variable with a default.
func parseFloatEnv(key string, defaultValue float64) (float64, error) {
	valStr := os.Getenv(key)
	if valStr == "" {
		return defaultValue, nil
	}
	val, err := strconv.ParseFloat(valStr, 64)
	if err != nil {
		return 0.0, fmt.Errorf("environment variable %s ('%s') is not a valid float: %w", key, valStr, err)
	}
	return val, nil
}
