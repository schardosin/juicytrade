import streamlit as st
import os
from dotenv import load_dotenv
from datetime import date
import requests

# Load API keys and URLs from .env
load_dotenv()
ALPACA_API_KEY_LIVE = os.getenv("APCA_API_KEY_ID_LIVE")
ALPACA_API_SECRET_LIVE = os.getenv("APCA_API_SECRET_KEY_LIVE")
ALPACA_API_KEY_PAPER = os.getenv("APCA_API_KEY_ID_PAPER")
ALPACA_API_SECRET_PAPER = os.getenv("APCA_API_SECRET_KEY_PAPER")
ALPACA_BASE_URL_LIVE = os.getenv("ALPACA_BASE_URL_LIVE")
ALPACA_BASE_URL_PAPER = os.getenv("ALPACA_BASE_URL_PAPER")
ALPACA_DATA_URL = os.getenv("ALPACA_DATA_URL")

# Function to get appropriate API key and secret based on the operation
def get_api_credentials(is_paper_trade=True):
    if is_paper_trade:
        return ALPACA_API_KEY_PAPER, ALPACA_API_SECRET_PAPER
    else:
        return ALPACA_API_KEY_LIVE, ALPACA_API_SECRET_LIVE

# Function to get appropriate base URL based on the operation
def get_base_url(is_paper_trade=True):
    if is_paper_trade:
        return ALPACA_BASE_URL_PAPER
    else:
        return ALPACA_BASE_URL_LIVE

st.set_page_config(page_title="Iron Butterfly Trade", layout="centered")

# Verbose logging option
verbose = st.sidebar.checkbox("Verbose Logging (-v)", value=False)

st.title("Robust Iron Butterfly Trade")

# Step 1: Trade setup
st.header("Step 1: Configure Butterfly Trade")

# Get next valid market date from Alpaca calendar API
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

# Expiry date picker (default: next valid market date)
expiry = st.date_input("Option Expiry Date", value=next_market_date)

# Leg width input (default: 2)
leg_width = st.number_input("Butterfly Leg Width", min_value=1, max_value=100, value=2, step=1)

# Order price offset input
order_offset = st.number_input(
    "Order Price Offset",
    min_value=-10.0,
    max_value=10.0,
    value=0.0,
    step=0.01,
    help="Amount to add/subtract from the calculated butterfly price to set your order price."
)

# Symbol input (default: SPY)
symbol = st.text_input("Underlying Symbol", value="SPY", max_chars=10)

# Fetch underlying price and options chain from Alpaca API

def get_underlying_price(symbol):
    url = f"{ALPACA_DATA_URL}/v2/stocks/{symbol}/quotes/latest"
    api_key, api_secret = get_api_credentials(is_paper_trade=False)
    headers = {
        "APCA-API-KEY-ID": api_key,
        "APCA-API-SECRET-KEY": api_secret,
    }
    print(f"[DEBUG] GET {url}")
    print(f"[DEBUG] Headers: {headers}")
    resp = requests.get(url, headers=headers)
    print(f"[DEBUG] Status code: {resp.status_code}")
    try:
        print(f"[DEBUG] Response: {resp.json()}")
    except Exception as e:
        print(f"[DEBUG] Response decode error: {e}")
    if verbose:
        st.sidebar.write(f"GET {url}")
        st.sidebar.write(f"Headers: {headers}")
        st.sidebar.write(f"Status code: {resp.status_code}")
        try:
            st.sidebar.write(f"Response: {resp.json()}")
        except Exception as e:
            st.sidebar.write(f"Response decode error: {e}")
    if resp.status_code == 200:
        data = resp.json()
        quote = data.get("quote", {})
        ap = quote.get("ap")
        bp = quote.get("bp")
        return ap if ap != 0 else bp  # Use bid price if ask price is 0
    return None

