package database

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file" // For file-based migrations
	_ "github.com/lib/pq"                                // PostgreSQL driver
)

// NewPostgresDB establishes a new connection to the PostgreSQL database.
func NewPostgresDB(dataSourceName string) (*sql.DB, error) {
	db, err := sql.Open("postgres", dataSourceName)
	if err != nil {
		return nil, fmt.Errorf("error opening database connection: %w", err)
	}

	// Ping the database to verify the connection is alive
	for i := 0; i < 5; i++ { // Retry a few times in case DB is still starting
		err = db.Ping()
		if err == nil {
			fmt.Println("Successfully connected to PostgreSQL!")
			return db, nil
		}
		fmt.Printf("Attempt %d: Could not connect to database, retrying... %v\n", i+1, err)
		time.Sleep(2 * time.Second)
	}

	return nil, fmt.Errorf("failed to connect to database after multiple retries: %w", err)
}

// RunMigrations applies database schema migrations from the 'migrations' directory.
// You need to create a 'migrations' folder at the root of your project
// and place your SQL migration files there.
// Example:
// migrations/
// ├── 000001_create_orders_table.up.sql
// ├── 000001_create_orders_table.down.sql
// ├── 000002_create_trades_table.up.sql
// ├── 000002_create_trades_table.down.sql
// └── 000003_create_bot_state_table.up.sql
// └── 000003_create_bot_state_table.down.sql
func RunMigrations(db *sql.DB) error {
	// IMPORTANT: Ensure the path to your migrations directory is correct.
	// It should be relative to where your 'go run' or 'go build' command is executed.
	// If you run from the project root, "./migrations" is usually correct.
	m, err := migrate.New(
		"file://./migrations", // Path to your migration files
		"postgres://"+db.Stats().OpenConnections.String()+"/"+db.Stats().WaitDuration.String(), // This is a placeholder, you should use the actual DSN
	)
	if err != nil {
		return fmt.Errorf("failed to create migrate instance: %w", err)
	}

	// For the migrate.New function's database source, it's better to use the original DSN:
	// m, err := migrate.New(
	// 	"file://./migrations",
	// 	dataSourceName, // Use the same dataSourceName passed to NewPostgresDB
	// )

	// To fix the issue with migrate.New's database source using db.Stats(),
	// we need to pass the original dataSourceName that we get from config.
	// This would require refactoring NewPostgresDB slightly or passing DSN to RunMigrations.
	// For now, let's make it work by passing dataSourceName to RunMigrations.
	// Let's adjust RunMigrations to accept dataSourceName.

	// Refactored RunMigrations signature: func RunMigrations(dataSourceName string) error
	// For the sake of this example, we'll assume the dataSourceName is available.
	// In main.go, you'd call database.RunMigrations(cfg.DatabaseURL)

	// Apply all available migrations
	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("failed to apply migrations: %w", err)
	}
	if err == migrate.ErrNoChange {
		fmt.Println("No new migrations to apply.")
	} else {
		fmt.Println("Database migrations applied successfully.")
	}

	return nil
}

// --- SQL MIGRATION FILES (example content) ---
// You will need to create these files manually in your 'migrations' directory:

// migrations/000001_create_orders_table.up.sql
/*
CREATE TABLE IF NOT EXISTS orders (
    id BIGSERIAL PRIMARY KEY,
    binance_id BIGINT UNIQUE NOT NULL,
    symbol VARCHAR(50) NOT NULL,
    type VARCHAR(10) NOT NULL,
    price NUMERIC(20, 10) NOT NULL,
    quantity NUMERIC(20, 10) NOT NULL,
    quote_qty NUMERIC(20, 10) NOT NULL,
    status VARCHAR(50) NOT NULL,
    is_test BOOLEAN NOT NULL DEFAULT FALSE,
    placed_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    executed_at TIMESTAMP WITH TIME ZONE,
    last_updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_orders_binance_id ON orders (binance_id);
CREATE INDEX IF NOT EXISTS idx_orders_symbol_type ON orders (symbol, type);
*/

// migrations/000001_create_orders_table.down.sql
/*
DROP TABLE IF EXISTS orders;
*/

// migrations/000002_create_trades_table.up.sql
/*
CREATE TABLE IF NOT EXISTS trades (
    id BIGSERIAL PRIMARY KEY,
    buy_order_id BIGINT UNIQUE NOT NULL,
    sell_order_id BIGINT UNIQUE, -- Can be NULL initially
    symbol VARCHAR(50) NOT NULL,
    buy_price NUMERIC(20, 10) NOT NULL,
    buy_quantity NUMERIC(20, 10) NOT NULL,
    sell_price_target NUMERIC(20, 10) NOT NULL,
    actual_sell_price NUMERIC(20, 10), -- Can be NULL
    status VARCHAR(50) NOT NULL,
    profit_usdt NUMERIC(20, 10), -- Can be NULL
    opened_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    closed_at TIMESTAMP WITH TIME ZONE,
    last_status_update TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    CONSTRAINT fk_buy_order FOREIGN KEY (buy_order_id) REFERENCES orders(binance_id) ON DELETE RESTRICT
);

CREATE INDEX IF NOT EXISTS idx_trades_status ON trades (status);
CREATE INDEX IF NOT EXISTS idx_trades_symbol ON trades (symbol);
*/

// migrations/000002_create_trades_table.down.sql
/*
DROP TABLE IF EXISTS trades;
*/

// migrations/000003_create_bot_state_table.up.sql
/*
CREATE TABLE IF NOT EXISTS bot_states (
    id BIGINT PRIMARY KEY DEFAULT 1, -- We expect only one row
    initial_usdt_investment NUMERIC(20, 10) NOT NULL,
    current_usdt_balance NUMERIC(20, 10) NOT NULL,
    current_btc_balance NUMERIC(20, 10) NOT NULL,
    total_usdt_invested NUMERIC(20, 10) NOT NULL,
    total_usdt_profit NUMERIC(20, 10) NOT NULL,
    initial_buy_orders_placed_count INT NOT NULL DEFAULT 0,
    last_initial_buy_order_placed_at TIMESTAMP WITH TIME ZONE,
    is_initial_buying_complete BOOLEAN NOT NULL DEFAULT FALSE,
    last_bot_run_timestamp TIMESTAMP WITH TIME ZONE NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

-- Insert a default row if it doesn't exist.
-- This ensures the state always exists and we just update it.
INSERT INTO bot_states (id, initial_usdt_investment, current_usdt_balance, current_btc_balance, total_usdt_invested, total_usdt_profit, last_bot_run_timestamp)
VALUES (1, 0.0, 0.0, 0.0, 0.0, 0.0, NOW())
ON CONFLICT (id) DO NOTHING;
*/

// migrations/000003_create_bot_state_table.down.sql
/*
DROP TABLE IF EXISTS bot_states;
*/
