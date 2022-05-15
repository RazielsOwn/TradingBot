package jetcrypto

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"time"
	"trading_bot/config"
	"trading_bot/internal/common"
	"trading_bot/internal/entity"
	"trading_bot/pkg/logger"

	"github.com/gofrs/uuid"
	"github.com/shopspring/decimal"
	"gopkg.in/guregu/null.v4"
)

type JetCryptoRequests struct {
	logger        logger.ILogger
	helperMethods common.IHelperMethods
	cacheUpdate   time.Time
	balancesCache []*entity.BalanceObject
	baseUrl       string
	publicKey     string
	secretKey     string
}

func New(l logger.ILogger, hm common.IHelperMethods, cs config.InternalSettings) *JetCryptoRequests {
	return &JetCryptoRequests{
		logger:        l,
		helperMethods: hm,
		cacheUpdate:   time.Time{},
		balancesCache: []*entity.BalanceObject{},
		baseUrl:       cs.Url,
		publicKey:     cs.Key,
		secretKey:     cs.Secret,
	}
}

func (jc *JetCryptoRequests) GetOrders(ctx context.Context, jetCryptoPair string) map[uuid.UUID]*entity.InternalOrder {
	// make request object
	var requestData map[string]string = make(map[string]string)
	requestData["tradingPair"] = jetCryptoPair

	// get JetCrypto orders
	var jetCryptoOrders, statusCode = jc.query(ctx, "api/Trading/ActiveOrders", "get", requestData)

	if statusCode == 503 {
		jc.logger.Error("JetCrypto : error on GetOrders - ServiceUnavailable!")
		return nil
	}
	if len(jetCryptoOrders) == 0 {
		return nil
	}

	var orders []*entity.InternalOrder
	err := json.Unmarshal([]byte(jetCryptoOrders), &orders)
	if err != nil {
		jc.logger.Error("JetCrypto : error on GetOrders - empty response for tradingPair: %v!", jetCryptoPair)
		return nil
	}

	result := map[uuid.UUID]*entity.InternalOrder{}
	for _, val := range orders {
		result[val.Id] = val
	}

	return result
}

func (jc *JetCryptoRequests) GetCompleteOrder(ctx context.Context, orderId uuid.UUID, jetCryptoPair string) []*entity.InternalOrder {
	// make request object
	var requestData map[string]string = make(map[string]string)
	requestData["tradingPair"] = jetCryptoPair
	requestData["orderId"] = orderId.String()

	// get JetCrypto orders
	var jetCryptoOrders, statusCode = jc.query(ctx, "api/Trading/CompletedOrderInfo", "get", requestData)

	if statusCode == 503 {
		jc.logger.Error("JetCrypto : error on GetCompleteOrder - ServiceUnavailable!")
		return nil
	}
	if len(jetCryptoOrders) == 0 {
		return nil
	}

	var orders []*entity.InternalOrder
	err := json.Unmarshal([]byte(jetCryptoOrders), &orders)
	if err != nil {
		jc.logger.Error("JetCrypto : error on GetCompleteOrder - empty response for tradingPair: %v!", jetCryptoPair)
		return nil
	}

	return orders
}

func (jc *JetCryptoRequests) GetBalances(ctx context.Context) map[string]*entity.BalanceObject {
	// make request object
	var requestData map[string]string = make(map[string]string)
	requestData["page"] = "1"
	requestData["itemsPerPage"] = "1000"

	// get JetCrypto orders
	var jetCryptoOrders, statusCode = jc.query(ctx, "api/Device/UserAccount", "get", requestData)

	if statusCode == 503 {
		jc.logger.Error("JetCrypto : error on GetBalances - ServiceUnavailable!")
		return nil
	}
	if len(jetCryptoOrders) == 0 {
		return nil
	}

	var balances []*entity.BalanceObject
	err := json.Unmarshal([]byte(jetCryptoOrders), &balances)
	if err != nil {
		jc.logger.Error("JetCrypto : error on GetBalances - empty response!")
		return nil
	}

	result := map[string]*entity.BalanceObject{}
	for _, val := range balances {
		result[val.Currency] = val
	}

	return result
}

