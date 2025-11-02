import { createApp } from "vue";
import PrimeVue from "primevue/config";
import App from "./App.vue";
import router from "./router";

// Smart Market Data Store
import smartMarketDataStore from "./services/smartMarketDataStore.js";

// Authentication Service
import authService from "./services/authService.js";

// PrimeVue components
import Button from "primevue/button";
import InputText from "primevue/inputtext";
import InputNumber from "primevue/inputnumber";
import Dropdown from "primevue/dropdown";
import Calendar from "primevue/calendar";
import DataTable from "primevue/datatable";
import Column from "primevue/column";
import Card from "primevue/card";
import Message from "primevue/message";
import Dialog from "primevue/dialog";
import ProgressSpinner from "primevue/progressspinner";
import Divider from "primevue/divider";
import Menubar from "primevue/menubar";
import Tag from "primevue/tag";
import Menu from "primevue/menu";
import Checkbox from "primevue/checkbox";
import TabView from "primevue/tabview";
import TabPanel from "primevue/tabpanel";
import Badge from "primevue/badge";

// PrimeVue styles
import "primevue/resources/themes/aura-dark-noir/theme.css";
import "primevue/resources/primevue.min.css";
import "primeicons/primeicons.css";

// Custom theme
import "./assets/styles/theme.css";

const app = createApp(App);

app.use(PrimeVue);
app.use(router);

// Global OAuth token detection (for development proxy issues)
function handleOAuthTokenFromURL() {
  const urlParams = new URLSearchParams(window.location.search);
  const authToken = urlParams.get('auth_token');
  
  if (authToken) {
    // Set the token as a cookie manually
    document.cookie = `juicytrade_session=${authToken}; path=/; max-age=86400; samesite=lax`;
    
    // Remove token from URL
    urlParams.delete('auth_token');
    const newUrl = window.location.pathname + (urlParams.toString() ? '?' + urlParams.toString() : '');
    window.history.replaceState({}, '', newUrl);
    
    return true; // Token was found and processed
  }
  
  return false; // No token found
}

// Handle OAuth token before initializing auth service
const tokenProcessed = handleOAuthTokenFromURL();

// Initialize Authentication Service first, then conditionally initialize store
authService.init().then(() => {
  console.log('Authentication service initialized');
  
  // SmartMarketDataStore will initialize itself based on auth state
  // No need to call initialize() here - it handles auth-aware initialization internally
}).catch(error => {
  console.error('Failed to initialize authentication service:', error);
});

// Note: configureDataSources() is now called from within SmartMarketDataStore.startServices()
// when the user is authenticated, not immediately at app startup

app.provide("smartMarketDataStore", smartMarketDataStore);
app.provide("authService", authService);

/**
 * Configure all data sources with their strategies
 * Called at app startup to ensure data is available on any route
 */
function configureDataSources() {
  // Auto-updating data (Periodic strategy)
  smartMarketDataStore.registerDataSource("balance", {
    strategy: "periodic",
    method: "getAccount",
    interval: 60000, // 1 minute
  });

  // Static data (One-time strategy)
  smartMarketDataStore.registerDataSource("accountInfo", {
    strategy: "once",
    method: "getAccount",
  });

  // On-demand data (Cached strategy)
  smartMarketDataStore.registerDataSource("optionsChain.*", {
    strategy: "on-demand",
    method: "getOptionsChain",
    ttl: 300000, // 5 minutes
  });

  smartMarketDataStore.registerDataSource("historicalData.*", {
    strategy: "on-demand",
    method: "getHistoricalData",
    ttl: 300000, // 5 minutes
  });

  smartMarketDataStore.registerDataSource("symbolLookup.*", {
    strategy: "on-demand",
    method: "lookupSymbols",
    ttl: 600000, // 10 minutes
  });

  smartMarketDataStore.registerDataSource("expirationDates.*", {
    strategy: "on-demand",
    method: "getAvailableExpirations",
    ttl: 3600000, // 1 hour
  });

}

// Register components globally
app.component("Button", Button);
app.component("InputText", InputText);
app.component("InputNumber", InputNumber);
app.component("Dropdown", Dropdown);
app.component("Calendar", Calendar);
app.component("DataTable", DataTable);
app.component("Column", Column);
app.component("Card", Card);
app.component("Message", Message);
app.component("Dialog", Dialog);
app.component("ProgressSpinner", ProgressSpinner);
app.component("Divider", Divider);
app.component("Menubar", Menubar);
app.component("Tag", Tag);
app.component("Menu", Menu);
app.component("Checkbox", Checkbox);
app.component("TabView", TabView);
app.component("TabPanel", TabPanel);
app.component("Badge", Badge);

app.mount("#app");
