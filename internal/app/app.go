// Package app configures and runs application.
package app

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"trading_bot/config"
	balancemanager "trading_bot/pkg/balance/manager"
	"trading_bot/pkg/logger"

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
		l.Fatal(fmt.Errorf("app - Run - BalanceManager.New: %w", err))
	}
	balancemanager.Start()

	// Waiting signal
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt, syscall.SIGTERM)

	select {
	case s := <-interrupt:
		l.Info("app - Run - signal: " + s.String())
	case err = <-balancemanager.Notify():
		l.Error(fmt.Errorf("app - Run - BalanceManager.Notify: %w", err))
	}

	cancel()
	wg.Wait()
}
