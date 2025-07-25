import json
import os
import logging
from typing import Dict, List, Optional, Any

logger = logging.getLogger(__name__)

CONFIG_FILE = "provider_config.json"

DEFAULT_ROUTING = {
    "expiration_dates": "public",
    "stock_quotes": "alpaca",
    "options_chain": "alpaca",
    "next_market_date": "alpaca",
    "trade_account": "alpaca",  # Unified: account, positions, orders
    "symbol_lookup": "tradier",
    "historical_data": "tradier",
    "market_calendar": "tradier",
    "streaming_quotes": "tradier"  # Dedicated streaming for market data
}

PROVIDER_CAPABILITIES = {
    "alpaca": {
        "rest": ["expiration_dates", "stock_quotes", "options_chain", "trade_account", "next_market_date", "symbol_lookup", "historical_data", "market_calendar"],
        "streaming": ["streaming_quotes", "trade_account"]
    },
    "alpaca_paper": {
        "rest": ["expiration_dates", "stock_quotes", "options_chain", "trade_account", "next_market_date", "symbol_lookup", "historical_data", "market_calendar"],
        "streaming": ["streaming_quotes", "trade_account"]
    },
    "public": {
        "rest": ["expiration_dates", "stock_quotes", "options_chain", "trade_account", "next_market_date"]
    },
    "tradier": {
        "rest": ["expiration_dates", "options_chain", "next_market_date", "stock_quotes", "trade_account", "symbol_lookup", "historical_data", "market_calendar"],
        "streaming": ["streaming_quotes"]
    },
    "tradier_paper": {
        "rest": ["expiration_dates", "options_chain", "next_market_date", "stock_quotes", "trade_account", "symbol_lookup", "historical_data", "market_calendar"],
        "streaming": ["streaming_quotes"]
    }
}

class ProviderConfigManager:
    def __init__(self):
        self._config = self._load_config()

    def _load_config(self) -> Dict[str, str]:
        if os.path.exists(CONFIG_FILE):
            try:
                with open(CONFIG_FILE, 'r') as f:
                    config = json.load(f)
                    logger.info(f"Loaded provider config from {CONFIG_FILE}")
                    return config
            except (json.JSONDecodeError, IOError) as e:
                logger.error(f"Error loading {CONFIG_FILE}: {e}. Using default config.")
        return DEFAULT_ROUTING.copy()

    def _save_config(self):
        try:
            with open(CONFIG_FILE, 'w') as f:
                json.dump(self._config, f, indent=2)
            logger.info(f"Saved provider config to {CONFIG_FILE}")
        except IOError as e:
            logger.error(f"Error saving {CONFIG_FILE}: {e}")

    def get_config(self) -> Dict[str, str]:
        return self._config.copy()

    def update_config(self, new_config: Dict[str, Any]) -> bool:
        validated_config = self._config.copy()
        for key, value in new_config.items():
            if key in DEFAULT_ROUTING:
                # Validate provider exists and supports the capability
                if value in PROVIDER_CAPABILITIES:
                    # Check if it's a streaming capability
                    if key == "streaming_quotes":
                        if "streaming" in PROVIDER_CAPABILITIES[value] and key in PROVIDER_CAPABILITIES[value]["streaming"]:
                            validated_config[key] = value
                        else:
                            logger.warning(f"Invalid streaming provider/capability: {key}:{value}")
                    # Check if it's a REST capability
                    elif "rest" in PROVIDER_CAPABILITIES[value] and key in PROVIDER_CAPABILITIES[value]["rest"]:
                        validated_config[key] = value
                    else:
                        logger.warning(f"Invalid REST provider/capability: {key}:{value}")
                else:
                    logger.warning(f"Unknown provider: {value}")
            else:
                logger.warning(f"Invalid config key: {key}")

        self._config = validated_config
        self._save_config()
        return True

    def reset_config(self):
        self._config = DEFAULT_ROUTING.copy()
        self._save_config()

    def get_available_providers(self) -> Dict[str, Dict[str, any]]:
        # This can be enhanced to check provider health
        return {
            provider: {"capabilities": caps}
            for provider, caps in PROVIDER_CAPABILITIES.items()
        }

provider_config_manager = ProviderConfigManager()
