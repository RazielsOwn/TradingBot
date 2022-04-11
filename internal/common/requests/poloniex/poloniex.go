package poloniex

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"
	"trading_bot/config"
	"trading_bot/internal/common"
	"trading_bot/internal/entity"
	"trading_bot/pkg/logger"

	"github.com/shopspring/decimal"
)

type PoloniexRequests struct {
	logger        logger.Interface
	helperMethods common.IHelperMethods
	cacheUpdate   time.Time
	balanceCache  map[string]*entity.BalanceObject
	baseUrl       string
	publicKey     string
	secretKey     string
}

func New(l logger.Interface, hm common.IHelperMethods, cs config.TradingSettings) *PoloniexRequests {
	return &PoloniexRequests{
		logger:        l,
		helperMethods: hm,
		cacheUpdate:   time.Time{},
		balanceCache:  make(map[string]*entity.BalanceObject),
		baseUrl:       cs.Url,
		publicKey:     cs.Key,
		secretKey:     cs.Secret,
	}
}

func (cr *PoloniexRequests) GetTradingBalances(ctx context.Context) map[string]*entity.BalanceObject {
	// 1 minute cache
	if cr.cacheUpdate.Add(1 * time.Minute).After(time.Now().UTC()) {
		return cr.balanceCache
	}

	var balancesStr = cr.queryPrivate(ctx, "returnBalances", map[string]string{})
	if len(balancesStr) == 0 {
		return nil
	}

	var balances map[string]interface{}
	json.Unmarshal([]byte(balancesStr), &balances)

	var res = make(map[string]*entity.BalanceObject)

	for key, val := range balances {
		var bal, _ = decimal.NewFromString(val.(string))
		res[key] = &entity.BalanceObject{
			Currency: key,
			Balance:  bal,
		}
	}

	cr.balanceCache = res
	cr.cacheUpdate = time.Now().UTC()

	return res
}

type orderItem struct {
	Volume decimal.Decimal
	Price  decimal.Decimal
}

func (ord *orderItem) UnmarshalJSON(data []byte) error {
	var v []interface{}
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	ord.Price, _ = decimal.NewFromString(v[0].(string))
	ord.Volume = decimal.NewFromFloat(v[1].(float64))

	return nil
}

func (cr *PoloniexRequests) GetPublicTradingOrders(ctx context.Context, tradingSystemPair string, usdcTradingLimit decimal.Decimal, cryptoTradingLimit decimal.Decimal, internalCryptoBalance decimal.Decimal, internalUsdcBalance decimal.Decimal, pairMinAmount decimal.Decimal) []*entity.TradingOrder {
	var requestData map[string]string = make(map[string]string)
	requestData["currencyPair"] = tradingSystemPair
	requestData["depth"] = "20"

	var tradingOrders = cr.queryPublic(ctx, "returnOrderBook", requestData)
	if len(tradingOrders) == 0 {
		return nil
	}

	var res []*entity.TradingOrder = []*entity.TradingOrder{}

	orders := struct {
		Asks []orderItem `json:"asks"`
		Bids []orderItem `json:"bids"`
	}{}

	err := json.Unmarshal([]byte(tradingOrders), &orders)
	if err != nil {
		return nil
	}

	var amountFound = usdcTradingLimit
	var breakProcess = false
	if amountFound.GreaterThan(decimal.Decimal{}) {
		for _, ask := range orders.Asks {
			if breakProcess {
				break
			}

			var order = &entity.TradingOrder{
				Rate:   ask.Price,
				Amount: ask.Volume,
			}

			if ask.Price.GreaterThan(internalCryptoBalance) {
				order.Amount = internalCryptoBalance
				breakProcess = true
			} else {
				internalCryptoBalance = internalCryptoBalance.Sub(order.Amount)
			}

			amountFound = amountFound.Sub(order.Amount.Mul(order.Rate))

			if amountFound.LessThan(decimal.Decimal{}) {
				order.Amount = order.Amount.Add(amountFound.Div(order.Rate))
				breakProcess = true
			}

			// ignore orders that lower than pair minimum amount
			if order.Amount.LessThanOrEqual(pairMinAmount) {
				continue
			}

			order.IsSellOrder = true

			res = append(res, order)
		}
	}

	amountFound = cryptoTradingLimit
	breakProcess = false
	if amountFound.GreaterThan(decimal.Decimal{}) {
		for _, ask := range orders.Bids {
			if breakProcess {
				break
			}

			var order = &entity.TradingOrder{
				Rate:   ask.Price,
				Amount: ask.Volume,
			}

			if (order.Amount.Mul(order.Rate)).GreaterThan(internalCryptoBalance) {
				order.Amount = internalCryptoBalance
				breakProcess = true
			} else {
				internalCryptoBalance = internalCryptoBalance.Sub(order.Amount)
			}

			if order.Amount.Mul(order.Rate).GreaterThan(internalUsdcBalance) {
				order.Amount = internalUsdcBalance.Div(order.Rate)
				breakProcess = true
			} else {
				internalUsdcBalance = internalUsdcBalance.Sub(order.Amount.Mul(order.Rate))
			}

			amountFound = amountFound.Sub(order.Amount)

			if amountFound.LessThan(decimal.Decimal{}) {
				order.Amount = order.Amount.Add(amountFound)
				breakProcess = true
			}

			// ignore orders that lower than pair minimum amount
			if order.Amount.LessThanOrEqual(pairMinAmount) {
				continue
			}

			order.IsSellOrder = false

			res = append(res, order)
		}
	}

	return res
}

