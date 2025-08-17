from typing import Dict, List, Any

# Provider type definitions with credential field specifications
PROVIDER_TYPES: Dict[str, Dict[str, Any]] = {
    "alpaca": {
        "name": "Alpaca",
        "description": "Alpaca Trading API",
        "supports_account_types": ["live", "paper"],
        "capabilities": {
            "rest": ["expiration_dates", "stock_quotes", "options_chain", "trade_account", "next_market_date", "symbol_lookup", "historical_data", "market_calendar", "greeks"],
            "streaming": ["streaming_quotes", "trade_account"]
        },
        "credential_fields": {
            "live": [
                {"name": "api_key", "label": "API Key", "type": "text", "required": True, "placeholder": "Your Alpaca Live API Key"},
                {"name": "api_secret", "label": "API Secret", "type": "password", "required": True, "placeholder": "Your Alpaca Live API Secret"},
                {"name": "base_url", "label": "Base URL", "type": "text", "required": False, "default": "https://api.alpaca.markets"},
                {"name": "data_url", "label": "Data URL", "type": "text", "required": False, "default": "https://data.alpaca.markets"}
            ],
            "paper": [
                {"name": "api_key", "label": "API Key", "type": "text", "required": True, "placeholder": "Your Alpaca Paper API Key"},
                {"name": "api_secret", "label": "API Secret", "type": "password", "required": True, "placeholder": "Your Alpaca Paper API Secret"},
                {"name": "base_url", "label": "Base URL", "type": "text", "required": False, "default": "https://paper-api.alpaca.markets"},
                {"name": "data_url", "label": "Data URL", "type": "text", "required": False, "default": "https://data.alpaca.markets"}
            ]
        }
    },
    "tradier": {
        "name": "Tradier",
        "description": "Tradier Brokerage API",
        "supports_account_types": ["live", "paper"],
        "capabilities": {
            "rest": ["expiration_dates", "options_chain", "next_market_date", "stock_quotes", "trade_account", "symbol_lookup", "historical_data", "market_calendar", "greeks"],
            "streaming": ["streaming_quotes"]
        },
        "credential_fields": {
            "live": [
                {"name": "api_key", "label": "API Key", "type": "password", "required": True, "placeholder": "Your Tradier Live API Key"},
                {"name": "account_id", "label": "Account ID", "type": "text", "required": True, "placeholder": "Your Tradier Live Account ID"},
                {"name": "base_url", "label": "Base URL", "type": "text", "required": False, "default": "https://api.tradier.com"},
                {"name": "stream_url", "label": "Stream URL", "type": "text", "required": False, "default": "wss://ws.tradier.com/v1/markets/events"}
            ],
            "paper": [
                {"name": "api_key", "label": "API Key", "type": "password", "required": True, "placeholder": "Your Tradier Sandbox API Key"},
                {"name": "account_id", "label": "Account ID", "type": "text", "required": True, "placeholder": "Your Tradier Sandbox Account ID"},
                {"name": "base_url", "label": "Base URL", "type": "text", "required": False, "default": "https://sandbox.tradier.com"},
                {"name": "stream_url", "label": "Stream URL", "type": "text", "required": False, "default": "wss://ws.sandbox.tradier.com/v1/markets/events"}
            ]
        }
    },
    "public": {
        "name": "Public.com",
        "description": "Public.com Trading API",
        "supports_account_types": ["live"],
        "capabilities": {
            "rest": ["expiration_dates", "stock_quotes", "options_chain", "trade_account", "next_market_date"]
        },
        "credential_fields": {
            "live": [
                {"name": "api_secret", "label": "API Secret", "type": "password", "required": True, "placeholder": "Your Public.com API Secret"},
                {"name": "account_id", "label": "Account ID", "type": "text", "required": True, "placeholder": "Your Public.com Account ID"}
            ]
        }
    },
    "tastytrade": {
        "name": "TastyTrade",
        "description": "TastyTrade Brokerage API",
        "supports_account_types": ["live", "paper"],
        "capabilities": {
            "rest": ["expiration_dates", "stock_quotes", "options_chain", "trade_account", "next_market_date", "symbol_lookup", "historical_data", "market_calendar"],
            "streaming": ["streaming_quotes", "trade_account", "streaming_greeks"]
        },
        "credential_fields": {
            "live": [
                {"name": "username", "label": "Username", "type": "text", "required": True, "placeholder": "Your TastyTrade Username"},
                {"name": "password", "label": "Password", "type": "password", "required": True, "placeholder": "Your TastyTrade Password"},
                {"name": "account_id", "label": "Account ID", "type": "text", "required": True, "placeholder": "Your TastyTrade Account ID"},
                {"name": "base_url", "label": "Base URL", "type": "text", "required": False, "default": "https://api.tastytrade.com"}
            ],
            "paper": [
                {"name": "username", "label": "Username", "type": "text", "required": True, "placeholder": "Your TastyTrade Sandbox Username"},
                {"name": "password", "label": "Password", "type": "password", "required": True, "placeholder": "Your TastyTrade Sandbox Password"},
                {"name": "account_id", "label": "Account ID", "type": "text", "required": True, "placeholder": "Your TastyTrade Sandbox Account ID"},
                {"name": "base_url", "label": "Base URL", "type": "text", "required": False, "default": "https://api.cert.tastyworks.com"}
            ]
        }
    }
}

