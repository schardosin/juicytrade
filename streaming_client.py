import requests
import time
import threading
from typing import Dict, List, Optional, Callable, Any
import logging

# Configure logging
logging.basicConfig(
    level=logging.INFO,
    format='%(asctime)s - %(name)s - %(levelname)s - %(message)s'
)
logger = logging.getLogger("streaming_client")

class StreamingClient:
    """Client for interacting with the streaming service API"""
    
    def __init__(self, base_url: str = "http://localhost:8008"):
        self.base_url = base_url
        self.option_callbacks = {}  # symbol -> list of callbacks
        self.stock_callbacks = {}   # symbol -> list of callbacks
        self.polling_thread = None
        self.running = False
        self.poll_interval = 0.5  # seconds
        
    def start_polling(self):
        """Start polling for price updates in a background thread"""
        if self.polling_thread and self.polling_thread.is_alive():
            logger.info("Polling already running")
            return
            
        self.running = True
        self.polling_thread = threading.Thread(target=self._polling_loop, daemon=True)
        self.polling_thread.start()
        logger.info("Started polling thread")
        
    def stop_polling(self):
        """Stop the polling thread"""
        self.running = False
        if self.polling_thread:
            self.polling_thread.join(timeout=2)
            logger.info("Stopped polling thread")
            
    def _polling_loop(self):
        """Background thread that polls for price updates"""
        option_symbols = set()
        stock_symbols = set()
        
        while self.running:
            try:
                # Get current symbols we're interested in
                option_symbols = set(self.option_callbacks.keys())
                stock_symbols = set(self.stock_callbacks.keys())
                
                # Fetch option prices if we have callbacks registered
                if option_symbols:
                    symbols_param = ",".join(option_symbols)
                    response = requests.get(f"{self.base_url}/prices/options?symbols={symbols_param}")
                    if response.status_code == 200:
                        data = response.json()
                        for symbol, price_data in data.items():
                            if symbol in self.option_callbacks and price_data.get("ask") is not None:
                                for callback in self.option_callbacks[symbol]:
                                    try:
                                        callback(symbol, price_data)
                                    except Exception as e:
                                        logger.error(f"Error in option callback for {symbol}: {e}")
                
                # Fetch stock prices if we have callbacks registered
                if stock_symbols:
                    symbols_param = ",".join(stock_symbols)
                    response = requests.get(f"{self.base_url}/prices/stocks?symbols={symbols_param}")
                    if response.status_code == 200:
                        data = response.json()
                        for symbol, price_data in data.items():
                            if symbol in self.stock_callbacks and price_data.get("ask") is not None:
                                for callback in self.stock_callbacks[symbol]:
                                    try:
                                        callback(symbol, price_data)
                                    except Exception as e:
                                        logger.error(f"Error in stock callback for {symbol}: {e}")
                
            except Exception as e:
                logger.error(f"Error in polling loop: {e}")
                
            # Sleep before next poll
            time.sleep(self.poll_interval)
    
    def subscribe_option(self, symbol: str, callback: Callable[[str, Dict[str, Any]], None]) -> bool:
        """
        Subscribe to option price updates
        
        Args:
            symbol: Option symbol to subscribe to
            callback: Function to call when price updates are received
                     The callback will receive (symbol, price_data) where price_data is a dict
                     with keys 'ask', 'bid', and 'timestamp'
                     
        Returns:
            bool: True if subscription was successful
        """
        try:
            # Register callback locally
            if symbol not in self.option_callbacks:
                self.option_callbacks[symbol] = []
            self.option_callbacks[symbol].append(callback)
            
            # Subscribe on the server
            response = requests.post(
                f"{self.base_url}/subscribe/options",
                json={"symbols": [symbol]}
            )
            
            if response.status_code == 200:
                logger.info(f"Subscribed to option: {symbol}")
                # Start polling if not already running
                if not self.polling_thread or not self.polling_thread.is_alive():
                    self.start_polling()
                return True
            else:
                logger.error(f"Failed to subscribe to option {symbol}: {response.text}")
                return False
                
        except Exception as e:
            logger.error(f"Error subscribing to option {symbol}: {e}")
            return False
    
    def subscribe_stock(self, symbol: str, callback: Callable[[str, Dict[str, Any]], None]) -> bool:
        """
        Subscribe to stock price updates
        
        Args:
            symbol: Stock symbol to subscribe to
            callback: Function to call when price updates are received
                     The callback will receive (symbol, price_data) where price_data is a dict
                     with keys 'ask', 'bid', and 'timestamp'
                     
        Returns:
            bool: True if subscription was successful
        """
        try:
            # Register callback locally
            if symbol not in self.stock_callbacks:
                self.stock_callbacks[symbol] = []
            self.stock_callbacks[symbol].append(callback)
            
            # Subscribe on the server
            response = requests.post(
                f"{self.base_url}/subscribe/stocks",
                json={"symbols": [symbol]}
            )
            
            if response.status_code == 200:
                logger.info(f"Subscribed to stock: {symbol}")
                # Start polling if not already running
                if not self.polling_thread or not self.polling_thread.is_alive():
                    self.start_polling()
                return True
            else:
                logger.error(f"Failed to subscribe to stock {symbol}: {response.text}")
                return False
                
        except Exception as e:
            logger.error(f"Error subscribing to stock {symbol}: {e}")
            return False
    
    def unsubscribe_option(self, symbol: str, callback: Optional[Callable] = None) -> bool:
        """
        Unsubscribe from option price updates
        
        Args:
            symbol: Option symbol to unsubscribe from
            callback: Specific callback to remove, or None to remove all callbacks
                     
        Returns:
            bool: True if unsubscription was successful
        """
        try:
            # Remove callback locally
            if symbol in self.option_callbacks:
                if callback is None:
                    del self.option_callbacks[symbol]
                else:
                    self.option_callbacks[symbol] = [cb for cb in self.option_callbacks[symbol] if cb != callback]
                    if not self.option_callbacks[symbol]:
                        del self.option_callbacks[symbol]
            
            # If no more callbacks for this symbol, unsubscribe on the server
            if symbol not in self.option_callbacks:
                response = requests.post(
                    f"{self.base_url}/unsubscribe/options",
                    json={"symbols": [symbol]}
                )
                
                if response.status_code == 200:
                    logger.info(f"Unsubscribed from option: {symbol}")
                    return True
                else:
                    logger.error(f"Failed to unsubscribe from option {symbol}: {response.text}")
                    return False
            return True
                
        except Exception as e:
            logger.error(f"Error unsubscribing from option {symbol}: {e}")
            return False
    
    def unsubscribe_stock(self, symbol: str, callback: Optional[Callable] = None) -> bool:
        """
        Unsubscribe from stock price updates
        
        Args:
            symbol: Stock symbol to unsubscribe from
            callback: Specific callback to remove, or None to remove all callbacks
                     
        Returns:
            bool: True if unsubscription was successful
        """
        try:
            # Remove callback locally
            if symbol in self.stock_callbacks:
                if callback is None:
                    del self.stock_callbacks[symbol]
                else:
                    self.stock_callbacks[symbol] = [cb for cb in self.stock_callbacks[symbol] if cb != callback]
                    if not self.stock_callbacks[symbol]:
                        del self.stock_callbacks[symbol]
            
            # If no more callbacks for this symbol, unsubscribe on the server
            if symbol not in self.stock_callbacks:
                response = requests.post(
                    f"{self.base_url}/unsubscribe/stocks",
                    json={"symbols": [symbol]}
                )
                
                if response.status_code == 200:
                    logger.info(f"Unsubscribed from stock: {symbol}")
                    return True
                else:
                    logger.error(f"Failed to unsubscribe from stock {symbol}: {response.text}")
                    return False
            return True
                
        except Exception as e:
            logger.error(f"Error unsubscribing from stock {symbol}: {e}")
            return False
    
    def get_option_price(self, symbol: str) -> Optional[Dict[str, Any]]:
        """
        Get the latest price for an option symbol
        
        Args:
            symbol: Option symbol to get price for
                     
        Returns:
            dict: Price data with keys 'ask', 'bid', and 'timestamp', or None if not available
        """
        try:
            response = requests.get(f"{self.base_url}/prices/option/{symbol}")
            if response.status_code == 200:
                return response.json()
            else:
                logger.error(f"Failed to get option price for {symbol}: {response.text}")
                return None
        except Exception as e:
            logger.error(f"Error getting option price for {symbol}: {e}")
            return None
    
    def get_stock_price(self, symbol: str) -> Optional[Dict[str, Any]]:
        """
        Get the latest price for a stock symbol
        
        Args:
            symbol: Stock symbol to get price for
                     
        Returns:
            dict: Price data with keys 'ask', 'bid', and 'timestamp', or None if not available
        """
        try:
            response = requests.get(f"{self.base_url}/prices/stock/{symbol}")
            if response.status_code == 200:
                return response.json()
            else:
                logger.error(f"Failed to get stock price for {symbol}: {response.text}")
                return None
        except Exception as e:
            logger.error(f"Error getting stock price for {symbol}: {e}")
            return None
    
    def restart_streaming(self) -> bool:
        """
        Restart the streaming service
        
        Returns:
            bool: True if restart was successful
        """
        try:
            response = requests.post(f"{self.base_url}/restart")
            if response.status_code == 200:
                logger.info("Streaming service restarted")
                return True
            else:
                logger.error(f"Failed to restart streaming service: {response.text}")
                return False
        except Exception as e:
            logger.error(f"Error restarting streaming service: {e}")
            return False
    
    def check_service_status(self) -> Optional[Dict[str, Any]]:
        """
        Check the status of the streaming service
        
        Returns:
            dict: Status information, or None if service is not available
        """
        try:
            response = requests.get(f"{self.base_url}/")
            if response.status_code == 200:
                return response.json()
            else:
                logger.error(f"Failed to get service status: {response.text}")
                return None
        except Exception as e:
            logger.error(f"Error checking service status: {e}")
            return None
