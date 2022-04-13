package worker

import (
	"context"
	"sync"
	"testing"
	"trading_bot/config"
	"trading_bot/mocks"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/mock"
	"gopkg.in/guregu/null.v4"
)

func balanceWorker(t *testing.T) *BalanceWorker {
	t.Helper()

	var err = make(chan error, 1)
	var wg = &sync.WaitGroup{}

	var currencySettings = config.CryptoCurrency{
		CurrencyId:       2001,
		BalancePercent:   decimal.NewFromFloat(0.8),
		ThresholdPercent: decimal.NewFromFloat(0.1),
		ThresholdAbs:     decimal.NewFromFloat(0.2),
		SellMultiplier:   decimal.NewFromFloat(1.005),
		BuyMultiplier:    decimal.NewFromFloat(0.995),
		TimeoutMinutes:   60,
		InternalSettings: config.InternalSettings{
			Url:            "",
			Key:            "",
			Secret:         "",
			Pair:           "BTC,USDC",
			Currency:       "BTC",
			CryptoAddress:  "",
			UsdcUsageLimit: decimal.NewFromFloat(0.4),
		},
		TradingSettings: config.TradingSettings{
			Url:            "https://poloniex.com",
			Key:            "",
			Secret:         "",
			Pair:           "USDC_BTC",
			Currency:       "BTC",
			CryptoAddress:  "",
			UsdcUsageLimit: decimal.NewFromFloat(0.8),
		},
	}

	var l = &mocks.ILogger{}
	l.On("Info", mock.Anything).Return()
	l.On("Info", mock.Anything, mock.Anything).Return()
	l.On("Info", mock.Anything, mock.Anything, mock.Anything).Return()
	l.On("Info", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return()
	l.On("Info", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return()
	l.On("Info", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return()

	var tradingSystemRequests = &mocks.ITradingSystemRequest{}
	tradingSystemRequests.On("Withdraw", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(true)

	var internalRequests = &mocks.IInternalRequest{}
	internalRequests.On("Withdraw", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(null.NewInt(10, true))

	return &BalanceWorker{
		notify:                err,
		running:               false,
		logger:                l,
		settings:              currencySettings,
		tradingSystemRequests: tradingSystemRequests,
		internalRequests:      internalRequests,
		WaitGroup:             wg,
	}
}

func TestTransferLogic_TradingSystemWithdraw_Success(t *testing.T) {
	t.Parallel()

	var bw = balanceWorker(t)

	var diffABS = decimal.NewFromFloat32(100)
	var thresholdAbs = decimal.NewFromFloat32(1)
	var tradingBalance = decimal.NewFromFloat32(100)
	var totalBalanceLower = decimal.NewFromFloat32(90)
	var internalBalance = decimal.NewFromFloat32(100)
	var totalBalanceUpper = decimal.NewFromFloat32(110)

	got := bw.TransferLogic(diffABS, thresholdAbs, tradingBalance, totalBalanceLower, internalBalance, totalBalanceUpper, context.Background())
	want := true

	if got != want {
		t.Errorf("got %t, wanted %t", got, want)
	}
}

func TestTransferLogic_InternalWithdraw_Success(t *testing.T) {
	t.Parallel()

	var bw = balanceWorker(t)

	var diffABS = decimal.NewFromFloat32(100)
	var thresholdAbs = decimal.NewFromFloat32(1)
	var tradingBalance = decimal.NewFromFloat32(100)
	var totalBalanceLower = decimal.NewFromFloat32(80)
	var internalBalance = decimal.NewFromFloat32(100)
	var totalBalanceUpper = decimal.NewFromFloat32(90)

	got := bw.TransferLogic(diffABS, thresholdAbs, tradingBalance, totalBalanceLower, internalBalance, totalBalanceUpper, context.Background())
	want := true

	if got != want {
		t.Errorf("got %t, wanted %t", got, want)
	}
}
