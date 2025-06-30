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

# Strategy type dropdown
strategy_type = st.selectbox(
    "Butterfly Strategy Type",
    options=["IRON Butterfly", "CALL Butterfly", "PUT Butterfly"],
    index=0
)

# Leg width input (default: 2)
leg_width = st.number_input("Butterfly Leg Width", min_value=1, max_value=100, value=2, step=1)

# Order price offset input
order_offset = st.number_input(
    "Order Price Offset",
    min_value=-10.0,
    max_value=10.0,
    value=0.05,
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
    # try:
    #     print(f"[DEBUG] Response: {resp.json()}")
    # except Exception as e:
    #     print(f"[DEBUG] Response decode error: {e}")
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
    print(f"[DEBUG] GET {url}")
    print(f"[DEBUG] Headers: {headers}")
    print(f"[DEBUG] Params: {params}")
    resp = requests.get(url, headers=headers, params=params)
    print(f"[DEBUG] Status code: {resp.status_code}")
    try:
        response_data = resp.json()
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

options_chain = get_options_chain(symbol, expiry, strategy_type)
#print(f"[DEBUG] Options chain: {options_chain}")
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
if options_chain and underlying_price:
    strikes, strike_to_option = get_strike_prices(options_chain, strategy_type)
    print(f"[DEBUG] Underlying price: {underlying_price}")
    if len(strikes) >= 3:  # Ensure we have at least 3 strikes to construct a butterfly
        atm_strike = find_closest_strike(strikes, underlying_price)
        atm_index = strikes.index(atm_strike)
        print(f"[DEBUG] ATM strike: {atm_strike}")
        
        # Find the closest available strikes for lower and upper wings using the selected leg_width
        lower_strike = None
        upper_strike = None

        # Find the closest available strike below ATM with the requested width
        for s in strikes:
            if s < atm_strike and abs(atm_strike - s - leg_width) < 1e-6:
                lower_strike = s
        # Find the closest available strike above ATM with the requested width
        for s in strikes:
            if s > atm_strike and abs(s - atm_strike - leg_width) < 1e-6:
                upper_strike = s

        # Fallback to closest available if exact width not found
        if lower_strike is None:
            lower_strike = max([s for s in strikes if s < atm_strike], default=min(strikes))
        if upper_strike is None:
            upper_strike = min([s for s in strikes if s > atm_strike], default=max(strikes))

        print(f"[DEBUG] Lower strike: {lower_strike}, Upper strike: {upper_strike}, Leg width: {leg_width}")

        if lower_strike != atm_strike and upper_strike != atm_strike:
            # For IRON Butterfly, get correct prices from puts and calls for display only
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

            st.subheader("Selected Butterfly Strikes")
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

            st.info(f"Note: The selected strikes may not be exactly {leg_width} apart due to available options.")
            
            # Add a check to ensure all strikes are different
            if len(set([lower_strike, atm_strike, upper_strike])) != 3:
                st.warning("Warning: The selected strikes are not all different. The butterfly may not be properly constructed.")
        else:
            st.warning("Could not find all required strikes for butterfly construction.")
    else:
        st.warning("No valid strikes found in options chain.")
    # Plot payoff if butterfly is valid
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
        import matplotlib.pyplot as plt

        # Calculate net premium (debit for regular butterfly, credit for iron butterfly)
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

        # Payoff calculation
        # Use integer steps for x, center at atm_strike
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
            # Iron butterfly payoff calculation
            # Long put at lower strike, short put at ATM, short call at ATM, long call at upper strike
            # This is a credit spread, so net_premium should be positive (money received)
            
            # For clarity, let's use the absolute value of the width
            width = abs(atm_strike - lower_strike)
            
            # Calculate max profit and max loss
            max_profit = abs(net_premium) * 100  # Credit received
            max_loss = (width - abs(net_premium)) * 100  # Width minus credit
            
            payoff = np.zeros_like(x)
            for i, price in enumerate(x):
                if price <= lower_strike:
                    # Below lower strike: max loss
                    payoff[i] = -max_loss
                elif price < atm_strike:
                    # Between lower and ATM: linear increase from max loss to max profit
                    ratio = (price - lower_strike) / width
                    payoff[i] = -max_loss + ratio * (max_loss + max_profit)
                elif price <= upper_strike:
                    # Between ATM and upper: linear decrease from max profit to max loss
                    ratio = (upper_strike - price) / width
                    payoff[i] = -max_loss + ratio * (max_loss + max_profit)
                else:
                    # Above upper strike: max loss
                    payoff[i] = -max_loss
        else:
            # Regular butterfly payoff
            payoff = (
                np.maximum(x - butterfly_info["lower_strike"], 0)
                - 2 * np.maximum(x - butterfly_info["atm_strike"], 0)
                + np.maximum(x - butterfly_info["upper_strike"], 0)
                - net_premium
            ) * 100  # Multiply by 100 for options contracts

        # Calculate break-even points
        lower_be = butterfly_info["lower_strike"] + net_premium
        upper_be = butterfly_info["upper_strike"] - net_premium

        fig, ax = plt.subplots()
        ax.plot(x, payoff, label="Butterfly Payoff")
        ax.axhline(0, color="gray", linestyle="--")
        ax.axvline(lower_be, color="red", linestyle=":", label="Lower Break Even")
        ax.axvline(upper_be, color="blue", linestyle=":", label="Upper Break Even")
        # Add vertical line for current price
        ax.axvline(underlying_price, color="purple", linestyle="--", label=f"Current Price: {underlying_price:.2f}")
        # Add labels for break even points
        ax.text(lower_be, ax.get_ylim()[0], f"{lower_be:.2f}", color="red", ha="center", va="bottom", fontsize=9, fontweight="bold", bbox=dict(facecolor='white', edgecolor='red', boxstyle='round,pad=0.2'))
        ax.text(upper_be, ax.get_ylim()[0], f"{upper_be:.2f}", color="blue", ha="center", va="bottom", fontsize=9, fontweight="bold", bbox=dict(facecolor='white', edgecolor='blue', boxstyle='round,pad=0.2'))
        ax.text(underlying_price, ax.get_ylim()[1], f"{underlying_price:.2f}", color="purple", ha="center", va="top", fontsize=9, fontweight="bold", bbox=dict(facecolor='white', edgecolor='purple', boxstyle='round,pad=0.2'))
        ax.set_xlabel("Underlying Price at Expiry")
        ax.set_ylabel("Profit / Loss ($)")
        ax.set_title("Butterfly Payoff Diagram")
        st.pyplot(fig)

        # Show break-even points and center in the UI
        st.write(f"**ATM Strike (Center):** {center}")
        st.write(f"**Lower Break Even:** {lower_be:.2f}")
        st.write(f"**Upper Break Even:** {upper_be:.2f}")
else:
    st.info("Butterfly construction and payoff plot will appear here.")

# Place order button
if "order_confirm" not in st.session_state:
    st.session_state["order_confirm"] = False
if "order_result" not in st.session_state:
    st.session_state["order_result"] = None

@st.dialog("Order Confirmation")
def confirm_order_dialog(order_price, underlying_price, max_profit, max_loss, current_options_price, legs):
    st.write(f"**Current Options Price (legs combined):** ${current_options_price:.2f}")
    st.write(f"**Order Price:** ${order_price:.2f}")
    st.write(f"**Underlying Price:** ${underlying_price:.2f}")
    st.write(f"**Max Profit:** ${max_profit:.2f}")
    st.write(f"**Max Loss:** ${max_loss:.2f}")
    st.write("**Legs to be Traded:**")
    for leg in legs:
        st.write(f"{leg['side'].capitalize()} {leg['ratio_qty']} of {leg['symbol']}")
        
    if st.button("Confirm Order"):
        st.session_state["order_confirm"] = True
        st.rerun()

if st.button("Place Butterfly Order") or st.session_state["order_confirm"]:
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

        # Find contract IDs for each leg
        lower_option = strike_to_option[butterfly_info["lower_strike"]]
        atm_option = strike_to_option[butterfly_info["atm_strike"]]
        upper_option = strike_to_option[butterfly_info["upper_strike"]]

        # Calculate max profit and max loss
        max_profit = (leg_width - abs(order_price)) * 100
        max_loss = abs(order_price) * 100

        # Calculate current options price for the legs combined
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

        # Prepare legs for Alpaca API based on strategy type
        if strategy_type == "IRON Butterfly":
            # Find correct put/call contracts for each leg
            put_lower = next((o for o in options_chain if float(o["strike_price"]) == butterfly_info["lower_strike"] and o["type"] == "put"), None)
            put_atm = next((o for o in options_chain if float(o["strike_price"]) == butterfly_info["atm_strike"] and o["type"] == "put"), None)
            call_atm = next((o for o in options_chain if float(o["strike_price"]) == butterfly_info["atm_strike"] and o["type"] == "call"), None)
            call_upper = next((o for o in options_chain if float(o["strike_price"]) == butterfly_info["upper_strike"] and o["type"] == "call"), None)
            legs = []
            # Only add legs if all contracts are found and all symbols are unique
            if put_lower and put_atm and call_atm and call_upper:
                symbols = [put_lower["symbol"], put_atm["symbol"], call_atm["symbol"], call_upper["symbol"]]
                if len(set(symbols)) == 4:
                    legs = [
                        {
                            "symbol": put_lower["symbol"],
                            "side": "buy",
                            "ratio_qty": "1"
                        },
                        {
                            "symbol": put_atm["symbol"],
                            "side": "sell",
                            "ratio_qty": "1"
                        },
                        {
                            "symbol": call_atm["symbol"],
                            "side": "sell",
                            "ratio_qty": "1"
                        },
                        {
                            "symbol": call_upper["symbol"],
                            "side": "buy",
                            "ratio_qty": "1"
                        }
                    ]
                else:
                    legs = []
            else:
                legs = []
        elif strategy_type == "CALL Butterfly":
            legs = [
                {
                    "symbol": lower_option["symbol"],
                    "side": "buy",
                    "ratio_qty": "1"
                },
                {
                    "symbol": atm_option["symbol"],
                    "side": "sell",
                    "ratio_qty": "2"
                },
                {
                    "symbol": upper_option["symbol"],
                    "side": "buy",
                    "ratio_qty": "1"
                }
            ]
        elif strategy_type == "PUT Butterfly":
            legs = [
                {
                    "symbol": lower_option["symbol"],
                    "side": "buy",
                    "ratio_qty": "1"
                },
                {
                    "symbol": atm_option["symbol"],
                    "side": "sell",
                    "ratio_qty": "2"
                },
                {
                    "symbol": upper_option["symbol"],
                    "side": "buy",
                    "ratio_qty": "1"
                }
            ]

        # Show confirmation dialog using st.dialog
        if not st.session_state["order_confirm"]:
            confirm_order_dialog(order_price, underlying_price, max_profit, max_loss, current_options_price, legs)
            st.stop()
        # Do NOT redefine legs here; use the legs already constructed above for the order

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

        # Show result as a modal
        if st.session_state["order_result"]:
            status, result = st.session_state["order_result"]
            if status == "success":
                st.success("Order Submitted Successfully!")
                st.json(result)
            else:
                st.error(f"Order Failed: {result}")
            if st.button("Close Result"):
                st.session_state["order_result"] = None
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
