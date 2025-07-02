import streamlit as st
import requests
import time
import numpy as np
import pandas as pd
import matplotlib.pyplot as plt
from streaming_client import StreamingClient
from datetime import datetime
import logging
import warnings

# Set up logging
logging.basicConfig(level=logging.DEBUG, format='%(asctime)s - %(name)s - %(levelname)s - %(message)s')
logger = logging.getLogger(__name__)

# Suppress Streamlit's warnings
logging.getLogger('streamlit').setLevel(logging.ERROR)
logging.getLogger('matplotlib').setLevel(logging.ERROR)

# Suppress "missing ScriptRunContext" warning from Streamlit internals
try:
    script_runner_logger = logging.getLogger("streamlit.runtime.scriptrunner.script_run_context")
    script_runner_logger.propagate = False
    script_runner_logger.setLevel(logging.CRITICAL)
except Exception:
    pass

warnings.filterwarnings("ignore", category=DeprecationWarning)
warnings.filterwarnings("ignore", category=FutureWarning)

# Disable threading warnings
logging.getLogger('threading').setLevel(logging.CRITICAL)

# Initialize streaming client
streaming_client = StreamingClient(base_url="http://localhost:8008")

# API base URL
API_BASE_URL = "http://localhost:8008"

st.set_page_config(page_title="Iron Butterfly Trade", layout="centered")
st.title("Robust Iron Butterfly Trade")
st.header("Step 1: Configure Butterfly Trade")

# Check streaming service status
st.write("Checking streaming service status...")
service_status = streaming_client.check_service_status()
if service_status:
    st.success(f"Streaming service is running. Options subscribed: {service_status.get('options_subscribed', 0)}, Stocks subscribed: {service_status.get('stocks_subscribed', 0)}")
else:
    st.error("Streaming service is not running or not responding. Please ensure the service is started with 'python streaming_service.py' and check the console for error messages.")
    st.write("Attempting to connect to: " + streaming_client.base_url)

# Add a restart button for the streaming service
if st.button("Restart Streaming Service"):
    if streaming_client.restart_streaming():
        st.success("Streaming service restarted successfully")
    else:
        st.error("Failed to restart streaming service")

def get_next_market_date():
    response = requests.get(f"{API_BASE_URL}/next_market_date")
    if response.status_code == 200:
        return response.json()["next_market_date"]
    return datetime.now().strftime("%Y-%m-%d")

next_market_date_str = get_next_market_date()
next_market_date = datetime.strptime(next_market_date_str, "%Y-%m-%d").date()
expiry = st.date_input("Option Expiry Date", value=next_market_date)
strategy_type = st.selectbox(
    "Butterfly Strategy Type",
    options=["IRON Butterfly", "CALL Butterfly", "PUT Butterfly"],
    index=0
)
leg_width = st.number_input("Butterfly Leg Width", min_value=1, max_value=100, value=2, step=1)
order_offset = st.number_input(
    "Order Price Offset",
    min_value=-10.0,
    max_value=10.0,
    value=0.05,
    step=0.01,
    help="Amount to add/subtract from the calculated butterfly price to set your order price."
)
symbol = st.text_input("Underlying Symbol", value="SPY", max_chars=10)

def get_underlying_price(symbol):
    price = st.session_state.get("underlying_stream_price")
    if price is None:
        # If streaming price is not available, fetch from the API
        response = requests.get(f"{API_BASE_URL}/prices/stocks?symbols={symbol}")
        logger.debug(f"API response for {symbol}: {response.text}")
        if response.status_code == 200:
            data = response.json()
            logger.debug(f"Parsed JSON data: {data}")
            if symbol in data:
                price_data = data[symbol]
                logger.debug(f"Price data for {symbol}: {price_data}")
                if isinstance(price_data, dict):
                    ask = price_data.get('ask')
                    bid = price_data.get('bid')
                    if ask is not None and bid is not None:
                        price = (ask + bid) / 2
                    elif ask is not None:
                        price = ask
                    elif bid is not None:
                        price = bid
                elif isinstance(price_data, (int, float)):
                    price = price_data
        if price is None:
            logger.error(f"Unable to fetch price for {symbol}")
    return price

