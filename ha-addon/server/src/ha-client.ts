import { readSettings } from "./config.js";
import type { DashboardDocument, EntitySummary, KindleMessage, ServiceInfo, ZoneInfo } from "./types.js";

interface HAState {
  entity_id: string;
  state: string;
  attributes: Record<string, unknown>;
}

interface HAArea {
  area_id: string;
  name: string;
}

interface HAWebSocketMessage {
  type?: string;
  id?: number;
  success?: boolean;
  result?: unknown;
  error?: unknown;
}

export class HomeAssistantClient {
  async status(): Promise<{ configured: boolean; baseUrl: string; authMode: "supervisor" | "token" | "missing"; reachable: boolean }> {
    const settings = await readSettings();
    const baseUrl = resolveBaseUrl(settings.ha_url);
    const token = resolveToken(settings.ha_token);
    const explicitToken = settings.ha_token || process.env.HA_TOKEN;
    const authMode = process.env.SUPERVISOR_TOKEN && !explicitToken ? "supervisor" : token ? "token" : "missing";
    if (!token) return { configured: false, baseUrl, authMode, reachable: false };
    try {
      await this.request("/api/", { method: "GET" });
      return { configured: true, baseUrl, authMode, reachable: true };
    } catch {
      return { configured: true, baseUrl, authMode, reachable: false };
    }
  }

  async listZones(): Promise<ZoneInfo[]> {
    const areas = await this.listAreas();
    return areas
      .map((area) => ({
        entity_id: `area.${area.area_id}`,
        zone_id: area.area_id,
        name: area.name || area.area_id,
        state: ""
      }))
      .sort((a, b) => a.name.localeCompare(b.name, "zh-CN"));
  }

  async listEntities(): Promise<EntitySummary[]> {
    const states = await this.getStates();
    return states
      .filter((state) => !state.entity_id.startsWith("sensor.kindle_dashboard_"))
      .map((state) => ({
        entity_id: state.entity_id,
        state: state.state,
        name: stringValue(state.attributes.friendly_name) || state.entity_id,
        domain: state.entity_id.split(".", 1)[0],
        attribute_names: Object.keys(state.attributes).filter((key) => key !== "friendly_name").sort()
      }))
      .sort((a, b) => a.entity_id.localeCompare(b.entity_id));
  }

  async listServices(): Promise<ServiceInfo[]> {
    const response = await this.request("/api/services", { method: "GET" });
    const raw = await response.json() as unknown;
    if (!Array.isArray(raw)) return [];
    const services: ServiceInfo[] = [];
    for (const domainEntry of raw) {
      if (!isObject(domainEntry) || typeof domainEntry.domain !== "string" || !isObject(domainEntry.services)) continue;
      for (const [service, description] of Object.entries(domainEntry.services)) {
        const detail = isObject(description) ? description : {};
        services.push({
          domain: domainEntry.domain,
          service,
          name: stringValue(detail.name),
          description: stringValue(detail.description),
          fields: isObject(detail.fields) ? detail.fields as ServiceInfo["fields"] : {}
        });
      }
    }
    return services.sort((a, b) => `${a.domain}.${a.service}`.localeCompare(`${b.domain}.${b.service}`));
  }

  async publish(document: DashboardDocument): Promise<string> {
    const entityId = document.target.mode === "portable"
      ? "sensor.kindle_dashboard_portable"
      : `sensor.kindle_dashboard_zone_${slug(document.target.zone_id || "unknown")}`;
    await this.request(`/api/states/${encodeURIComponent(entityId)}`, {
      method: "POST",
      body: JSON.stringify({
        state: String(document.revision),
        attributes: {
          friendly_name: document.target.mode === "portable" ? "Kindle Dashboard Portable" : `Kindle Dashboard ${document.target.zone_id}`,
          schema: document.schema,
          revision: document.revision,
          document
        }
      })
    });
    return entityId;
  }

  async sendMessage(message: KindleMessage): Promise<void> {
    await this.request("/api/events/kindle_dashboard_message", {
      method: "POST",
      body: JSON.stringify(message)
    });
  }

  private async getStates(): Promise<HAState[]> {
    const response = await this.request("/api/states", { method: "GET" });
    const states = await response.json() as unknown;
    return Array.isArray(states) ? states as HAState[] : [];
  }

