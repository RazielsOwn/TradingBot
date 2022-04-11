package entity

import (
	"github.com/gofrs/uuid"
	"github.com/shopspring/decimal"
)

type InternalOrder struct {
	Id          uuid.UUID       `json:"id"`
	Amount      decimal.Decimal `json:"initialAmount"`
	AmountLeft  decimal.Decimal `json:"amountLeft"`
	Price       decimal.Decimal `json:"price"`
	IsSellOrder bool            `json:"isSellOrder"`
}