func (jc *JetCryptoRequests) GetOrder(ctx context.Context, orderId uuid.UUID, jetCryptoPair string) *entity.InternalOrder {
	// make request object
	var requestData map[string]string = make(map[string]string)
	requestData["tradingPair"] = jetCryptoPair
	requestData["orderId"] = orderId.String()

	// get JetCrypto order
	var jetCryptoOrder, statusCode = jc.query(ctx, "api/Trading/OrderInfo", "get", requestData)

	if statusCode == 503 {
		jc.logger.Error("JetCrypto : error on GetOrder - ServiceUnavailable!")
		return nil
	}
	if len(jetCryptoOrder) == 0 {
		return nil
	}

	var order *entity.InternalOrder
	err := json.Unmarshal([]byte(jetCryptoOrder), &order)
	if err != nil {
		jc.logger.Error("JetCrypto : error on GetOrder - empty response for tradingPair: %v, orderId:%v !", jetCryptoPair, orderId)
		return nil
	}

	return order
}

func (jc *JetCryptoRequests) IsPaymentCompleted(ctx context.Context, orderId uuid.UUID) bool {
	// make request object
	var requestData map[string]string = make(map[string]string)
	requestData["orderId"] = orderId.String()

	// get JetCrypto order
	var jetCryptoOrders, statusCode = jc.query(ctx, "api/Trovemat/Payment", "get", requestData)

	if statusCode == 503 {
		jc.logger.Error("JetCrypto : error on IsPaymentCompleted - ServiceUnavailable!")
		return false
	}
	if len(jetCryptoOrders) == 0 {
		return false
	}

	order := struct {
		StatusId int `json:"statusId"`
	}{}
	err := json.Unmarshal([]byte(jetCryptoOrders), &order)
	if err != nil {
		jc.logger.Error("JetCrypto : error on IsPaymentCompleted - empty response for orderId:%v !", orderId)
		return false
	}

	return order.StatusId >= 2
}

func (jc *JetCryptoRequests) GetCryptoAddress(ctx context.Context, currency string) string {
	// make request object
	var requestData map[string]string = make(map[string]string)
	requestData["currencyName"] = currency

	// get JetCrypto order
	var jetCryptoOrders, statusCode = jc.query(ctx, "api/Trovemat/UserAccount/getCryptoAddress", "get", requestData)

	if statusCode == 503 {
		jc.logger.Error("JetCrypto : error on GetCryptoAddress - ServiceUnavailable!")
		return ""
	}
	if len(jetCryptoOrders) == 0 {
		return ""
	}

	result := struct {
		CryptoAddress string `json:"cryptoAddress"`
	}{}
	err := json.Unmarshal([]byte(jetCryptoOrders), &result)
	if err != nil {
		jc.logger.Error("JetCrypto : error on GetCryptoAddress - empty response for currencyName:%v !", currency)
		return ""
	}

	return result.CryptoAddress
}

func (jc *JetCryptoRequests) GetTradingPairInfo(ctx context.Context, jetCryptoPair string) decimal.Decimal {
	// make request object
	var requestData map[string]string = make(map[string]string)
	requestData["tradingPair"] = jetCryptoPair

	// get JetCrypto order
	var jetCryptoOrders, statusCode = jc.query(ctx, "api/Trading/Info", "get", requestData)

	if statusCode == 503 {
		jc.logger.Error("JetCrypto : error on GetTradingPairInfo - ServiceUnavailable!")
		return decimal.Decimal{}
	}
	if len(jetCryptoOrders) == 0 {
		return decimal.Decimal{}
	}

	order := struct {
		MinAmount decimal.Decimal `json:"minAmount"`
	}{}
	err := json.Unmarshal([]byte(jetCryptoOrders), &order)
	if err != nil {
		jc.logger.Error("JetCrypto : error on GetTradingPairInfo - empty response!")
		return decimal.Decimal{}
	}

	return order.MinAmount
}

func (jc *JetCryptoRequests) Withdraw(ctx context.Context, addr string, destinationTag string, withdrawalAmount decimal.Decimal, currentCurrencyId string) null.Int {

	params := struct {
		Address        string `json:"address"`
		DestinationTag string `json:"destinationTag"`
	}{
		Address:        addr,
		DestinationTag: destinationTag,
	}

	// make request object
	var requestData map[string]string = make(map[string]string)
	data, _ := json.Marshal(params)
	requestData["params"] = string(data)
	requestData["currencyId"] = currentCurrencyId
	requestData["amount"] = withdrawalAmount.String()
	requestData["withdrawalAmount"] = withdrawalAmount.String()
	requestData["withdrawalCurrencyId"] = currentCurrencyId
	requestData["trovematFee"] = "0"
	var uuid, _ = uuid.NewV4()
	requestData["uuId"] = uuid.String()
	requestData["moneySource"] = "0"
	requestData["moneySourceId"] = "0"

	if len(destinationTag) > 0 {
		requestData["destinationTag"] = destinationTag
	}
	// get JetCrypto order
	var jetCryptoOrders, statusCode = jc.query(ctx, "api/Trovemat/Payment", "post", requestData)

	if statusCode == 503 {
		jc.logger.Error("JetCrypto : error on Withdraw - ServiceUnavailable!")
		return null.Int{}
	}
	if len(jetCryptoOrders) == 0 {
		return null.Int{}
	}

	order := struct {
		Id null.Int `json:"id"`
	}{}
	err := json.Unmarshal([]byte(jetCryptoOrders), &order)
	if err != nil {
		jc.logger.Error("JetCrypto : error on Withdraw - empty response!")
		return null.Int{}
	}

	return order.Id
}

