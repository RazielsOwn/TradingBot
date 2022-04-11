package common

import (
	"context"
	"net/url"
	"trading_bot/internal/entity"

	"github.com/gofrs/uuid"
	"github.com/shopspring/decimal"
	"gopkg.in/guregu/null.v4"
)

type (
	IHelperMethods interface {
		HttpGet(ctx context.Context, url *url.URL, contentType string, webHeaders map[string]string) (string, int, error)
		HttpPost(ctx context.Context, url *url.URL, data []byte, contentType string, webHeaders map[string]string) (string, int, error)
		HmacSha512(keyBytes []byte, messageBytes []byte) ([]byte, error)
	}

	ITradingSystemRequest interface {
		GetTradingBalances(ctx context.Context) map[string]*entity.BalanceObject
		GetPublicTradingOrders(ctx context.Context, tradingSystemPair string, usdcTradingLimit decimal.Decimal, cryptoTradingLimit decimal.Decimal, internalCryptoBalance decimal.Decimal, internalUsdcBalance decimal.Decimal, pairMinAmount decimal.Decimal) []*entity.TradingOrder
		Buy(ctx context.Context, tradingSystemPair string, tradingSystemPrice decimal.Decimal, internalPrice decimal.Decimal, amount decimal.Decimal, internalPair string) bool
		Sell(ctx context.Context, tradingSystemPair string, tradingSystemPrice decimal.Decimal, internalPrice decimal.Decimal, amount decimal.Decimal, internalPair string) bool
		Withdraw(ctx context.Context, addr string, withdrawalAmount decimal.Decimal, currency string, tradingSystemWithdrawalNetwork string) bool
		GetCryptoAddress(ctx context.Context, currency string, tradingSystemWithdrawalNetwork string) string
	}

	IInternalRequest interface {
		GetOrders(ctx context.Context, jetCryptoPair string) map[uuid.UUID]*entity.InternalOrder
		GetOrder(ctx context.Context, orderId uuid.UUID, jetCryptoPair string) *entity.InternalOrder
		IsPaymentCompleted(ctx context.Context, orderId uuid.UUID) bool
		GetCompleteOrder(ctx context.Context, orderId uuid.UUID, jetCryptoPair string) []*entity.InternalOrder
		GetBalances(ctx context.Context) map[string]*entity.BalanceObject
		GetTradingPairInfo(ctx context.Context, jetCryptoPair string) decimal.Decimal
		Withdraw(ctx context.Context, addr string, destinationTag string, withdrawalAmount decimal.Decimal, currentCurrencyId string) null.Int
		GetCryptoAddress(ctx context.Context, currency string) string
	}
)
