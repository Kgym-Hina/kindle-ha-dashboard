import path from "node:path";
import { fileURLToPath } from "node:url";
import express, { type NextFunction, type Request, type Response } from "express";
import { configuredPort, readSettings, saveSettings } from "./config.js";
import { HomeAssistantClient } from "./ha-client.js";
import { loadDocument, saveDocument } from "./storage.js";
import type { DashboardDocument, DashboardMode, KindleMessage } from "./types.js";
import { validateDocument } from "./validation.js";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const app = express();
const ha = new HomeAssistantClient();

app.disable("x-powered-by");
app.use(express.json({ limit: "12mb" }));

app.get("/api/health", async (_request, response) => {
  response.json({ ok: true, service: "kindle-ha-dashboard", home_assistant: await safeStatus() });
});

app.get("/api/config", async (_request, response) => {
  const settings = await readSettings();
  const status = await safeStatus();
  response.json({
    ha_url: settings.ha_url || status.baseUrl,
    ha_configured: status.configured,
    ha_reachable: status.reachable,
    auth_mode: status.authMode,
    token_configured: Boolean(settings.ha_token || process.env.HA_TOKEN || process.env.SUPERVISOR_TOKEN)
  });
});

app.post("/api/settings", async (request, response) => {
  try {
    const body = request.body as Record<string, unknown>;
    if (body.ha_url !== undefined && typeof body.ha_url !== "string") throw new Error("ha_url must be a string");
    if (body.ha_token !== undefined && typeof body.ha_token !== "string") throw new Error("ha_token must be a string");
    const settings = await saveSettings({ ha_url: body.ha_url as string | undefined, ha_token: body.ha_token as string | undefined });
    response.json({ ok: true, ha_url: settings.ha_url || (await safeStatus()).baseUrl, token_configured: Boolean(settings.ha_token || process.env.HA_TOKEN || process.env.SUPERVISOR_TOKEN) });
  } catch (error) {
    response.status(400).json({ error: errorMessage(error) });
  }
});

app.get("/api/zones", async (_request, response) => {
  try {
    response.json({ zones: await ha.listZones() });
  } catch (error) {
    response.status(502).json({ error: errorMessage(error), zones: [] });
  }
});

app.get("/api/entities", async (_request, response) => {
  try {
    response.json({ entities: await ha.listEntities() });
  } catch (error) {
    response.status(502).json({ error: errorMessage(error), entities: [] });
  }
});

app.get("/api/services", async (_request, response) => {
  try {
    response.json({ services: await ha.listServices() });
  } catch (error) {
    response.status(502).json({ error: errorMessage(error), services: [] });
  }
});

app.get("/api/dashboard", async (request, response) => {
  try {
    const mode = parseMode(request.query.mode);
    const zoneId = typeof request.query.zoneId === "string" ? request.query.zoneId : undefined;
    const document = await loadDocument(mode, zoneId);
    response.json({ document });
  } catch (error) {
    response.status(400).json({ error: errorMessage(error) });
  }
});

app.post("/api/dashboard/publish", async (request, response) => {
  try {
    const incoming = validateDocument(request.body);
    const previous = await loadDocument(incoming.target.mode, incoming.target.zone_id);
    const document: DashboardDocument = {
      ...incoming,
      revision: Math.max(incoming.revision, (previous?.revision || 0) + 1),
      updated_at: new Date().toISOString()
    };
    await saveDocument(document);
    const entityId = await ha.publish(document);
    response.json({ ok: true, document, entity_id: entityId });
  } catch (error) {
    response.status(400).json({ error: errorMessage(error) });
  }
});

app.post("/api/message", async (request, response) => {
  try {
    const message = validateMessage(request.body);
    await ha.sendMessage(message);
    response.json({ ok: true });
  } catch (error) {
    response.status(400).json({ error: errorMessage(error) });
  }
});

const publicDir = path.resolve(__dirname, "../public");
app.use(express.static(publicDir, { index: "index.html" }));
app.use((request, response, next) => {
  if (request.path.startsWith("/api/")) {
    next();
    return;
  }
  response.sendFile(path.join(publicDir, "index.html"));
});

app.use((error: unknown, _request: Request, response: Response, _next: NextFunction) => {
  response.status(500).json({ error: errorMessage(error) });
});

app.listen(configuredPort(), "0.0.0.0", () => {
  console.log(`Kindle HA Dashboard listening on ${configuredPort()}`);
});

async function safeStatus() {
  try {
    return await ha.status();
  } catch {
    return { configured: false, baseUrl: "", authMode: "missing" as const, reachable: false };
  }
}

function parseMode(value: unknown): DashboardMode {
  if (value === "zone" || value === "portable") return value;
  throw new Error("mode must be portable or zone");
}

function validateMessage(value: unknown): KindleMessage {
  if (!value || typeof value !== "object" || Array.isArray(value)) throw new Error("message must be an object");
  const body = value as Record<string, unknown>;
  if (typeof body.title !== "string" || !body.title.trim()) throw new Error("message.title is required");
  if (typeof body.message !== "string" || !body.message.trim()) throw new Error("message.message is required");
  if (body.target_device_id !== undefined && typeof body.target_device_id !== "string") throw new Error("target_device_id must be a string");
  if (body.timeout_ms !== undefined && (!Number.isInteger(body.timeout_ms) || Number(body.timeout_ms) < 0)) throw new Error("timeout_ms must be a non-negative integer");
  return body as unknown as KindleMessage;
}

function errorMessage(error: unknown): string {
  return error instanceof Error ? error.message : String(error);
}
