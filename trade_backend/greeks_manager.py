import logging
from typing import List, Dict, Optional
from .provider_config import provider_config_manager
from .provider_manager import provider_manager

logger = logging.getLogger(__name__)

class GreeksManager:
    """
    Smart Greeks Manager that handles both API and streaming Greeks
    with intelligent fallback logic.
    """
    
    def __init__(self):
        self.streaming_provider = None
        self.api_provider = None
        self.current_strategy = None
        self.is_initialized = False
    
    async def initialize(self):
        """Initialize Greeks manager with current provider configuration."""
        try:
            config = provider_config_manager.get_config()
            
            # Reset providers
            self.streaming_provider = None
            self.api_provider = None
            self.current_strategy = None
            
            # Priority: streaming_greeks > greeks > none
            if config.get("streaming_greeks"):
                provider_name = config["streaming_greeks"]
                self.streaming_provider = provider_manager.get_provider(provider_name)
                if self.streaming_provider:
                    self.current_strategy = "streaming"
                    logger.info(f"✅ Greeks streaming provider: {provider_name}")
            
            if config.get("greeks"):
                provider_name = config["greeks"]
                self.api_provider = provider_manager.get_provider(provider_name)
                if self.api_provider:
                    if not self.current_strategy:
                        self.current_strategy = "api"
                    logger.info(f"✅ Greeks API provider: {provider_name}")
            
            if not self.current_strategy:
                self.current_strategy = "none"
                logger.warning("⚠️ No Greeks provider configured")
            
            # Store current config for change detection
            self._last_streaming_config = config.get("streaming_greeks")
            self._last_api_config = config.get("greeks")
            
            self.is_initialized = True
            logger.info(f"✅ Greeks manager initialized with strategy: {self.current_strategy}")
            
        except Exception as e:
            logger.error(f"❌ Error initializing Greeks manager: {e}")
            self.current_strategy = "none"
            self.is_initialized = True
    
    async def get_greeks_batch(self, symbols: List[str]) -> Dict[str, Dict]:
        """Get Greeks for multiple symbols using the best available strategy."""
        if not self.is_initialized:
            await self.initialize()
        
        # Check if configuration has changed and refresh if needed
        current_config = provider_config_manager.get_config()
        if (current_config.get("streaming_greeks") != getattr(self, '_last_streaming_config', None) or
            current_config.get("greeks") != getattr(self, '_last_api_config', None)):
            logger.info("🔄 Configuration changed, refreshing Greeks manager")
            await self.initialize()
        
        if not symbols:
            return {}
        
        try:
            if self.current_strategy == "streaming" and self.streaming_provider:
                # Try streaming first (TastyTrade)
                try:
                    if hasattr(self.streaming_provider, 'get_streaming_greeks_batch'):
                        logger.info(f"📊 Using streaming Greeks for {len(symbols)} symbols")
                        return await self.streaming_provider.get_streaming_greeks_batch(symbols)
                    else:
                        logger.warning("Streaming provider doesn't support get_streaming_greeks_batch")
                except Exception as e:
                    logger.warning(f"Streaming Greeks failed, falling back to API: {e}")
                    # Fallback to API if available
                    if self.api_provider:
                        logger.info(f"📊 Falling back to API Greeks for {len(symbols)} symbols")
                        return await self.api_provider.get_options_greeks_batch(symbols)
                    return {}
            
            elif self.current_strategy == "api" and self.api_provider:
                # Use API Greeks (Tradier/Alpaca)
                logger.info(f"📊 Using API Greeks for {len(symbols)} symbols")
                return await self.api_provider.get_options_greeks_batch(symbols)
            
            else:
                logger.debug(f"No Greeks provider available (strategy: {self.current_strategy})")
                return {}
                
        except Exception as e:
            logger.error(f"❌ Error getting Greeks batch: {e}")
            return {}
    
    def get_current_strategy(self) -> str:
        """Get the current Greeks strategy being used."""
        return self.current_strategy if self.is_initialized else "unknown"
    
    def get_provider_info(self) -> Dict[str, str]:
        """Get information about configured providers."""
        return {
            "strategy": self.current_strategy,
            "streaming_provider": self.streaming_provider.name if self.streaming_provider else None,
            "api_provider": self.api_provider.name if self.api_provider else None
        }
    
    async def refresh_configuration(self):
        """Refresh the Greeks manager configuration (useful after config changes)."""
        logger.info("🔄 Refreshing Greeks manager configuration")
        self.is_initialized = False
        await self.initialize()

# Create global instance
greeks_manager = GreeksManager()
