// Package app configures and runs application.
package app

import (
	"context"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"trading_bot/config"
	balancemanager "trading_bot/pkg/balance/manager"
	"trading_bot/pkg/logger"
	tradingmanager "trading_bot/pkg/trading/manager"

	"github.com/shopspring/decimal"
)

// Run creates objects via constructors.
func Run(cfg *config.Config) {
	l := logger.New(cfg.Log.Level)

	decimal.DivisionPrecision = 8

	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup

	balancemanager, err := balancemanager.New(ctx, &wg, cfg.CryptoCurrencies, l)
	if err != nil {
		l.Fatal("app - Run - BalanceManager.New: %w", err)
	}
	balancemanager.Start()

	tradingmanager, err1 := tradingmanager.New(ctx, &wg, cfg.CryptoCurrencies, l)
	if err1 != nil {
		l.Fatal("app - Run - TradingManager.New: %w", err1)
	}
	balancemanager.Start()

	// Waiting signal
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt, syscall.SIGTERM)

	select {
	case s := <-interrupt:
		l.Info("app - Run - signal: " + s.String())
	case err = <-balancemanager.Notify():
		l.Error("app - Run - BalanceManager.Notify: %w", err)
	case err = <-tradingmanager.Notify():
		l.Error("app - Run - TradingManager.Notify: %w", err)
	}

	cancel()
	wg.Wait()
}
