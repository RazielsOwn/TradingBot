package worker

import (
	"context"
	"strings"
	"sync"
	"time"
	"trading_bot/config"
	"trading_bot/internal/common"
	"trading_bot/internal/common/helpermethods"
	"trading_bot/internal/entity"
	"trading_bot/pkg/logger"

	jetcryptoReq "trading_bot/internal/common/requests/jetcrypto"
	tradingsystemReq "trading_bot/internal/common/requests/poloniex"

	"github.com/gofrs/uuid"
	"github.com/shopspring/decimal"
)

type TradingWorker struct {
	logger                logger.ILogger
	settings              config.CryptoCurrency
	tradingSystemRequests common.ITradingSystemRequest
	internalRequests      common.IInternalRequest
	waitGroup             *sync.WaitGroup
	internalOrdersCache   map[uuid.UUID]*tradingOrderPair
	pairMinAmount         decimal.Decimal
	notify                chan error
	running               bool
}

type tradingOrderPair struct {
	InternalId          uuid.UUID
	InternalAmount      decimal.Decimal
	InternalPrice       decimal.Decimal
	TradingSystemAmount decimal.Decimal
	TradingSystemPrice  decimal.Decimal
	IsSellOrder         bool
}

func New(ctx context.Context, wg *sync.WaitGroup, currencySettings config.CryptoCurrency, l logger.ILogger, err chan error) (*TradingWorker, error) {
	s := &TradingWorker{
		notify:                err,
		running:               false,
		logger:                l,
		settings:              currencySettings,
		tradingSystemRequests: tradingsystemReq.New(l, helpermethods.New(l), currencySettings.TradingSettings),
		internalRequests:      jetcryptoReq.New(l, helpermethods.New(l), currencySettings.InternalSettings),
		waitGroup:             wg,
		internalOrdersCache:   make(map[uuid.UUID]*tradingOrderPair),
	}

	go func(bw *TradingWorker) {
		bw.DoWork(ctx)
	}(s)

	return s, nil
}

// Start worker
func (s *TradingWorker) Start() {
	s.pairMinAmount = s.internalRequests.GetTradingPairInfo(context.Background(), s.settings.InternalSettings.Pair)
	s.waitGroup.Add(1)
	s.running = true
	s.logger.Debug("Start TradingWorker called")
}

// Shutdown -.
func (s *TradingWorker) Stop() {
	s.waitGroup.Done()
	s.running = false
	s.logger.Debug("Stop TradingWorker called")
}

func (s *TradingWorker) DoWork(ctx context.Context) {
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

		var tradingBalanceCache = s.tradingSystemRequests.GetTradingBalances(ctx)
		if len(tradingBalanceCache) == 0 {
			s.logger.Error("TradingWorker Error : Can't get tradingSystemBalances!!!")
			continue
		}

		var internalBalanceCache = s.internalRequests.GetBalances(ctx)
		if len(internalBalanceCache) == 0 {
			s.logger.Error("TradingWorker Error : Can't get own internalBalances!!!")
			continue
		}

		// first time or empty cache
		if len(s.internalOrdersCache) == 0 {
			var internalOrders = s.internalRequests.GetOrders(ctx, s.settings.InternalSettings.Pair)
			if len(internalOrders) == 0 {
				s.logger.Error("TradingWorker Error : Can't get own internalOrders!!!")
				continue
			}
			for key, item := range internalOrders {
				var newPair = &tradingOrderPair{
					InternalId:          item.Id,
					InternalAmount:      item.Amount,
					InternalPrice:       item.Price,
					TradingSystemAmount: item.Amount,
					IsSellOrder:         item.IsSellOrder,
				}
				// remove 1% from price
				if newPair.IsSellOrder {
					newPair.TradingSystemPrice = newPair.InternalPrice.Mul(s.settings.BuyMultiplier).RoundDown(8)
				} else {
					newPair.TradingSystemPrice = newPair.InternalPrice.Mul(s.settings.SellMultiplier).RoundDown(8)
				}

				s.internalOrdersCache[key] = newPair
			}
		}

		var errorState = s.removeOldOrders(ctx)
		if errorState {
			continue
		}

		// clear orders cache
		s.internalOrdersCache = make(map[uuid.UUID]*tradingOrderPair)

		var intBalance, found = internalBalanceCache[s.settings.InternalSettings.Currency]
		if !found {
			s.logger.Error("TradingWorker Error : Can't get own internalBalance!!!")
			continue
		}
		var internalBalance = intBalance.Balance

		var intUSDCBalance, found1 = internalBalanceCache["USDC"]
		if !found1 {
			s.logger.Error("TradingWorker Error : Can't get own USDC balance!!!")
			continue
		}
		var internalUSDCBalance = intUSDCBalance.Balance.Mul(s.settings.InternalSettings.UsdcUsageLimit).RoundDown(8)

		var tsBalance, found2 = tradingBalanceCache[s.settings.TradingSettings.Currency]
		if !found2 {
			s.logger.Error("TradingWorker Error : Can't get tradingSystemBalance!!!")
			continue
		}
		var cryptoTradingLimit = tsBalance.Balance

		var tsUSDCBalance, found3 = tradingBalanceCache["USDC"]
		if !found3 {
			s.logger.Error("TradingWorker Error : Can't get tradingSystem USDC Balance!!!")
			continue
		}
		var usdcTradingLimit = tsUSDCBalance.Balance.Mul(s.settings.TradingSettings.UsdcUsageLimit).RoundDown(8)

		// get trading system orders
		var allTradingOrders = s.tradingSystemRequests.GetPublicTradingOrders(ctx, s.settings.TradingSettings.Pair, usdcTradingLimit, cryptoTradingLimit, internalBalance, internalUSDCBalance, s.pairMinAmount)
		if len(allTradingOrders) == 0 {
			s.logger.Error("TradingWorker %v : getAllTradingOrders empty response!", s.settings.InternalSettings.Currency)
			allTradingOrders = make([]*entity.TradingOrder, 0)
		}

		// 2) Add orders from trading system to internal system
		for _, tradingOrder := range allTradingOrders {
			// add new internal order
			if !s.addNewOrderPair(ctx, tradingOrder) {
				// if error just continue
				s.logger.Error("TradingWorker %v :  Error on add order to Internal system : %v!", s.settings.InternalSettings.Currency, tradingOrder)
			}
		}
	}
}

