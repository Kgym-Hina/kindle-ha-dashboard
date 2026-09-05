import { readSettings } from "./config.js";
import type { DashboardDocument, EntitySummary, KindleMessage, ZoneInfo } from "./types.js";

interface HAState {
  entity_id: string;
  state: string;
  attributes: Record<string, unknown>;
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
    const states = await this.getStates();
    return states
      .filter((state) => state.entity_id.startsWith("zone."))
      .map((state) => ({
        entity_id: state.entity_id,
        zone_id: state.entity_id.slice("zone.".length),
        name: stringValue(state.attributes.friendly_name) || state.entity_id.slice("zone.".length),
        state: state.state
      }))
      .sort((a, b) => a.name.localeCompare(b.name));
  }

  async listEntities(): Promise<EntitySummary[]> {
    const states = await this.getStates();
    return states
      .filter((state) => !state.entity_id.startsWith("sensor.kindle_dashboard_"))
      .map((state) => ({
        entity_id: state.entity_id,
        state: state.state,
        name: stringValue(state.attributes.friendly_name) || state.entity_id,
        domain: state.entity_id.split(".", 1)[0]
      }))
      .sort((a, b) => a.entity_id.localeCompare(b.entity_id));
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

function stringValue(value: unknown): string | undefined {
  return typeof value === "string" ? value : undefined;
}

function slug(value: string): string {
  return value.toLowerCase().trim().replace(/\s+/g, "-").replace(/[^a-z0-9_-]+/g, "-").replace(/^-+|-+$/g, "") || "unknown";
}
