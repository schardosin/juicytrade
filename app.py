import streamlit as st
import os
from dotenv import load_dotenv
from datetime import date
import requests
import time

# Load API keys and URLs from .env
load_dotenv()
ALPACA_API_KEY_LIVE = os.getenv("APCA_API_KEY_ID_LIVE")
ALPACA_API_SECRET_LIVE = os.getenv("APCA_API_SECRET_KEY_LIVE")
ALPACA_API_KEY_PAPER = os.getenv("APCA_API_KEY_ID_PAPER")
ALPACA_API_SECRET_PAPER = os.getenv("APCA_API_SECRET_KEY_PAPER")
ALPACA_BASE_URL_LIVE = os.getenv("ALPACA_BASE_URL_LIVE")
ALPACA_BASE_URL_PAPER = os.getenv("ALPACA_BASE_URL_PAPER")
ALPACA_DATA_URL = os.getenv("ALPACA_DATA_URL")

def get_api_credentials(is_paper_trade=True):
    if is_paper_trade:
        return ALPACA_API_KEY_PAPER, ALPACA_API_SECRET_PAPER
    else:
        return ALPACA_API_KEY_LIVE, ALPACA_API_SECRET_LIVE

def get_base_url(is_paper_trade=True):
    if is_paper_trade:
        return ALPACA_BASE_URL_PAPER
    else:
        return ALPACA_BASE_URL_LIVE

st.set_page_config(page_title="Iron Butterfly Trade", layout="centered")
st.title("Robust Iron Butterfly Trade")
st.header("Step 1: Configure Butterfly Trade")

def get_next_market_date():
    url = f"{get_base_url(is_paper_trade=False)}/v2/calendar"
    params = {
        "start": date.today().strftime("%Y-%m-%d"),
        "end": (date.today().replace(year=date.today().year + 1)).strftime("%Y-%m-%d")
    }
    api_key, api_secret = get_api_credentials(is_paper_trade=False)
    headers = {
        "APCA-API-KEY-ID": api_key,
        "APCA-API-SECRET-KEY": api_secret,
    }
    resp = requests.get(url, headers=headers, params=params)
    if resp.status_code == 200:
        data = resp.json()
        for day in data:
            if day.get("date"):
                d = day["date"]
                if d >= date.today().strftime("%Y-%m-%d"):
                    return d
    return date.today().strftime("%Y-%m-%d")

import datetime
next_market_date_str = get_next_market_date()
next_market_date = datetime.datetime.strptime(next_market_date_str, "%Y-%m-%d").date()
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
    url = f"{ALPACA_DATA_URL}/v2/stocks/{symbol}/quotes/latest"
    api_key, api_secret = get_api_credentials(is_paper_trade=False)
    headers = {
        "APCA-API-KEY-ID": api_key,
        "APCA-API-SECRET-KEY": api_secret,
    }
    resp = requests.get(url, headers=headers)
    if resp.status_code == 200:
        data = resp.json()
        quote = data.get("quote", {})
        ap = quote.get("ap")
        bp = quote.get("bp")
        return ap if ap != 0 else bp
    return None

def get_options_chain(symbol, expiry, strategy_type):
    url = f"{get_base_url(is_paper_trade=True)}/v2/options/contracts"
    api_key, api_secret = get_api_credentials(is_paper_trade=True)
    option_type = "call" if strategy_type == "CALL Butterfly" else "put" if strategy_type == "PUT Butterfly" else None
    params = {
        "underlying_symbols": symbol,
        "expiration_date": expiry.strftime("%Y-%m-%d"),
        "root_symbol": symbol,
        "type": option_type,
        "limit": 1000
    }
    headers = {
        "APCA-API-KEY-ID": api_key,
        "APCA-API-SECRET-KEY": api_secret,
        "accept": "application/json"
    }
    resp = requests.get(url, headers=headers, params=params)
    try:
        response_data = resp.json()
        if resp.status_code == 200:
            contracts = response_data.get("option_contracts", [])
            return contracts
    except Exception:
        pass
    return []

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

import numpy as np
from stream_option_data import OptionStreamClientAlpacaSDK
import threading
import queue

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
                return (bid + ask) / 2
            elif ask is not None:
                return ask
            elif bid is not None:
                return bid
    price = float(opt["close_price"] or 0)
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

if "price_queue" not in st.session_state:
    st.session_state["price_queue"] = queue.Queue(maxsize=1000)
if "option_leg_prices" not in st.session_state:
    st.session_state["option_leg_prices"] = {}
if "stock_price_queue" not in st.session_state:
    st.session_state["stock_price_queue"] = queue.Queue(maxsize=100)
if "underlying_stream_price" not in st.session_state:
    st.session_state["underlying_stream_price"] = None

def get_option_symbols_for_legs():
    print(f"[DEBUG] get_option_symbols_for_legs: options_chain is {'set' if options_chain else 'empty'}, butterfly_info is {'set' if butterfly_info else 'empty'}")
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
    return list(set(symbols))

