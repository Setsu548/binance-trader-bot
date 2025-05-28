package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	// Necesario para time.Duration
)

// Config holds all the application's configuration pulled from environment variables.
type Config struct {
	BinanceAPIKey           string
	BinanceAPISecret        string
	UseTestnet              bool
	DatabaseURL             string
	InitialUSDT             float64
	OrderAmount             float64
	Symbol                  string
	InitialBuyPercentage    float64
	OrderIntervalMinutes    int // Changed to int for easier time.Duration conversion
	BuyPercentages          []float64
	SellProfitPercentage    float64
	BotCycleIntervalSeconds int // New: How often the main bot loop runs
}

// LoadConfig loads configuration from environment variables.
func LoadConfig() (*Config, error) {
	cfg := &Config{}
	var err error

	// Load Binance API Keys
	cfg.BinanceAPIKey = os.Getenv("BINANCE_API_KEY")
	if cfg.BinanceAPIKey == "" {
		return nil, fmt.Errorf("BINANCE_API_KEY environment variable not set")
	}
	cfg.BinanceAPISecret = os.Getenv("BINANCE_API_SECRET")
	if cfg.BinanceAPISecret == "" {
		return nil, fmt.Errorf("BINANCE_API_SECRET environment variable not set")
	}

	// Load USE_TESTNET
	useTestnetStr := os.Getenv("USE_TESTNET")
	if useTestnetStr == "" {
		useTestnetStr = "true" // Default value as per requirements
	}
	cfg.UseTestnet, err = strconv.ParseBool(useTestnetStr)
	if err != nil {
		return nil, fmt.Errorf("invalid value for USE_TESTNET: %w", err)
	}

	// Load Database URL (for PostgreSQL)
	cfg.DatabaseURL = os.Getenv("DATABASE_URL")
	if cfg.DatabaseURL == "" {
		// Provide a default for local development if not set, but warn.
		// For production, this should definitely be set.
		fmt.Println("WARNING: DATABASE_URL environment variable not set. Using default local PostgreSQL URL.")
		cfg.DatabaseURL = "postgresql://user:password@localhost:5432/trading_bot_db?sslmode=disable"
	}

	// Load Trading Parameters with defaults
	cfg.InitialUSDT, err = parseFloatEnv("INITIAL_USDT", 100.0)
	if err != nil {
		return nil, err
	}
	if cfg.InitialUSDT < 100 {
		return nil, fmt.Errorf("INITIAL_USDT must be at least 100")
	}

	cfg.OrderAmount, err = parseFloatEnv("ORDER_AMOUNT", 10.0)
	if err != nil {
		return nil, err
	}
	if cfg.OrderAmount < 10 {
		return nil, fmt.Errorf("ORDER_AMOUNT must be at least 10 USDT")
	}
	// Ensure InitialUSDT is a multiple of OrderAmount for clean division
	if int(cfg.InitialUSDT/cfg.OrderAmount)*int(cfg.OrderAmount) != int(cfg.InitialUSDT) {
		return nil, fmt.Errorf("INITIAL_USDT (%f) must be a multiple of ORDER_AMOUNT (%f) for clean division into orders", cfg.InitialUSDT, cfg.OrderAmount)
	}

	cfg.Symbol = os.Getenv("SYMBOL")
	if cfg.Symbol == "" {
		cfg.Symbol = "BTCUSDT" // Default value
	}

	cfg.InitialBuyPercentage, err = parseFloatEnv("INITIAL_BUY_PERCENTAGE", 1.0) // Default 1%
	if err != nil {
		return nil, err
	}
	if cfg.InitialBuyPercentage <= 0 {
		return nil, fmt.Errorf("INITIAL_BUY_PERCENTAGE must be greater than 0")
	}

	cfg.OrderIntervalMinutes, err = parseIntEnv("ORDER_INTERVAL_MINUTES", 2) // Default 2 minutes
	if err != nil {
		return nil, err
	}
	if cfg.OrderIntervalMinutes <= 0 {
		return nil, fmt.Errorf("ORDER_INTERVAL_MINUTES must be greater than 0")
	}

	// Load BUY_PERCENTAGES (comma-separated string)
	buyPercentagesStr := os.Getenv("BUY_PERCENTAGES")
	if buyPercentagesStr == "" {
		buyPercentagesStr = "1,2,5,10" // Default value
	}
	cfg.BuyPercentages, err = parsePercentages(buyPercentagesStr)
	if err != nil {
		return nil, fmt.Errorf("invalid value for BUY_PERCENTAGES: %w", err)
	}
	for _, p := range cfg.BuyPercentages {
		if p <= 0 {
			return nil, fmt.Errorf("all BUY_PERCENTAGES must be greater than 0")
		}
	}

	cfg.SellProfitPercentage, err = parseFloatEnv("SELL_PROFIT_PERCENTAGE", 2.0) // Default 2%
	if err != nil {
		return nil, err
	}
	if cfg.SellProfitPercentage <= 0 {
		return nil, fmt.Errorf("SELL_PROFIT_PERCENTAGE must be greater than 0")
	}

	// New: Bot cycle interval for the main loop (how often bot checks things)
	cfg.BotCycleIntervalSeconds, err = parseIntEnv("BOT_CYCLE_INTERVAL_SECONDS", 30) // Default every 30 seconds
	if err != nil {
		return nil, err
	}
	if cfg.BotCycleIntervalSeconds <= 0 {
		return nil, fmt.Errorf("BOT_CYCLE_INTERVAL_SECONDS must be greater than 0")
	}

	return cfg, nil
}

// Helper function to parse float environment variables with a default value
func parseFloatEnv(envVar string, defaultValue float64) (float64, error) {
	valStr := os.Getenv(envVar)
	if valStr == "" {
		return defaultValue, nil
	}
	val, err := strconv.ParseFloat(valStr, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid float value for %s: %w", envVar, err)
	}
	return val, nil
}

// Helper function to parse int environment variables with a default value
func parseIntEnv(envVar string, defaultValue int) (int, error) {
	valStr := os.Getenv(envVar)
	if valStr == "" {
		return defaultValue, nil
	}
	val, err := strconv.Atoi(valStr)
	if err != nil {
		return 0, fmt.Errorf("invalid integer value for %s: %w", envVar, err)
	}
	return val, nil
}

// Helper function to parse a comma-separated string of percentages
func parsePercentages(percentagesStr string) ([]float64, error) {
	parts := strings.Split(percentagesStr, ",")
	var percentages []float64
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue // Skip empty parts if any
		}
		val, err := strconv.ParseFloat(p, 64)
		if err != nil {
			return nil, fmt.Errorf("could not parse percentage '%s': %w", p, err)
		}
		percentages = append(percentages, val)
	}
	return percentages, nil
}
