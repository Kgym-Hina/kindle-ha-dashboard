import type { DashboardDocument } from "./types.js";

const elementTypes = new Set(["text", "button", "image_button", "image", "rect", "line"]);
const actionTypes = new Set(["navigate_page", "call_service", "show_message"]);

export function validateDocument(input: unknown): DashboardDocument {
  if (!isObject(input)) throw new Error("document must be an object");
  if (input.schema !== "kindle-dashboard/v1") throw new Error("unsupported document schema");
  if (!Number.isInteger(input.revision) || Number(input.revision) < 1) throw new Error("revision must be a positive integer");
  if (!isObject(input.target)) throw new Error("target is required");
  const target = input.target;
  if (target.mode !== "portable" && target.mode !== "zone") throw new Error("target.mode must be portable or zone");
  if (target.mode === "zone" && (typeof target.zone_id !== "string" || !target.zone_id.trim())) {
    throw new Error("zone documents require target.zone_id");
  }
  if (!Number.isInteger(target.width) || Number(target.width) < 1 || !Number.isInteger(target.height) || Number(target.height) < 1) {
    throw new Error("target dimensions must be positive integers");
  }
  if (typeof target.background !== "string") throw new Error("target.background is required");
  if (!Array.isArray(input.pages) || input.pages.length === 0) throw new Error("at least one page is required");
  for (const [pageIndex, page] of input.pages.entries()) {
    if (!isObject(page) || !nonEmptyString(page.id) || !nonEmptyString(page.name)) throw new Error(`invalid page at index ${pageIndex}`);
    if (!Array.isArray(page.elements)) throw new Error(`page ${page.id} elements must be an array`);
    for (const [elementIndex, element] of page.elements.entries()) validateElement(element, `page ${page.id} element ${elementIndex}`);
  }
  const encodedSize = Buffer.byteLength(JSON.stringify(input), "utf8");
  if (encodedSize > 9 * 1024 * 1024) throw new Error("document is larger than 9 MiB");
  return input as unknown as DashboardDocument;
}

function validateElement(value: unknown, label: string): void {
  if (!isObject(value) || !nonEmptyString(value.id) || typeof value.type !== "string" || !elementTypes.has(value.type)) {
    throw new Error(`${label} has an invalid id or type`);
  }
  if (!isObject(value.frame)) throw new Error(`${label} frame is required`);
  for (const key of ["x", "y", "width", "height"]) {
    if (typeof value.frame[key] !== "number" || !Number.isFinite(value.frame[key])) throw new Error(`${label} frame.${key} must be a number`);
  }
  if (value.frame.width <= 0 || value.frame.height <= 0) throw new Error(`${label} frame dimensions must be positive`);
  if (value.type === "image" || value.type === "image_button") {
    if (!isObject(value.image) || !nonEmptyString(value.image.src)) throw new Error(`${label} image.src is required`);
  }
  if (value.action !== undefined) {
    if (!isObject(value.action) || typeof value.action.type !== "string" || !actionTypes.has(value.action.type)) throw new Error(`${label} action is invalid`);
    if (value.action.type === "call_service" && (!nonEmptyString(value.action.domain) || !nonEmptyString(value.action.service))) throw new Error(`${label} call_service requires domain and service`);
    if (value.action.service_data !== undefined && !isObject(value.action.service_data)) throw new Error(`${label} action.service_data must be an object`);
    if (value.action.type === "navigate_page" && !nonEmptyString(value.action.page_id)) throw new Error(`${label} navigate_page requires page_id`);
    if (value.action.type === "show_message" && !nonEmptyString(value.action.message)) throw new Error(`${label} show_message requires message`);
  }
}

function isObject(value: unknown): value is Record<string, any> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

function nonEmptyString(value: unknown): value is string {
  return typeof value === "string" && value.trim().length > 0;
}