def process_price_queue():
    q = st.session_state["price_queue"]
    updated = False
    while not q.empty():
        msg = q.get()
        symbol = msg.get("S")
        ap = msg.get("ap")
        bp = msg.get("bp")
        st.session_state["option_leg_prices"][symbol] = {"ask": ap, "bid": bp}
        updated = True
    # Process underlying price queue
    sq = st.session_state["stock_price_queue"]
    while not sq.empty():
        msg = sq.get()
        ap = msg.get("ap")
        bp = msg.get("bp")
        # Use mid price if both available, else fallback
        if ap is not None and bp is not None:
            price = (ap + bp) / 2
        elif ap is not None:
            price = ap
        elif bp is not None:
            price = bp
        else:
            price = None
        st.session_state["underlying_stream_price"] = price
        updated = True
    return updated

def start_streaming_if_needed():
    print("[DEBUG] start_streaming_if_needed called")
    symbols = get_option_symbols_for_legs()
    print(f"[DEBUG] Option symbols for legs: {symbols}")

    # Track the last set of symbols in session state
    last_symbols = st.session_state.get("last_stream_symbols", None)
    client = st.session_state.get("option_stream_client", None)
    stock_symbol = symbol
    # If the symbols have changed, no client exists, restart the stream
    if symbols and (client is None or last_symbols != symbols):
        # Attempt to stop the old client if it exists
        if client is not None and hasattr(client, "stop"):
            print("[DEBUG] Stopping previous OptionStreamClientAlpacaSDK...")
            try:
                client.stop()
            except Exception as e:
                print(f"[DEBUG] Error stopping previous client: {e}")
        osc = OptionStreamClientAlpacaSDK(
            symbols,
            st.session_state["price_queue"],
            stock_symbol=stock_symbol,
            stock_price_queue=st.session_state["stock_price_queue"]
        )
        print("[DEBUG] OptionStreamClientAlpacaSDK instantiated, starting thread...")
        osc.start()
        st.session_state["option_stream_client"] = osc
        st.session_state["last_stream_symbols"] = symbols
    elif not symbols:
        print("[DEBUG] No symbols found for streaming, not starting OptionStreamClient.")

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
            if strategy_type == "IRON Butterfly":
                put_lower = next((o for o in options_chain if float(o["strike_price"]) == lower_strike and o["type"] == "put"), None)
                put_atm = next((o for o in options_chain if float(o["strike_price"]) == atm_strike and o["type"] == "put"), None)
                call_atm = next((o for o in options_chain if float(o["strike_price"]) == atm_strike and o["type"] == "call"), None)
                call_upper = next((o for o in options_chain if float(o["strike_price"]) == upper_strike and o["type"] == "call"), None)
                butterfly_info = {
                    "lower_strike": lower_strike,
                    "atm_strike": atm_strike,
                    "upper_strike": upper_strike,
                    "lower_price": option_price(put_lower) if put_lower else 0,
                    "atm_put_price": option_price(put_atm) if put_atm else 0,
                    "atm_call_price": option_price(call_atm) if call_atm else 0,
                    "upper_price": option_price(call_upper) if call_upper else 0,
                }
            else:
                atm_option = strike_to_option[atm_strike]
                lower_option = strike_to_option[lower_strike]
                upper_option = strike_to_option[upper_strike]
                butterfly_info = {
                    "lower_strike": lower_strike,
                    "atm_strike": atm_strike,
                    "upper_strike": upper_strike,
                    "lower_price": option_price(lower_option),
                    "atm_price": option_price(atm_option),
                    "upper_price": option_price(upper_option),
                }
    return butterfly_info

# Ensure butterfly_info is initialized before streaming
butterfly_info = ensure_butterfly_info_initialized()
start_streaming_if_needed()