def get_options_chain(symbol, expiry, strategy_type):
    response = requests.get(f"{API_BASE_URL}/options_chain", params={
        "symbol": symbol,
        "expiry": expiry.strftime("%Y-%m-%d"),
        "strategy_type": strategy_type
    })
    logger.debug(f"Options chain API response: {response.text}")
    if response.status_code == 200:
        chain = response.json()
        if not chain:
            logger.debug(f"Empty options chain returned for {symbol}, {expiry}, {strategy_type}")
        return chain
    else:
        logger.error(f"Failed to fetch options chain: {response.status_code} - {response.text}")
    return []

# Add this function to fetch option prices
def fetch_option_prices(symbols):
    if not symbols:
        return {}
    response = requests.get(f"{API_BASE_URL}/prices/options", params={"symbols": ",".join(symbols)})
    if response.status_code == 200:
        return response.json()
    else:
        logger.error(f"Failed to fetch option prices: {response.status_code} - {response.text}")
        return {}

# Initialize session state for streaming data
if "option_leg_prices" not in st.session_state:
    st.session_state["option_leg_prices"] = {}
if "underlying_stream_price" not in st.session_state:
    st.session_state["underlying_stream_price"] = None

# Callback for stock price updates
def stock_price_callback(symbol, price_data):
    if price_data.get("ask") is not None and price_data.get("bid") is not None:
        st.session_state["underlying_stream_price"] = (price_data["ask"] + price_data["bid"]) / 2
    elif price_data.get("ask") is not None:
        st.session_state["underlying_stream_price"] = price_data["ask"]
    elif price_data.get("bid") is not None:
        st.session_state["underlying_stream_price"] = price_data["bid"]
    st.experimental_rerun = getattr(st, "experimental_rerun", None)
    if st.experimental_rerun:
        st.experimental_rerun()

# Callback for option price updates
def option_price_callback(symbol, price_data):
    logger.debug(f"Received option price update for {symbol}")
    if price_data.get("ask") is not None and price_data.get("bid") is not None:
        st.session_state["option_leg_prices"][symbol] = {
            "ask": price_data["ask"],
            "bid": price_data["bid"]
        }
        logger.debug(f"Updated option price: {symbol} - Ask: {price_data['ask']}, Bid: {price_data['bid']}")
    else:
        logger.debug(f"Invalid price data for {symbol}: {price_data}")
    print_option_leg_prices()

# Debug function to print current option leg prices
def print_option_leg_prices():
    logger.debug("Current option leg prices:")
    for symbol, prices in st.session_state["option_leg_prices"].items():
        logger.debug(f"  {symbol}: Ask: {prices['ask']}, Bid: {prices['bid']}")

# Use streaming price for underlying if available
underlying_stream_price = st.session_state.get("underlying_stream_price", None)
if underlying_stream_price is not None:
    st.success(f"Underlying ({symbol}) price (live): ${underlying_stream_price:.2f}")
else:
    underlying = get_underlying_price(symbol)
    if underlying:
        st.success(f"Underlying ({symbol}) price: ${underlying:.2f}")
    else:
        st.error(f"Could not fetch price for {symbol}.")

options_chain = get_options_chain(symbol, expiry, strategy_type)
if options_chain:
    st.info(f"Fetched {len(options_chain)} option contracts for {symbol} expiring {expiry}.")
else:
    st.error("Could not fetch options chain or no contracts available.")

def find_closest_strike(strikes, target):
    return min(strikes, key=lambda x: abs(x - target))

def option_price(opt):
    if opt and "symbol" in opt and "option_leg_prices" in st.session_state:
        symbol = opt["symbol"]
        price_info = st.session_state["option_leg_prices"].get(symbol)
        if price_info:
            bid = price_info.get("bid")
            ask = price_info.get("ask")
            if bid is not None and ask is not None:
                price = (bid + ask) / 2
                logger.debug(f"Live price for {symbol}: {price} (bid: {bid}, ask: {ask})")
                return price
            elif ask is not None:
                logger.debug(f"Live price for {symbol}: {ask} (ask only)")
                return ask
            elif bid is not None:
                logger.debug(f"Live price for {symbol}: {bid} (bid only)")
                return bid
    price = float(opt.get("close_price") or 0)
    logger.debug(f"Using close price for {opt['symbol']}: {price}")
    return price