def get_options_chain(symbol, expiry):
    url = f"{get_base_url(is_paper_trade=True)}/v2/options/contracts"
    api_key, api_secret = get_api_credentials(is_paper_trade=True)
    params = {
        "underlying_symbols": symbol,
        "expiration_date": expiry.strftime("%Y-%m-%d"),
        "root_symbol": symbol,
        "type": "call",
        "limit": 1000
    }
    headers = {
        "APCA-API-KEY-ID": api_key,
        "APCA-API-SECRET-KEY": api_secret,
        "accept": "application/json"
    }
    print(f"[DEBUG] GET {url}")
    print(f"[DEBUG] Headers: {headers}")
    print(f"[DEBUG] Params: {params}")
    resp = requests.get(url, headers=headers, params=params)
    print(f"[DEBUG] Status code: {resp.status_code}")
    print(f"[DEBUG] Full response: {resp.text}")
    try:
        response_data = resp.json()
        print(f"[DEBUG] JSON response: {response_data}")
        if resp.status_code == 200:
            contracts = response_data.get("option_contracts", [])
            print(f"[DEBUG] Number of contracts: {len(contracts)}")
            return contracts
        else:
            print(f"[DEBUG] Error response: {response_data}")
    except Exception as e:
        print(f"[DEBUG] Response decode error: {e}")
    return []

underlying_price = get_underlying_price(symbol)
if underlying_price:
    st.success(f"Underlying ({symbol}) price: ${underlying_price:.2f}")
else:
    st.error(f"Could not fetch price for {symbol}.")

options_chain = get_options_chain(symbol, expiry)
print(f"[DEBUG] Options chain: {options_chain}")
if options_chain:
    st.info(f"Fetched {len(options_chain)} option contracts for {symbol} expiring {expiry}.")
else:
    st.error("Could not fetch options chain or no contracts available.")

# Butterfly construction and plot

import numpy as np

def find_closest_strike(strikes, target):
    return min(strikes, key=lambda x: abs(x - target))

def option_price(opt):
    price = float(opt["close_price"] or 0)
    print(f"[DEBUG] Option price for {opt['strike_price']}: {price}")
    return price

def get_strike_prices(options_chain):
    # Only use call options for iron butterfly (could be extended for puts)
    strikes = []
    strike_to_option = {}
    for opt in options_chain:
        if opt["type"] == "call":
            strike = float(opt["strike_price"])
            strikes.append(strike)
            strike_to_option[strike] = opt
    print(f"[DEBUG] Available strikes: {strikes}")
    return sorted(strikes), strike_to_option

butterfly_info = None
if options_chain and underlying_price:
    strikes, strike_to_option = get_strike_prices(options_chain)
    print(f"[DEBUG] Underlying price: {underlying_price}")
    if len(strikes) >= 3:  # Ensure we have at least 3 strikes to construct a butterfly
        atm_strike = find_closest_strike(strikes, underlying_price)
        atm_index = strikes.index(atm_strike)
        print(f"[DEBUG] ATM strike: {atm_strike}")
        
        # Find the closest available strikes for lower and upper wings
        lower_index = max(0, atm_index - 1)
        upper_index = min(len(strikes) - 1, atm_index + 1)
        
        lower_strike = strikes[lower_index]
        upper_strike = strikes[upper_index]
        print(f"[DEBUG] Lower strike: {lower_strike}, Upper strike: {upper_strike}")

        if lower_strike != atm_strike and upper_strike != atm_strike:
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

            st.subheader("Selected Butterfly Strikes")
            st.write(f"Buy 1 Call at {lower_strike} (${butterfly_info['lower_price']:.2f})")
            st.write(f"Sell 2 Calls at {atm_strike} (${butterfly_info['atm_price']:.2f})")
            st.write(f"Buy 1 Call at {upper_strike} (${butterfly_info['upper_price']:.2f})")

            st.info(f"Note: The selected strikes may not be exactly {leg_width} apart due to available options.")
            
            # Add a check to ensure all strikes are different
            if len(set([lower_strike, atm_strike, upper_strike])) != 3:
                st.warning("Warning: The selected strikes are not all different. The butterfly may not be properly constructed.")
        else:
            st.warning("Could not find all required strikes for butterfly construction.")
    else:
        st.warning("No valid strikes found in options chain.")
    # Plot payoff if butterfly is valid
    if butterfly_info and all(
        price is not None for price in [
            butterfly_info["lower_price"],
            butterfly_info["atm_price"],
            butterfly_info["upper_price"]
        ]
    ):
        import matplotlib.pyplot as plt

        # Calculate net premium (debit)
        net_premium = (
            butterfly_info["lower_price"]
            - 2 * butterfly_info["atm_price"]
            + butterfly_info["upper_price"]
        )

        # Payoff calculation
        x = np.linspace(
            butterfly_info["lower_strike"] - leg_width,
            butterfly_info["upper_strike"] + leg_width,
            200
        )
        payoff = (
            np.maximum(x - butterfly_info["lower_strike"], 0)
            - 2 * np.maximum(x - butterfly_info["atm_strike"], 0)
            + np.maximum(x - butterfly_info["upper_strike"], 0)
            - net_premium
        )

        fig, ax = plt.subplots()
        ax.plot(x, payoff, label="Butterfly Payoff")
        ax.axhline(0, color="gray", linestyle="--")
        ax.set_xlabel("Underlying Price at Expiry")
        ax.set_ylabel("Profit / Loss")
        ax.set_title("Butterfly Payoff Diagram")
        ax.legend()
        st.pyplot(fig)
