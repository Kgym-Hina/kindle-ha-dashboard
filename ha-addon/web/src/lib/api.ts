import type { DashboardDocument, DashboardMode, EntitySummary, KindleMessage, RuntimeConfig, ServiceInfo, ZoneInfo } from "../types";

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const response = await fetch(apiEndpoint(path), {
    ...init,
    headers: { "Content-Type": "application/json", ...(init?.headers || {}) }
  });
  const body = await response.json().catch(() => ({}));
  if (!response.ok) throw new Error(typeof body.error === "string" ? body.error : `请求失败（${response.status}）`);
  return body as T;
}

function apiEndpoint(path: string): string {
  const currentPath = window.location.pathname;
  const basePath = currentPath.endsWith("/") ? currentPath : `${currentPath}/`;
  return `${basePath}${path.replace(/^\//, "")}`;
}

export async function getRuntimeConfig(): Promise<RuntimeConfig> {
  return request<RuntimeConfig>("/api/config");
}

export async function saveRuntimeConfig(payload: { ha_url: string; ha_token?: string }): Promise<void> {
  await request("/api/settings", { method: "POST", body: JSON.stringify(payload) });
}

export async function getZones(): Promise<ZoneInfo[]> {
  const body = await request<{ zones: ZoneInfo[] }>("/api/zones");
  return body.zones || [];
}

export async function getEntities(): Promise<EntitySummary[]> {
  const body = await request<{ entities: EntitySummary[] }>("/api/entities");
  return body.entities || [];
}

export async function getServices(): Promise<ServiceInfo[]> {
  const body = await request<{ services: ServiceInfo[] }>("/api/services");
  return body.services || [];
}

export async function getDashboard(mode: DashboardMode, zoneId?: string | null): Promise<DashboardDocument | null> {
  const query = new URLSearchParams({ mode });
  if (zoneId) query.set("zoneId", zoneId);
  const body = await request<{ document: DashboardDocument | null }>(`/api/dashboard?${query}`);
  return body.document;
}

export async function publishDashboard(document: DashboardDocument): Promise<{ document: DashboardDocument; entity_id: string }> {
  return request<{ document: DashboardDocument; entity_id: string }>("/api/dashboard/publish", { method: "POST", body: JSON.stringify(document) });
}

export async function sendMessage(message: KindleMessage): Promise<void> {
  await request("/api/message", { method: "POST", body: JSON.stringify(message) });
}