  private async listAreas(): Promise<HAArea[]> {
    const settings = await readSettings();
    const baseUrl = resolveBaseUrl(settings.ha_url);
    const token = resolveToken(settings.ha_token);
    if (!token) throw new Error("Home Assistant token is not configured");
    const websocketUrl = toWebSocketUrl(`${baseUrl}/api/websocket`);
    const commandId = 1;
    return new Promise<HAArea[]>((resolve, reject) => {
      const socket = new WebSocket(websocketUrl);
      let settled = false;
      const timeout = setTimeout(() => finish(new Error("Home Assistant area request timed out")), 8000);
      const finish = (error?: Error, areas: HAArea[] = []) => {
        if (settled) return;
        settled = true;
        clearTimeout(timeout);
        try { socket.close(); } catch { /* socket may already be closed */ }
        if (error) reject(error);
        else resolve(areas);
      };
      const send = (message: Record<string, unknown>) => socket.send(JSON.stringify(message));
      socket.addEventListener("message", (event) => {
        let message: HAWebSocketMessage;
        try {
          message = JSON.parse(String(event.data)) as HAWebSocketMessage;
        } catch {
          finish(new Error("Home Assistant returned an invalid WebSocket message"));
          return;
        }
        if (message.type === "auth_required") {
          try {
            send({ type: "auth", access_token: token });
          } catch {
            finish(new Error("Home Assistant WebSocket authentication could not be sent"));
          }
          return;
        }
        if (message.type === "auth_invalid") {
          finish(new Error("Home Assistant WebSocket authentication failed"));
          return;
        }
        if (message.type === "auth_ok") {
          try {
            send({ id: commandId, type: "config/area_registry/list" });
          } catch {
            finish(new Error("Home Assistant area request could not be sent"));
          }
          return;
        }
        if (message.type !== "result" || message.id !== commandId) return;
        if (!message.success) {
          finish(new Error(`Home Assistant area request failed: ${JSON.stringify(message.error)}`));
          return;
        }
        const areas = Array.isArray(message.result) ? message.result.filter(isArea) : [];
        finish(undefined, areas);
      });
      socket.addEventListener("error", () => finish(new Error("Home Assistant WebSocket connection failed")));
      socket.addEventListener("close", () => {
        if (!settled) finish(new Error("Home Assistant WebSocket closed before returning areas"));
      });
      socket.addEventListener("open", () => {
        // Home Assistant sends auth_required after the connection opens.
      });
    });
  }

  private async request(path: string, init: RequestInit): Promise<Response> {
    const settings = await readSettings();
    const token = resolveToken(settings.ha_token);
    if (!token) throw new Error("Home Assistant token is not configured");
    const response = await fetch(`${resolveBaseUrl(settings.ha_url)}${path}`, {
      ...init,
      signal: init.signal || AbortSignal.timeout(8000),
      headers: {
        Authorization: `Bearer ${token}`,
        "Content-Type": "application/json",
        ...(init.headers || {})
      }
    });
    if (!response.ok) throw new Error(`Home Assistant ${response.status}: ${await response.text()}`);
    return response;
  }
}

function resolveBaseUrl(configuredUrl?: string): string {
  const value = configuredUrl?.trim() || process.env.HA_URL?.trim();
  if (value) return value.replace(/\/$/, "");
  if (process.env.SUPERVISOR_TOKEN) return "http://supervisor/core";
  return "http://homeassistant:8123";
}

function resolveToken(configuredToken?: string): string | undefined {
  return configuredToken?.trim() || process.env.HA_TOKEN?.trim() || process.env.SUPERVISOR_TOKEN?.trim() || undefined;
}

function toWebSocketUrl(value: string): string {
  return value.replace(/^http:/i, "ws:").replace(/^https:/i, "wss:");
}

function stringValue(value: unknown): string | undefined {
  return typeof value === "string" ? value : undefined;
}

function isObject(value: unknown): value is Record<string, any> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

function isArea(value: unknown): value is HAArea {
  return isObject(value) && typeof value.area_id === "string" && value.area_id.trim().length > 0 && typeof value.name === "string";
}

function slug(value: string): string {
  return value.toLowerCase().trim().replace(/\s+/g, "-").replace(/[^a-z0-9_-]+/g, "-").replace(/^-+|-+$/g, "") || "unknown";
}
