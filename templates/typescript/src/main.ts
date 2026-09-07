import { fetchStatus } from "./services/api";
import { renderStatusCard } from "./components/StatusCard";

async function init() {
  const app = document.getElementById("app");
  if (!app) return;

  const status = await fetchStatus();
  app.appendChild(renderStatusCard(status));
}

init();