def get_strike_prices(options_chain, strategy_type):
    strikes = []
    strike_to_option = {}
    for opt in options_chain:
        if strategy_type == "IRON Butterfly":
            strike = float(opt["strike_price"])
            strikes.append(strike)
            strike_to_option[strike] = opt
        elif strategy_type == "CALL Butterfly" and opt["type"] == "call":
            strike = float(opt["strike_price"])
            strikes.append(strike)
            strike_to_option[strike] = opt
        elif strategy_type == "PUT Butterfly" and opt["type"] == "put":
            strike = float(opt["strike_price"])
            strikes.append(strike)
            strike_to_option[strike] = opt
    return sorted(strikes), strike_to_option

butterfly_info = None

def get_option_symbols_for_legs():
    if not options_chain or not butterfly_info:
        return []
    symbols = []
    if strategy_type == "IRON Butterfly":
        for o in options_chain:
            strike = float(o["strike_price"])
            if o["type"] == "put" and abs(strike - butterfly_info["lower_strike"]) < 0.01:
                symbols.append(o["symbol"])
            if o["type"] == "put" and abs(strike - butterfly_info["atm_strike"]) < 0.01:
                symbols.append(o["symbol"])
            if o["type"] == "call" and abs(strike - butterfly_info["atm_strike"]) < 0.01:
                symbols.append(o["symbol"])
            if o["type"] == "call" and abs(strike - butterfly_info["upper_strike"]) < 0.01:
                symbols.append(o["symbol"])
    else:
        for o in options_chain:
            strike = float(o["strike_price"])
            if abs(strike - butterfly_info["lower_strike"]) < 0.01:
                symbols.append(o["symbol"])
            if abs(strike - butterfly_info["atm_strike"]) < 0.01:
                symbols.append(o["symbol"])
            if abs(strike - butterfly_info["upper_strike"]) < 0.01:
                symbols.append(o["symbol"])
    symbols = list(set(symbols))
    logger.debug(f"Option symbols for legs: {symbols}")
    return symbols

def start_streaming_if_needed():
    symbols = get_option_symbols_for_legs()
    
    logger.debug(f"Attempting to subscribe to stock: {symbol}")
    # Subscribe to stock symbol
    if streaming_client.subscribe_stock(symbol, stock_price_callback):
        logger.debug(f"Successfully subscribed to stock: {symbol}")
    else:
        logger.error(f"Failed to subscribe to stock: {symbol}")
    
    # Subscribe to option symbols
    for option_symbol in symbols:
        logger.debug(f"Attempting to subscribe to option: {option_symbol}")
        if streaming_client.subscribe_option(option_symbol, option_price_callback):
            logger.debug(f"Successfully subscribed to option: {option_symbol}")
        else:
            logger.error(f"Failed to subscribe to option: {option_symbol}")
    
    # Print initial option leg prices
    print_option_leg_prices()

# This function has been removed as it's a duplicate of the one using logger

# --- LIVE UPDATE UI SECTION ---
price_placeholder = st.empty()
chart_placeholder = st.empty()

