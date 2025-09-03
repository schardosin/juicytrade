import { createApp } from "vue";
import PrimeVue from "primevue/config";
import App from "./App.vue";
import router from "./router";

// Smart Market Data Store
import smartMarketDataStore from "./services/smartMarketDataStore.js";

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

// Initialize and provide Smart Market Data Store globally
smartMarketDataStore.initialize();

// Configure all data sources immediately at app startup
configureDataSources();

app.provide("smartMarketDataStore", smartMarketDataStore);

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
