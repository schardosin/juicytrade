<template>
  <div class="options-chain">
    <!-- Header -->
    <div class="chain-header">
      <div class="calls-header">
        <div class="header-cell">Delta</div>
        <div class="header-cell">Theta</div>
        <div class="header-cell">Bid</div>
        <div class="header-cell">Ask</div>
      </div>
      <div class="strike-header">Strike</div>
      <div class="puts-header">
        <div class="header-cell">Bid</div>
        <div class="header-cell">Ask</div>
        <div class="header-cell">Theta</div>
        <div class="header-cell">Delta</div>
      </div>
    </div>

    <!-- Options Rows -->
    <div class="chain-body">
      <div
        v-for="strike in strikeList"
        :key="strike"
        class="option-row"
        :class="{ 'at-the-money': isAtTheMoney(strike) }"
      >
        <!-- Call Side -->
        <div class="call-side">
          <div
            v-if="getCallOption(strike)"
            class="option-data"
            :class="getCallSelectionClass(strike)"
            @click="selectCallOption(strike)"
          >
            <div class="greek-cell">{{ getCallDelta(strike) }}</div>
            <div class="greek-cell">{{ getCallTheta(strike) }}</div>
            <div
              class="price-cell bid"
              @click.stop="selectCallOption(strike, 'sell')"
            >
              {{ formatPrice(getCallBid(strike)) }}
            </div>
            <div
              class="price-cell ask"
              @click.stop="selectCallOption(strike, 'buy')"
            >
              {{ formatPrice(getCallAsk(strike)) }}
            </div>
          </div>
          <div v-else class="option-data empty">
            <div class="greek-cell">-</div>
            <div class="greek-cell">-</div>
            <div class="price-cell">-</div>
            <div class="price-cell">-</div>
          </div>
        </div>

        <!-- Strike Price -->
        <div class="strike-cell">
          <span class="strike-price">${{ strike.toFixed(0) }}</span>
        </div>

        <!-- Put Side -->
        <div class="put-side">
          <div
            v-if="getPutOption(strike)"
            class="option-data"
            :class="getPutSelectionClass(strike)"
            @click="selectPutOption(strike)"
          >
            <div
              class="price-cell bid"
              @click.stop="selectPutOption(strike, 'sell')"
            >
              {{ formatPrice(getPutBid(strike)) }}
            </div>
            <div
              class="price-cell ask"
              @click.stop="selectPutOption(strike, 'buy')"
            >
              {{ formatPrice(getPutAsk(strike)) }}
            </div>
            <div class="greek-cell">{{ getPutTheta(strike) }}</div>
            <div class="greek-cell">{{ getPutDelta(strike) }}</div>
          </div>
          <div v-else class="option-data empty">
            <div class="price-cell">-</div>
            <div class="price-cell">-</div>
            <div class="greek-cell">-</div>
            <div class="greek-cell">-</div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script>
import { computed, toRefs } from "vue";

