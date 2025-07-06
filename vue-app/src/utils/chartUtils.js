export function generateButterflyPayoff(butterflyInfo, strategyType, legWidth) {
  const { lower_strike, atm_strike, upper_strike } = butterflyInfo;

  // Calculate net premium
  let netPremium;
  if (strategyType === "IRON Butterfly") {
    netPremium =
      butterflyInfo.lower_price -
      butterflyInfo.atm_put_price -
      butterflyInfo.atm_call_price +
      butterflyInfo.upper_price;
  } else {
    netPremium =
      butterflyInfo.lower_price -
      2 * butterflyInfo.atm_price +
      butterflyInfo.upper_price;
  }

  // Generate price range
  const lowerBound = Math.floor(lower_strike - legWidth);
  const upperBound = Math.ceil(upper_strike + legWidth);
  const step =
    upperBound - lowerBound > 100 ? (upperBound - lowerBound > 200 ? 5 : 2) : 1;

  const prices = [];
  const payoffs = [];

  for (let price = lowerBound; price <= upperBound; price += step) {
    prices.push(price);

    let payoff;
    if (strategyType === "IRON Butterfly") {
      const width = Math.abs(atm_strike - lower_strike);
      const maxProfit = Math.abs(netPremium) * 100;
      const maxLoss = (width - Math.abs(netPremium)) * 100;

      if (price <= lower_strike) {
        payoff = -maxLoss;
      } else if (price < atm_strike) {
        const ratio = (price - lower_strike) / width;
        payoff = -maxLoss + ratio * (maxLoss + maxProfit);
      } else if (price <= upper_strike) {
        const ratio = (upper_strike - price) / width;
        payoff = -maxLoss + ratio * (maxLoss + maxProfit);
      } else {
        payoff = -maxLoss;
      }
    } else {
      payoff =
        (Math.max(price - lower_strike, 0) -
          2 * Math.max(price - atm_strike, 0) +
          Math.max(price - upper_strike, 0) -
          netPremium) *
        100;
    }

    payoffs.push(payoff);
  }

  // Calculate break-even points
  let lowerBreakEven, upperBreakEven;
  if (strategyType === "IRON Butterfly") {
    lowerBreakEven = atm_strike - Math.abs(netPremium);
    upperBreakEven = atm_strike + Math.abs(netPremium);
  } else {
    lowerBreakEven = lower_strike + netPremium;
    upperBreakEven = upper_strike - netPremium;
  }

  const result = {
    prices,
    payoffs,
    lowerBreakEven,
    upperBreakEven,
    netPremium,
  };

  return result;
}

