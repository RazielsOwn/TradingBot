package jetcrypto

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
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
		jc.logger.Error("error on GetInternalOrders - ServiceUnavailable!")
		return nil
	}
	if len(jetCryptoOrders) == 0 {
		return nil
	}

	var orders []*entity.InternalOrder
	err := json.Unmarshal([]byte(jetCryptoOrders), &orders)
	if err != nil {
		jc.logger.Error(fmt.Sprintf("error on GetInternalOrders - empty response for tradingPair: %v!", jetCryptoPair))
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
		jc.logger.Error("error on GetInternalCompleteOrder - ServiceUnavailable!")
		return nil
	}
	if len(jetCryptoOrders) == 0 {
		return nil
	}

	var orders []*entity.InternalOrder
	err := json.Unmarshal([]byte(jetCryptoOrders), &orders)
	if err != nil {
		jc.logger.Error(fmt.Sprintf("error on GetInternalCompleteOrder - empty response for tradingPair: %v!", jetCryptoPair))
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
		jc.logger.Error("error on GetInternalBalances - ServiceUnavailable!")
		return nil
	}
	if len(jetCryptoOrders) == 0 {
		return nil
	}

	var balances []*entity.BalanceObject
	err := json.Unmarshal([]byte(jetCryptoOrders), &balances)
	if err != nil {
		jc.logger.Error("error on GetInternalBalances - empty response!")
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
	var jetCryptoOrders, statusCode = jc.query(ctx, "api/Trading/OrderInfo", "get", requestData)

	if statusCode == 503 {
		jc.logger.Error("error on GetInternalOrder - ServiceUnavailable!")
		return nil
	}
	if len(jetCryptoOrders) == 0 {
		return nil
	}

	var order *entity.InternalOrder
	err := json.Unmarshal([]byte(jetCryptoOrders), &order)
	if err != nil {
		jc.logger.Error(fmt.Sprintf("error on GetInternalOrder - empty response for tradingPair: %v, orderId:%v !", jetCryptoPair, orderId))
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
		jc.logger.Error("error on IsInternalPaymentCompleted - ServiceUnavailable!")
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
		jc.logger.Error(fmt.Sprintf("error on IsInternalPaymentCompleted - empty response for orderId:%v !", orderId))
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
		jc.logger.Error("error on IsInternalPaymentCompleted - ServiceUnavailable!")
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
		jc.logger.Error(fmt.Sprintf("error on IsInternalPaymentCompleted - empty response for currencyName:%v !", currency))
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
		jc.logger.Error("error on InternalGetTradingPairInfo - ServiceUnavailable!")
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
		jc.logger.Error("error on InternalGetTradingPairInfo - empty response!")
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
	requestData["uuId"] = string(uuid.String())
	requestData["moneySource"] = "0"
	requestData["moneySourceId"] = "0"

	if len(destinationTag) > 0 {
		requestData["destinationTag"] = destinationTag
	}
	// get JetCrypto order
	var jetCryptoOrders, statusCode = jc.query(ctx, "api/Trovemat/Payment", "post", requestData)

	if statusCode == 503 {
		jc.logger.Error("error on InternalGetTradingPairInfo - ServiceUnavailable!")
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
		jc.logger.Error("error on InternalGetTradingPairInfo - empty response!")
		return null.Int{}
	}

	return order.Id
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
		jc.logger.Error(fmt.Sprintf("response status : %v, %v : %v", statusCode, err1, resText))
	}

	return resText, statusCode
}