else:
    st.info("Butterfly construction and payoff plot will appear here.")

# Place order button
if st.button("Place Butterfly Order"):
    if butterfly_info and all(
        price is not None for price in [
            butterfly_info["lower_price"],
            butterfly_info["atm_price"],
            butterfly_info["upper_price"]
        ]
    ):
        # Calculate order price with offset
        net_premium = (
            butterfly_info["lower_price"]
            - 2 * butterfly_info["atm_price"]
            + butterfly_info["upper_price"]
        )
        order_price = net_premium + order_offset

        # Find contract IDs for each leg
        lower_option = strike_to_option[butterfly_info["lower_strike"]]
        atm_option = strike_to_option[butterfly_info["atm_strike"]]
        upper_option = strike_to_option[butterfly_info["upper_strike"]]

        legs = [
            {
                "symbol": lower_option["symbol"],
                "qty": 1,
                "side": "buy_to_open",
                "type": "option",
                "option_type": "call",
                "strike_price": lower_option["strike_price"],
                "expiration_date": lower_option["expiration_date"],
                "option_symbol": lower_option["id"]
            },
            {
                "symbol": atm_option["symbol"],
                "qty": 2,
                "side": "sell_to_open",
                "type": "option",
                "option_type": "call",
                "strike_price": atm_option["strike_price"],
                "expiration_date": atm_option["expiration_date"],
                "option_symbol": atm_option["id"]
            },
            {
                "symbol": upper_option["symbol"],
                "qty": 1,
                "side": "buy_to_open",
                "type": "option",
                "option_type": "call",
                "strike_price": upper_option["strike_price"],
                "expiration_date": upper_option["expiration_date"],
                "option_symbol": upper_option["id"]
            }
        ]

        order_payload = {
            "type": "limit",
            "price": round(order_price, 2),
            "time_in_force": "day",
            "legs": legs
        }

        order_url = f"{get_base_url(is_paper_trade=True)}/v2/options/orders"
        api_key, api_secret = get_api_credentials(is_paper_trade=True)
        headers = {
            "APCA-API-KEY-ID": api_key,
            "APCA-API-SECRET-KEY": api_secret,
            "Content-Type": "application/json"
        }

        import json
        resp = requests.post(order_url, headers=headers, data=json.dumps(order_payload))
        if resp.status_code in (200, 201):
            st.success("Butterfly order submitted successfully!")
            st.json(resp.json())
        else:
            st.error(f"Order failed: {resp.status_code} {resp.text}")
    else:
        st.error("Butterfly construction or pricing incomplete. Cannot place order.")

# Show API key status for debugging (remove in production)
if not ALPACA_API_KEY_LIVE or not ALPACA_API_SECRET_LIVE:
    st.error("Alpaca live API key or secret not found in .env file.")
elif not ALPACA_API_KEY_PAPER or not ALPACA_API_SECRET_PAPER:
    st.error("Alpaca paper API key or secret not found in .env file.")
else:
    st.success("Alpaca API keys loaded successfully.")

# Testing Plan (Step 1)
st.sidebar.header("Testing Plan")
st.sidebar.markdown("""
- API key loads from .env and is detected.
- UI fields (expiry, width, offset) render and update.
- Button is present and triggers placeholder logic.
- Next: Integrate Alpaca API for price and options chain.
""")