// Convert butterfly info to multi-leg position format for unified chart component
export function convertButterflyToPositions(
  butterflyInfo,
  strategyType,
  symbol,
  expiry
) {
  if (!butterflyInfo) return [];

  const positions = [];
  const expiryDate = expiry.toISOString().split("T")[0];

  if (strategyType === "IRON Butterfly") {
    // Buy Put (Lower Strike)
    positions.push({
      symbol: `${symbol}_PUT_${butterflyInfo.lower_strike}_${expiryDate}`,
      asset_class: "us_option",
      side: "long",
      qty: 1,
      strike_price: butterflyInfo.lower_strike,
      option_type: "put",
      expiry_date: expiryDate,
      current_price: butterflyInfo.lower_price,
      avg_entry_price: butterflyInfo.lower_price,
      cost_basis: butterflyInfo.lower_price * 100,
      market_value: butterflyInfo.lower_price * 100,
      unrealized_pl: 0,
      unrealized_plpc: 0,
      underlying_symbol: symbol,
    });

    // Sell Put (ATM Strike)
    positions.push({
      symbol: `${symbol}_PUT_${butterflyInfo.atm_strike}_${expiryDate}`,
      asset_class: "us_option",
      side: "short",
      qty: -1,
      strike_price: butterflyInfo.atm_strike,
      option_type: "put",
      expiry_date: expiryDate,
      current_price: butterflyInfo.atm_put_price,
      avg_entry_price: butterflyInfo.atm_put_price,
      cost_basis: -butterflyInfo.atm_put_price * 100,
      market_value: -butterflyInfo.atm_put_price * 100,
      unrealized_pl: 0,
      unrealized_plpc: 0,
      underlying_symbol: symbol,
    });

    // Sell Call (ATM Strike)
    positions.push({
      symbol: `${symbol}_CALL_${butterflyInfo.atm_strike}_${expiryDate}`,
      asset_class: "us_option",
      side: "short",
      qty: -1,
      strike_price: butterflyInfo.atm_strike,
      option_type: "call",
      expiry_date: expiryDate,
      current_price: butterflyInfo.atm_call_price,
      avg_entry_price: butterflyInfo.atm_call_price,
      cost_basis: -butterflyInfo.atm_call_price * 100,
      market_value: -butterflyInfo.atm_call_price * 100,
      unrealized_pl: 0,
      unrealized_plpc: 0,
      underlying_symbol: symbol,
    });

    // Buy Call (Upper Strike)
    positions.push({
      symbol: `${symbol}_CALL_${butterflyInfo.upper_strike}_${expiryDate}`,
      asset_class: "us_option",
      side: "long",
      qty: 1,
      strike_price: butterflyInfo.upper_strike,
      option_type: "call",
      expiry_date: expiryDate,
      current_price: butterflyInfo.upper_price,
      avg_entry_price: butterflyInfo.upper_price,
      cost_basis: butterflyInfo.upper_price * 100,
      market_value: butterflyInfo.upper_price * 100,
      unrealized_pl: 0,
      unrealized_plpc: 0,
      underlying_symbol: symbol,
    });
  } else {
    // CALL or PUT Butterfly
    const optionType = strategyType === "CALL Butterfly" ? "call" : "put";
    const typeUpper = optionType.toUpperCase();

    // Buy Lower Strike
    positions.push({
      symbol: `${symbol}_${typeUpper}_${butterflyInfo.lower_strike}_${expiryDate}`,
      asset_class: "us_option",
      side: "long",
      qty: 1,
      strike_price: butterflyInfo.lower_strike,
      option_type: optionType,
      expiry_date: expiryDate,
      current_price: butterflyInfo.lower_price,
      avg_entry_price: butterflyInfo.lower_price,
      cost_basis: butterflyInfo.lower_price * 100,
      market_value: butterflyInfo.lower_price * 100,
      unrealized_pl: 0,
      unrealized_plpc: 0,
      underlying_symbol: symbol,
    });

    // Sell ATM Strike (2 contracts)
    positions.push({
      symbol: `${symbol}_${typeUpper}_${butterflyInfo.atm_strike}_${expiryDate}`,
      asset_class: "us_option",
      side: "short",
      qty: -2,
      strike_price: butterflyInfo.atm_strike,
      option_type: optionType,
      expiry_date: expiryDate,
      current_price: butterflyInfo.atm_price,
      avg_entry_price: butterflyInfo.atm_price,
      cost_basis: -butterflyInfo.atm_price * 200, // 2 contracts
      market_value: -butterflyInfo.atm_price * 200,
      unrealized_pl: 0,
      unrealized_plpc: 0,
      underlying_symbol: symbol,
    });

    // Buy Upper Strike
    positions.push({
      symbol: `${symbol}_${typeUpper}_${butterflyInfo.upper_strike}_${expiryDate}`,
      asset_class: "us_option",
      side: "long",
      qty: 1,
      strike_price: butterflyInfo.upper_strike,
      option_type: optionType,
      expiry_date: expiryDate,
      current_price: butterflyInfo.upper_price,
      avg_entry_price: butterflyInfo.upper_price,
      cost_basis: butterflyInfo.upper_price * 100,
      market_value: butterflyInfo.upper_price * 100,
      unrealized_pl: 0,
      unrealized_plpc: 0,
      underlying_symbol: symbol,
    });
  }

  return positions;
}