def get_provider_types() -> Dict[str, Dict[str, Any]]:
    """Get all available provider types"""
    return PROVIDER_TYPES

def get_provider_type(provider_type: str) -> Dict[str, Any]:
    """Get specific provider type definition"""
    return PROVIDER_TYPES.get(provider_type, {})

def get_credential_fields(provider_type: str, account_type: str) -> List[Dict[str, Any]]:
    """Get credential fields for a specific provider type and account type"""
    provider_def = PROVIDER_TYPES.get(provider_type, {})
    credential_fields = provider_def.get("credential_fields", {})
    return credential_fields.get(account_type, [])

def validate_credentials(provider_type: str, account_type: str, credentials: Dict[str, str]) -> List[str]:
    """Validate credentials against provider type requirements"""
    errors = []
    fields = get_credential_fields(provider_type, account_type)
    
    for field in fields:
        field_name = field["name"]
        if field.get("required", False) and not credentials.get(field_name):
            errors.append(f"Missing required field: {field['label']}")
    
    return errors

def apply_defaults(provider_type: str, account_type: str, credentials: Dict[str, str]) -> Dict[str, str]:
    """Apply default values to credentials"""
    fields = get_credential_fields(provider_type, account_type)
    result = credentials.copy()
    
    for field in fields:
        field_name = field["name"]
        if field_name not in result and "default" in field:
            result[field_name] = field["default"]
    
    return result

def is_sensitive_field(field_name: str) -> bool:
    """Determine if a credential field contains sensitive information"""
    sensitive_fields = ['password', 'api_key', 'api_secret']
    return field_name in sensitive_fields

def get_visible_credentials(instance_data: Dict[str, Any]) -> Dict[str, str]:
    """Get non-sensitive credential values that can be displayed in UI"""
    credentials = instance_data.get('credentials', {})
    visible_creds = {}
    
    for field_name, field_value in credentials.items():
        if not is_sensitive_field(field_name):
            visible_creds[field_name] = field_value
    
    return visible_creds

def get_masked_credentials(instance_data: Dict[str, Any]) -> Dict[str, bool]:
    """Get indicators for which sensitive fields have values set"""
    credentials = instance_data.get('credentials', {})
    masked_creds = {}
    
    for field_name, field_value in credentials.items():
        if is_sensitive_field(field_name):
            masked_creds[field_name] = bool(field_value and field_value.strip())
    
    return masked_creds

def get_default_credentials(provider_type: str, account_type: str) -> Dict[str, str]:
    """Get default credential values for a provider type and account type"""
    fields = get_credential_fields(provider_type, account_type)
    defaults = {}
    
    for field in fields:
        if "default" in field:
            defaults[field["name"]] = field["default"]
    
    return defaults