def render_live_prices_and_chart():
    process_price_queue()
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
                    "lower_price": option_price(put_lower) if put_lower else 0,
                    "atm_put_price": option_price(put_atm) if put_atm else 0,
                    "atm_call_price": option_price(call_atm) if call_atm else 0,
                    "upper_price": option_price(call_upper) if call_upper else 0,
                }
            else:
                atm_option = strike_to_option[atm_strike]
                lower_option = strike_to_option[lower_strike]
                upper_option = strike_to_option[upper_strike]
                butterfly_info = {
                    "lower_strike": lower_strike,
                    "atm_strike": atm_strike,
                    "upper_strike": upper_strike,
                    "lower_price": option_price(lower_option),
                    "atm_price": option_price(atm_option),
                    "upper_price": option_price(upper_option),
                }
            with price_placeholder.container():
                st.subheader("Selected Butterfly Strikes (Live)")
                st.markdown(f"""
                    <div style="font-size: 18px;">
                        <b>Buy 1 Put at {lower_strike}:</b> <span style="color: #0074D9;">${butterfly_info['lower_price']:.2f}</span><br>
                        <b>Sell 1 Put at {atm_strike}:</b> <span style="color: #FF4136;">${butterfly_info.get('atm_put_price', butterfly_info.get('atm_price', 0)):.2f}</span><br>
                        <b>Sell 1 Call at {atm_strike}:</b> <span style="color: #FF851B;">${butterfly_info.get('atm_call_price', butterfly_info.get('atm_price', 0)):.2f}</span><br>
                        <b>Buy 1 Call at {upper_strike}:</b> <span style="color: #2ECC40;">${butterfly_info['upper_price']:.2f}</span>
                    </div>
                """, unsafe_allow_html=True)
                if 'center' in locals():
                    st.markdown(f"""
                        <div style="font-size: 16px; margin-top: 10px;">
                            <b>ATM Strike (Center):</b> {center}<br>
                            <b>Lower Break Even:</b> <span style="color: #FF4136;">{lower_be:.2f}</span><br>
                            <b>Upper Break Even:</b> <span style="color: #0074D9;">{upper_be:.2f}</span>
                        </div>
                    """, unsafe_allow_html=True)
            with chart_placeholder.container():
                import matplotlib.pyplot as plt
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
                # Use a unique key for the button based on the current ATM strike to avoid duplicate key errors on rerun
                btn_key = f"place_butterfly_order_btn_{butterfly_info['atm_strike']}" if butterfly_info else "place_butterfly_order_btn"
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

                        order_payload = {
                            "order_class": "mleg",
                            "time_in_force": "day",
                            "qty": "1",
                            "type": "limit",
                            "limit_price": f"{order_price:.2f}",
                            "legs": legs
                        }
                        order_url = f"{get_base_url(is_paper_trade=True)}/v2/orders"
                        api_key, api_secret = get_api_credentials(is_paper_trade=True)
                        headers = {
                            "APCA-API-KEY-ID": api_key,
                            "APCA-API-SECRET-KEY": api_secret,
                            "Content-Type": "application/json"
                        }
                        import json
                        resp = requests.post(order_url, headers=headers, data=json.dumps(order_payload))
                        if resp.status_code in (200, 201):
                            st.session_state["order_result"] = ("success", resp.json())
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

# Main live update call for price/chart area only
import time

# Use a polling loop for just the live section (prices/chart), so the rest of the UI interactive
live_placeholder = st.empty()
# Only run the polling loop if not already in a rerun (to avoid duplicate button keys)
if "live_polling_active" not in st.session_state:
    st.session_state["live_polling_active"] = True
    for _ in range(1000):
        with live_placeholder:
            # Only show the button in the first iteration of the polling loop to avoid duplicate keys
            if _ == 0:
                render_live_prices_and_chart()
            else:
                # Render the live section but skip the order button to avoid duplicate keys
                def render_live_prices_and_chart_no_button():
                    # Copy of render_live_prices_and_chart but skip the button section
                    process_price_queue()
                    strikes, strike_to_option = get_strike_prices(options_chain, strategy_type)
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
                                    "lower_price": option_price(put_lower) if put_lower else 0,
                                    "atm_put_price": option_price(put_atm) if put_atm else 0,
                                    "atm_call_price": option_price(call_atm) if call_atm else 0,
                                    "upper_price": option_price(call_upper) if call_upper else 0,
                                }
                            else:
                                atm_option = strike_to_option[atm_strike]
                                lower_option = strike_to_option[lower_strike]
                                upper_option = strike_to_option[upper_strike]
                                butterfly_info = {
                                    "lower_strike": lower_strike,
                                    "atm_strike": atm_strike,
                                    "upper_strike": upper_strike,
                                    "lower_price": option_price(lower_option),
                                    "atm_price": option_price(atm_option),
                                    "upper_price": option_price(upper_option),
                                }
                            with price_placeholder.container():
                                st.subheader("Selected Butterfly Strikes (Live)")
                                if strategy_type == "IRON Butterfly":
                                    st.write(f"Buy 1 Put at {lower_strike} (${butterfly_info['lower_price']:.2f})")
                                    st.write(f"Sell 1 Put at {atm_strike} (${butterfly_info['atm_put_price']:.2f})")
                                    st.write(f"Sell 1 Call at {atm_strike} (${butterfly_info['atm_call_price']:.2f})")
                                    st.write(f"Buy 1 Call at {upper_strike} (${butterfly_info['upper_price']:.2f})")
                                elif strategy_type == "CALL Butterfly":
                                    st.write(f"Buy 1 Call at {lower_strike} (${butterfly_info['lower_price']:.2f})")
                                    st.write(f"Sell 2 Calls at {atm_strike} (${butterfly_info['atm_price']:.2f})")
                                    st.write(f"Buy 1 Call at {upper_strike} (${butterfly_info['upper_price']:.2f})")
                                elif strategy_type == "PUT Butterfly":
                                    st.write(f"Buy  Put at {lower_strike} (${butterfly_info['lower_price']:.2f})")
                                    st.write(f"Sell 2 Puts at {atm_strike} (${butterfly_info['atm_price']:.2f})")
                                    st.write(f"Buy 1 Put at {upper_strike} (${butterfly_info['upper_price']:.2f})")
                            with chart_placeholder.container():
                                import matplotlib.pyplot as plt
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
                render_live_prices_and_chart_no_button()
        time.sleep(1)
    st.session_state["live_polling_active"] = False
else:
    # If already in a rerun, just render once
    render_live_prices_and_chart()