export default {
  name: "OptionsChain",
  props: {
    optionsData: {
      type: Array,
      default: () => [],
    },
    underlyingPrice: {
      type: Number,
      default: null,
    },
    selectedOptions: {
      type: Array,
      default: () => [],
    },
  },
  emits: ["option-selected", "option-deselected"],
  setup(props, { emit }) {
    const { optionsData, underlyingPrice, selectedOptions } = toRefs(props);

    // Computed properties
    const strikeList = computed(() => {
      if (!optionsData.value.length) return [];

      // Get unique strikes and sort them
      const strikes = [
        ...new Set(optionsData.value.map((opt) => opt.strike_price)),
      ];
      strikes.sort((a, b) => a - b);

      // Return all strikes - no filtering
      return strikes;
    });

    // Methods
    const getCallOption = (strike) => {
      return optionsData.value.find(
        (opt) =>
          opt.type === "call" && Math.abs(opt.strike_price - strike) < 0.01
      );
    };

    const getPutOption = (strike) => {
      return optionsData.value.find(
        (opt) =>
          opt.type === "put" && Math.abs(opt.strike_price - strike) < 0.01
      );
    };

    const getCallBid = (strike) => {
      const option = getCallOption(strike);
      return option ? option.bid : 0;
    };

    const getCallAsk = (strike) => {
      const option = getCallOption(strike);
      return option ? option.ask : 0;
    };

    const getPutBid = (strike) => {
      const option = getPutOption(strike);
      return option ? option.bid : 0;
    };

    const getPutAsk = (strike) => {
      const option = getPutOption(strike);
      return option ? option.ask : 0;
    };

    const getCallDelta = (strike) => {
      const option = getCallOption(strike);
      return option && option.delta ? option.delta.toFixed(2) : "-";
    };

    const getCallTheta = (strike) => {
      const option = getCallOption(strike);
      return option && option.theta ? option.theta.toFixed(2) : "-";
    };

    const getPutDelta = (strike) => {
      const option = getPutOption(strike);
      return option && option.delta ? option.delta.toFixed(2) : "-";
    };

    const getPutTheta = (strike) => {
      const option = getPutOption(strike);
      return option && option.theta ? option.theta.toFixed(2) : "-";
    };

    const formatPrice = (price) => {
      if (!price || price === 0) return "-";
      return price.toFixed(2);
    };

    const isAtTheMoney = (strike) => {
      if (!underlyingPrice.value) return false;
      return Math.abs(strike - underlyingPrice.value) < 2.5;
    };

    const isSelected = (symbol) => {
      return selectedOptions.value.some((sel) => sel.symbol === symbol);
    };

    const getSelectionType = (symbol) => {
      const selection = selectedOptions.value.find(
        (sel) => sel.symbol === symbol
      );
      return selection ? selection.side : null;
    };

    const getCallSelectionClass = (strike) => {
      const option = getCallOption(strike);
      if (!option) return "";

      const selectionType = getSelectionType(option.symbol);
      return {
        "selected-buy": selectionType === "buy",
        "selected-sell": selectionType === "sell",
      };
    };

    const getPutSelectionClass = (strike) => {
      const option = getPutOption(strike);
      if (!option) return "";

      const selectionType = getSelectionType(option.symbol);
      return {
        "selected-buy": selectionType === "buy",
        "selected-sell": selectionType === "sell",
      };
    };

    const selectCallOption = (strike, side = "buy") => {
      const option = getCallOption(strike);
      if (!option) return;

      const existingSelection = selectedOptions.value.find(
        (sel) => sel.symbol === option.symbol
      );

      if (existingSelection && existingSelection.side === side) {
        // Deselect if clicking the same side
        emit("option-deselected", option.symbol);
      } else {
        // Select or change side
        emit("option-selected", {
          ...option,
          side: side,
          quantity: 1,
        });
      }
    };

    const selectPutOption = (strike, side = "buy") => {
      const option = getPutOption(strike);
      if (!option) return;

      const existingSelection = selectedOptions.value.find(
        (sel) => sel.symbol === option.symbol
      );

      if (existingSelection && existingSelection.side === side) {
        // Deselect if clicking the same side
        emit("option-deselected", option.symbol);
      } else {
        // Select or change side
        emit("option-selected", {
          ...option,
          side: side,
          quantity: 1,
        });
      }
    };

    return {
      // Computed
      strikeList,

      // Methods
      getCallOption,
      getPutOption,
      getCallBid,
      getCallAsk,
      getPutBid,
      getPutAsk,
      getCallDelta,
      getCallTheta,
      getPutDelta,
      getPutTheta,
      formatPrice,
      isAtTheMoney,
      isSelected,
      getSelectionType,
      getCallSelectionClass,
      getPutSelectionClass,
      selectCallOption,
      selectPutOption,
    };
  },
};
</script>

<style scoped>
.options-chain {
  display: flex;
  flex-direction: column;
  height: 100%;
  background-color: var(--options-grid-bg);
  border-radius: var(--radius-lg);
  overflow: hidden;
}

.chain-header {
  display: grid;
  grid-template-columns: 1fr 100px 1fr;
  background-color: var(--options-grid-bg);
  border-bottom: 2px solid var(--border-primary);
  font-weight: var(--font-weight-semibold);
  font-size: var(--font-size-base);
  color: var(--text-secondary);
  overflow-y: scroll;
}

.calls-header,
.puts-header {
  display: grid;
  grid-template-columns: 1fr 1fr 1fr 1fr;
  gap: var(--spacing-sm);
  padding: var(--spacing-md) var(--spacing-lg);
}

