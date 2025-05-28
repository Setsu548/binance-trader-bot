package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"
	"trading/config"

	"trading/database"
	"trading/repositories"
	"trading/services"
	"trading/utils" // Asumiendo que utils/logger.go está aquí
)

func main() {
	// 1. Inicializar el Logger
	logger := utils.NewLogger()
	logger.Info("Starting Binance Trading Bot...")

	// 2. Cargar Configuración desde Variables de Entorno
	cfg, err := config.LoadConfig()
	if err != nil {
		logger.Fatalf("Failed to load configuration: %v", err)
	}
	logger.Infof("Configuration loaded. Using Testnet: %t", cfg.UseTestnet)

	// 3. Conectar a PostgreSQL
	db, err := database.NewPostgresDB(cfg.DatabaseURL)
	if err != nil {
		logger.Fatalf("Failed to connect to PostgreSQL: %v", err)
	}
	defer func() {
		sqlDB, closeErr := db.DB()
		if closeErr != nil {
			logger.Errorf("Error getting underlying SQL DB for closing: %v", closeErr)
		}
		if sqlDB != nil {
			if err := sqlDB.Close(); err != nil {
				logger.Errorf("Error closing database connection: %v", err)
			} else {
				logger.Info("Database connection closed.")
			}
		}
	}()

	// Ejecutar migraciones (opcional, pero muy recomendado)
	if err := database.RunMigrations(db); err != nil {
		logger.Fatalf("Failed to run database migrations: %v", err)
	}
	logger.Info("Database migrations completed successfully.")

	// 4. Inicializar Repositorios
	tradeRepo := repositories.NewTradeRepository(db)
	logger.Info("Trade Repository initialized.")

	// 5. Inicializar Servicios
	binanceService := services.NewBinanceService(cfg.BinanceAPIKey, cfg.BinanceAPISecret, cfg.UseTestnet, logger)
	stateManager := services.NewStateManager(tradeRepo, logger) // stateManager necesita el repositorio para persistir
	tradingStrategy := services.NewTradingStrategy(binanceService, stateManager, cfg, logger)
	logger.Info("Services initialized.")

	// 6. Contexto para manejo de señales de terminación
	ctx, cancel := context.WithCancel(context.Background())

	// Capturar señales de sistema (Ctrl+C, etc.) para una terminación limpia
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigChan
		logger.Warnf("Received signal: %v. Shutting down bot...", sig)
		cancel() // Cancela el contexto para detener el bucle del bot
	}()

	// 7. Ejecutar la lógica principal del bot en un goroutine
	// Usa un ticker para ejecutar la lógica cada cierto intervalo.
	// El intervalo puede ser configurado en `config.go` o ser dinámico.
	// Para empezar, usaremos un intervalo fijo o el ORDER_INTERVAL_MINUTES.
	// La lógica real de temporización entre órdenes iniciales estará dentro de trading_strategy.
	ticker := time.NewTicker(time.Duration(cfg.OrderIntervalMinutes) * time.Minute) // Puedes ajustar este tick principal
	defer ticker.Stop()

	// Cargar el estado inicial del bot al inicio
	err = stateManager.LoadBotState(ctx)
	if err != nil {
		logger.Fatalf("Failed to load initial bot state: %v", err)
	}
	logger.Info("Bot state loaded successfully.")

	// Bucle principal del bot
	for {
		select {
		case <-ticker.C:
			// Aquí se llamaría a la función principal que ejecuta toda la lógica del bot
			// Por ejemplo, CheckAndPlaceOrders, ManageOpenOrders, etc.
			logger.Info("Executing bot cycle...")
			if err := tradingStrategy.ExecuteTradingCycle(ctx); err != nil {
				logger.Errorf("Error during trading cycle: %v", err)
			}
		case <-ctx.Done():
			logger.Info("Bot context cancelled. Exiting main loop.")
			return // Salir del bucle principal
		}
	}
}
