import os
import threading
import queue
from alpaca.data.live import OptionDataStream, StockDataStream

import streamlit as st
import os
import threading
import queue
from alpaca.data.live import OptionDataStream, StockDataStream

class OptionStreamClientAlpacaSDK:
    _instance = None

    def __new__(cls, *args, **kwargs):
        if cls._instance is None:
            cls._instance = super().__new__(cls)
        return cls._instance

    def __init__(self, option_symbols, price_queue, stock_symbol=None, stock_price_queue=None):
        if hasattr(self, "initialized") and self.initialized:
            # Only update symbols if already initialized
            self.update_symbols(option_symbols)
            return
        self.api_key = os.getenv("APCA_API_KEY_ID_LIVE") or os.getenv("APCA_API_KEY_ID_PAPER")
        self.api_secret = os.getenv("APCA_API_SECRET_KEY_LIVE") or os.getenv("APCA_API_SECRET_KEY_PAPER")
        self.option_symbols = set(option_symbols)
        self.price_queue = price_queue
        self.stock_symbol = stock_symbol
        self.stock_price_queue = stock_price_queue
        self.thread = None
        self.running = False
        self.option_stream = None
        self.stock_stream = None
        self.initialized = True

    def update_symbols(self, new_symbols):
        # Update the set of tracked symbols on the fly
        new_set = set(new_symbols)
        to_add = new_set - self.option_symbols
        to_remove = self.option_symbols - new_set
        if self.option_stream:
            for symbol in to_add:
                self.option_stream.subscribe_quotes(self._quote_handler, symbol)
            for symbol in to_remove:
                self.option_stream.unsubscribe_quotes(symbol)
        self.option_symbols = new_set

    async def _quote_handler(self, data):
        self.price_queue.put({
            "S": data.symbol,
            "ap": data.ask_price,
            "bp": data.bid_price
        })
        try:
            st.experimental_rerun()
        except Exception:
            pass

    async def _stock_quote_handler(self, data):
        self.stock_price_queue.put({
            "symbol": data.symbol,
            "ap": data.ask_price,
            "bp": data.bid_price
        })
        try:
            st.experimental_rerun()
        except Exception:
            pass

    def start(self):
        # Only start if not already running
        if hasattr(self, "thread") and self.thread and self.thread.is_alive():
            print("[OptionStreamClientAlpacaSDK] Stream already running, not starting a new one.")
            return
        # If a previous thread exists, try to join it before starting a new one
        if hasattr(self, "thread") and self.thread:
            try:
                self.running = False
                self.thread.join(timeout=5)
                print("[OptionStreamClientAlpacaSDK] Previous thread joined/stopped.")
            except Exception as e:
                print(f"[OptionStreamClientAlpacaSDK] Error joining previous thread: {e}")
        self.running = True
        self.thread = threading.Thread(target=self._run, daemon=True)
        self.thread.start()

    def stop(self):
        self.running = False
        # OptionDataStream/StockDataStream do not have a stop method
        # But we can try to close the thread by setting running to False
        if hasattr(self, "thread") and self.thread and self.thread.is_alive():
            print("[OptionStreamClientAlpacaSDK] Attempting to stop stream thread...")
            # No direct way to kill thread, but setting running to False will exit _run loop on reconnect

    def _run(self):
        print("[OptionStreamClientAlpacaSDK] Connecting to Alpaca OptionDataStream...")
        self.option_stream = OptionDataStream(self.api_key, self.api_secret)
        for symbol in self.option_symbols:
            self.option_stream.subscribe_quotes(self._quote_handler, symbol)

        if self.stock_symbol and self.stock_price_queue:
            print("[OptionStreamClientAlpacaSDK] Connecting to Alpaca StockDataStream for underlying...")
            self.stock_stream = StockDataStream(self.api_key, self.api_secret)
            self.stock_stream.subscribe_quotes(self._stock_quote_handler, self.stock_symbol)
            import asyncio
            async def run_both():
                await asyncio.gather(
                    self.option_stream._run_forever(),
                    self.stock_stream._run_forever()
                )
            asyncio.run(run_both())
        else:
            self.option_stream.run()