func (cr *PoloniexRequests) Buy(ctx context.Context, tradingSystemPair string, tradingSystemPrice decimal.Decimal, internalPrice decimal.Decimal, amount decimal.Decimal, internalPair string) bool {
	// Poloniex taker fee (trade volume < 26k USD @ 30 days): 0.25%
	var requiredAmount = amount.Mul(decimal.NewFromFloat(1.003))
	var orderPrice = tradingSystemPrice

	for {
		// make request object
		var requestData map[string]string = make(map[string]string)
		requestData["currencyPair"] = tradingSystemPair
		requestData["rate"] = orderPrice.String()
		requestData["fillOrKill"] = "1"
		requestData["amount"] = requiredAmount.String()

		var paymentResponse = cr.queryPrivate(ctx, "buy", requestData)
		if len(paymentResponse) == 0 {
			continue
		}

		cr.logger.Info(fmt.Sprintf("buy response is : %v", paymentResponse))

		orders := struct {
			OrderNumber     string `json:"orderNumber"`
			Fee             string `json:"fee"`
			ClientOrderId   string `json:"clientOrderId"`
			CurrencyPair    string `json:"currencyPair"`
			ResultingTrades []struct {
				Amount          string    `json:"amount"`
				Date            time.Time `json:"date"`
				Rate            string    `json:"rate"`
				Total           string    `json:"total"`
				TradeID         string    `json:"tradeID"`
				Type            string    `json:"type"`
				TakerAdjustment string    `json:"takerAdjustment"`
			} `json:"resultingTrades"`
		}{}

		err := json.Unmarshal([]byte(paymentResponse), &orders)
		if err != nil {
			// handle error response
			var result map[string]interface{}
			json.Unmarshal([]byte(paymentResponse), &result)

			val, found := result["error"]
			if found {

				if strings.Contains(val.(string), "This market is frozen") {
					time.Sleep(10 * time.Second)
					continue
				}
				// too many requests...
				if strings.Contains(val.(string), "This IP has been temporarily throttled.") {
					time.Sleep(10 * time.Second)
					continue
				}
				if strings.Contains(val.(string), "Unable to fill order") {
					// rising price
					orderPrice = orderPrice.Mul(decimal.NewFromFloat(1.001)).RoundDown(8)
					if orderPrice.GreaterThan(internalPrice) {
						// send message to owner
						cr.logger.Error(fmt.Sprintf("can't buy order in Poloniex. Max price reached : %v, amount : %v, pair : %v!", internalPrice, requiredAmount, internalPair))
						break
					}
					continue
				}
			}

			cr.logger.Error(fmt.Sprintf("error on Buy request in Poloniex for pair : %v, amount: %v, response is : %v!", internalPair, requiredAmount, paymentResponse))
			break
		}

		var resultedAmount decimal.Decimal

		for _, trade := range orders.ResultingTrades {
			var am, _ = decimal.NewFromString(trade.TakerAdjustment)
			resultedAmount = resultedAmount.Add(am)
		}

		resultedAmount = resultedAmount.RoundDown(8)
		//check if withdrewAmount different from requiredAmount
		if resultedAmount.LessThan(requiredAmount) {
			cr.logger.Error(fmt.Sprintf("resulted amount in Poloniex : %v, is lower than Required Amount : %v", resultedAmount, requiredAmount))
			return false
		}
		return true
	}

	return false
}

