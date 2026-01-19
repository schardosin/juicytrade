from pathlib import Path
from pydantic_settings import BaseSettings, SettingsConfigDict

class Settings(BaseSettings):
    # Service
    service_name: str = "strategy-service"
    port: int = 8009
    host: str = "0.0.0.0"
    
    # Data directories
    data_dir: Path = Path("/app/data")
    parquet_dir: Path = Path("/app/data/parquet")
    
    # Database
    database_url: str = "sqlite:///data/strategies.db"
    
    # Logging
    log_level: str = "INFO"
    
    model_config = SettingsConfigDict(
        env_prefix="STRATEGY_",
        env_file=".env",
        extra="ignore"
    )

settings = Settings()
