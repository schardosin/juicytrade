package config

import (
	"os"
	"strconv"

	"github.com/spf13/viper"
)

// Settings represents the configuration settings for the trading backend.
// This is an exact conversion of the Python Settings class.
type Settings struct {
	// Provider selection
	Provider string `mapstructure:"provider" json:"provider"`

	// Alpaca API credentials
	AlpacaAPIKeyLive      string `mapstructure:"alpaca_api_key_live" json:"alpaca_api_key_live"`
	AlpacaAPISecretLive   string `mapstructure:"alpaca_api_secret_live" json:"alpaca_api_secret_live"`
	AlpacaAPIKeyPaper     string `mapstructure:"alpaca_api_key_paper" json:"alpaca_api_key_paper"`
	AlpacaAPISecretPaper  string `mapstructure:"alpaca_api_secret_paper" json:"alpaca_api_secret_paper"`
	AlpacaBaseURLLive     string `mapstructure:"alpaca_base_url_live" json:"alpaca_base_url_live"`
	AlpacaBaseURLPaper    string `mapstructure:"alpaca_base_url_paper" json:"alpaca_base_url_paper"`
	AlpacaDataURL         string `mapstructure:"alpaca_data_url" json:"alpaca_data_url"`

	// Public API credentials
	PublicSecretKey string `mapstructure:"public_secret_key" json:"public_secret_key"`
	PublicAccountID string `mapstructure:"public_account_id" json:"public_account_id"`

	// Tradier API credentials
	TradierSecretKey       string `mapstructure:"tradier_secret_key" json:"tradier_secret_key"`
	TradierAccountID       string `mapstructure:"tradier_account_id" json:"tradier_account_id"`
	TradierSecretKeyPaper  string `mapstructure:"tradier_secret_key_paper" json:"tradier_secret_key_paper"`
	TradierAccountIDPaper  string `mapstructure:"tradier_account_id_paper" json:"tradier_account_id_paper"`
	TradierBaseURLLive     string `mapstructure:"tradier_base_url_live" json:"tradier_base_url_live"`
	TradierBaseURLPaper    string `mapstructure:"tradier_base_url_paper" json:"tradier_base_url_paper"`
	TradierStreamURLLive   string `mapstructure:"tradier_stream_url_live" json:"tradier_stream_url_live"`
	TradierStreamURLPaper  string `mapstructure:"tradier_stream_url_paper" json:"tradier_stream_url_paper"`

	// Server settings
	Host   string `mapstructure:"host" json:"host"`
	Port   int    `mapstructure:"port" json:"port"`
	Reload bool   `mapstructure:"reload" json:"reload"`

	// Logging settings
	LogLevel string `mapstructure:"log_level" json:"log_level"`

	// WebSocket settings
	SubscriptionKeepaliveTimeout int `mapstructure:"subscription_keepalive_timeout" json:"subscription_keepalive_timeout"`

	// Streaming settings
	StreamingQuotes string `mapstructure:"streaming_quotes" json:"streaming_quotes"`
	StreamingGreeks string `mapstructure:"streaming_greeks" json:"streaming_greeks"`
}

// Global settings instance - exact equivalent of Python's global settings
var GlobalSettings *Settings