# Ensure butterfly_info is initialized before streaming starts
def ensure_butterfly_info_initialized():
    global butterfly_info
    strikes, strike_to_option = get_strike_prices(options_chain, strategy_type)
    # Use live underlying price if available, else fallback to REST
    underlying_for_butterfly = st.session_state.get("underlying_stream_price", None)
    if underlying_for_butterfly is None:
        underlying_for_butterfly = get_underlying_price(symbol)
    if len(strikes) >= 3 and underlying_for_butterfly is not None:
        atm_strike = find_closest_strike(strikes, underlying_for_butterfly)
        lower_strike = None
        upper_strike = None
        for s in strikes:
            if s < atm_strike and abs(atm_strike - s - leg_width) < 1e-6:
                lower_strike = s
        for s in strikes:
            if s > atm_strike and abs(s - atm_strike - leg_width) < 1e-6:
                upper_strike = s
        if lower_strike is None:
            lower_strike = max([s for s in strikes if s < atm_strike], default=min(strikes))
        if upper_strike is None:
            upper_strike = min([s for s in strikes if s > atm_strike], default=max(strikes))
        if lower_strike != atm_strike and upper_strike != atm_strike:
            option_symbols = []
            if strategy_type == "IRON Butterfly":
                put_lower = next((o for o in options_chain if float(o["strike_price"]) == lower_strike and o["type"] == "put"), None)
                put_atm = next((o for o in options_chain if float(o["strike_price"]) == atm_strike and o["type"] == "put"), None)
                call_atm = next((o for o in options_chain if float(o["strike_price"]) == atm_strike and o["type"] == "call"), None)
                call_upper = next((o for o in options_chain if float(o["strike_price"]) == upper_strike and o["type"] == "call"), None)
                option_symbols = [o["symbol"] for o in [put_lower, put_atm, call_atm, call_upper] if o]
            else:
                atm_option = strike_to_option[atm_strike]
                lower_option = strike_to_option[lower_strike]
                upper_option = strike_to_option[upper_strike]
                option_symbols = [o["symbol"] for o in [lower_option, atm_option, upper_option] if o]
            
            option_prices = fetch_option_prices(option_symbols)
            
            if strategy_type == "IRON Butterfly":
                butterfly_info = {
                    "lower_strike": lower_strike,
                    "atm_strike": atm_strike,
                    "upper_strike": upper_strike,
                    "lower_price": option_prices.get(put_lower["symbol"], {}).get("mid", 0) if put_lower else 0,
                    "atm_put_price": option_prices.get(put_atm["symbol"], {}).get("mid", 0) if put_atm else 0,
                    "atm_call_price": option_prices.get(call_atm["symbol"], {}).get("mid", 0) if call_atm else 0,
                    "upper_price": option_prices.get(call_upper["symbol"], {}).get("mid", 0) if call_upper else 0,
                }
            else:
                butterfly_info = {
                    "lower_strike": lower_strike,
                    "atm_strike": atm_strike,
                    "upper_strike": upper_strike,
                    "lower_price": option_prices.get(lower_option["symbol"], {}).get("mid", 0),
                    "atm_price": option_prices.get(atm_option["symbol"], {}).get("mid", 0),
                    "upper_price": option_prices.get(upper_option["symbol"], {}).get("mid", 0),
                }
    return butterfly_info

# Ensure butterfly_info is initialized before streaming
butterfly_info = ensure_butterfly_info_initialized()
start_streaming_if_needed()

