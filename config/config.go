package config

import (
	"fmt"
	"path/filepath"

	"github.com/ilyakaznacheev/cleanenv"
	"github.com/shopspring/decimal"
)

type (
	// Config -.
	Config struct {
		App              `json:"app"`
		Log              `json:"logger"`
		PG               `json:"postgres"`
		CryptoCurrencies []CryptoCurrency `json:"CryptoCurrencies"`
	}

	// App -.
	App struct {
		Name    string `env-required:"true" json:"name"    env:"APP_NAME"`
		Version string `env-required:"true" json:"version" env:"APP_VERSION"`
	}

	// Log -.
	Log struct {
		Level string `env-required:"true" json:"log_level"  env:"LOG_LEVEL"`
	}

	// PG -.
	PG struct {
		PoolMax int    `env-required:"true" json:"pool_max" env:"PG_POOL_MAX"`
		URL     string `env-required:"true"                 env:"PG_URL"`
	}

	// CryptoCurrency
	CryptoCurrency struct {
		CurrencyId       int             `json:"CurrencyId"`
		BalancePercent   decimal.Decimal `json:"BalancePercent"`
		ThresholdPercent decimal.Decimal `json:"ThresholdPercent"`
		ThresholdAbs     decimal.Decimal `json:"ThresholdAbs"`
		SellMultiplier   decimal.Decimal `json:"SellMultiplier"`
		BuyMultiplier    decimal.Decimal `json:"BuyMultiplier"`
		TimeoutMinutes   int             `json:"TimeoutMinutes"`
		InternalSettings `json:"InternalSettings"`
		TradingSettings  `json:"TradingSettings"`
	}

	InternalSettings struct {
		Url            string          `json:"Url"`
		Key            string          `json:"Key"`
		Secret         string          `json:"Secret"`
		Pair           string          `json:"Pair"`
		Currency       string          `json:"Currency"`
		CryptoAddress  string          `json:"CryptoAddress"`
		UsdcUsageLimit decimal.Decimal `json:"UsdcUsageLimit"`
	}
	TradingSettings struct {
		Url               string          `json:"Url"`
		Key               string          `json:"Key"`
		Secret            string          `json:"Secret"`
		Pair              string          `json:"Pair"`
		Currency          string          `json:"Currency"`
		CryptoAddress     string          `json:"CryptoAddress"`
		DestinationTag    string          `json:"DestinationTag"`
		WithdrawalNetwork string          `json:"WithdrawalNetwork"`
		UsdcUsageLimit    decimal.Decimal `json:"UsdcUsageLimit"`
	}
)

// NewConfig returns app config.
func NewConfig() (*Config, error) {
	cfg := &Config{}

	configPath := filepath.Join("./config", "config.json")
	err := cleanenv.ReadConfig(configPath, cfg)
	if err != nil {
		return nil, fmt.Errorf("config error: %w", err)
	}

	err = cleanenv.ReadEnv(cfg)
	if err != nil {
		return nil, err
	}

	return cfg, nil
}
