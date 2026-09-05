import type { DashboardDocument, DashboardElement, DashboardPage, DashboardTarget } from "../types";

export function createDocument(target: DashboardTarget): DashboardDocument {
  return {
    schema: "kindle-dashboard/v1",
    revision: 1,
    target,
    pages: [{ id: "home", name: "Home", parent_id: null, background: target.background, elements: [] }]
  };
}

export function createElement(type: DashboardElement["type"], index: number): DashboardElement {
  const shared = {
    id: `${type}-${Date.now()}-${index}`,
    frame: { x: 48, y: 72 + index * 18, width: type === "line" ? 504 : 240, height: type === "line" ? 3 : 72 },
    style: { color: "#171717", fill: type === "button" || type === "image_button" ? "#171717" : "#ffffff", stroke: "#171717", border_width: type === "line" ? 3 : 2, radius: type === "button" || type === "image_button" ? 14 : 0, font_size: 22, font_weight: "normal" as const, align: "center" as const }
  };
  switch (type) {
    case "text": return { ...shared, type, text: "新文本" };
    case "button": return { ...shared, type, text: "新按钮", style: { ...shared.style, color: "#ffffff" }, action: { type: "show_message", title: "Kindle", message: "按钮已按下", timeout_ms: 6000 } };
    case "image_button": return { ...shared, type, text: "", image: { src: "", fit: "contain" }, style: { ...shared.style, color: "transparent", fill: "transparent", stroke: "transparent", border_width: 0, radius: 0 }, action: { type: "show_message", title: "Kindle", message: "图片按钮已按下", timeout_ms: 6000 } };
    case "image": return { ...shared, type, image: { src: "", fit: "contain" }, frame: { x: 48, y: 72 + index * 18, width: 240, height: 160 } };
    case "rect": return { ...shared, type, frame: { x: 48, y: 72 + index * 18, width: 300, height: 120 }, style: { ...shared.style, fill: "#f0eee8", radius: 16 } };
    case "line": return { ...shared, type };
    case "switch": return { ...shared, type, frame: { x: 48, y: 72 + index * 18, width: 300, height: 108 }, style: { ...shared.style, fill: "#f0f4ef", stroke: "#79927e", radius: 14, font_size: 18 }, text: "开关" };
    case "climate": return { ...shared, type, frame: { x: 48, y: 72 + index * 18, width: 480, height: 246 }, style: { ...shared.style, fill: "#f0f4ef", stroke: "#79927e", radius: 16, font_size: 18 }, text: "温控", climate: { temperature_step: 0.5 } };
  }
  return { ...shared, type: "text", text: "新文本" };
}

export function patchElement(page: DashboardPage, elementId: string, patch: Partial<DashboardElement>): DashboardPage {
  return { ...page, elements: page.elements.map((element) => element.id === elementId ? { ...element, ...patch } : element) };
}

export function updateTarget(document: DashboardDocument, target: DashboardTarget): DashboardDocument {
  return { ...document, target, pages: document.pages.map((page, index) => index === 0 && !page.background ? { ...page, background: target.background } : page) };
}