export function generateMultiLegPayoff(positions, underlyingPrice) {
  // console.log("generateMultiLegPayoff called with:", {
  //   positionsCount: positions.length,
  //   underlyingPrice,
  // });

  if (!positions || positions.length === 0) {
    return null;
  }

  // Extract option positions only
  const optionPositions = positions.filter(
    (pos) => pos.asset_class === "us_option"
  );

  if (optionPositions.length === 0) {
    return null;
  }

  // Find price range based on strikes
  const strikes = optionPositions.map((pos) => pos.strike_price);
  const minStrike = Math.min(...strikes);
  const maxStrike = Math.max(...strikes);
  const strikeRange = maxStrike - minStrike;

  // Create price range for chart (extend beyond strikes)
  const lowerBound = Math.floor(minStrike - strikeRange * 0.2);
  const upperBound = Math.ceil(maxStrike + strikeRange * 0.2);
  const step = strikeRange > 50 ? (strikeRange > 100 ? 5 : 2) : 1;

  const prices = [];
  const payoffs = [];

  // Calculate total cost basis (what we paid/received for the positions)
  const totalCostBasis = optionPositions.reduce((sum, pos) => {
    return sum + (pos.cost_basis || 0);
  }, 0);

  for (let price = lowerBound; price <= upperBound; price += step) {
    prices.push(price);

    let totalPayoff = 0;

    // Calculate payoff for each position at this price
    for (const position of optionPositions) {
      const { strike_price, option_type, qty, avg_entry_price } = position;

      let intrinsicValue = 0;
      if (option_type === "call") {
        intrinsicValue = Math.max(price - strike_price, 0);
      } else if (option_type === "put") {
        intrinsicValue = Math.max(strike_price - price, 0);
      }

      // Calculate P&L for this position
      // For long positions (qty > 0): (intrinsic_value - premium_paid) * qty * 100
      // For short positions (qty < 0): (premium_received - intrinsic_value) * |qty| * 100
      const premiumPaid = avg_entry_price || 0;
      let positionPayoff;

      if (qty > 0) {
        // Long position
        positionPayoff = (intrinsicValue - premiumPaid) * qty * 100;
      } else {
        // Short position
        positionPayoff = (premiumPaid - intrinsicValue) * Math.abs(qty) * 100;
      }

      totalPayoff += positionPayoff;
    }

    payoffs.push(totalPayoff);
  }

  // Find break-even points (where payoff crosses zero)
  const breakEvenPoints = [];
  for (let i = 1; i < payoffs.length; i++) {
    if (
      (payoffs[i - 1] <= 0 && payoffs[i] >= 0) ||
      (payoffs[i - 1] >= 0 && payoffs[i] <= 0)
    ) {
      // Linear interpolation to find more precise break-even
      const x1 = prices[i - 1];
      const x2 = prices[i];
      const y1 = payoffs[i - 1];
      const y2 = payoffs[i];

      if (y2 !== y1) {
        const breakEven = x1 - (y1 * (x2 - x1)) / (y2 - y1);
        breakEvenPoints.push(breakEven);
      }
    }
  }

  // Calculate current unrealized P&L
  const currentUnrealizedPL = optionPositions.reduce((sum, pos) => {
    return sum + (pos.unrealized_pl || 0);
  }, 0);

  // Calculate max profit and max loss
  const maxProfit = Math.max(...payoffs);
  const maxLoss = Math.min(...payoffs);

  const result = {
    prices,
    payoffs,
    breakEvenPoints,
    maxProfit,
    maxLoss,
    currentUnrealizedPL,
    totalCostBasis,
    positionCount: optionPositions.length,
  };

  // console.log("generateMultiLegPayoff result:", {
  //   pricesLength: prices.length,
  //   payoffsLength: payoffs.length,
  //   breakEvenPoints,
  //   maxProfit,
  //   maxLoss,
  //   currentUnrealizedPL,
  //   positionCount: optionPositions.length,
  // });

  return result;
}