// LoadSettings loads configuration from environment variables and .env file
// This replicates the Python behavior exactly
func LoadSettings() *Settings {
	viper.SetConfigName(".env")
	viper.SetConfigType("env")
	viper.AddConfigPath(".")
	viper.AddConfigPath("../")
	viper.AddConfigPath("../../")

	// Read .env file if it exists (equivalent to load_dotenv())
	viper.ReadInConfig()

	// Set defaults - exact same as Python
	viper.SetDefault("provider", "alpaca")
	viper.SetDefault("host", "0.0.0.0")
	viper.SetDefault("port", 8008)
	viper.SetDefault("reload", true)
	viper.SetDefault("log_level", "INFO")
	viper.SetDefault("subscription_keepalive_timeout", 60)
	viper.SetDefault("alpaca_base_url_live", "https://api.alpaca.markets")
	viper.SetDefault("alpaca_base_url_paper", "https://paper-api.alpaca.markets")
	viper.SetDefault("alpaca_data_url", "https://data.alpaca.markets")
	viper.SetDefault("tradier_base_url_live", "https://api.tradier.com")
	viper.SetDefault("tradier_base_url_paper", "https://sandbox.tradier.com")
	viper.SetDefault("tradier_stream_url_live", "wss://ws.tradier.com/v1/markets/events")
	viper.SetDefault("tradier_stream_url_paper", "wss://ws.sandbox.tradier.com/v1/markets/events")
	viper.SetDefault("streaming_quotes", "")
	viper.SetDefault("streaming_greeks", "")

	// Enable automatic environment variable binding
	viper.AutomaticEnv()

	// Map environment variable names to match Python exactly
	viper.BindEnv("alpaca_api_key_live", "APCA_API_KEY_ID_LIVE")
	viper.BindEnv("alpaca_api_secret_live", "APCA_API_SECRET_KEY_LIVE")
	viper.BindEnv("alpaca_api_key_paper", "APCA_API_KEY_ID_PAPER")
	viper.BindEnv("alpaca_api_secret_paper", "APCA_API_SECRET_KEY_PAPER")
	viper.BindEnv("alpaca_base_url_live", "ALPACA_BASE_URL_LIVE")
	viper.BindEnv("alpaca_base_url_paper", "ALPACA_BASE_URL_PAPER")
	viper.BindEnv("alpaca_data_url", "ALPACA_DATA_URL")
	viper.BindEnv("public_secret_key", "PUBLIC_SECRET_KEY")
	viper.BindEnv("public_account_id", "PUBLIC_ACCOUNT_ID")
	viper.BindEnv("tradier_secret_key", "TRADIER_SECRET_KEY")
	viper.BindEnv("tradier_account_id", "TRADIER_ACCOUNT_ID")
	viper.BindEnv("tradier_secret_key_paper", "TRADIER_SECRET_KEY_PAPER")
	viper.BindEnv("tradier_account_id_paper", "TRADIER_ACCOUNT_ID_PAPER")
	viper.BindEnv("tradier_base_url_live", "TRADIER_BASE_URL_LIVE")
	viper.BindEnv("tradier_base_url_paper", "TRADIER_BASE_URL_PAPER")
	viper.BindEnv("tradier_stream_url_live", "TRADIER_STREAM_URL_LIVE")
	viper.BindEnv("tradier_stream_url_paper", "TRADIER_STREAM_URL_PAPER")
	viper.BindEnv("streaming_quotes", "STREAMING_QUOTES")
	viper.BindEnv("streaming_greeks", "STREAMING_GREEKS")

	settings := &Settings{}
	if err := viper.Unmarshal(settings); err != nil {
		// If unmarshal fails, create settings with defaults and environment variables
		settings = &Settings{
			Provider:                     getEnvOrDefault("PROVIDER", "alpaca"),
			AlpacaAPIKeyLive:            getEnvOrDefault("APCA_API_KEY_ID_LIVE", ""),
			AlpacaAPISecretLive:         getEnvOrDefault("APCA_API_SECRET_KEY_LIVE", ""),
			AlpacaAPIKeyPaper:           getEnvOrDefault("APCA_API_KEY_ID_PAPER", ""),
			AlpacaAPISecretPaper:        getEnvOrDefault("APCA_API_SECRET_KEY_PAPER", ""),
			AlpacaBaseURLLive:           getEnvOrDefault("ALPACA_BASE_URL_LIVE", "https://api.alpaca.markets"),
			AlpacaBaseURLPaper:          getEnvOrDefault("ALPACA_BASE_URL_PAPER", "https://paper-api.alpaca.markets"),
			AlpacaDataURL:               getEnvOrDefault("ALPACA_DATA_URL", "https://data.alpaca.markets"),
			PublicSecretKey:             getEnvOrDefault("PUBLIC_SECRET_KEY", ""),
			PublicAccountID:             getEnvOrDefault("PUBLIC_ACCOUNT_ID", ""),
			TradierSecretKey:            getEnvOrDefault("TRADIER_SECRET_KEY", ""),
			TradierAccountID:            getEnvOrDefault("TRADIER_ACCOUNT_ID", ""),
			TradierSecretKeyPaper:       getEnvOrDefault("TRADIER_SECRET_KEY_PAPER", ""),
			TradierAccountIDPaper:       getEnvOrDefault("TRADIER_ACCOUNT_ID_PAPER", ""),
			TradierBaseURLLive:          getEnvOrDefault("TRADIER_BASE_URL_LIVE", "https://api.tradier.com"),
			TradierBaseURLPaper:         getEnvOrDefault("TRADIER_BASE_URL_PAPER", "https://sandbox.tradier.com"),
			TradierStreamURLLive:        getEnvOrDefault("TRADIER_STREAM_URL_LIVE", "wss://ws.tradier.com/v1/markets/events"),
			TradierStreamURLPaper:       getEnvOrDefault("TRADIER_STREAM_URL_PAPER", "wss://ws.sandbox.tradier.com/v1/markets/events"),
			Host:                        getEnvOrDefault("HOST", "0.0.0.0"),
			Port:                        getEnvIntOrDefault("PORT", 8008),
			Reload:                      getEnvBoolOrDefault("RELOAD", true),
			LogLevel:                    getEnvOrDefault("LOG_LEVEL", "INFO"),
			SubscriptionKeepaliveTimeout: getEnvIntOrDefault("SUBSCRIPTION_KEEPALIVE_TIMEOUT", 60),
			StreamingQuotes:             getEnvOrDefault("STREAMING_QUOTES", ""),
			StreamingGreeks:             getEnvOrDefault("STREAMING_GREEKS", ""),
		}
	}

	GlobalSettings = settings
	return settings
}

// Helper functions to replicate Python's os.getenv behavior exactly
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvIntOrDefault(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

func getEnvBoolOrDefault(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if boolValue, err := strconv.ParseBool(value); err == nil {
			return boolValue
		}
	}
	return defaultValue
}