func (s *TradingWorker) removeOldOrders(ctx context.Context) bool {
	var errorState = false
	// removing old orders
	for key, currentOrder := range s.internalOrdersCache {
		if !s.removeInternalOrder(ctx, key) {
			// checking of order is totally spent
			var completedOrderInfos = s.internalRequests.GetCompleteOrder(ctx, key, s.settings.InternalSettings.Pair)
			if len(completedOrderInfos) == 0 {
				s.logger.Error("TradingWorker Error : Can't cancel internal order : %v", key)
				errorState = true
				break
			}

			// checking of order is partially spent
			completedOrderInfos = s.internalRequests.GetCompleteOrder(ctx, key, s.settings.InternalSettings.Pair)
			if len(completedOrderInfos) > 0 {
				var completedAmount = decimal.Decimal{}

				for _, item := range completedOrderInfos {
					completedAmount = completedAmount.Add(item.Amount)
				}

				// create order in trading system
				var operationResult = false
				s.logger.Info("TradingWorker %v : Creating new order in Trading API for params : amount : %v, price : %v", s.settings.InternalSettings.Pair, completedAmount, currentOrder.TradingSystemPrice)
				if currentOrder.IsSellOrder {
					operationResult = s.tradingSystemRequests.Buy(ctx, s.settings.TradingSettings.Pair, currentOrder.TradingSystemPrice, currentOrder.InternalPrice, completedAmount, s.settings.InternalSettings.Pair)
				} else {
					operationResult = s.tradingSystemRequests.Sell(ctx, s.settings.TradingSettings.Pair, currentOrder.TradingSystemPrice, currentOrder.InternalPrice, completedAmount, s.settings.InternalSettings.Pair)
				}
				s.logger.Info("TradingWorker %v : New order in Trading API for params : amount : %v, price : %v result is : %t", s.settings.InternalSettings.Pair, completedAmount, currentOrder.TradingSystemPrice, operationResult)
			}
		}
	}
	return errorState
}

func (s *TradingWorker) removeInternalOrder(ctx context.Context, orderId uuid.UUID) bool {

	var currencies = strings.Split(s.settings.InternalSettings.Pair, ",")
	var currFrom = currencies[0]
	var currTo = currencies[1]

	var deleteRes = s.internalRequests.RemoveOrder(ctx, orderId, currFrom, currTo)

	return deleteRes
}

func (s *TradingWorker) addNewOrderPair(ctx context.Context, order *entity.TradingOrder) bool {

	var newOrder = &tradingOrderPair{
		TradingSystemAmount: order.Amount,
		InternalAmount:      order.Amount,
		TradingSystemPrice:  order.Rate,
		IsSellOrder:         order.IsSellOrder,
	}
	// Add 1% to price
	if newOrder.IsSellOrder {
		newOrder.InternalPrice = newOrder.TradingSystemPrice.Mul(s.settings.SellMultiplier).RoundDown(8)
	} else {
		newOrder.InternalPrice = newOrder.TradingSystemPrice.Mul(s.settings.BuyMultiplier).RoundDown(8)
	}

	var currencies = strings.Split(s.settings.InternalSettings.Pair, ",")
	var currFrom = currencies[0]
	var currTo = currencies[1]

	var success, id = s.internalRequests.AddOrder(ctx, currFrom, currTo, newOrder.InternalAmount, newOrder.InternalPrice, newOrder.IsSellOrder)
	if success {
		newOrder.InternalId = id
	}
	// save order to cache
	s.internalOrdersCache[newOrder.InternalId] = newOrder

	return success
}