export function createChartConfig(chartData, underlyingPrice) {
  const { prices, payoffs, lowerBreakEven, upperBreakEven } = chartData;

  // Create data points for the chart
  const chartPoints = prices.map((price, index) => ({
    x: price,
    y: payoffs[index],
  }));

  const zeroLinePoints = prices.map((price) => ({
    x: price,
    y: 0,
  }));

  // Create current price line (same as in Trade Management)
  const maxPayoff = Math.max(...payoffs);
  const minPayoff = Math.min(...payoffs);
  const currentPricePoints = [
    { x: underlyingPrice, y: minPayoff - Math.abs(minPayoff) * 0.1 },
    { x: underlyingPrice, y: maxPayoff + Math.abs(maxPayoff) * 0.1 },
  ];

  return {
    type: "line",
    data: {
      datasets: [
        {
          label: "Butterfly Payoff",
          data: chartPoints,
          borderColor: "rgb(75, 192, 192)",
          backgroundColor: "rgba(75, 192, 192, 0.1)",
          borderWidth: 2,
          pointRadius: 4,
          pointHoverRadius: 6,
          fill: false,
          tension: 0, // Set to 0 for straight lines
        },
        {
          label: "Zero Line",
          data: zeroLinePoints,
          borderColor: "rgba(128, 128, 128, 0.5)",
          borderWidth: 1,
          borderDash: [5, 5],
          pointRadius: 0,
          fill: false,
        },
        {
          label: "Current Price",
          data: currentPricePoints,
          borderColor: "rgba(54, 162, 235, 0.8)",
          backgroundColor: "rgba(54, 162, 235, 0.8)",
          borderWidth: 2,
          borderDash: [3, 3],
          pointRadius: 0,
          fill: false,
        },
      ],
    },
    options: {
      responsive: true,
      maintainAspectRatio: false,
      animation: false, // Disable animations to avoid canvas issues
      plugins: {
        title: {
          display: true,
          text: "Butterfly Payoff Diagram",
          font: {
            size: 16,
          },
        },
        legend: {
          display: true,
          position: "top",
        },
        tooltip: {
          mode: "nearest",
          intersect: false,
          callbacks: {
            title: function (context) {
              return `Price: $${context[0].parsed.x.toFixed(2)}`;
            },
            label: function (context) {
              if (context.datasetIndex === 0) {
                return `P&L: $${context.parsed.y.toFixed(2)}`;
              } else if (context.datasetIndex === 2) {
                return `Current: $${underlyingPrice.toFixed(2)}`;
              }
              return `${context.dataset.label}: $${context.parsed.y.toFixed(
                2
              )}`;
            },
          },
        },
      },
      scales: {
        x: {
          type: "linear",
          title: {
            display: true,
            text: "Underlying Price at Expiry ($)",
          },
          grid: {
            display: true,
            color: "rgba(0, 0, 0, 0.1)",
          },
        },
        y: {
          title: {
            display: true,
            text: "Profit / Loss ($)",
          },
          grid: {
            display: true,
            color: "rgba(0, 0, 0, 0.1)",
          },
        },
      },
    },
  };
}