def render_live_prices_and_chart():
    global butterfly_info
    strikes, strike_to_option = get_strike_prices(options_chain, strategy_type)
    # Use live underlying price if available, else fallback to REST
    underlying_for_butterfly = st.session_state.get("underlying_stream_price", None)
    if underlying_for_butterfly is None:
        underlying_for_butterfly = get_underlying_price(symbol)
    logger.debug(f"Underlying price: {underlying_for_butterfly}")
    print_option_leg_prices()
    
    # Fetch latest option prices
    option_symbols = get_option_symbols_for_legs()
    latest_option_prices = fetch_option_prices(option_symbols)
    logger.debug(f"Latest option prices: {latest_option_prices}")
    
    if len(strikes) >= 3 and underlying_for_butterfly is not None:
        atm_strike = find_closest_strike(strikes, underlying_for_butterfly)
        lower_strike = None
        upper_strike = None
        for s in strikes:
            if s < atm_strike and abs(atm_strike - s - leg_width) < 1e-6:
                lower_strike = s
        for s in strikes:
            if s > atm_strike and abs(s - atm_strike - leg_width) < 1e-6:
                upper_strike = s
        if lower_strike is None:
            lower_strike = max([s for s in strikes if s < atm_strike], default=min(strikes))
        if upper_strike is None:
            upper_strike = min([s for s in strikes if s > atm_strike], default=max(strikes))
        global butterfly_info
        if lower_strike != atm_strike and upper_strike != atm_strike:
            if strategy_type == "IRON Butterfly":
                put_lower = next((o for o in options_chain if float(o["strike_price"]) == lower_strike and o["type"] == "put"), None)
                put_atm = next((o for o in options_chain if float(o["strike_price"]) == atm_strike and o["type"] == "put"), None)
                call_atm = next((o for o in options_chain if float(o["strike_price"]) == atm_strike and o["type"] == "call"), None)
                call_upper = next((o for o in options_chain if float(o["strike_price"]) == upper_strike and o["type"] == "call"), None)
                butterfly_info = {
                    "lower_strike": lower_strike,
                    "atm_strike": atm_strike,
                    "upper_strike": upper_strike,
                    "lower_price": latest_option_prices.get(put_lower["symbol"], {}).get("mid", 0) if put_lower else 0,
                    "atm_put_price": latest_option_prices.get(put_atm["symbol"], {}).get("mid", 0) if put_atm else 0,
                    "atm_call_price": latest_option_prices.get(call_atm["symbol"], {}).get("mid", 0) if call_atm else 0,
                    "upper_price": latest_option_prices.get(call_upper["symbol"], {}).get("mid", 0) if call_upper else 0,
                }
            else:
                atm_option = strike_to_option[atm_strike]
                lower_option = strike_to_option[lower_strike]
                upper_option = strike_to_option[upper_strike]
                butterfly_info = {
                    "lower_strike": lower_strike,
                    "atm_strike": atm_strike,
                    "upper_strike": upper_strike,
                    "lower_price": latest_option_prices.get(lower_option["symbol"], {}).get("mid", 0),
                    "atm_price": latest_option_prices.get(atm_option["symbol"], {}).get("mid", 0),
                    "upper_price": latest_option_prices.get(upper_option["symbol"], {}).get("mid", 0),
                }
            with price_placeholder.container():
                st.subheader("Selected Butterfly Strikes (Live)")
                import pandas as pd
                legs_table = []
                if strategy_type == "IRON Butterfly":
                    legs_table = [
                        {"Action": "Buy", "Type": "Put", "Strike": lower_strike, "Price": butterfly_info['lower_price']},
                        {"Action": "Sell", "Type": "Put", "Strike": atm_strike, "Price": butterfly_info.get('atm_put_price', 0)},
                        {"Action": "Sell", "Type": "Call", "Strike": atm_strike, "Price": butterfly_info.get('atm_call_price', 0)},
                        {"Action": "Buy", "Type": "Call", "Strike": upper_strike, "Price": butterfly_info['upper_price']},
                    ]
                elif strategy_type == "CALL Butterfly":
                    legs_table = [
                        {"Action": "Buy", "Type": "Call", "Strike": lower_strike, "Price": butterfly_info['lower_price']},
                        {"Action": "Sell", "Type": "Call", "Strike": atm_strike, "Price": butterfly_info.get('atm_price', 0)},
                        {"Action": "Buy", "Type": "Call", "Strike": upper_strike, "Price": butterfly_info['upper_price']},
                    ]
                elif strategy_type == "PUT Butterfly":
                    legs_table = [
                        {"Action": "Buy", "Type": "Put", "Strike": lower_strike, "Price": butterfly_info['lower_price']},
                        {"Action": "Sell", "Type": "Put", "Strike": atm_strike, "Price": butterfly_info.get('atm_price', 0)},
                        {"Action": "Buy", "Type": "Put", "Strike": upper_strike, "Price": butterfly_info['upper_price']},
                    ]
                # Always fetch the latest price from st.session_state["option_leg_prices"] for each symbol
                for leg in legs_table:
                    # Find the matching option in options_chain for this leg
                    opt = None
                    for o in options_chain:
                        if float(o["strike_price"]) == leg["Strike"]:
                            if (leg["Type"].lower() == o["type"]) or (o["type"] in ["call", "put"] and leg["Type"].lower() in o["type"]):
                                opt = o
                                break
                    if opt:
                        live_price = option_price(opt)
                        leg["Price"] = live_price
                st.dataframe(pd.DataFrame(legs_table), use_container_width=True)
            with chart_placeholder.container():
                import matplotlib.pyplot as plt
                plt.close("all")
                if strategy_type == "IRON Butterfly":
                    net_premium = (
                        butterfly_info["lower_price"]
                        - butterfly_info["atm_put_price"]
                        - butterfly_info["atm_call_price"]
                        + butterfly_info["upper_price"]
                    )
                else:
                    net_premium = (
                        butterfly_info["lower_price"]
                        - 2 * butterfly_info["atm_price"]
                        + butterfly_info["upper_price"]
                    )
                lower_bound = int(butterfly_info["lower_strike"] - leg_width)
                upper_bound = int(butterfly_info["upper_strike"] + leg_width)
                center = butterfly_info["atm_strike"]
                step = 1
                if upper_bound - lower_bound > 100:
                    step = 2
                if upper_bound - lower_bound > 200:
                    step = 5
                x = np.arange(lower_bound, upper_bound + step, step)
                if strategy_type == "IRON Butterfly":
                    width = abs(atm_strike - lower_strike)
                    max_profit = abs(net_premium) * 100
                    max_loss = (width - abs(net_premium)) * 100
                    payoff = np.zeros_like(x)
                    for i, price in enumerate(x):
                        if price <= lower_strike:
                            payoff[i] = -max_loss
                        elif price < atm_strike:
                            ratio = (price - lower_strike) / width
                            payoff[i] = -max_loss + ratio * (max_loss + max_profit)
                        elif price <= upper_strike:
                            ratio = (upper_strike - price) / width
                            payoff[i] = -max_loss + ratio * (max_loss + max_profit)
                        else:
                            payoff[i] = -max_loss
                else:
                    payoff = (
                        np.maximum(x - butterfly_info["lower_strike"], 0)
                        - 2 * np.maximum(x - butterfly_info["atm_strike"], 0)
                        + np.maximum(x - butterfly_info["upper_strike"], 0)
                        - net_premium
                    ) * 100
                if strategy_type == "IRON Butterfly":
                    lower_be = butterfly_info["atm_strike"] - abs(net_premium)
                    upper_be = butterfly_info["atm_strike"] + abs(net_premium)
                else:
                    lower_be = butterfly_info["lower_strike"] + net_premium
                    upper_be = butterfly_info["upper_strike"] - net_premium
                fig, ax = plt.subplots()
                ax.plot(x, payoff, label="Butterfly Payoff")
                ax.axhline(0, color="gray", linestyle="--")
                ax.axvline(lower_be, color="red", linestyle=":", label="Lower Break Even")
                ax.axvline(upper_be, color="blue", linestyle=":", label="Upper Break Even")
                ax.axvline(underlying_for_butterfly, color="purple", linestyle="--", label=f"Current Price: {underlying_for_butterfly:.2f}")
                ax.text(lower_be, ax.get_ylim()[0], f"{lower_be:.2f}", color="red", ha="center", va="bottom", fontsize=9, fontweight="bold", bbox=dict(facecolor='white', edgecolor='red', boxstyle='round,pad=0.2'))
                ax.text(upper_be, ax.get_ylim()[0], f"{upper_be:.2f}", color="blue", ha="center", va="bottom", fontsize=9, fontweight="bold", bbox=dict(facecolor='white', edgecolor='blue', boxstyle='round,pad=0.2'))
                ax.text(underlying_for_butterfly, ax.get_ylim()[1], f"{underlying_for_butterfly:.2f}", color="purple", ha="center", va="top", fontsize=9, fontweight="bold", bbox=dict(facecolor='white', edgecolor='purple', boxstyle='round,pad=0.2'))
                ax.set_xlabel("Underlying Price at Expiry")
                ax.set_ylabel("Profit / Loss ($)")
                ax.set_title("Butterfly Payoff Diagram")
                st.pyplot(fig)
                plt.close(fig)
                st.write(f"**ATM Strike (Center):** {center}")
                st.write(f"**Lower Break Even:** {lower_be:.2f}")
                st.write(f"**Upper Break Even:** {upper_be:.2f}")

