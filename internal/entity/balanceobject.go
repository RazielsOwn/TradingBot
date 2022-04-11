package entity

import (
	"github.com/shopspring/decimal"
)

type BalanceObject struct {
	Currency string          `json:"currencyIsoCode"`
	Balance  decimal.Decimal `json:"balance"`
	Reserved decimal.Decimal `json:"reserved"`
}
