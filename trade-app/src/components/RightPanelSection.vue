<template>
  <div class="right-panel-section">
    <div
      class="section-header"
      @click="toggleExpanded"
      :class="{ expanded: isExpanded }"
    >
      <div class="section-title">
        <i :class="icon" class="section-icon"></i>
        <span class="section-label">{{ title }}</span>
      </div>
      <i
        class="pi pi-chevron-up expand-chevron"
        :class="{ rotated: !isExpanded }"
      ></i>
    </div>

    <div
      class="section-content"
      :class="{ expanded: isExpanded }"
      ref="contentRef"
    >
      <div class="section-content-inner">
        <slot></slot>
      </div>
    </div>
  </div>
</template>

<script>
import { ref, watch, nextTick } from "vue";

export default {
  name: "RightPanelSection",
  props: {
    title: {
      type: String,
      required: true,
    },
    icon: {
      type: String,
      required: true,
    },
    defaultExpanded: {
      type: Boolean,
      default: false,
    },
  },
  emits: ["toggle"],
  setup(props, { emit }) {
    const isExpanded = ref(props.defaultExpanded);
    const contentRef = ref(null);

    const toggleExpanded = () => {
      isExpanded.value = !isExpanded.value;
      emit("toggle", { title: props.title, expanded: isExpanded.value });
    };

    // Handle smooth height transitions
    watch(isExpanded, async (newValue) => {
      if (contentRef.value) {
        if (newValue) {
          // Expanding
          contentRef.value.style.height = "auto";
        } else {
          // Collapsing
          const height = contentRef.value.scrollHeight;
          contentRef.value.style.height = height + "px";
          await nextTick();
          contentRef.value.style.height = "0px";
        }
      }
    });

    return {
      isExpanded,
      contentRef,
      toggleExpanded,
    };
  },
};
</script>

<style scoped>
.right-panel-section {
  border-bottom: 1px solid var(--border-secondary, #2a2d33);
}

.section-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 12px 16px;
  background-color: var(--bg-secondary, #141519);
  cursor: pointer;
  transition: background-color 0.2s ease;
  user-select: none;
}

.section-header:hover {
  background-color: var(--bg-tertiary, #1a1d23);
}

.section-header.expanded {
  background-color: var(--bg-tertiary, #1a1d23);
}

.section-title {
  display: flex;
  align-items: center;
  gap: 8px;
}

.section-icon {
  font-size: 14px;
  color: var(--text-secondary, #cccccc);
  width: 16px;
  text-align: center;
}

.section-label {
  font-size: 13px;
  font-weight: 500;
  color: var(--text-primary, #ffffff);
}

.expand-chevron {
  font-size: 12px;
  color: var(--text-tertiary, #888888);
  transition: transform 0.2s ease;
}

.expand-chevron.rotated {
  transform: rotate(180deg);
}

.section-content {
  height: 0;
  overflow: hidden;
  transition: height 0.3s ease;
  background-color: var(--bg-primary, #0b0d10);
}

.section-content.expanded {
  height: auto;
  overflow: visible;
}

.section-content-inner {
  padding: 0;
}
</style>