func (cr *PoloniexRequests) Sell(ctx context.Context, tradingSystemPair string, tradingSystemPrice decimal.Decimal, internalPrice decimal.Decimal, amount decimal.Decimal, internalPair string) bool {

	var requiredAmount = amount.Mul(internalPrice).RoundDown(8)
	var orderPrice = tradingSystemPrice

	for {
		// make request object
		var requestData map[string]string = make(map[string]string)
		requestData["currencyPair"] = tradingSystemPair
		requestData["rate"] = orderPrice.String()
		requestData["fillOrKill"] = "1"
		requestData["amount"] = requiredAmount.String()

		var paymentResponse = cr.queryPrivate(ctx, "sell", requestData)
		if len(paymentResponse) == 0 {
			continue
		}

		cr.logger.Info(fmt.Sprintf("sell response is : %v", paymentResponse))

		orders := struct {
			OrderNumber     string `json:"orderNumber"`
			Fee             string `json:"fee"`
			ClientOrderId   string `json:"clientOrderId"`
			CurrencyPair    string `json:"currencyPair"`
			ResultingTrades []struct {
				Amount          string    `json:"amount"`
				Date            time.Time `json:"date"`
				Rate            string    `json:"rate"`
				Total           string    `json:"total"`
				TradeID         string    `json:"tradeID"`
				Type            string    `json:"type"`
				TakerAdjustment string    `json:"takerAdjustment"`
			} `json:"resultingTrades"`
		}{}

		err := json.Unmarshal([]byte(paymentResponse), &orders)
		if err != nil {
			// handle error response
			var result map[string]interface{}
			json.Unmarshal([]byte(paymentResponse), &result)

			val, found := result["error"]
			if found {

				if strings.Contains(val.(string), "This market is frozen") {
					time.Sleep(10 * time.Second)
					continue
				}
				// too many requests...
				if strings.Contains(val.(string), "This IP has been temporarily throttled.") {
					time.Sleep(10 * time.Second)
					continue
				}
				if strings.Contains(val.(string), "Unable to fill order") {
					// rising price
					orderPrice = orderPrice.Mul(decimal.NewFromFloat(0.999)).RoundDown(8)
					if orderPrice.LessThan(internalPrice) {
						// send message to owner
						cr.logger.Error(fmt.Sprintf("can't sell order in Poloniex. Max price reached : %v, amount : %v, pair : %v!", internalPrice, requiredAmount, internalPair))
						break
					}
					continue
				}
			}

			cr.logger.Error(fmt.Sprintf("error on Buy request in Poloniex for pair : %v, amount: %v, response is : %v!", internalPair, requiredAmount, paymentResponse))
			break
		}

		var resultedAmount decimal.Decimal

		for _, trade := range orders.ResultingTrades {
			var am, _ = decimal.NewFromString(trade.TakerAdjustment)
			resultedAmount = resultedAmount.Add(am)
		}

		resultedAmount = resultedAmount.RoundDown(8)
		//check if withdrewAmount different from requiredAmount
		if resultedAmount.LessThan(requiredAmount) {
			// send message to owner
			cr.logger.Error(fmt.Sprintf("resulted amount in Poloniex : %v, is lower than Required Amount : %v", resultedAmount, requiredAmount))
			return false
		}
		return true
	}

	return false
}