.puts-header {
  text-align: right;
}

.strike-header {
  display: flex;
  align-items: center;
  justify-content: center;
  padding: var(--spacing-md) var(--spacing-lg);
  background-color: var(--options-strike-bg);
  font-weight: var(--font-weight-bold);
  color: var(--text-primary);
  border-left: 1px solid var(--border-secondary);
  border-right: 1px solid var(--border-secondary);
}

.header-cell {
  text-align: center;
  font-size: var(--font-size-sm);
  font-weight: var(--font-weight-semibold);
  color: var(--text-tertiary);
}

.chain-body {
  flex: 1;
  overflow-y: auto;
}

.option-row {
  display: grid;
  grid-template-columns: 1fr 100px 1fr;
  border-bottom: 1px solid var(--border-primary);
  transition: var(--transition-normal);
}

.option-row:hover {
  background-color: var(--options-row-hover);
}

.option-row.at-the-money {
  background-color: var(--options-atm-bg);
  border-top: 1px solid var(--options-atm-border);
  border-bottom: 1px solid var(--options-atm-border);
}

.call-side,
.put-side {
  display: flex;
  align-items: center;
}

.option-data {
  display: grid;
  grid-template-columns: 1fr 1fr 1fr 1fr;
  gap: var(--spacing-sm);
  padding: var(--spacing-md) var(--spacing-lg);
  width: 100%;
  cursor: pointer;
  transition: var(--transition-normal);
}

.option-data:hover:not(.empty) {
  background-color: var(--bg-quaternary);
}

.option-data.selected-buy {
  background-color: var(--options-selected-buy);
  border-left: 3px solid var(--color-success);
}

.option-data.selected-sell {
  background-color: var(--options-selected-sell);
  border-left: 3px solid var(--color-danger);
}

.option-data.empty {
  cursor: default;
  color: var(--text-quaternary);
}

.greek-cell,
.price-cell {
  font-size: var(--font-size-base);
  text-align: center;
  padding: var(--spacing-xs);
  border-radius: var(--radius-sm);
  transition: var(--transition-normal);
}

.greek-cell {
  color: var(--text-secondary);
  font-weight: var(--font-weight-medium);
}

.price-cell {
  color: var(--text-primary);
  font-weight: var(--font-weight-semibold);
  cursor: pointer;
}

.price-cell:hover {
  background-color: rgba(255, 255, 255, 0.1);
}

.price-cell.bid:hover {
  background-color: rgba(255, 68, 68, 0.3);
}

.price-cell.ask:hover {
  background-color: rgba(0, 200, 81, 0.3);
}

.strike-cell {
  display: flex;
  align-items: center;
  justify-content: center;
  padding: var(--spacing-md) var(--spacing-lg);
  background-color: var(--options-strike-bg);
  border-left: 1px solid var(--border-secondary);
  border-right: 1px solid var(--border-secondary);
}

.strike-price {
  font-size: var(--font-size-md);
  font-weight: var(--font-weight-bold);
  color: var(--text-primary);
}

/* Scrollbar styling */
.chain-body::-webkit-scrollbar {
  width: 6px;
}

.chain-body::-webkit-scrollbar-track {
  background: var(--bg-secondary);
}

.chain-body::-webkit-scrollbar-thumb {
  background: var(--border-secondary);
  border-radius: var(--radius-sm);
}

.chain-body::-webkit-scrollbar-thumb:hover {
  background: var(--border-tertiary);
}

/* Hide header scrollbar but keep the space */
.chain-header::-webkit-scrollbar {
  width: 6px;
}

.chain-header::-webkit-scrollbar-track {
  background: transparent;
}

.chain-header::-webkit-scrollbar-thumb {
  background: transparent;
}

/* Responsive adjustments */
@media (max-width: 1200px) {
  .chain-header {
    font-size: var(--font-size-sm);
  }

  .calls-header,
  .puts-header {
    padding: var(--spacing-sm) var(--spacing-md);
    gap: var(--spacing-xs);
  }

  .option-data {
    padding: 6px var(--spacing-md);
    gap: var(--spacing-xs);
  }

  .greek-cell,
  .price-cell {
    font-size: var(--font-size-sm);
    padding: 2px;
  }

  .strike-price {
    font-size: var(--font-size-base);
  }
}
</style>
