export function generateButterflyPayoff(butterflyInfo, strategyType, legWidth) {
  if (strategyType === "IRON Condor") {
    const {
      long_put_strike,
      short_put_strike,
      short_call_strike,
      long_call_strike,
      long_put_price,
      short_put_price,
      short_call_price,
      long_call_price,
    } = butterflyInfo;

    const netPremium =
      short_put_price + short_call_price - long_put_price - long_call_price;

    const lowerBound = Math.floor(long_put_strike - legWidth);
    const upperBound = Math.ceil(long_call_strike + legWidth);
    const step =
      upperBound - lowerBound > 100
        ? upperBound - lowerBound > 200
          ? 5
          : 2
        : 1;

    const prices = [];
    const payoffs = [];

    for (let price = lowerBound; price <= upperBound; price += step) {
      prices.push(price);
      const payoff =
        (Math.max(0, price - long_call_strike) -
          Math.max(0, price - short_call_strike) +
          (Math.max(0, long_put_strike - price) -
            Math.max(0, short_put_strike - price)) -
          netPremium) *
        100;
      payoffs.push(payoff);
    }

    const lowerBreakEven = short_put_strike + netPremium;
    const upperBreakEven = short_call_strike - netPremium;

    return {
      prices,
      payoffs,
      lowerBreakEven,
      upperBreakEven,
      netPremium,
    };
  }

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

  if (strategyType === "IRON Condor") {
    const {
      long_put_strike,
      short_put_strike,
      short_call_strike,
      long_call_strike,
      long_put_price,
      short_put_price,
      short_call_price,
      long_call_price,
    } = butterflyInfo;

    // Buy Put
    positions.push({
      symbol: `${symbol}_PUT_${long_put_strike}_${expiryDate}`,
      asset_class: "us_option",
      side: "long",
      qty: 1,
      strike_price: long_put_strike,
      option_type: "put",
      expiry_date: expiryDate,
      current_price: long_put_price,
      avg_entry_price: long_put_price,
      cost_basis: long_put_price * 100,
      market_value: long_put_price * 100,
      unrealized_pl: 0,
      unrealized_plpc: 0,
      underlying_symbol: symbol,
    });

    // Sell Put
    positions.push({
      symbol: `${symbol}_PUT_${short_put_strike}_${expiryDate}`,
      asset_class: "us_option",
      side: "short",
      qty: -1,
      strike_price: short_put_strike,
      option_type: "put",
      expiry_date: expiryDate,
      current_price: short_put_price,
      avg_entry_price: short_put_price,
      cost_basis: -short_put_price * 100,
      market_value: -short_put_price * 100,
      unrealized_pl: 0,
      unrealized_plpc: 0,
      underlying_symbol: symbol,
    });

    // Sell Call
    positions.push({
      symbol: `${symbol}_CALL_${short_call_strike}_${expiryDate}`,
      asset_class: "us_option",
      side: "short",
      qty: -1,
      strike_price: short_call_strike,
      option_type: "call",
      expiry_date: expiryDate,
      current_price: short_call_price,
      avg_entry_price: short_call_price,
      cost_basis: -short_call_price * 100,
      market_value: -short_call_price * 100,
      unrealized_pl: 0,
      unrealized_plpc: 0,
      underlying_symbol: symbol,
    });

    // Buy Call
    positions.push({
      symbol: `${symbol}_CALL_${long_call_strike}_${expiryDate}`,
      asset_class: "us_option",
      side: "long",
      qty: 1,
      strike_price: long_call_strike,
      option_type: "call",
      expiry_date: expiryDate,
      current_price: long_call_price,
      avg_entry_price: long_call_price,
      cost_basis: long_call_price * 100,
      market_value: long_call_price * 100,
      unrealized_pl: 0,
      unrealized_plpc: 0,
      underlying_symbol: symbol,
    });
  } else if (strategyType === "IRON Butterfly") {
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

  // Create EXPANDED price range for chart to show both profit and loss areas
  let lowerBound, upperBound, step;

  if (strikeRange === 0) {
    // Single leg: create much wider range to show full P&L spectrum
    const singleStrike = minStrike;

    // Use a much wider range to show both profit and loss areas
    let range;
    if (singleStrike < 50) {
      range = Math.max(singleStrike * 0.8, 25); // 80% range for low-priced stocks
    } else if (singleStrike < 200) {
      range = Math.max(singleStrike * 0.5, 50); // 50% range for mid-priced stocks
    } else {
      range = Math.max(singleStrike * 0.3, 100); // 30% range for high-priced stocks
    }

    lowerBound = Math.floor(singleStrike - range);
    upperBound = Math.ceil(singleStrike + range);
    step = range > 200 ? 5 : range > 100 ? 2 : 1;
  } else {
    // Multi-leg: significantly expand the range to show full profit/loss spectrum
    const baseExtension = Math.max(strikeRange * 0.5, 50); // At least 50% extension or $50
    const maxExtension = Math.min(baseExtension, 200); // Cap at $200 for very wide spreads

    lowerBound = Math.floor(minStrike - maxExtension);
    upperBound = Math.ceil(maxStrike + maxExtension);
    step =
      upperBound - lowerBound > 200 ? 5 : upperBound - lowerBound > 100 ? 2 : 1;
  }

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
          mode: "x",
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
  const { prices, payoffs, maxProfit, maxLoss, positionCount } = chartData;

  // Calculate strikes for initial zoom range
  const strikes = [];
  // Extract strikes from the prices array - we'll use a reasonable range around current price
  const currentPrice = underlyingPrice;

  // Data preparation code remains the same
  const chartPoints = prices.map((price, index) => ({
    x: price,
    y: payoffs[index],
  }));
  const zeroLinePoints = prices.map((price) => ({ x: price, y: 0 }));
  const currentPricePoints = [
    { x: underlyingPrice, y: Math.min(...payoffs) - Math.abs(maxLoss) * 0.1 },
    { x: underlyingPrice, y: Math.max(...payoffs) + Math.abs(maxProfit) * 0.1 },
  ];
  const datasets = [
    {
      label: `Position Payoff (${positionCount} legs)`,
      data: chartPoints,
      borderColor: "rgb(33, 37, 41)",
      borderWidth: 2,
      pointRadius: 0, // Hide default points
      fill: "origin",
      tension: 0,
      order: 1,
      // background color logic from your code...
      backgroundColor: function (context) {
        const chart = context.chart;
        const { ctx, chartArea } = chart;
        if (!chartArea) return null;
        const gradient = ctx.createLinearGradient(
          0,
          chartArea.bottom,
          0,
          chartArea.top
        );
        const yScale = chart.scales.y;
        const zeroPixel = yScale.getPixelForValue(0);
        const topPixel = chartArea.top;
        const bottomPixel = chartArea.bottom;
        const zeroPosition =
          (bottomPixel - zeroPixel) / (bottomPixel - topPixel);
        gradient.addColorStop(0, "rgba(244, 67, 54, 0.3)");
        gradient.addColorStop(
          Math.max(0, Math.min(1, zeroPosition)),
          "rgba(244, 67, 54, 0.1)"
        );
        gradient.addColorStop(
          Math.max(0, Math.min(1, zeroPosition)),
          "rgba(76, 175, 80, 0.1)"
        );
        gradient.addColorStop(1, "rgba(76, 175, 80, 0.3)");
        return gradient;
      },
    },
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
      label: "Current Price",
      data: currentPricePoints,
      borderColor: "rgba(54, 162, 235, 0.8)",
      borderWidth: 2,
      borderDash: [3, 3],
      pointRadius: 0,
      fill: false,
      order: 3,
    },
  ];

  const customCrosshairPlugin = {
    id: "customCrosshair",

    afterEvent: (chart, args) => {
      const { event } = args;
      if (event.type === "mousemove") {
        chart.crosshair = args.inChartArea ? { x: event.x, y: event.y } : null;
        chart.draw();
      } else if (event.type === "mouseout") {
        chart.crosshair = null;
        chart.draw();
      }
    },

    afterDraw: (chart, args, options) => {
      const { crosshair } = chart;
      if (!crosshair) return;

      const {
        ctx,
        chartArea: { top, bottom, left, right },
        scales,
      } = chart;

      // 1. Draw Vertical Line
      ctx.save();
      ctx.beginPath();
      ctx.moveTo(crosshair.x, top);
      ctx.lineTo(crosshair.x, bottom);
      ctx.lineWidth = 1;
      ctx.strokeStyle = "#555";
      ctx.setLineDash([6, 6]);
      ctx.stroke();

      // 2. Perform Interpolation to find P&L
      const xVal = scales.x.getValueForPixel(crosshair.x);
      let i = options.prices.findIndex((p) => p >= xVal);
      if (i === -1) i = options.prices.length - 1;
      if (i === 0) i = 1;

      const x1 = options.prices[i - 1],
        x2 = options.prices[i];
      const y1 = options.payoffs[i - 1],
        y2 = options.payoffs[i];
      const t = x2 !== x1 ? (xVal - x1) / (x2 - x1) : 0;
      const yVal = y1 + t * (y2 - y1);
      const yPixel = scales.y.getPixelForValue(yVal);

      // 3. Draw Intersection Circle on the P&L line
      if (yPixel >= top && yPixel <= bottom) {
        ctx.beginPath();
        ctx.fillStyle = "rgb(33, 37, 41)"; // Match line color
        ctx.arc(crosshair.x, yPixel, 5, 0, 2 * Math.PI);
        ctx.fill();
      }

      // 4. Draw the Text Box with Price and P&L
      const priceText = `$${xVal.toFixed(2)}`;
      const plText = `P&L: $${yVal.toFixed(2)}`;
      ctx.font = "bold 12px sans-serif";
      const priceWidth = ctx.measureText(priceText).width;
      const plWidth = ctx.measureText(plText).width;
      const boxWidth = Math.max(priceWidth, plWidth) + 16;
      const boxHeight = 40;

      // Position box smartly to avoid edges
      let boxX =
        crosshair.x > chart.width / 2
          ? crosshair.x - boxWidth - 10
          : crosshair.x + 10;
      let boxY = crosshair.y - boxHeight / 2;
      if (boxY < top) boxY = top;
      if (boxY + boxHeight > bottom) boxY = bottom - boxHeight;

      // Draw box
      ctx.fillStyle = "rgba(255, 255, 255, 0.9)";
      ctx.strokeStyle = "#ccc";
      ctx.lineWidth = 1;
      ctx.beginPath();
      ctx.roundRect(boxX, boxY, boxWidth, boxHeight, 5);
      ctx.fill();
      ctx.stroke();

      // Draw text
      ctx.fillStyle = "#333";
      ctx.textAlign = "left";
      ctx.fillText(priceText, boxX + 8, boxY + 16);
      ctx.fillText(plText, boxX + 8, boxY + 32);

      ctx.restore();
    },
  };

  return {
    type: "line",
    data: { datasets },
    plugins: [customCrosshairPlugin],
    options: {
      responsive: true,
      maintainAspectRatio: false,
      animation: false,
      interaction: {
        intersect: false,
        mode: "index",
      },
      plugins: {
        // We pass the data to our plugin via this options block
        customCrosshair: {
          prices: prices,
          payoffs: payoffs,
        },
        // IMPORTANT: Disable the default tooltip
        tooltip: {
          enabled: false,
        },
        title: {
          display: true,
          text: `Multi-Leg Position Payoff (${positionCount} legs)`,
          font: { size: 16 },
        },
        legend: {
          display: true,
          position: "top",
        },
        // Add zoom and pan functionality (horizontal only for payoff charts)
        zoom: {
          zoom: {
            wheel: {
              enabled: true,
            },
            pinch: {
              enabled: true,
            },
            mode: "x", // Only allow horizontal zoom
            scaleMode: "x", // Only scale X-axis
            onZoomComplete: function (context) {
              // Optional: Add any zoom completion logic here
            },
          },
          pan: {
            enabled: true,
            mode: "x", // Only allow horizontal panning
            modifierKey: null, // Allow panning without modifier key
            onPanComplete: function (context) {
              // Optional: Add any pan completion logic here
            },
          },
          limits: {
            x: {
              min: Math.min(...prices) - 50,
              max: Math.max(...prices) + 50,
            },
            // Remove Y limits to prevent vertical zoom/pan
          },
        },
      },
      scales: {
        x: {
          type: "linear",
          title: { display: true, text: "Underlying Price at Expiry ($)" },
          grid: { display: true, color: "rgba(0, 0, 0, 0.1)" },
          // Set initial view to be tightly focused around current price
          min: function (context) {
            const currentPrice = underlyingPrice;

            // Use a fixed range that makes sense for options trading
            // Most options strategies work within a $15-25 range for initial analysis
            const initialRange = 15; // Fixed $15 range on each side

            return Math.max(currentPrice - initialRange, Math.min(...prices));
          },
          max: function (context) {
            const currentPrice = underlyingPrice;

            // Use a fixed range that makes sense for options trading
            // Most options strategies work within a $15-25 range for initial analysis
            const initialRange = 15; // Fixed $15 range on each side

            return Math.min(currentPrice + initialRange, Math.max(...prices));
          },
        },
        y: {
          title: { display: true, text: "Profit / Loss ($)" },
          grid: { display: true, color: "rgba(0, 0, 0, 0.1)" },
          // Dynamically adjust Y-axis based on visible X-axis range
          min: function (context) {
            const chart = context.chart;
            if (!chart || !chart.scales || !chart.scales.x) {
              // Fallback to full range if chart not ready
              const maxAbsValue = Math.max(
                Math.abs(Math.max(...payoffs)),
                Math.abs(Math.min(...payoffs))
              );
              return -maxAbsValue * 1.1;
            }

            // Get visible X-axis range
            const xScale = chart.scales.x;
            const visibleMin = xScale.min || underlyingPrice - 15;
            const visibleMax = xScale.max || underlyingPrice + 15;

            // Find P&L values within visible price range
            const visiblePayoffs = [];
            for (let i = 0; i < prices.length; i++) {
              if (prices[i] >= visibleMin && prices[i] <= visibleMax) {
                visiblePayoffs.push(payoffs[i]);
              }
            }

            if (visiblePayoffs.length === 0) {
              // Fallback if no data in visible range
              return -1000;
            }

            // Use symmetric scaling around zero for visible data
            const maxAbsValue = Math.max(
              Math.abs(Math.max(...visiblePayoffs)),
              Math.abs(Math.min(...visiblePayoffs))
            );

            // Ensure minimum range for readability
            const minRange = 100;
            const finalRange = Math.max(maxAbsValue * 1.2, minRange);

            return -finalRange;
          },
          max: function (context) {
            const chart = context.chart;
            if (!chart || !chart.scales || !chart.scales.x) {
              // Fallback to full range if chart not ready
              const maxAbsValue = Math.max(
                Math.abs(Math.max(...payoffs)),
                Math.abs(Math.min(...payoffs))
              );
              return maxAbsValue * 1.1;
            }

            // Get visible X-axis range
            const xScale = chart.scales.x;
            const visibleMin = xScale.min || underlyingPrice - 15;
            const visibleMax = xScale.max || underlyingPrice + 15;

            // Find P&L values within visible price range
            const visiblePayoffs = [];
            for (let i = 0; i < prices.length; i++) {
              if (prices[i] >= visibleMin && prices[i] <= visibleMax) {
                visiblePayoffs.push(payoffs[i]);
              }
            }

            if (visiblePayoffs.length === 0) {
              // Fallback if no data in visible range
              return 1000;
            }

            // Use symmetric scaling around zero for visible data
            const maxAbsValue = Math.max(
              Math.abs(Math.max(...visiblePayoffs)),
              Math.abs(Math.min(...visiblePayoffs))
            );

            // Ensure minimum range for readability
            const minRange = 100;
            const finalRange = Math.max(maxAbsValue * 1.2, minRange);

            return finalRange;
          },
        },
      },
    },
  };
}