func (pr *PoloniexRequests) Withdraw(ctx context.Context, addr string, withdrawalAmount decimal.Decimal, currency string, tradingSystemWithdrawalNetwork string) bool {
	// make request object
	var requestData map[string]string = make(map[string]string)
	requestData["address"] = addr
	requestData["amount"] = withdrawalAmount.String()
	requestData["currency"] = currency
	if len(tradingSystemWithdrawalNetwork) > 0 {
		requestData["currencyToWithdrawAs"] = tradingSystemWithdrawalNetwork
	}

	pr.logger.Info("withdraw request is : %v", requestData)
	var paymentResponse = pr.queryPrivate(ctx, "withdraw", requestData)
	if len(paymentResponse) == 0 {
		return false
	}

	// handle error response
	var result map[string]interface{}
	json.Unmarshal([]byte(paymentResponse), &result)
	val, found := result["error"]
	if found {
		pr.logger.Error(fmt.Sprintf("error on Poloniex 'withdraw' response is : %v", val))
		return false
	}

	return true
}

func (pr *PoloniexRequests) GetCryptoAddress(ctx context.Context, currency string, tradingSystemWithdrawalNetwork string) string {

	var addresses = pr.queryPrivate(ctx, "returnDepositAddresses", map[string]string{})
	if len(addresses) == 0 {
		return ""
	}

	if tradingSystemWithdrawalNetwork != "" {
		currency = tradingSystemWithdrawalNetwork
	}

	var result map[string]interface{}
	json.Unmarshal([]byte(addresses), &result)
	val, found := result[currency]
	if found {
		return val.(string)
	}
	return ""
}

func (cr *PoloniexRequests) queryPublic(ctx context.Context, method string, requestData map[string]string) string {

	requestData["command"] = method
	u, err := url.Parse(cr.baseUrl + "/public")
	if err != nil {
		cr.logger.Error(err)
		return ""
	}

	q := u.Query()
	if len(requestData) > 0 {
		for key, val := range requestData {
			q.Set(key, val)
		}
	}
	u.RawQuery = q.Encode()

	var resText, statusCode, err1 = cr.helperMethods.HttpGet(ctx, u, "", map[string]string{})

	if statusCode != 200 {
		cr.logger.Error(fmt.Sprintf("response status : %v, %v : %v", statusCode, err1, resText))
		return ""
	}

	return resText
}

func (pr *PoloniexRequests) queryPrivate(ctx context.Context, method string, requestData map[string]string) string {

	requestData["command"] = method
	var rawResponse = ""
	var statusCode = 500

	// 10 chances to process
	for i := 0; i < 10; i++ {
		// generate a 64 bit nonce using a timestamp at tick resolution
		var nonce = time.Now().UnixNano()

		rawResponse, statusCode = pr.doRequest(ctx, requestData, nonce)

		if statusCode == 200 {
			break
		}

		// handle nonce error
		var result map[string]interface{}
		json.Unmarshal([]byte(rawResponse), &result)

		val, found := result["error"]
		if found {
			rawResponse = ""
			if strings.Contains(val.(string), "Nonce must be greater than") {
				continue
			}
		}

		break
	}

	return rawResponse
}

func (pr *PoloniexRequests) doRequest(ctx context.Context, requestData map[string]string, nonce int64) (string, int) {

	requestData["nonce"] = strconv.FormatInt(nonce, 10)
	u, err := url.Parse(pr.baseUrl + "/tradingApi")
	if err != nil {
		pr.logger.Error(err)
		return "", 500
	}

	q := u.Query()
	if len(requestData) > 0 {
		for key, val := range requestData {
			q.Set(key, val)
		}
	}
	u.RawQuery = q.Encode()

	var keyBytes = []byte(pr.secretKey)
	var dataBytes = []byte(u.RawQuery)

	var hash512Bytes, _ = pr.helperMethods.HmacSha512(keyBytes, dataBytes)

	var signature = fmt.Sprintf("%x", hash512Bytes)

	var webHeaders = map[string]string{
		"Key":  pr.publicKey,
		"Sign": signature,
	}

	var resText, statusCode, err1 = pr.helperMethods.HttpPost(ctx, u, dataBytes, "application/x-www-form-urlencoded", webHeaders)

	if statusCode != 200 {
		pr.logger.Error(fmt.Sprintf("response status : %v, %v : %v", statusCode, err1, resText))
	}

	return resText, statusCode
}