# --- Place Order Button and Dialog Logic ---
st.markdown("---")
# Place order button
if "order_confirm" not in st.session_state:
    st.session_state["order_confirm"] = False
if "order_result" not in st.session_state:
    st.session_state["order_result"] = None

import pandas as pd
import re

def parse_option_symbol(symbol):
    match = re.match(r'([A-Z]+)(\d{6})([CP])(\d+)', symbol)
    if match:
        ticker, date_str, option_type, strike_str = match.groups()
        date = f"20{date_str[:2]}-{date_str[2:4]}-{date_str[4:6]}"
        strike = float(strike_str) / 1000
        option_type = "Call" if option_type == "C" else "Put"
        return ticker, date, option_type, strike
    return symbol, "", "", 0

@st.dialog("Order Confirmation")
def confirm_order_dialog(order_price, underlying_price, max_profit, max_loss, current_options_price, legs):
    st.write(f"**Current Options Price (legs combined):** ${current_options_price:.2f}")
    st.write(f"**Order Price:** ${order_price:.2f}")
    st.write(f"**Underlying Price:** ${underlying_price:.2f}")
    st.write(f"**Max Profit:** ${max_profit:.2f}")
    st.write(f"**Max Loss:** ${max_loss:.2f}")
    st.write("**Legs to be Traded:**")
    table_data = []
    for leg in legs:
        ticker, date, option_type, strike = parse_option_symbol(leg['symbol'])
        price = 0.0
        if strategy_type == "IRON Butterfly":
            if option_type == "Put" and abs(strike - butterfly_info["lower_strike"]) < 0.01:
                price = butterfly_info["lower_price"]
            elif option_type == "Put" and abs(strike - butterfly_info["atm_strike"]) < 0.01:
                price = butterfly_info["atm_put_price"]
            elif option_type == "Call" and abs(strike - butterfly_info["atm_strike"]) < 0.01:
                price = butterfly_info["atm_call_price"]
            elif option_type == "Call" and abs(strike - butterfly_info["upper_strike"]) < 0.01:
                price = butterfly_info["upper_price"]
        else:
            if abs(strike - butterfly_info["lower_strike"]) < 0.01:
                price = butterfly_info["lower_price"]
            elif abs(strike - butterfly_info["atm_strike"]) < 0.01:
                price = butterfly_info["atm_price"]
            elif abs(strike - butterfly_info["upper_strike"]) < 0.01:
                price = butterfly_info["upper_price"]
        table_data.append({
            "Action": leg['side'].capitalize(),
            "Symbol": ticker,
            "Date": date,
            "Type": option_type,
            "Strike": f"${strike:.2f}",
            "Price": f"${price:.2f}"
        })
    st.table(pd.DataFrame(table_data))
    if st.button("Confirm Order", key="confirm_order_btn"):
        st.session_state["order_confirm"] = True
        st.rerun()

