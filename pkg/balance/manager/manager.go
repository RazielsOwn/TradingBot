package manager

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"trading_bot/config"
	balance "trading_bot/pkg/balance/worker"
	"trading_bot/pkg/logger"
)

type BalanceManager struct {
	Workers []*balance.BalanceWorker
	notify  chan error
}

func New(ctx context.Context, wg *sync.WaitGroup, cryptoCurrencies []config.CryptoCurrency, l logger.ILogger) (*BalanceManager, error) {
	if len(cryptoCurrencies) == 0 {
		return nil, errors.New("balancemanager no currencies provided")
	}

	s := &BalanceManager{
		notify:  make(chan error, 1),
		Workers: make([]*balance.BalanceWorker, 0, len(cryptoCurrencies)),
	}

	for _, item := range cryptoCurrencies {
		wk, err := balance.New(ctx, wg, item, l, s.notify)
		if err != nil {
			return nil, fmt.Errorf("BalanceWorker.New: %w", err)
		}

		s.Workers = append(s.Workers, wk)
	}

	return s, nil
}

// Notify -.
func (s *BalanceManager) Notify() <-chan error {
	return s.notify
}

// Start workers
func (s *BalanceManager) Start() {
	for _, worker := range s.Workers {
		worker.Start()
	}
}

// Shutdown -.
func (s *BalanceManager) Shutdown() {
	for _, worker := range s.Workers {
		worker.Stop()
	}
}
