import type { DashboardDocument, DashboardElement, DashboardPage, DashboardTarget } from "../types";

export function createDocument(target: DashboardTarget): DashboardDocument {
  return {
    schema: "kindle-dashboard/v1",
    revision: 1,
    target,
    pages: [{ id: "home", name: "Home", background: target.background, elements: [] }]
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
    case "image_button": return { ...shared, type, text: "图片按钮", image: { src: "", fit: "contain" }, style: { ...shared.style, color: "#ffffff" }, action: { type: "show_message", title: "Kindle", message: "图片按钮已按下", timeout_ms: 6000 } };
    case "image": return { ...shared, type, image: { src: "", fit: "contain" }, frame: { x: 48, y: 72 + index * 18, width: 240, height: 160 } };
    case "rect": return { ...shared, type, frame: { x: 48, y: 72 + index * 18, width: 300, height: 120 }, style: { ...shared.style, fill: "#f0eee8", radius: 16 } };
    case "line": return { ...shared, type };
  }
  return { ...shared, type: "text", text: "新文本" };
}

export function patchElement(page: DashboardPage, elementId: string, patch: Partial<DashboardElement>): DashboardPage {
  return { ...page, elements: page.elements.map((element) => element.id === elementId ? { ...element, ...patch } : element) };
}

export function updateTarget(document: DashboardDocument, target: DashboardTarget): DashboardDocument {
  return { ...document, target, pages: document.pages.map((page, index) => index === 0 && !page.background ? { ...page, background: target.background } : page) };
}
