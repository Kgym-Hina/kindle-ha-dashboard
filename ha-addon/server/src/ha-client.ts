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

interface HAEntityRegistryEntry {
  entity_id: string;
  area_id?: string | null;
  device_id?: string | null;
}

interface HADeviceRegistryEntry {
  id: string;
  area_id?: string | null;
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
    const [states, registryEntries, deviceEntries, areas] = await Promise.all([
      this.getStates(),
      this.listEntityRegistry().catch(() => []),
      this.listDeviceRegistry().catch(() => []),
      this.listAreas().catch(() => [])
    ]);
    const registryByEntity = new Map(registryEntries.map((entry) => [entry.entity_id, entry]));
    const devicesById = new Map(deviceEntries.map((entry) => [entry.id, entry]));
    const areaNames = new Map(areas.map((area) => [area.area_id, area.name]));
    return states
      .filter((state) => !state.entity_id.startsWith("sensor.kindle_dashboard_"))
      .map((state) => {
        const registryEntry = registryByEntity.get(state.entity_id);
        const deviceAreaId = registryEntry?.device_id ? devicesById.get(registryEntry.device_id)?.area_id : null;
        const areaId = registryEntry?.area_id || deviceAreaId || null;
        return {
          entity_id: state.entity_id,
          state: state.state,
          name: stringValue(state.attributes.friendly_name) || state.entity_id,
          domain: state.entity_id.split(".", 1)[0],
          attribute_names: Object.keys(state.attributes).filter((key) => key !== "friendly_name").sort(),
          current_temperature: scalarValue(state.attributes.current_temperature),
          temperature: scalarValue(state.attributes.temperature),
          hvac_mode: stringValue(state.attributes.hvac_mode),
          hvac_modes: stringArray(state.attributes.hvac_modes),
          area_id: areaId,
          area_name: areaId ? areaNames.get(areaId) || null : null
        };
      })
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
    return this.listWebSocketCollection("config/area_registry/list", isArea, "area");
  }

  private async listEntityRegistry(): Promise<HAEntityRegistryEntry[]> {
    return this.listWebSocketCollection("config/entity_registry/list", isEntityRegistryEntry, "entity");
  }

  private async listDeviceRegistry(): Promise<HADeviceRegistryEntry[]> {
    return this.listWebSocketCollection("config/device_registry/list", isDeviceRegistryEntry, "device");
  }

  private async listWebSocketCollection<T>(commandType: string, isItem: (value: unknown) => value is T, label: string): Promise<T[]> {
    const settings = await readSettings();
    const baseUrl = resolveBaseUrl(settings.ha_url);
    const token = resolveToken(settings.ha_token);
    if (!token) throw new Error("Home Assistant token is not configured");
    const websocketUrl = toWebSocketUrl(`${baseUrl}/api/websocket`);
    const commandId = 1;
    return new Promise<T[]>((resolve, reject) => {
      const socket = new WebSocket(websocketUrl);
      let settled = false;
      const finish = (error?: Error, items: T[] = []) => {
        if (settled) return;
        settled = true;
        clearTimeout(timeout);
        try { socket.close(); } catch { /* socket may already be closed */ }
        if (error) reject(error);
        else resolve(items);
      };
      const timeout = setTimeout(() => finish(new Error("Home Assistant " + label + " request timed out")), 8000);
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
            send({ id: commandId, type: commandType });
          } catch {
            finish(new Error("Home Assistant " + label + " request could not be sent"));
          }
          return;
        }
        if (message.type !== "result" || message.id !== commandId) return;
        if (!message.success) {
          finish(new Error("Home Assistant " + label + " request failed: " + JSON.stringify(message.error)));
          return;
        }
        const items = Array.isArray(message.result) ? message.result.filter(isItem) : [];
        finish(undefined, items);
      });
      socket.addEventListener("error", () => finish(new Error("Home Assistant WebSocket connection failed")));
      socket.addEventListener("close", () => {
        if (!settled) finish(new Error("Home Assistant " + label + " WebSocket closed before returning data"));
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

function scalarValue(value: unknown): number | string | undefined {
  if (typeof value === "number" && Number.isFinite(value)) return value;
  if (typeof value === "string") return value;
  return undefined;
}

function stringArray(value: unknown): string[] | undefined {
  if (!Array.isArray(value)) return undefined;
  const items = value.filter((item): item is string => typeof item === "string" && item.trim().length > 0).map((item) => item.trim());
  return items.length > 0 ? items : undefined;
}

function isObject(value: unknown): value is Record<string, any> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

function isArea(value: unknown): value is HAArea {
  return isObject(value) && typeof value.area_id === "string" && value.area_id.trim().length > 0 && typeof value.name === "string";
}

function isEntityRegistryEntry(value: unknown): value is HAEntityRegistryEntry {
  return isObject(value)
    && typeof value.entity_id === "string"
    && (value.area_id === undefined || value.area_id === null || typeof value.area_id === "string")
    && (value.device_id === undefined || value.device_id === null || typeof value.device_id === "string");
}

function isDeviceRegistryEntry(value: unknown): value is HADeviceRegistryEntry {
  return isObject(value)
    && typeof value.id === "string"
    && (value.area_id === undefined || value.area_id === null || typeof value.area_id === "string");
}

function slug(value: string): string {
  return value.toLowerCase().trim().replace(/\s+/g, "-").replace(/[^a-z0-9_-]+/g, "-").replace(/^-+|-+$/g, "") || "unknown";
}
