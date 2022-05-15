package manager

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"trading_bot/config"
	"trading_bot/pkg/logger"

	trading "trading_bot/pkg/trading/worker"
)

type TradingManager struct {
	Workers []*trading.TradingWorker
	notify  chan error
}

func New(ctx context.Context, wg *sync.WaitGroup, cryptoCurrencies []config.CryptoCurrency, l logger.ILogger) (*TradingManager, error) {
	if len(cryptoCurrencies) == 0 {
		return nil, errors.New("tradingmanager no currencies provided")
	}

	s := &TradingManager{
		notify:  make(chan error, 1),
		Workers: make([]*trading.TradingWorker, 0, len(cryptoCurrencies)),
	}

	for _, item := range cryptoCurrencies {
		// skip empty currencies
		if len(item.InternalSettings.Currency) == 0 {
			continue
		}
		wk, err := trading.New(ctx, wg, item, l, s.notify)
		if err != nil {
			return nil, fmt.Errorf("TradingWorker.New: %w", err)
		}

		s.Workers = append(s.Workers, wk)
	}

	return s, nil
}

// Notify -.
func (s *TradingManager) Notify() <-chan error {
	return s.notify
}

// Start workers
func (s *TradingManager) Start() {
	for _, worker := range s.Workers {
		worker.Start()
	}
}

// Shutdown -.
func (s *TradingManager) Shutdown() {
	for _, worker := range s.Workers {
		worker.Stop()
	}
}
