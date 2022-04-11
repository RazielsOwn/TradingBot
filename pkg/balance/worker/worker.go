package worker

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"time"
	"trading_bot/config"
	"trading_bot/internal/common"
	"trading_bot/internal/common/helpermethods"
	jetcryptoReq "trading_bot/internal/common/requests/jetcrypto"
	tradingsystemReq "trading_bot/internal/common/requests/poloniex"
	"trading_bot/pkg/logger"

	"github.com/shopspring/decimal"
)

type BalanceWorker struct {
	logger                logger.Interface
	settings              config.CryptoCurrency
	tradingSystemRequests common.ITradingSystemRequest
	internalRequests      common.IInternalRequest
	WaitGroup             *sync.WaitGroup
	notify                chan error
	running               bool
}

func New(ctx context.Context, wg *sync.WaitGroup, currencySettings config.CryptoCurrency, l logger.Interface, err chan error) (*BalanceWorker, error) {
	s := &BalanceWorker{
		notify:                err,
		running:               false,
		logger:                l,
		settings:              currencySettings,
		tradingSystemRequests: tradingsystemReq.New(l, helpermethods.New(l), currencySettings.TradingSettings),
		internalRequests:      jetcryptoReq.New(l, helpermethods.New(l), currencySettings.InternalSettings),
		WaitGroup:             wg,
	}

	go func(bw *BalanceWorker) {
		bw.DoWork(ctx)
	}(s)

	return s, nil
}

// Start worker
func (s *BalanceWorker) Start() {
	s.WaitGroup.Add(1)
	s.running = true
	s.logger.Debug("Start BalanceWorker called")
}

// Shutdown -.
func (s *BalanceWorker) Stop() {
	s.WaitGroup.Done()
	s.running = false
	s.logger.Debug("Stop BalanceWorker called")
}

func (s *BalanceWorker) DoWork(ctx context.Context) {
	defer s.Stop()
	for {
		if !s.running {
			continue
		}

		select {
		case <-time.After(10 * time.Second):

		case <-ctx.Done():
			s.logger.Debug("Context cancelled")
			return
		}

		var internalBalanceCache = s.internalRequests.GetBalances(ctx)
		if len(internalBalanceCache) == 0 {
			s.logger.Error("tradingWorker Error : Can't get own internalBalances!!!")
			continue
		}

		var intBalance, found = internalBalanceCache[s.settings.InternalSettings.Currency]
		if !found {
			s.logger.Error("tradingWorker Error : Can't get own internalBalances!!!")
			continue
		}

		var internalBalance = intBalance.Balance.Add(intBalance.Reserved)

		var tradingBalanceCache = s.tradingSystemRequests.GetTradingBalances(ctx)
		if len(tradingBalanceCache) == 0 {
			s.logger.Error("tradingWorker Error : Can't get own tradingSystemBalance!!!")
			continue
		}

		var tsBalance, found1 = tradingBalanceCache[s.settings.InternalSettings.Currency]
		if !found1 {
			s.logger.Error("tradingWorker Error : Can't get own tradingSystemBalance!!!")
			continue
		}

		var tradingBalance = tsBalance.Balance
		s.logger.Debug(fmt.Sprintf("Balancer %v tradingBalance is : %v, intenalBalance is : %v", s.settings.InternalSettings.Currency, tradingBalance, internalBalance))

		var totalBalance = tradingBalance.Add(internalBalance)
		var diffABS = tradingBalance.Sub(totalBalance.Mul((decimal.NewFromInt(1).Sub(s.settings.BalancePercent)))).Abs()
		var totalBalanceLower = totalBalance.Mul((decimal.NewFromInt(1).Sub(s.settings.BalancePercent.Sub(s.settings.ThresholdPercent))))
		var totalBalanceUpper = totalBalance.Mul((s.settings.BalancePercent.Add(s.settings.ThresholdPercent)))

		if diffABS.GreaterThan(s.settings.ThresholdAbs) && (tradingBalance.GreaterThan(totalBalanceLower) || internalBalance.GreaterThan(totalBalanceUpper)) {
			s.logger.Info(fmt.Sprintf("Balancer %v tradingBalance is : %v, intenalBalance is : %v", s.settings.InternalSettings.Currency, tradingBalance, internalBalance))

			var internalToTradingSystem = false
			if internalBalance.GreaterThan(totalBalanceUpper) {
				internalToTradingSystem = true
			}

			if internalToTradingSystem {
				s.logger.Info(fmt.Sprintf("Balancer %v diffABS is : %v > thresholdAbs : %v AND internalBalance : %v > totalBalanceUpper %v starting Balancer!", s.settings.InternalSettings.Currency, diffABS, s.settings.ThresholdAbs, internalBalance, totalBalanceUpper))

				// sending currency internal -> trading system
				var amountToWithdraw = diffABS
				s.logger.Info(fmt.Sprintf("Balancer %v : Creating withdraw order Internal -> Trading system, amountToWithdraw %v", s.settings.InternalSettings.Currency, amountToWithdraw))

				if len(s.settings.TradingSettings.CryptoAddress) == 0 {
					// try to get trading system crypto address
					s.settings.TradingSettings.CryptoAddress = s.tradingSystemRequests.GetCryptoAddress(ctx, s.settings.TradingSettings.Currency, s.settings.TradingSettings.WithdrawalNetwork)
					if len(s.settings.TradingSettings.CryptoAddress) == 0 {
						s.logger.Error(fmt.Sprintf("Balancer %v : Can't get Trading system CryptoAddress!", s.settings.InternalSettings.Currency))
						continue
					}
				}

				var paymentId = s.internalRequests.Withdraw(ctx, s.settings.TradingSettings.CryptoAddress, s.settings.TradingSettings.DestinationTag, amountToWithdraw, strconv.Itoa(s.settings.CurrencyId))
				s.logger.Info(fmt.Sprintf("Balancer %v : Withdraw order Internal -> Trading system, amountToWithdraw %v result PaymentId is : %v", s.settings.InternalSettings.Currency, amountToWithdraw, paymentId))
			} else {
				s.logger.Info(fmt.Sprintf("Balancer %v diffABS is : %v > thresholdAbs : %v AND tradingBalance : %v > totalBalanceLower %v starting Balancer!", s.settings.TradingSettings.Currency, diffABS, s.settings.ThresholdAbs, tradingBalance, totalBalanceLower))

				// receiving currency trading system -> internal
				var amountToWithdraw = diffABS
				s.logger.Info(fmt.Sprintf("Balancer %v : Creating withdraw order Trading system -> Internal, amountToWithdraw %v", s.settings.TradingSettings.Currency, amountToWithdraw))

				if len(s.settings.InternalSettings.CryptoAddress) == 0 {
					// try to get internal crypto address
					s.settings.InternalSettings.CryptoAddress = s.internalRequests.GetCryptoAddress(ctx, s.settings.InternalSettings.Currency)
					if len(s.settings.InternalSettings.CryptoAddress) == 0 {
						s.logger.Error(fmt.Sprintf("Balancer %v : Can't get Internal CryptoAddress!", s.settings.InternalSettings.Currency))
						continue
					}
				}

				var success = s.tradingSystemRequests.Withdraw(ctx, s.settings.InternalSettings.CryptoAddress, amountToWithdraw, s.settings.TradingSettings.Currency, s.settings.TradingSettings.WithdrawalNetwork)
				s.logger.Info(fmt.Sprintf("Balancer %v : Withdraw order Trading system -> Internal, amountToWithdraw %v result is : %t", s.settings.InternalSettings.Currency, amountToWithdraw, success))
			}
		}
	}
}
