import json
import os
import logging
from typing import Dict, List, Optional, Any

logger = logging.getLogger(__name__)

CONFIG_FILE = "provider_config.json"

DEFAULT_ROUTING = {
    "stock_quotes": "alpaca",
    "options_chain": "alpaca",
    "trade_account": "alpaca",  # Unified: account, positions, orders
    "symbol_lookup": "tradier",
    "historical_data": "tradier",
    "market_calendar": "tradier",
    "streaming_quotes": "tradier",  # Dedicated streaming for market data
    "greeks": "tradier",  # NEW: API-based Greeks
    "streaming_greeks": None  # NEW: Real-time streaming Greeks
}

PROVIDER_CAPABILITIES = {
    "alpaca": {
        "capabilities": {
            "rest": ["stock_quotes", "options_chain", "trade_account", "next_market_date", "symbol_lookup", "historical_data", "market_calendar", "greeks"],
            "streaming": ["streaming_quotes", "trade_account"]
        },
        "paper": False,
        "display_name": "Alpaca"
    },
    "alpaca_paper": {
        "capabilities": {
            "rest": ["stock_quotes", "options_chain", "trade_account", "next_market_date", "symbol_lookup", "historical_data", "market_calendar", "greeks"],
            "streaming": ["streaming_quotes", "trade_account"]
        },
        "paper": True,
        "display_name": "Alpaca"
    },
    "public": {
        "capabilities": {
            "rest": ["stock_quotes", "options_chain", "trade_account", "next_market_date"]
        },
        "paper": False,
        "display_name": "Public.com"
    },
    "tradier": {
        "capabilities": {
            "rest": ["options_chain", "next_market_date", "stock_quotes", "trade_account", "symbol_lookup", "historical_data", "market_calendar", "greeks"],
            "streaming": ["streaming_quotes"]
        },
        "paper": False,
        "display_name": "Tradier"
    },
    "tradier_paper": {
        "capabilities": {
            "rest": ["options_chain", "next_market_date", "stock_quotes", "trade_account", "symbol_lookup", "historical_data", "market_calendar", "greeks"],
            "streaming": ["streaming_quotes"]
        },
        "paper": True,
        "display_name": "Tradier"
    },
    "tastytrade": {
        "capabilities": {
            "rest": ["stock_quotes", "options_chain", "trade_account", "next_market_date", "symbol_lookup", "historical_data", "market_calendar"],
            "streaming": ["streaming_quotes", "trade_account", "streaming_greeks"]
        },
        "paper": False,
        "display_name": "TastyTrade"
    },
    "tastytrade_paper": {
        "capabilities": {
            "rest": ["stock_quotes", "options_chain", "trade_account", "next_market_date", "symbol_lookup", "historical_data", "market_calendar"],
            "streaming": ["streaming_quotes", "trade_account", "streaming_greeks"]
        },
        "paper": True,
        "display_name": "TastyTrade"
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
        
        # Import here to avoid circular imports
        from .provider_manager import provider_manager
        from .provider_types import get_provider_types
        
        # Get available provider instances
        available_instances = provider_manager.get_available_provider_instances()
        
        for key, value in new_config.items():
            if key in DEFAULT_ROUTING:
                # Allow null values for optional configurations
                if value is None:
                    validated_config[key] = value
                    logger.info(f"Set {key} to null")
                # Check if it's a legacy provider name (for backward compatibility)
                elif value in PROVIDER_CAPABILITIES:
                    # Legacy provider validation
                    if key == "streaming_quotes":
                        if "streaming" in PROVIDER_CAPABILITIES[value]["capabilities"] and key in PROVIDER_CAPABILITIES[value]["capabilities"]["streaming"]:
                            validated_config[key] = value
                        else:
                            logger.warning(f"Invalid streaming provider/capability: {key}:{value}")
                    elif "rest" in PROVIDER_CAPABILITIES[value]["capabilities"] and key in PROVIDER_CAPABILITIES[value]["capabilities"]["rest"]:
                        validated_config[key] = value
                    else:
                        logger.warning(f"Invalid REST provider/capability: {key}:{value}")
                # Check if it's a new provider instance ID
                elif value in available_instances:
                    instance_data = available_instances[value]
                    provider_type = instance_data.get('provider_type')
                    
                    # Get provider type capabilities
                    provider_types = get_provider_types()
                    if provider_type in provider_types:
                        capabilities = provider_types[provider_type].get('capabilities', {})
                        
                        # Check if provider supports this capability
                        if key in ["streaming_quotes", "streaming_greeks"]:
                            if key in capabilities.get('streaming', []):
                                validated_config[key] = value
                            else:
                                logger.warning(f"Provider instance {value} doesn't support streaming capability: {key}")
                        elif key in capabilities.get('rest', []):
                            validated_config[key] = value
                        else:
                            logger.warning(f"Provider instance {value} doesn't support REST capability: {key}")
                    else:
                        logger.warning(f"Unknown provider type for instance {value}: {provider_type}")
                else:
                    logger.warning(f"Unknown provider or instance: {value}")
            else:
                logger.warning(f"Invalid config key: {key}")

        self._config = validated_config
        self._save_config()
        return True

    def reset_config(self):
        self._config = DEFAULT_ROUTING.copy()
        self._save_config()

    def get_available_providers(self) -> Dict[str, Dict[str, any]]:
        # Import here to avoid circular imports
        from .provider_manager import provider_manager
        from .provider_types import get_provider_types
        
        # Get available provider instances
        available_instances = provider_manager.get_available_provider_instances()
        provider_types = get_provider_types()
        
        result = {}
        
        # Add dynamic provider instances
        for instance_id, instance_data in available_instances.items():
            if instance_data.get('active', False):  # Only include active instances
                provider_type = instance_data.get('provider_type')
                account_type = instance_data.get('account_type')
                
                if provider_type in provider_types:
                    type_info = provider_types[provider_type]
                    result[instance_id] = {
                        "capabilities": type_info.get('capabilities', {}),
                        "paper": (account_type == "paper"),
                        "display_name": instance_data.get('display_name', instance_id),
                        "provider_type": provider_type,
                        "account_type": account_type,
                        "instance_id": instance_id
                    }
        
        # Add legacy providers for backward compatibility (only if no instances exist)
        if not result:
            result = {
                provider: {
                    "capabilities": caps["capabilities"],
                    "paper": caps["paper"],
                    "display_name": caps["display_name"]
                }
                for provider, caps in PROVIDER_CAPABILITIES.items()
            }
        
        return result

provider_config_manager = ProviderConfigManager()
