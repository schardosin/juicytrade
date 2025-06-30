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
    def __init__(self, option_symbols, price_queue, stock_symbol=None, stock_price_queue=None):
        self.api_key = os.getenv("APCA_API_KEY_ID_LIVE") or os.getenv("APCA_API_KEY_ID_PAPER")
        self.api_secret = os.getenv("APCA_API_SECRET_KEY_LIVE") or os.getenv("APCA_API_SECRET_KEY_PAPER")
        self.option_symbols = option_symbols
        self.price_queue = price_queue
        self.stock_symbol = stock_symbol
        self.stock_price_queue = stock_price_queue
        self.thread = None
        self.running = False

    def start(self):
        self.running = True
        self.thread = threading.Thread(target=self._run, daemon=True)
        self.thread.start()

    def stop(self):
        self.running = False
        # OptionDataStream/StockDataStream do not have a stop method

    def _run(self):
        print("[OptionStreamClientAlpacaSDK] Connecting to Alpaca OptionDataStream...")
        option_stream = OptionDataStream(self.api_key, self.api_secret)

        async def quote_handler(data):
            self.price_queue.put({
                "S": data.symbol,
                "ap": data.ask_price,
                "bp": data.bid_price
            })
            # Trigger a UI update for live sections only
            try:
                st.experimental_rerun()
            except Exception:
                pass

        for symbol in self.option_symbols:
            option_stream.subscribe_quotes(quote_handler, symbol)

        # If stock_symbol and stock_price_queue are provided, stream underlying price too
        if self.stock_symbol and self.stock_price_queue:
            print("[OptionStreamClientAlpacaSDK] Connecting to Alpaca StockDataStream for underlying...")
            stock_stream = StockDataStream(self.api_key, self.api_secret)

            async def stock_quote_handler(data):
                self.stock_price_queue.put({
                    "symbol": data.symbol,
                    "ap": data.ask_price,
                    "bp": data.bid_price
                })
                try:
                    st.experimental_rerun()
                except Exception:
                    pass

            stock_stream.subscribe_quotes(stock_quote_handler, self.stock_symbol)

            # Run both streams concurrently
            import asyncio
            async def run_both():
                await asyncio.gather(
                    option_stream._run_forever(),
                    stock_stream._run_forever()
                )
            asyncio.run(run_both())
        else:
            option_stream.run()
