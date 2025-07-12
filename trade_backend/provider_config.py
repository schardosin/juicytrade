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
    "positions": "alpaca",
    "orders": "alpaca",
    "streaming": {
        "stock_quotes": "tradier",
        "option_quotes": "tradier",
        "positions": "alpaca",
        "orders": "alpaca"
    }
}

PROVIDER_CAPABILITIES = {
    "alpaca": {
        "rest": ["expiration_dates", "stock_quotes", "options_chain", "positions", "orders"],
        "streaming": ["stock_quotes", "option_quotes", "positions", "orders"]
    },
    "alpaca_paper": {
        "rest": ["expiration_dates", "stock_quotes", "options_chain", "positions", "orders"],
        "streaming": ["stock_quotes", "option_quotes", "positions", "orders"]
    },
    "public": {
        "rest": ["expiration_dates"]
    },
    "tradier": {
        "rest": ["orders"],
        "streaming": ["stock_quotes", "option_quotes"]
    },
    "tradier_paper": {
        "rest": ["orders"],
        "streaming": ["stock_quotes", "option_quotes"]
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
            if key == "streaming" and isinstance(value, dict):
                if "streaming" not in validated_config:
                    validated_config["streaming"] = {}
                for stream_key, stream_value in value.items():
                    if stream_value in PROVIDER_CAPABILITIES and "streaming" in PROVIDER_CAPABILITIES[stream_value] and stream_key in PROVIDER_CAPABILITIES[stream_value]["streaming"]:
                        validated_config["streaming"][stream_key] = stream_value
                    else:
                        logger.warning(f"Invalid streaming provider/capability: {stream_key}:{stream_value}")
            elif key in DEFAULT_ROUTING:
                 if value in PROVIDER_CAPABILITIES and "rest" in PROVIDER_CAPABILITIES[value] and key in PROVIDER_CAPABILITIES[value]["rest"]:
                    validated_config[key] = value
                 else:
                    logger.warning(f"Invalid REST provider/capability: {key}:{value}")
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
