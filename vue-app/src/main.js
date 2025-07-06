import { createApp } from "vue";
import PrimeVue from "primevue/config";
import App from "./App.vue";
import router from "./router";

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

// PrimeVue styles
import "primevue/resources/themes/aura-light-purple/theme.css";
import "primevue/resources/primevue.min.css";
import "primeicons/primeicons.css";

const app = createApp(App);

app.use(PrimeVue);
app.use(router);

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

app.mount("#app");
