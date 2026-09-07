import { ServiceStatus } from "../services/api";

export function renderStatusCard(status: ServiceStatus): HTMLElement {
  const card = document.createElement("div");
  card.className = "status-card";
  card.innerHTML = `
    <h2>${status.service}</h2>
    <p>Status: <strong>${status.status}</strong></p>
    <small>Updated: ${status.timestamp}</small>
  `;
  return card;
}