# Only show the button if not already confirmed to avoid duplicate keys
show_btn = not st.session_state.get("order_confirm", False)
# Use a unique key for the button based on the current time to avoid duplicate key errors
btn_key = f"place_butterfly_order_btn_{int(time.time())}"
if (show_btn and st.button("Place Butterfly Order", key=btn_key)) or st.session_state["order_confirm"]:
    if butterfly_info and (
        (strategy_type == "IRON Butterfly" and all(
            price is not None for price in [
                butterfly_info["lower_price"],
                butterfly_info["atm_put_price"],
                butterfly_info["atm_call_price"],
                butterfly_info["upper_price"]
            ]
        )) or (
            strategy_type != "IRON Butterfly" and all(
                price is not None for price in [
                    butterfly_info["lower_price"],
                    butterfly_info["atm_price"],
                    butterfly_info["upper_price"]
                ]
            )
        )
    ):
        # Calculate order price with offset
        if strategy_type == "IRON Butterfly":
            net_premium = (
                butterfly_info["lower_price"]
                - butterfly_info["atm_put_price"]
                - butterfly_info["atm_call_price"]
                + butterfly_info["upper_price"]
            )
        else:
            net_premium = (
                butterfly_info["lower_price"]
                - 2 * butterfly_info["atm_price"]
                + butterfly_info["upper_price"]
            )
        order_price = net_premium + order_offset

        lower_option = strike_to_option[butterfly_info["lower_strike"]]
        atm_option = strike_to_option[butterfly_info["atm_strike"]]
        upper_option = strike_to_option[butterfly_info["upper_strike"]]

        if strategy_type == "IRON Butterfly":
            max_profit = abs(order_price) * 100
            max_loss = (leg_width - abs(order_price)) * 100
        else:
            max_profit = (leg_width - abs(order_price)) * 100
            max_loss = abs(order_price) * 100

        if strategy_type == "IRON Butterfly":
            current_options_price = (
                butterfly_info["lower_price"]
                - butterfly_info["atm_put_price"]
                - butterfly_info["atm_call_price"]
                + butterfly_info["upper_price"]
            )
        else:
            current_options_price = (
                butterfly_info["lower_price"]
                - 2 * butterfly_info["atm_price"]
                + butterfly_info["upper_price"]
            )

        if strategy_type == "IRON Butterfly":
            put_lower = next((o for o in options_chain if float(o["strike_price"]) == butterfly_info["lower_strike"] and o["type"] == "put"), None)
            put_atm = next((o for o in options_chain if float(o["strike_price"]) == butterfly_info["atm_strike"] and o["type"] == "put"), None)
            call_atm = next((o for o in options_chain if float(o["strike_price"]) == butterfly_info["atm_strike"] and o["type"] == "call"), None)
            call_upper = next((o for o in options_chain if float(o["strike_price"]) == butterfly_info["upper_strike"] and o["type"] == "call"), None)
            legs = []
            if put_lower and put_atm and call_atm and call_upper:
                symbols = [put_lower["symbol"], put_atm["symbol"], call_atm["symbol"], call_upper["symbol"]]
                if len(set(symbols)) == 4:
                    legs = [
                        {"symbol": put_lower["symbol"], "side": "buy", "ratio_qty": "1"},
                        {"symbol": put_atm["symbol"], "side": "sell", "ratio_qty": "1"},
                        {"symbol": call_atm["symbol"], "side": "sell", "ratio_qty": "1"},
                        {"symbol": call_upper["symbol"], "side": "buy", "ratio_qty": "1"}
                    ]
        elif strategy_type == "CALL Butterfly":
            legs = [
                {"symbol": lower_option["symbol"], "side": "buy", "ratio_qty": "1"},
                {"symbol": atm_option["symbol"], "side": "sell", "ratio_qty": "2"},
                {"symbol": upper_option["symbol"], "side": "buy", "ratio_qty": "1"}
            ]
        elif strategy_type == "PUT Butterfly":
            legs = [
                {"symbol": lower_option["symbol"], "side": "buy", "ratio_qty": "1"},
                {"symbol": atm_option["symbol"], "side": "sell", "ratio_qty": "2"},
                {"symbol": upper_option["symbol"], "side": "buy", "ratio_qty": "1"}
            ]

        if not st.session_state["order_confirm"]:
            confirm_order_dialog(order_price, underlying_for_butterfly, max_profit, max_loss, current_options_price, legs)
            st.stop()

        # Place order via backend API
        import json
        order_payload = {
            "symbol": symbol,
            "expiry": expiry.strftime("%Y-%m-%d"),
            "strategy_type": strategy_type,
            "legs": legs,
            "order_price": order_price,
            "order_offset": order_offset,
            "qty": 1,
            "time_in_force": "day",
            "order_type": "limit"
        }
        resp = requests.post(f"{API_BASE_URL}/place_butterfly_order", json=order_payload)
        if resp.status_code == 200:
            result = resp.json()
            if result.get("success"):
                st.session_state["order_result"] = ("success", result.get("order"))
                st.session_state["order_confirm"] = False
            else:
                st.session_state["order_result"] = ("error", result.get("error"))
                st.session_state["order_confirm"] = False
        else:
            st.session_state["order_result"] = ("error", f"Order failed: {resp.status_code} {resp.text}")
            st.session_state["order_confirm"] = False

        if st.session_state["order_result"]:
            status, result = st.session_state["order_result"]
            if status == "success":
                st.success("Order Submitted Successfully!")
                st.json(result)
            else:
                st.error(f"Order Failed: {result}")
            if st.button("Close Result", key="close_result_btn"):
                st.session_state["order_result"] = None
    else:
        st.error("Butterfly construction or pricing incomplete. Cannot place order.")

# Set up a container for live updates
live_update_container = st.empty()

# Function to update the UI
def update_ui():
    with live_update_container:
        render_live_prices_and_chart()

# Add a footer with streaming service info
st.markdown("---")
st.markdown("### Streaming Service Info")
service_status = streaming_client.check_service_status()
if service_status:
    st.json(service_status)
else:
    st.error("Streaming service is not running or not responding")

# Cleanup on app exit
def cleanup():
    streaming_client.stop_polling()

# Register cleanup handler
import atexit
atexit.register(cleanup)

# Set up periodic updates
import time

while True:
    update_ui()
    time.sleep(1)  # Update every second
