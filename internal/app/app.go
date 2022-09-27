// Package app configures and runs application.
package app

import (
	"context"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"trading_bot/config"
	balanceManager "trading_bot/pkg/balance/manager"
	"trading_bot/pkg/logger"
	tradingManager "trading_bot/pkg/trading/manager"

	"github.com/shopspring/decimal"
)

// Run creates objects via constructors.
func Run(cfg *config.Config) {
	l := logger.New(cfg.Log.Level)

	decimal.DivisionPrecision = 8

	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup

	balManager, err := balanceManager.New(ctx, &wg, cfg.CryptoCurrencies, l)
	if err != nil {
		l.Fatal("app - Run - BalanceManager.New: %w", err)
	}
	balManager.Start()

	tradeManager, err1 := tradingManager.New(ctx, &wg, cfg.CryptoCurrencies, l)
	if err1 != nil {
		l.Fatal("app - Run - TradingManager.New: %w", err1)
	}
	tradeManager.Start()

	// Waiting signal
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt, syscall.SIGTERM)

	select {
	case s := <-interrupt:
		l.Info("app - Run - signal: " + s.String())
	case err = <-balManager.Notify():
		l.Error("app - Run - BalanceManager.Notify: %w", err)
	case err = <-tradeManager.Notify():
		l.Error("app - Run - TradingManager.Notify: %w", err)
	}

	cancel()
	wg.Wait()
}
