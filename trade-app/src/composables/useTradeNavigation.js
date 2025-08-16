import { reactive, toRefs } from "vue";

const state = reactive({
  pendingOrder: null,
});

export function useTradeNavigation() {
  const setPendingOrder = (order) => {
    state.pendingOrder = order;
  };

  const clearPendingOrder = () => {
    state.pendingOrder = null;
  };

  return {
    ...toRefs(state),
    setPendingOrder,
    clearPendingOrder,
  };
}
