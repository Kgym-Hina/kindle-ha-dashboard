import { mkdir, readFile, rename, writeFile } from "node:fs/promises";
import path from "node:path";
import process from "node:process";
import type { DashboardDocument, DashboardMode } from "./types.js";
import { validateDocument } from "./validation.js";

const dataDir = process.env.KINDLE_DATA_DIR || "/data";
const documentsDir = path.join(dataDir, "documents");

export async function loadDocument(mode: DashboardMode, zoneId?: string | null): Promise<DashboardDocument | null> {
  try {
    return validateDocument(JSON.parse(await readFile(documentPath(mode, zoneId), "utf8")));
  } catch {
    return null;
  }
}

export async function saveDocument(document: DashboardDocument): Promise<void> {
  await mkdir(documentsDir, { recursive: true });
  const targetPath = documentPath(document.target.mode, document.target.zone_id);
  const temporaryPath = `${targetPath}.${process.pid}.tmp`;
  await writeFile(temporaryPath, `${JSON.stringify(document, null, 2)}\n`, "utf8");
  await rename(temporaryPath, targetPath);
}

export function documentKey(mode: DashboardMode, zoneId?: string | null): string {
  return mode === "portable" ? "portable" : `zone-${slug(zoneId || "unknown")}`;
}

function documentPath(mode: DashboardMode, zoneId?: string | null): string {
  return path.join(documentsDir, `${documentKey(mode, zoneId)}.json`);
}

function slug(value: string): string {
  return value.toLowerCase().trim().replace(/\s+/g, "-").replace(/[^a-z0-9_-]+/g, "-").replace(/^-+|-+$/g, "") || "unknown";
}
