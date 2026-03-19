package schwab

// =============================================================================
// Schwab Streaming Field Maps
//
// Schwab's WebSocket streaming service returns market data with numerical
// string keys (e.g., "1", "2", "3") instead of named fields. These maps
// translate those keys to human-readable field names.
//
// Keys are strings because Schwab streaming JSON uses string keys,
// avoiding int→string conversion during message processing.
//
// Source: Schwab API documentation — LEVELONE_EQUITIES and LEVELONE_OPTIONS
// =============================================================================

// equityFieldMap maps Schwab LEVELONE_EQUITIES streaming field indices to names.
// 52 fields (indices 0–51).
var equityFieldMap = map[string]string{
	"0":  "SYMBOL",
	"1":  "BID_PRICE",
	"2":  "ASK_PRICE",
	"3":  "LAST_PRICE",
	"4":  "BID_SIZE",
	"5":  "ASK_SIZE",
	"6":  "ASK_ID",
	"7":  "BID_ID",
	"8":  "TOTAL_VOLUME",
	"9":  "LAST_SIZE",
	"10": "HIGH_PRICE",
	"11": "LOW_PRICE",
	"12": "CLOSE_PRICE",
	"13": "EXCHANGE_ID",
	"14": "MARGINABLE",
	"15": "DESCRIPTION",
	"16": "LAST_ID",
	"17": "OPEN_PRICE",
	"18": "NET_CHANGE",
	"19": "FIFTY_TWO_WEEK_HIGH",
	"20": "FIFTY_TWO_WEEK_LOW",
	"21": "PE_RATIO",
	"22": "DIVIDEND_AMOUNT",
	"23": "DIVIDEND_YIELD",
	"24": "NAV",
	"25": "EXCHANGE_NAME",
	"26": "DIVIDEND_DATE",
	"27": "IS_REGULAR_MARKET_QUOTE",
	"28": "IS_REGULAR_MARKET_TRADE",
	"29": "REGULAR_MARKET_LAST_PRICE",
	"30": "REGULAR_MARKET_LAST_SIZE",
	"31": "REGULAR_MARKET_NET_CHANGE",
	"32": "SECURITY_STATUS",
	"33": "MARK",
	"34": "QUOTE_TIME_MILLIS",
	"35": "TRADE_TIME_MILLIS",
	"36": "REGULAR_MARKET_TRADE_TIME_MILLIS",
	"37": "BID_TIME_MILLIS",
	"38": "ASK_TIME_MILLIS",
	"39": "ASK_MIC_ID",
	"40": "BID_MIC_ID",
	"41": "LAST_MIC_ID",
	"42": "NET_CHANGE_PERCENT",
	"43": "REGULAR_MARKET_NET_CHANGE_PERCENT",
	"44": "MARK_CHANGE",
	"45": "MARK_CHANGE_PERCENT",
	"46": "HTB_QUANTITY",
	"47": "HTB_RATE",
	"48": "IS_HTB",
	"49": "IS_SHORTABLE",
	"50": "POST_MARKET_NET_CHANGE",
	"51": "POST_MARKET_NET_CHANGE_PERCENT",
}

// optionFieldMap maps Schwab LEVELONE_OPTIONS streaming field indices to names.
// 55 fields (indices 0–54). Includes Greeks at indices 28–32.
var optionFieldMap = map[string]string{
	"0":  "SYMBOL",
	"1":  "DESCRIPTION",
	"2":  "BID_PRICE",
	"3":  "ASK_PRICE",
	"4":  "LAST_PRICE",
	"5":  "HIGH_PRICE",
	"6":  "LOW_PRICE",
	"7":  "CLOSE_PRICE",
	"8":  "TOTAL_VOLUME",
	"9":  "OPEN_INTEREST",
	"10": "VOLATILITY",
	"11": "MONEY_INTRINSIC_VALUE",
	"12": "EXPIRATION_YEAR",
	"13": "MULTIPLIER",
	"14": "DIGITS",
	"15": "OPEN_PRICE",
	"16": "BID_SIZE",
	"17": "ASK_SIZE",
	"18": "LAST_SIZE",
	"19": "NET_CHANGE",
	"20": "STRIKE_TYPE",
	"21": "CONTRACT_TYPE",
	"22": "UNDERLYING",
	"23": "EXPIRATION_MONTH",
	"24": "DELIVERABLES",
	"25": "TIME_VALUE",
	"26": "EXPIRATION_DAY",
	"27": "DAYS_TO_EXPIRATION",
	"28": "DELTA",
	"29": "GAMMA",
	"30": "THETA",
	"31": "VEGA",
	"32": "RHO",
	"33": "SECURITY_STATUS",
	"34": "THEORETICAL_OPTION_VALUE",
	"35": "UNDERLYING_PRICE",
	"36": "UV_EXPIRATION_TYPE",
	"37": "MARK",
	"38": "QUOTE_TIME_MILLIS",
	"39": "TRADE_TIME_MILLIS",
	"40": "EXCHANGE_ID",
	"41": "EXCHANGE_NAME",
	"42": "LAST_TRADING_DAY",
	"43": "SETTLEMENT_TYPE",
	"44": "NET_CHANGE_PERCENT",
	"45": "MARK_CHANGE",
	"46": "MARK_CHANGE_PERCENT",
	"47": "IMPLIED_YIELD",
	"48": "IS_PENNY",
	"49": "OPTION_ROOT",
	"50": "FIFTY_TWO_WEEK_HIGH",
	"51": "FIFTY_TWO_WEEK_LOW",
	"52": "INDICATIVE_ASK_PRICE",
	"53": "INDICATIVE_BID_PRICE",
	"54": "INDICATIVE_QUOTE_TIME",
}

// equitySubscriptionFields is the comma-separated list of field indices to
// request when subscribing to LEVELONE_EQUITIES streaming data.
//
// Includes: SYMBOL, BID_PRICE, ASK_PRICE, LAST_PRICE, BID_SIZE, ASK_SIZE,
// TOTAL_VOLUME, HIGH_PRICE, LOW_PRICE, CLOSE_PRICE, OPEN_PRICE, NET_CHANGE,
// MARK, QUOTE_TIME_MILLIS, TRADE_TIME_MILLIS
const equitySubscriptionFields = "0,1,2,3,4,5,8,10,11,12,17,18,33,34,35"

// optionSubscriptionFields is the comma-separated list of field indices to
// request when subscribing to LEVELONE_OPTIONS streaming data.
//
// Includes: SYMBOL, BID_PRICE, ASK_PRICE, LAST_PRICE, HIGH_PRICE, LOW_PRICE,
// CLOSE_PRICE, TOTAL_VOLUME, OPEN_INTEREST, VOLATILITY, EXPIRATION_YEAR,
// OPEN_PRICE, NET_CHANGE, CONTRACT_TYPE, UNDERLYING, EXPIRATION_MONTH,
// EXPIRATION_DAY, DELTA, GAMMA, THETA, VEGA, RHO, UNDERLYING_PRICE, MARK
const optionSubscriptionFields = "0,2,3,4,5,6,7,8,9,10,12,15,19,21,22,23,26,28,29,30,31,32,35,37"
