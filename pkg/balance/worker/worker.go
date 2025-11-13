package worker

import (
	"context"
	"errors"
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
	"gopkg.in/guregu/null.v4"
)

type BalanceWorker struct {
	logger                logger.ILogger
	settings              config.CryptoCurrency
	tradingSystemRequests common.ITradingSystemRequest
	internalRequests      common.IInternalRequest
	waitGroup             *sync.WaitGroup
	notify                chan error
	running               bool
}

func New(ctx context.Context, wg *sync.WaitGroup, currencySettings config.CryptoCurrency, l logger.ILogger, err chan error) (*BalanceWorker, error) {
	s := &BalanceWorker{
		notify:                err,
		running:               false,
		logger:                l,
		settings:              currencySettings,
		tradingSystemRequests: tradingsystemReq.New(l, helpermethods.New(l), currencySettings.TradingSettings),
		internalRequests:      jetcryptoReq.New(l, helpermethods.New(l), currencySettings.InternalSettings),
		waitGroup:             wg,
	}

	go func(bw *BalanceWorker) {
		bw.DoWork(ctx)
	}(s)

	return s, nil
}

// Start worker
func (s *BalanceWorker) Start() {
	s.waitGroup.Add(1)
	s.running = true
	s.logger.Debug("Start BalanceWorker called")
}

// Shutdown -.
func (s *BalanceWorker) Stop() {
	s.running = false
	s.waitGroup.Done()
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

		if len(s.settings.TradingSettings.CryptoAddress) == 0 {
			// try to get trading system crypto address
			s.settings.TradingSettings.CryptoAddress = s.tradingSystemRequests.GetCryptoAddress(ctx, s.settings.TradingSettings.Currency, s.settings.TradingSettings.WithdrawalNetwork)
			if len(s.settings.TradingSettings.CryptoAddress) == 0 {
				var errStr = fmt.Sprintf("Balancer %v : Can't get Trading system CryptoAddress!", s.settings.TradingSettings.Currency)
				s.logger.Error(errStr)
				s.notify <- errors.New(errStr)
				break
			}
		}

		if len(s.settings.InternalSettings.CryptoAddress) == 0 {
			// try to get internal crypto address
			s.settings.InternalSettings.CryptoAddress = s.internalRequests.GetCryptoAddress(ctx, s.settings.InternalSettings.Currency)
			if len(s.settings.InternalSettings.CryptoAddress) == 0 {
				var errStr = fmt.Sprintf("Balancer %v : Can't get Internal CryptoAddress!", s.settings.InternalSettings.Currency)
				s.logger.Error(errStr)
				s.notify <- errors.New(errStr)
				break
			}
		}

		var internalBalanceCache = s.internalRequests.GetBalances(ctx)
		if len(internalBalanceCache) == 0 {
			s.logger.Error("Balancer Error : Can't get own internalBalances!!!")
			continue
		}

		var intBalance, found = internalBalanceCache[s.settings.InternalSettings.Currency]
		if !found {
			s.logger.Error("Balancer Error : Can't get own internalBalance!!!")
			continue
		}

		var internalBalance = intBalance.Balance.Add(intBalance.Reserved)

		var tradingBalanceCache = s.tradingSystemRequests.GetTradingBalances(ctx)
		if len(tradingBalanceCache) == 0 {
			s.logger.Error("Balancer Error : Can't get tradingSystemBalances!!!")
			continue
		}

		var tsBalance, found1 = tradingBalanceCache[s.settings.TradingSettings.Currency]
		if !found1 {
			s.logger.Error("Balancer Error : Can't get tradingSystemBalance!!!")
			continue
		}

		var tradingBalance = tsBalance.Balance
		s.logger.Debug("Balancer %v tradingBalance is : %v, internalBalance is : %v", s.settings.TradingSettings.Currency, tradingBalance, internalBalance)

		var totalBalance = tradingBalance.Add(internalBalance).RoundDown(8)
		var diffABS = tradingBalance.Sub(totalBalance.Mul((decimal.NewFromInt(1).Sub(s.settings.BalancePercent)))).Abs().RoundDown(8)
		var totalBalanceLower = totalBalance.Mul((decimal.NewFromInt(1).Sub(s.settings.BalancePercent.Sub(s.settings.ThresholdPercent)))).RoundDown(8)
		var totalBalanceUpper = totalBalance.Mul((s.settings.BalancePercent.Add(s.settings.ThresholdPercent))).RoundDown(8)

		s.transferLogic(diffABS, s.settings.ThresholdAbs, tradingBalance, totalBalanceLower, internalBalance, totalBalanceUpper, ctx)
	}
}

func (s *BalanceWorker) transferLogic(diffABS decimal.Decimal, thresholdAbs decimal.Decimal, tradingBalance decimal.Decimal, totalBalanceLower decimal.Decimal, internalBalance decimal.Decimal, totalBalanceUpper decimal.Decimal, ctx context.Context) bool {
	var success = false

	if diffABS.GreaterThan(thresholdAbs) && (tradingBalance.GreaterThan(totalBalanceLower) || internalBalance.GreaterThan(totalBalanceUpper)) {
		s.logger.Info("Balancer %v tradingBalance is : %v, internalBalance is : %v", s.settings.InternalSettings.Currency, tradingBalance, internalBalance)

		var internalToTradingSystem = false
		if internalBalance.GreaterThan(totalBalanceUpper) {
			internalToTradingSystem = true
		}

		if internalToTradingSystem {
			s.logger.Info("Balancer %v diffABS is : %v > thresholdAbs : %v AND internalBalance : %v > totalBalanceUpper %v starting Balancer!", s.settings.InternalSettings.Currency, diffABS, thresholdAbs, internalBalance, totalBalanceUpper)

			// sending currency internal -> trading system
			var amountToWithdraw = diffABS
			s.logger.Info("Balancer %v : Creating withdraw order Internal -> Trading system, amountToWithdraw %v", s.settings.InternalSettings.Currency, amountToWithdraw)

			var paymentId = s.internalRequests.Withdraw(ctx, s.settings.TradingSettings.CryptoAddress, s.settings.TradingSettings.DestinationTag, amountToWithdraw, strconv.Itoa(s.settings.CurrencyId))
			s.logger.Info("Balancer %v : Withdraw order Internal -> Trading system, amountToWithdraw %v result PaymentId is : %v", s.settings.InternalSettings.Currency, amountToWithdraw, paymentId)
			success = paymentId != null.Int{}
		} else {
			s.logger.Info("Balancer %v diffABS is : %v > thresholdAbs : %v AND tradingBalance : %v > totalBalanceLower %v starting Balancer!", s.settings.TradingSettings.Currency, diffABS, thresholdAbs, tradingBalance, totalBalanceLower)

			// receiving currency trading system -> internal
			var amountToWithdraw = diffABS
			s.logger.Info("Balancer %v : Creating withdraw order Trading system -> Internal, amountToWithdraw %v", s.settings.TradingSettings.Currency, amountToWithdraw)

			success = s.tradingSystemRequests.Withdraw(ctx, s.settings.InternalSettings.CryptoAddress, amountToWithdraw, s.settings.TradingSettings.Currency, s.settings.TradingSettings.WithdrawalNetwork)
			s.logger.Info("Balancer %v : Withdraw order Trading system -> Internal, amountToWithdraw %v result is : %t", s.settings.InternalSettings.Currency, amountToWithdraw, success)
		}

	}

	return success
}
