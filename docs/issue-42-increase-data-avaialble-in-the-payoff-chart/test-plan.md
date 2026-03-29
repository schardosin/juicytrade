# QA Test Plan: Issue #42 — Increase Data Available in the Payoff Chart

**Issue:** [#42](https://github.com/schardosin/juicytrade/issues/42)
**Requirements:** [requirements.md](./requirements.md)
**Implementation:** [implementation-plan.md](./implementation-plan.md)
**Source under test:** `trade-app/src/utils/chartUtils.js`
**Existing dev tests:** `trade-app/tests/chartUtils-range.test.js`

---

## 1. Multi-Leg Range Formula Verification

Verify `extension = Math.max(underlyingPrice * 0.10, strikeRange * 3.0, 50)` for the multi-leg path in `generateMultiLegPayoff()`.

| # | Test Case | Input | Expected |
|---|-----------|-------|----------|
| 1.1 | **NDX — percentage dominates** | strikes 23100/23150, underlying=23132 | extension = max(2313.2, 150, 50) = 2313.2; lowerBound <= 23100-2313; upperBound >= 23150+2313 |
| 1.2 | **Cheap stock — strikeRange dominates** | strikes 48/53, underlying=50 | extension = max(5, 15, 50) = 50; lowerBound <= -2; upperBound >= 103 |
| 1.3 | **$50 floor dominates** | strikes 8/10, underlying=9 | extension = max(0.9, 6, 50) = 50; range is at least +/-50 from strikes |
| 1.4 | **Mid-price stock** | strikes 195/205, underlying=200 | extension = max(20, 30, 50) = 50; verify floor kicks in |
| 1.5 | **SPX-level** | strikes 5900/5950, underlying=5920 | extension = max(592, 150, 50) = 592; verify >=+/-500 from strikes |
| 1.6 | **Mega-cap index ~$50,000** | strikes 49900/50000, underlying=50000 | extension = max(5000, 300, 50) = 5000; range extends +/-5000 |
| 1.7 | **Penny stock ~$1** | strikes 0.90/1.10, underlying=1.0 | extension = max(0.1, 0.6, 50) = 50; $50 floor prevents absurdly narrow range |

---

## 2. Single-Leg Range Formula Verification

Verify the single-leg path: tiered multiplier with `Math.max(tierRange, underlyingPrice * 0.10, 50)`.

| # | Test Case | Input | Expected |
|---|-----------|-------|----------|
| 2.1 | **NDX single call** | strike=23100, underlying=23132 | tierRange = max(23100*3, 1000) = 69300; range = max(69300, 2313.2, 50) = 69300; <=600 pts |
| 2.2 | **Cheap stock single call (strike < 50)** | strike=25, underlying=25 | tierRange = max(25*8, 250) = 250; range = max(250, 2.5, 50) = 250 |
| 2.3 | **Mid-price single call (strike < 200)** | strike=150, underlying=155 | tierRange = max(150*5, 500) = 750; range = max(750, 15.5, 50) = 750 |
| 2.4 | **Expensive single call (strike >= 200)** | strike=5900, underlying=5920 | tierRange = max(5900*3, 1000) = 17700; range = max(17700, 592, 50) = 17700 |
| 2.5 | **Penny stock single put** | strike=1.0, underlying=1.05 | tierRange = max(1*8, 250) = 250; range = max(250, 0.105, 50) = 250 |
| 2.6 | **$50,000 single leg** | strike=50000, underlying=50000 | tierRange = max(50000*3, 1000) = 150000; range = max(150000, 5000, 50) = 150000; verify <=600 pts via adaptive step |

---

## 3. Adaptive Step Size Verification

Verify `step = Math.max(Math.ceil(totalRange / 400), 1)` across all price tiers.

| # | Test Case | Input (totalRange) | Expected Step | Expected Points |
|---|-----------|---------------------|---------------|-----------------|
| 3.1 | **NDX multi-leg** | ~4626 (2*2313) | ceil(4626/400) = 12 | ~386 + strikes |
| 3.2 | **Cheap stock multi-leg** | ~105 (2*50 + 5) | ceil(105/400) = 1 | ~105 + strikes |
| 3.3 | **$50,000 underlying multi-leg** | ~10100 (2*5000 + 100) | ceil(10100/400) = 26 | ~389 + strikes |
| 3.4 | **Penny stock multi-leg** | ~100.2 (2*50 + 0.2) | ceil(100/400) = 1 | ~100 + strikes |
| 3.5 | **Boundary: totalRange exactly 400** | engineered: strikes and underlying so totalRange=400 | step = 1 | 400 + strikes |
| 3.6 | **Boundary: totalRange = 401** | engineered | step = ceil(401/400) = 2 | ~201 + strikes |
| 3.7 | **Very small range (<50 not possible due to floor)** | Cheap stock with floor = 50, totalRange = 100 | step = 1 | ~100 + strikes |

---

## 4. Data Point Count Cap (Max 600)

No scenario should produce more than 600 data points.

| # | Test Case | Input | Expected |
|---|-----------|-------|----------|
| 4.1 | **NDX multi-leg** | strikes 23100/23150, underlying=23132 | prices.length <= 600 |
| 4.2 | **$50,000 multi-leg** | strikes 49900/50000, underlying=50000 | prices.length <= 600 |
| 4.3 | **Penny stock multi-leg** | strikes 0.90/1.10, underlying=1.0 | prices.length <= 600 |
| 4.4 | **NDX single-leg** | strike=23100, underlying=23132 | prices.length <= 600 |
| 4.5 | **$50,000 single-leg** | strike=50000, underlying=50000 | prices.length <= 600 |
| 4.6 | **Cheap stock single-leg** | strike=25, underlying=25 | prices.length <= 600 |
| 4.7 | **Iron Condor high-value** | NDX-level IC, legWidth=50 | prices.length <= 600 |
| 4.8 | **Standard butterfly high-value** | NDX-level butterfly, legWidth=50 | prices.length <= 600 |

---

## 5. Strike Inclusion in Generated Data Points

All strike prices must appear in the `prices` array regardless of step size.

| # | Test Case | Input | Expected |
|---|-----------|-------|----------|
| 5.1 | **Multi-leg, step > 1** | NDX spread (23100/23150, step~12) | prices includes both 23100 and 23150 |
| 5.2 | **Multi-leg, step = 1** | Cheap stock spread (48/53, step=1) | prices includes both 48 and 53 |
| 5.3 | **Single-leg, step > 1** | NDX single (strike=23100, step > 1) | prices includes 23100 |
| 5.4 | **Single-leg, step = 1** | Cheap stock single (strike=25, step=1) | prices includes 25 |
| 5.5 | **Strikes not on step boundary** | Multi-leg with strikes 23103/23147 (unlikely to land on step grid) | Both 23103 and 23147 present in prices |
| 5.6 | **Prices array is sorted** | Any scenario with step > 1 and injected strikes | prices is strictly monotonically non-decreasing |

---

## 6. Extreme Price Levels

### 6a. Penny Stocks (~$1)

| # | Test Case | Input | Expected |
|---|-----------|-------|----------|
| 6a.1 | **Multi-leg put spread** | strikes 0.90/1.10, underlying=1.0 | $50 floor: lowerBound <= -49, upperBound >= 51; step=1; <=600 pts |
| 6a.2 | **Single-leg call** | strike=1.0, underlying=1.05 | tierRange=max(8, 250)=250; range=max(250, 0.105, 50)=250; step=1 |
| 6a.3 | **Negative lowerBound allowed** | strike=0.50, underlying=0.50 | lowerBound = floor(0.50 - 250) = -250; verify no crash, prices include negatives |

### 6b. Mega-Cap Indices (~$50,000)

| # | Test Case | Input | Expected |
|---|-----------|-------|----------|
| 6b.1 | **$10 spread on $50,000 underlying** | strikes 49995/50005, underlying=50000 | extension = max(5000, 30, 50) = 5000; range ~10000; step=25; ~400 pts |
| 6b.2 | **$10 spread on $20,000 underlying** | strikes 19995/20005, underlying=20000 | extension = max(2000, 30, 50) = 2000; range ~4010; step=ceil(4010/400)=11 |
| 6b.3 | **$500 spread on $50,000 underlying** | strikes 49750/50250, underlying=50000 | extension = max(5000, 1500, 50) = 5000; percentage still dominates |

---

## 7. Very Narrow Spreads on Expensive Symbols

| # | Test Case | Input | Expected |
|---|-----------|-------|----------|
| 7.1 | **$10 spread on $20,000 underlying** | strikes 19995/20005, underlying=20000 | extension = max(2000, 30, 50) = 2000; chart shows meaningful +-10% range |
| 7.2 | **$5 spread on $5,000 underlying** | strikes 4998/5003, underlying=5000 | extension = max(500, 15, 50) = 500; step=ceil(1005/400)=3 |
| 7.3 | **$1 spread on $23,000 underlying** | strikes 23000/23001, underlying=23000 | extension = max(2300, 3, 50) = 2300; step=ceil(4601/400)=12 |
| 7.4 | **Initial view still shows strategy structure** | same as 7.3 | viewPad = max(1150, 1.5, 25) = 1150; strikes visible within initial window |

---

## 8. Very Wide Spreads on Cheap Symbols

| # | Test Case | Input | Expected |
|---|-----------|-------|----------|
| 8.1 | **$40 spread on $25 stock** | strikes 5/45, underlying=25 | extension = max(2.5, 120, 50) = 120; range covers +/-120 from strikes |
| 8.2 | **$20 spread on $10 stock** | strikes 1/21, underlying=10 | extension = max(1, 60, 50) = 60; floor at strikeRange * 3 |
| 8.3 | **Range wider than underlying price** | strikes 5/45 on $25 stock | lowerBound could be negative (5-120 = -115); verify no crash |

---

## 9. Initial View Window (FR-4)

Verify `viewPad = Math.max(underlyingPrice * 0.05, strikeSpan * 1.5, 25)` in `createMultiLegChartConfig()`.

| # | Test Case | Input | Expected |
|---|-----------|-------|----------|
| 9.1 | **NDX — percentage dominates** | strikes 23100/23150, underlying=23132 | viewPad = max(1156.6, 75, 25) = 1156.6; window >= 2*1156.6 + 50 = ~2363 |
| 9.2 | **Initial view width >= 5% of underlying for expensive symbols** | underlying=50000, strikes 49950/50050 | viewPad = max(2500, 150, 25) = 2500; window >= 5100 |
| 9.3 | **Cheap stock — strikeSpan * 1.5 dominates** | strikes 48/53, underlying=50 | viewPad = max(2.5, 7.5, 25) = 25; window >= 55 |
| 9.4 | **$25 floor dominates for very cheap** | strikes 0.90/1.10, underlying=1.0 | viewPad = max(0.05, 0.3, 25) = 25; window >= 50.2 |
| 9.5 | **suggestedMin < minStrike** | Any scenario | suggestedMin < minStrike always |
| 9.6 | **suggestedMax > maxStrike** | Any scenario | suggestedMax > maxStrike always |
| 9.7 | **Single-strike fallback branch** | single-leg: 1 strike only | Falls to else branch; verify no crash and reasonable window |

---

## 10. Pan/Zoom Limits (FR-5)

Verify `limits.x = { min: Math.min(...prices), max: Math.max(...prices) }`.

| # | Test Case | Input | Expected |
|---|-----------|-------|----------|
| 10.1 | **Limits equal full data extent** | NDX spread | limits.x.min === prices[0], limits.x.max === prices[prices.length-1] |
| 10.2 | **Limits wider than initial view** | NDX spread | limits.x.min < suggestedMin, limits.x.max > suggestedMax |
| 10.3 | **Limits allow panning to negative prices** | Penny stock with negative lowerBound | limits.x.min < 0 if prices[0] < 0 |
| 10.4 | **Cheap stock limits** | $50 stock | limits match full generated range |

---

## 11. Butterfly / Iron Condor Payoff Consistency (AC-8)

Verify `generateButterflyPayoff()` uses the adaptive range and step formula consistently.

### 11a. Iron Condor Path

| # | Test Case | Input | Expected |
|---|-----------|-------|----------|
| 11a.1 | **NDX Iron Condor — data points** | IC at 23000/23050/23150/23200, legWidth=50 | prices.length <= 600 |
| 11a.2 | **NDX IC — range coverage** | same | range extends at least +/-2000 from outer strikes (estimatedPrice = (23000+23200)/2 = 23100; extension = max(50, 2310, 50) = 2310) |
| 11a.3 | **NDX IC — step formula** | same | step = ceil(totalRange/400); verify step > 1 |
| 11a.4 | **Cheap IC — step = 1** | IC at 45/47/53/55 on $50 stock, legWidth=10 | extension = max(10, 5, 50) = 50; totalRange ~ 110; step = 1 |
| 11a.5 | **IC payoff correctness** | Known IC with netPremium; verify payoff at short_put_strike = netPremium * 100 | Payoff at max-profit zone matches expected net credit * 100 |
| 11a.6 | **IC break-even correctness** | Known IC | lowerBreakEven = short_put_strike + netPremium; upperBreakEven = short_call_strike - netPremium |

### 11b. Standard Butterfly / Iron Butterfly Path

| # | Test Case | Input | Expected |
|---|-----------|-------|----------|
| 11b.1 | **NDX Call Butterfly — data points** | lower=23050, atm=23100, upper=23150, legWidth=50 | prices.length <= 600 |
| 11b.2 | **NDX Call Butterfly — range** | same | estimatedPrice = (23050+23150)/2 = 23100; extension = max(50, 2310, 50) = 2310 |
| 11b.3 | **Iron Butterfly — data points** | NDX-level IB, legWidth=50 | prices.length <= 600 |
| 11b.4 | **Cheap Call Butterfly** | lower=45, atm=50, upper=55, legWidth=10 | extension = max(10, 5, 50) = 50; step = 1; reasonable point count |
| 11b.5 | **Butterfly payoff at ATM = max profit** | Known call butterfly | Verify payoff at atm_strike matches expected max profit |

---

## 12. No Regression on Low/Mid-Priced Symbols (FR-6, AC-7)

| # | Test Case | Input | Expected |
|---|-----------|-------|----------|
| 12.1 | **$50 stock spread — range no worse than before** | strikes 48/53, underlying=50 | Range extends at least +/-50 (old behavior was strikeRange*5=25 capped at 500, new is 50 via floor) |
| 12.2 | **$200 stock spread** | strikes 195/205, underlying=200 | extension = max(20, 30, 50) = 50; range similar to or wider than old behavior |
| 12.3 | **$100 stock single-leg** | strike=100, underlying=102 | tierRange=max(500, 500)=500; range=max(500, 10.2, 50)=500; same as old $500 floor |
| 12.4 | **Break-even calculation unchanged** | NDX spread with known premiums | breakEvenPoints match manual calculation |
| 12.5 | **Net credit calculation unchanged** | Known credit spread | netCredit matches expected value |
| 12.6 | **Payoff values at known prices** | Long call strike=100, entry=5 at price=110 | payoff = (110-100-5)*100 = $500 |
| 12.7 | **Existing tests pass** | Run `npx vitest run` | All tests in `qa-chartUtils.test.js` and `chartUtils-range.test.js` pass |

---

## 13. Edge Cases and Boundary Conditions

| # | Test Case | Input | Expected |
|---|-----------|-------|----------|
| 13.1 | **underlyingPrice = 0** | underlying=0, strike=5 | extension = max(0, strikeRange*3, 50) = 50; no crash |
| 13.2 | **underlyingPrice very small (0.01)** | underlying=0.01, strike=0.50 | $50 floor applies; no crash |
| 13.3 | **Identical strikes (strikeRange=0) treated as single-leg** | two calls at same strike, underlying=100 | Takes single-leg path; range uses tiered multiplier |
| 13.4 | **Empty positions** | positions=[] | returns null |
| 13.5 | **Non-option positions filtered out** | positions with asset_class="us_equity" only | returns null |
| 13.6 | **Mixed option types in single-leg** | one put at strike=100 | single-leg path; range reasonable |
| 13.7 | **Very large quantity positions** | qty=100, same strikes | Range and step unchanged (qty affects payoff magnitude, not range) |
| 13.8 | **Step size boundary: totalRange / 400 rounds up** | totalRange=801 | step = ceil(801/400) = 3, not 2 |

---

## 14. Cross-Cutting Concerns

| # | Test Case | Verification |
|---|-----------|-------------|
| 14.1 | **Prices array is always sorted ascending** | For every scenario above, verify `prices[i] <= prices[i+1]` |
| 14.2 | **Payoffs array has same length as prices** | `payoffs.length === prices.length` for every scenario |
| 14.3 | **No NaN or Infinity in prices or payoffs** | Check `prices.every(isFinite)` and `payoffs.every(isFinite)` |
| 14.4 | **lowerBound < upperBound always** | `prices[0] < prices[prices.length - 1]` |
| 14.5 | **generateButterflyPayoff returns all expected fields** | `prices`, `payoffs`, `lowerBreakEven`, `upperBreakEven`, `netPremium` all present |
| 14.6 | **generateMultiLegPayoff returns all expected fields** | `prices`, `payoffs`, `breakEvenPoints`, `maxProfit`, `maxLoss`, `netCredit`, `strikes` all present |

---

## 15. Coverage Gaps in Existing Dev Tests

The following scenarios are **not** covered by `chartUtils-range.test.js` and should be added for thorough QA:

| # | Missing Coverage | Priority |
|---|-----------------|----------|
| 15.1 | Penny stock (~$1) multi-leg and single-leg | High |
| 15.2 | $50,000 mega-cap multi-leg and single-leg | High |
| 15.3 | Narrow spread on expensive underlying ($10 on $20,000) | High |
| 15.4 | Wide spread on cheap underlying ($40 on $25 stock) | Medium |
| 15.5 | SPX-level scenarios (~$5,900) | Medium |
| 15.6 | Butterfly/IC range coverage assertions (not just point count) | Medium |
| 15.7 | Initial view window width >= 5% of underlying for expensive symbols | High |
| 15.8 | Negative lowerBound scenario | Low |
| 15.9 | Prices sorted and no NaN assertions | Medium |
| 15.10 | Step size boundary cases (totalRange exactly 400, 401) | Low |
| 15.11 | Single-leg path on createMultiLegChartConfig (< 2 strikes fallback) | Medium |

---

## Execution Notes

- **Automated tests:** Items in sections 1-14 marked with formula expectations should be implemented as Vitest unit tests in `trade-app/tests/chartUtils-range.test.js` (extending the existing file).
- **Manual verification:** AC-5 (pan/zoom limits) and visual chart appearance should be verified manually in the browser by loading NDX, SPX, penny stock, and mid-cap option chains.
- **Regression baseline:** Run the full test suite (`cd trade-app && npx vitest run`) before and after any new test additions to confirm no regressions.
