import { chmod, mkdir, readFile, writeFile } from "node:fs/promises";
import { readFileSync } from "node:fs";
import path from "node:path";
import process from "node:process";

export interface StoredSettings {
  ha_url?: string;
  ha_token?: string;
}

interface AddonOptions {
  ha_url?: string;
  ha_token?: string;
  port?: number;
}

const dataDir = process.env.KINDLE_DATA_DIR || "/data";
const settingsPath = path.join(dataDir, "settings.json");
const optionsPath = process.env.KINDLE_OPTIONS_PATH || "/data/options.json";

export async function readSettings(): Promise<StoredSettings> {
  const [stored, options] = await Promise.all([readJson(settingsPath), readJson(optionsPath)]);
  const storedSettings = isObject(stored) ? stored as StoredSettings : {};
  const addonOptions = isObject(options) ? options as AddonOptions : {};
  return {
    ha_url: cleanString(storedSettings.ha_url || addonOptions.ha_url),
    ha_token: cleanString(storedSettings.ha_token || addonOptions.ha_token)
  };
}

export async function saveSettings(patch: StoredSettings): Promise<StoredSettings> {
  await mkdir(dataDir, { recursive: true });
  const current = await readSettings();
  const next: StoredSettings = {
    ha_url: cleanString(patch.ha_url ?? current.ha_url),
    ha_token: cleanString(patch.ha_token ?? current.ha_token)
  };
  await writeFile(settingsPath, `${JSON.stringify(next, null, 2)}\n`, "utf8");
  await chmod(settingsPath, 0o600);
  return next;
}

export function configuredPort(): number {
  const optionsPort = readOptionPort();
  const value = Number(process.env.PORT || optionsPort || 8099);
  return Number.isInteger(value) && value > 0 ? value : 8099;
}

function readOptionPort(): number | undefined {
  try {
    const options = JSON.parse(readFileSync(optionsPath, "utf8")) as AddonOptions;
    return typeof options.port === "number" && Number.isInteger(options.port) ? options.port : undefined;
  } catch {
    return undefined;
  }
}

async function readJson(filePath: string): Promise<unknown> {
  try {
    return JSON.parse(await readFile(filePath, "utf8"));
  } catch {
    return {};
  }
}

function cleanString(value: unknown): string | undefined {
  if (typeof value !== "string") return undefined;
  const trimmed = value.trim();
  return trimmed || undefined;
}

function isObject(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}
