// main.go (ejemplo de cómo se verá la parte relevante)

package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"binance-trader-bot/config"
	"binance-trader-bot/database"
	"binance-trader-bot/repositories"
	"binance-trader-bot/services"
	"binance-trader-bot/utils"
)

func main() {
	logger := utils.NewLogger()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Cargar configuración
	cfg, err := config.LoadConfig()
	if err != nil {
		logger.Fatalf("Failed to load configuration: %v", err)
	}

	// Conectar a la base de datos
	db, err := database.NewPostgresDB(cfg.DatabaseURL)
	if err != nil {
		logger.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	// Ejecutar migraciones (CORRECCIÓN AQUÍ)
	err = database.RunMigrations(cfg.DatabaseURL) // <--- CORRECCIÓN CLAVE: Pasar cfg.DatabaseURL
	if err != nil {
		logger.Fatalf("Failed to run database migrations: %v", err)
	}

	// Inicializar repositorios
	tradeRepo := repositories.NewTradeRepository(db)

	// Inicializar servicios
	binanceService := services.NewBinanceService(cfg.BinanceAPIKey, cfg.BinanceSecretKey, cfg.UseTestnet, logger)
	stateManager := services.NewStateManager(tradeRepo, logger)
	tradingStrategy := services.NewTradingStrategy(binanceService, stateManager, cfg, logger)

	// Cargar estado inicial del bot
	if err := stateManager.LoadBotState(ctx); err != nil {
		logger.Fatalf("Failed to load bot state: %v", err)
	}

	// Manejo de señales para un apagado limpio
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Bucle principal del bot
	go func() {
		for {
			select {
			case <-ctx.Done():
				logger.Info("Shutting down trading cycle loop...")
				return
			default:
				if err := tradingStrategy.ExecuteTradingCycle(ctx); err != nil {
					logger.Errorf("Error during trading cycle: %v", err)
				}
				logger.Infof("Next trading cycle in %d seconds...", cfg.TradingCycleIntervalSeconds)
				time.Sleep(time.Duration(cfg.TradingCycleIntervalSeconds) * time.Second)
			}
		}
	}()

	// Esperar señal de apagado
	<-sigChan
	logger.Info("Shutdown signal received. Exiting.")
	cancel()                    // Notificar a las goroutines que se detengan
	time.Sleep(2 * time.Second) // Dar tiempo para que las goroutines terminen
}