export function createMultiLegChartConfig(chartData, underlyingPrice) {
  const {
    prices,
    payoffs,
    breakEvenPoints,
    maxProfit,
    maxLoss,
    positionCount,
  } = chartData;

  // Create data points for the chart
  const chartPoints = prices.map((price, index) => ({
    x: price,
    y: payoffs[index],
  }));

  const zeroLinePoints = prices.map((price) => ({
    x: price,
    y: 0,
  }));

  // Create current price line
  const currentPricePoints = [
    { x: underlyingPrice, y: Math.min(...payoffs) - Math.abs(maxLoss) * 0.1 },
    { x: underlyingPrice, y: Math.max(...payoffs) + Math.abs(maxProfit) * 0.1 },
  ];

  const datasets = [
    {
      label: "Zero Line",
      data: zeroLinePoints,
      borderColor: "rgba(128, 128, 128, 0.6)",
      borderWidth: 1,
      borderDash: [5, 5],
      pointRadius: 0,
      fill: false,
      order: 2,
    },
    {
      label: `Position Payoff (${positionCount} legs)`,
      data: chartPoints,
      borderColor: "rgb(33, 37, 41)",
      backgroundColor: function (context) {
        const chart = context.chart;
        const { ctx, chartArea } = chart;

        if (!chartArea) {
          return null;
        }

        // Create gradient that changes color at zero line
        const gradient = ctx.createLinearGradient(
          0,
          chartArea.bottom,
          0,
          chartArea.top
        );

        // Find the zero line position
        const yScale = chart.scales.y;
        const zeroPixel = yScale.getPixelForValue(0);
        const topPixel = chartArea.top;
        const bottomPixel = chartArea.bottom;

        // Calculate relative positions for gradient stops
        const zeroPosition =
          (bottomPixel - zeroPixel) / (bottomPixel - topPixel);

        // Add gradient stops
        gradient.addColorStop(0, "rgba(244, 67, 54, 0.3)"); // Red at bottom (loss)
        gradient.addColorStop(
          Math.max(0, Math.min(1, zeroPosition)),
          "rgba(244, 67, 54, 0.1)"
        ); // Fade to transparent at zero
        gradient.addColorStop(
          Math.max(0, Math.min(1, zeroPosition)),
          "rgba(76, 175, 80, 0.1)"
        ); // Fade from transparent at zero
        gradient.addColorStop(1, "rgba(76, 175, 80, 0.3)"); // Green at top (profit)

        return gradient;
      },
      borderWidth: 2,
      pointRadius: 1,
      pointHoverRadius: 4,
      fill: "origin", // Fill to zero line
      tension: 0,
      order: 1,
    },
    {
      label: "Current Price",
      data: currentPricePoints,
      borderColor: "rgba(54, 162, 235, 0.8)",
      backgroundColor: "rgba(54, 162, 235, 0.8)",
      borderWidth: 2,
      borderDash: [3, 3],
      pointRadius: 0,
      fill: false,
      order: 1,
    },
  ];

  return {
    type: "line",
    data: { datasets },
    options: {
      responsive: true,
      maintainAspectRatio: false,
      animation: false,
      interaction: {
        intersect: false,
        mode: "index",
      },
      plugins: {
        title: {
          display: true,
          text: `Multi-Leg Position Payoff (${positionCount} legs)`,
          font: {
            size: 16,
          },
        },
        legend: {
          display: true,
          position: "top",
          labels: {
            filter: function (item, chart) {
              // Hide the area fill datasets from legend, only show main payoff line
              return !item.text.includes("Area");
            },
          },
        },
        tooltip: {
          mode: "nearest",
          intersect: false,
          filter: function (tooltipItem) {
            // Only show tooltip for main payoff line and current price
            return (
              tooltipItem.datasetIndex === 2 || tooltipItem.datasetIndex === 4
            );
          },
          callbacks: {
            title: function (context) {
              return `Price: $${context[0].parsed.x.toFixed(2)}`;
            },
            label: function (context) {
              if (context.datasetIndex === 2) {
                return `P&L: $${context.parsed.y.toFixed(2)}`;
              } else if (context.datasetIndex === 4) {
                return `Current: $${underlyingPrice.toFixed(2)}`;
              }
              return `${context.dataset.label}: $${context.parsed.y.toFixed(
                2
              )}`;
            },
          },
        },
      },
      scales: {
        x: {
          type: "linear",
          title: {
            display: true,
            text: "Underlying Price at Expiry ($)",
          },
          grid: {
            display: true,
            color: "rgba(0, 0, 0, 0.1)",
          },
        },
        y: {
          title: {
            display: true,
            text: "Profit / Loss ($)",
          },
          grid: {
            display: true,
            color: "rgba(0, 0, 0, 0.1)",
          },
        },
      },
    },
  };
}
