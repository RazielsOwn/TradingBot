package entity

import (
	"github.com/shopspring/decimal"
)

type TradingOrder struct {
	Rate        decimal.Decimal
	Amount      decimal.Decimal
	IsSellOrder bool
}
