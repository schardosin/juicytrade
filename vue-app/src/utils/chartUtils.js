export function generateButterflyPayoff(butterflyInfo, strategyType, legWidth) {
  console.log("generateButterflyPayoff called with:", {
    butterflyInfo,
    strategyType,
    legWidth,
  });

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

  console.log("Calculated net premium:", netPremium);

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

  console.log("generateButterflyPayoff result:", {
    pricesLength: prices.length,
    payoffsLength: payoffs.length,
    samplePrices: prices.slice(0, 5),
    samplePayoffs: payoffs.slice(0, 5),
    lowerBreakEven,
    upperBreakEven,
    netPremium,
  });

  return result;
}

export function createChartConfig(chartData, underlyingPrice) {
  const { prices, payoffs, lowerBreakEven, upperBreakEven } = chartData;

  console.log("Creating chart config with data:", {
    pricesLength: prices.length,
    payoffsLength: payoffs.length,
    firstFewPrices: prices.slice(0, 5),
    firstFewPayoffs: payoffs.slice(0, 5),
    lowerBreakEven,
    upperBreakEven,
    underlyingPrice,
  });

  // Create data points for the chart
  const chartPoints = prices.map((price, index) => ({
    x: price,
    y: payoffs[index],
  }));

  const zeroLinePoints = prices.map((price) => ({
    x: price,
    y: 0,
  }));

  console.log("Chart points:", chartPoints.slice(0, 3));

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