func (jc *JetCryptoRequests) RemoveOrder(ctx context.Context, orderId uuid.UUID, currencyFrom string, currencyTo string) bool {
	// make request object
	var requestData map[string]string = make(map[string]string)
	requestData["id"] = orderId.String()
	requestData["currencyFrom"] = currencyFrom
	requestData["currencyTo"] = currencyTo

	// remove JetCrypto order
	var deleteResult, statusCode = jc.query(ctx, "api/Trading/RemoveOrder", "post", requestData)

	if statusCode == 503 {
		jc.logger.Error("JetCrypto : error on RemoveOrder - ServiceUnavailable!")
		return false
	}
	if len(deleteResult) == 0 {
		jc.logger.Error("JetCrypto : error on RemoveOrder - empty response!")
		return false
	}

	result := struct {
		ErrorCode int `json:"errorCode"`
	}{}
	err := json.Unmarshal([]byte(deleteResult), &result)
	if err != nil {
		jc.logger.Error("JetCrypto : error on RemoveOrder - empty response !")
		return false
	}

	if result.ErrorCode != 0 {
		return false
	}

	return true
}

func (jc *JetCryptoRequests) AddOrder(ctx context.Context, currencyFrom string, currencyTo string, amount decimal.Decimal, price decimal.Decimal, isSellOrder bool) (bool, uuid.UUID) {
	// make request object
	var requestData map[string]string = make(map[string]string)
	requestData["currencyFrom"] = currencyFrom
	requestData["currencyTo"] = currencyTo
	requestData["amount"] = amount.String()
	requestData["price"] = price.String()
	requestData["isSellOrder"] = strconv.FormatBool(isSellOrder)

	// add new JetCrypto order
	var addResult, statusCode = jc.query(ctx, "api/Trading/Trade", "post", requestData)

	if statusCode == 503 {
		jc.logger.Error("JetCrypto : error on AddOrder - ServiceUnavailable!")
		return false, uuid.Nil
	}
	if len(addResult) == 0 {
		jc.logger.Error("JetCrypto : error on AddOrder - empty response!")
		return false, uuid.Nil
	}

	result := struct {
		Id        string `json:"id"`
		ErrorCode int    `json:"errorCode"`
	}{}
	err := json.Unmarshal([]byte(addResult), &result)
	if err != nil {
		jc.logger.Error("JetCrypto : error on AddOrder - empty response !")
		return false, uuid.Nil
	}

	if result.ErrorCode != 0 {
		return false, uuid.Nil
	}

	return true, uuid.FromStringOrNil(result.Id)
}

func (jc *JetCryptoRequests) query(ctx context.Context, method string, requestType string, requestData map[string]string) (string, int) {
	u, err := url.Parse(jc.baseUrl + "/" + method)
	if err != nil {
		jc.logger.Error(err)
		return "", 500
	}

	q := u.Query()
	if len(requestData) > 0 {
		for key, val := range requestData {
			q.Set(key, val)
		}
	}
	u.RawQuery = q.Encode()

	var keyBytes = []byte(jc.secretKey)
	var dataBytes = []byte(u.String())

	var hash512Bytes, _ = jc.helperMethods.HmacSha512(keyBytes, dataBytes)
	var signature = fmt.Sprintf("%x", hash512Bytes)

	var webHeaders = map[string]string{
		"Key":  jc.publicKey,
		"Sign": signature,
	}

	var statusCode = 500
	var resText = ""
	var err1 error
	if requestType == "get" {
		resText, statusCode, err1 = jc.helperMethods.HttpGet(ctx, u, "application/x-www-form-urlencoded", webHeaders)
	} else {
		resText, statusCode, err1 = jc.helperMethods.HttpPost(ctx, u, []byte(u.RawQuery), "application/json", webHeaders)
	}

	if statusCode != 200 {
		jc.logger.Error("JetCrypto : response status : %v, %v : %v", statusCode, err1, resText)
	}

	return resText, statusCode
}
