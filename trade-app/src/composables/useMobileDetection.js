import { ref, computed, onMounted, onUnmounted } from 'vue';

export function useMobileDetection() {
  const windowWidth = ref(window.innerWidth);
  const windowHeight = ref(window.innerHeight);

  // Breakpoints
  const MOBILE_MAX = 768;
  const TABLET_MAX = 1024;

  // Reactive breakpoint states
  const isMobile = computed(() => windowWidth.value <= MOBILE_MAX);
  const isTablet = computed(() => windowWidth.value > MOBILE_MAX && windowWidth.value <= TABLET_MAX);
  const isDesktop = computed(() => windowWidth.value > TABLET_MAX);
  
  // Device orientation
  const isPortrait = computed(() => windowHeight.value > windowWidth.value);
  const isLandscape = computed(() => windowWidth.value > windowHeight.value);

  // Touch device detection
  const isTouchDevice = computed(() => {
    return 'ontouchstart' in window || navigator.maxTouchPoints > 0;
  });

  // Update dimensions on resize
  const updateDimensions = () => {
    windowWidth.value = window.innerWidth;
    windowHeight.value = window.innerHeight;
  };

  onMounted(() => {
    window.addEventListener('resize', updateDimensions);
    window.addEventListener('orientationchange', updateDimensions);
  });

  onUnmounted(() => {
    window.removeEventListener('resize', updateDimensions);
    window.removeEventListener('orientationchange', updateDimensions);
  });

  return {
    windowWidth,
    windowHeight,
    isMobile,
    isTablet,
    isDesktop,
    isPortrait,
    isLandscape,
    isTouchDevice,
    breakpoints: {
      mobile: MOBILE_MAX,
      tablet: TABLET_MAX
    }
  };
}
