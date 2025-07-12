import os
from dotenv import load_dotenv

try:
    from pydantic_settings import BaseSettings
except ImportError:
    from pydantic import BaseSettings

# Load environment variables
load_dotenv()

class Settings(BaseSettings):
    """
    Configuration settings for the trading backend.
    """
    # Provider selection
    provider: str = "alpaca"  # Default to Alpaca
    
    # Alpaca API credentials
    alpaca_api_key_live: str = os.getenv("APCA_API_KEY_ID_LIVE", "")
    alpaca_api_secret_live: str = os.getenv("APCA_API_SECRET_KEY_LIVE", "")
    alpaca_api_key_paper: str = os.getenv("APCA_API_KEY_ID_PAPER", "")
    alpaca_api_secret_paper: str = os.getenv("APCA_API_SECRET_KEY_PAPER", "")
    alpaca_base_url_live: str = os.getenv("ALPACA_BASE_URL_LIVE", "https://api.alpaca.markets")
    alpaca_base_url_paper: str = os.getenv("ALPACA_BASE_URL_PAPER", "https://paper-api.alpaca.markets")
    alpaca_data_url: str = os.getenv("ALPACA_DATA_URL", "https://data.alpaca.markets")
    
    # Public API credentials
    public_secret_key: str = os.getenv("PUBLIC_SECRET_KEY", "")
    public_account_id: str = os.getenv("PUBLIC_ACCOUNT_ID", "")

    # Tradier API credentials
    tradier_secret_key: str = os.getenv("TRADIER_SECRET_KEY", "")
    tradier_account_id: str = os.getenv("TRADIER_ACCOUNT_ID", "")
    tradier_secret_key_paper: str = os.getenv("TRADIER_SECRET_KEY_PAPER", "")
    tradier_account_id_paper: str = os.getenv("TRADIER_ACCOUNT_ID_PAPER", "")
    tradier_base_url_live: str = os.getenv("TRADIER_BASE_URL_LIVE", "https://api.tradier.com")
    tradier_base_url_paper: str = os.getenv("TRADIER_BASE_URL_PAPER", "https://sandbox.tradier.com")
    tradier_stream_url_live: str = os.getenv("TRADIER_STREAM_URL_LIVE", "wss://ws.tradier.com/v1/markets/events")
    tradier_stream_url_paper: str = os.getenv("TRADIER_STREAM_URL_PAPER", "wss://ws.sandbox.tradier.com/v1/markets/events")
    
    # Server settings
    host: str = "0.0.0.0"
    port: int = 8008
    reload: bool = True
    
    class Config:
        env_file = ".env"
        extra = "ignore"  # Ignore extra fields from environment

# Global settings instance
settings = Settings()
