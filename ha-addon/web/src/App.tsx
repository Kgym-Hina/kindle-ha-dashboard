import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { AlignmentToolbar, type Alignment } from "./components/AlignmentToolbar";
import { EditorCanvas } from "./components/EditorCanvas";
import { InspectorPanel } from "./components/InspectorPanel";
import { LayerMenu } from "./components/LayerMenu";
import { MessageComposer } from "./components/MessageComposer";
import { PageTabs } from "./components/PageTabs";
import { Palette } from "./components/Palette";
import { SettingsModal } from "./components/SettingsModal";
import { TargetBar } from "./components/TargetBar";
import { TopBar } from "./components/TopBar";
import { getDashboard, getEntities, getRuntimeConfig, getServices, getZones, publishDashboard, saveRuntimeConfig, sendMessage } from "./lib/api";
import { cloneElement, createDocument, createElement, patchElement } from "./lib/document";
import type { DashboardDocument, DashboardElement, DashboardFrame, DashboardMode, DashboardTarget, EntitySummary, KindleMessage, RuntimeConfig, ServiceInfo, ZoneInfo } from "./types";
import { canvasHeight, canvasWidth } from "./types";

const defaultTarget: DashboardTarget = { mode: "portable", zone_id: null, width: canvasWidth, height: canvasHeight, background: "#ffffff" };

export default function App() {
  const [document, setDocument] = useState<DashboardDocument>(() => createDocument(defaultTarget));
  const [activePage, setActivePage] = useState(0);
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const [mode, setMode] = useState<DashboardMode>("portable");
  const [zoneId, setZoneId] = useState<string | null>(null);
  const [zones, setZones] = useState<ZoneInfo[]>([]);
  const [entities, setEntities] = useState<EntitySummary[]>([]);
  const [services, setServices] = useState<ServiceInfo[]>([]);
  const [runtime, setRuntime] = useState<RuntimeConfig | null>(null);
  const [busy, setBusy] = useState(false);
  const [loading, setLoading] = useState(true);
  const [toast, setToast] = useState("");
  const [showSettings, setShowSettings] = useState(false);
  const [showMessage, setShowMessage] = useState(false);
  const [layerMenu, setLayerMenu] = useState<{ elementId: string; x: number; y: number } | null>(null);
  const elementClipboard = useRef<DashboardElement | null>(null);

  const page = document.pages[activePage] || document.pages[0]!;
  const selectedElement = useMemo(() => page?.elements.find((element) => element.id === selectedId) || null, [page, selectedId]);

  const refreshMeta = useCallback(async () => {
    const [runtimeResult, zonesResult, entitiesResult, servicesResult] = await Promise.allSettled([getRuntimeConfig(), getZones(), getEntities(), getServices()]);
    if (runtimeResult.status === "fulfilled") setRuntime(runtimeResult.value);
    if (zonesResult.status === "fulfilled") setZones(zonesResult.value);
    if (entitiesResult.status === "fulfilled") setEntities(entitiesResult.value);
    if (servicesResult.status === "fulfilled") setServices(servicesResult.value);
  }, []);

  useEffect(() => {
    refreshMeta().catch((reason) => notify(reason));
  }, [refreshMeta]);

  useEffect(() => {
    if (!layerMenu) return;
    const close = (event: PointerEvent) => {
      const target = event.target;
      if (target instanceof Element && target.closest(".layer-menu")) return;
      setLayerMenu(null);
    };
    const escape = (event: KeyboardEvent) => { if (event.key === "Escape") setLayerMenu(null); };
    window.document.addEventListener("pointerdown", close);
    window.document.addEventListener("keydown", escape);
    return () => {
      window.document.removeEventListener("pointerdown", close);
      window.document.removeEventListener("keydown", escape);
    };
  }, [layerMenu]);

  useEffect(() => {
    let cancelled = false;
    setLoading(true);
    getDashboard(mode, zoneId).then((stored) => {
      if (cancelled) return;
      const next = stored || createDocument({ ...defaultTarget, mode, zone_id: mode === "zone" ? zoneId : null });
      setDocument(next); setActivePage(0); setSelectedId(null);
    }).catch((reason) => { if (!cancelled) notify(reason); }).finally(() => { if (!cancelled) setLoading(false); });
    return () => { cancelled = true; };
  }, [mode, zoneId]);

  const updateCurrentPage = (nextPage: typeof page) => {
    setDocument((current) => ({ ...current, pages: current.pages.map((candidate, index) => index === activePage ? nextPage : candidate) }));
  };

  const handleModeChange = (nextMode: DashboardMode) => {
    setMode(nextMode);
    if (nextMode === "portable") setZoneId(null);
  };

  const handleZoneChange = (nextZoneId: string | null) => {
    setZoneId(nextZoneId);
    if (!nextZoneId) setMode("portable");
  };

  const addElement = (type: DashboardElement["type"]) => {
    const nextElement = createElement(type, page?.elements.length || 0);
    updateCurrentPage({ ...page, elements: [...page.elements, nextElement] });
    setSelectedId(nextElement.id);
  };

  const changeElement = (nextElement: DashboardElement) => updateCurrentPage(patchElement(page, nextElement.id, nextElement));

  const moveElement = useCallback((elementId: string, frame: DashboardFrame) => {
    setDocument((current) => ({ ...current, pages: current.pages.map((candidate, index) => index === activePage ? patchElement(candidate, elementId, { frame }) : candidate) }));
  }, [activePage]);

  const deleteElement = () => {
    if (!selectedId) return;
    updateCurrentPage({ ...page, elements: page.elements.filter((element) => element.id !== selectedId) });
    setSelectedId(null);
  };

  const duplicateElement = () => {
    if (!selectedElement) return;
    const contentHeight = canvasHeight - 72;
    const duplicate = cloneElement(selectedElement, `${selectedElement.type}-${Date.now()}`);
    duplicate.frame = { ...duplicate.frame, x: Math.max(0, Math.min(canvasWidth - duplicate.frame.width, duplicate.frame.x + 20)), y: Math.max(0, Math.min(contentHeight - duplicate.frame.height, duplicate.frame.y + 20)) };
    updateCurrentPage({ ...page, elements: [...page.elements, duplicate] }); setSelectedId(duplicate.id);
  };

  const addPage = (parentIndex: number | null = null) => {
    const index = document.pages.length + 1;
    const parentId = parentIndex !== null ? document.pages[parentIndex]?.id || null : null;
    const nextPage = { id: `page-${Date.now()}`, name: parentId ? `子页 ${index}` : `Page ${index}`, parent_id: parentId, background: document.target.background, elements: [] };
    setDocument((current) => ({ ...current, pages: [...current.pages, nextPage] })); setActivePage(index - 1); setSelectedId(null);
  };

  const renamePage = (index: number, name: string) => {
    setDocument((current) => ({ ...current, pages: current.pages.map((candidate, candidateIndex) => candidateIndex === index ? { ...candidate, name } : candidate) }));
  };

  const pasteElement = () => {
    const source = elementClipboard.current;
    if (!source) return;
    const contentHeight = canvasHeight - 72;
    const width = Math.min(canvasWidth, Math.max(1, source.frame.width));
    const height = Math.min(contentHeight, Math.max(1, source.frame.height));
    const pasted = cloneElement(source, `${source.type}-${Date.now()}-${page.elements.length}`);
    pasted.frame = {
      x: Math.max(0, Math.min(canvasWidth - width, source.frame.x + 20)),
      y: Math.max(0, Math.min(contentHeight - height, source.frame.y + 20)),
      width,
      height
    };
    updateCurrentPage({ ...page, elements: [...page.elements, pasted] });
    setSelectedId(pasted.id);
    notify("已粘贴组件");
  };

  useEffect(() => {
    const handleKeyDown = (event: KeyboardEvent) => {
      const target = event.target;
      if (target instanceof HTMLInputElement || target instanceof HTMLTextAreaElement || target instanceof HTMLSelectElement || (target instanceof HTMLElement && target.isContentEditable)) return;
      if ((event.key === "Delete" || event.key === "Backspace") && selectedElement) {
        event.preventDefault();
        deleteElement();
        notify("已删除组件");
        return;
      }
      if (!event.ctrlKey && !event.metaKey) return;
      const key = event.key.toLowerCase();
      if (key === "c" && selectedElement) {
        event.preventDefault();
        elementClipboard.current = cloneElement(selectedElement);
        notify("已复制组件");
      } else if (key === "x" && selectedElement) {
        event.preventDefault();
        elementClipboard.current = cloneElement(selectedElement);
        deleteElement();
        notify("已剪切组件");
      } else if (key === "v" && elementClipboard.current) {
        event.preventDefault();
        pasteElement();
      }
    };
    window.addEventListener("keydown", handleKeyDown);
    return () => window.removeEventListener("keydown", handleKeyDown);
  }, [activePage, page, selectedElement, selectedId]);

  const alignElement = (alignment: Alignment) => {
    if (!selectedElement) return;
    const contentHeight = canvasHeight - 72;
    const frame = selectedElement.frame;
    const nextFrame = { ...frame };
    if (alignment === "left") nextFrame.x = 0;
    if (alignment === "center") nextFrame.x = Math.max(0, Math.round((canvasWidth - frame.width) / 2));
    if (alignment === "right") nextFrame.x = Math.max(0, canvasWidth - frame.width);
    if (alignment === "top") nextFrame.y = 0;
    if (alignment === "middle") nextFrame.y = Math.max(0, Math.round((contentHeight - frame.height) / 2));
    if (alignment === "bottom") nextFrame.y = Math.max(0, contentHeight - frame.height);
    changeElement({ ...selectedElement, frame: nextFrame });
  };

  const reorderElement = (direction: "up" | "down" | "front" | "back") => {
    if (!selectedId) return;
    setDocument((current) => ({
      ...current,
      pages: current.pages.map((candidate, index) => {
        if (index !== activePage) return candidate;
        const elements = [...candidate.elements];
        const currentIndex = elements.findIndex((element) => element.id === selectedId);
        if (currentIndex < 0) return candidate;
        let nextIndex = currentIndex;
        if (direction === "up") nextIndex = Math.min(elements.length - 1, currentIndex + 1);
        if (direction === "down") nextIndex = Math.max(0, currentIndex - 1);
        if (direction === "front") nextIndex = elements.length - 1;
        if (direction === "back") nextIndex = 0;
        if (nextIndex === currentIndex) return candidate;
        const [moved] = elements.splice(currentIndex, 1);
        elements.splice(nextIndex, 0, moved);
        return { ...candidate, elements };
      })
    }));
    setLayerMenu(null);
  };

  const publish = async () => {
    setBusy(true);
    try {
      const result = await publishDashboard({ ...document, target: { ...document.target, mode, zone_id: mode === "zone" ? zoneId : null } });
      setDocument(result.document); notify(`已发布到 ${result.entity_id}`);
    } catch (reason) { notify(reason); } finally { setBusy(false); }
  };

  const exportDocument = () => {
    const targetDocument = { ...document, target: { ...document.target, mode, zone_id: mode === "zone" ? zoneId : null } };
    const blob = new Blob([JSON.stringify(targetDocument, null, 2)], { type: "application/json" });
    const url = URL.createObjectURL(blob); const anchor = window.document.createElement("a");
    anchor.href = url; anchor.download = `${mode === "portable" ? "portable" : `zone-${zoneId || "unknown"}`}.json`; anchor.click(); URL.revokeObjectURL(url);
  };

  const saveSettings = async (haUrl: string, token: string) => {
    await saveRuntimeConfig({ ha_url: haUrl, ...(token.trim() ? { ha_token: token.trim() } : {}) });
    await refreshMeta(); notify("连接设置已保存");
  };

  const sendPushMessage = async (message: KindleMessage) => { await sendMessage(message); notify("消息已发送"); };

  return <div className="app-shell">
    <TopBar runtime={runtime} busy={busy} onPublish={publish} onSettings={() => setShowSettings(true)} onMessage={() => setShowMessage(true)} onExport={exportDocument} />
    <main className="workspace">
      <TargetBar mode={mode} zoneId={zoneId} zones={zones} onModeChange={handleModeChange} onZoneChange={handleZoneChange} onRefreshZones={() => refreshMeta().catch((reason) => notify(reason))} />
      <div className="editor-layout">
        <Palette onAdd={addElement} />
        <section className="studio-column">
          <PageTabs pages={document.pages} activeIndex={activePage} onSelect={(index) => { setActivePage(index); setSelectedId(null); setLayerMenu(null); }} onAddRoot={() => addPage()} onAddChild={(index) => addPage(index)} onRename={renamePage} />
          <div className="canvas-card panel-card">
            <div className="canvas-card-header"><div><h2>{page?.name || "页面"}</h2></div><div className="canvas-header-tools"><AlignmentToolbar selected={selectedElement} onAlign={alignElement} /><div className="canvas-meta"><span>黑白预览</span><span className="meta-divider" /><span>{page?.elements.length || 0} 个组件</span></div></div></div>
            <div className="canvas-wrap">{loading ? <div className="canvas-loading">载入界面…</div> : <EditorCanvas page={page} width={canvasWidth} height={canvasHeight} entities={entities} selectedId={selectedId} onSelect={(id) => { setSelectedId(id || null); setLayerMenu(null); }} onMove={moveElement} onResize={moveElement} onContextMenu={(elementId, x, y) => { setSelectedId(elementId); setLayerMenu({ elementId, x, y }); }} />}</div>
            <div className="canvas-card-footer"><span>拖拽定位 · 控点缩放 · 右键图层 · Delete 删除 · ⌘/Ctrl+C/V/X</span><span>revision {document.revision}</span></div>
          </div>
        </section>
        <InspectorPanel element={selectedElement} pages={document.pages} entities={entities} services={services} onChange={changeElement} onDelete={deleteElement} onDuplicate={duplicateElement} />
      </div>
    </main>
    <footer className="status-bar"><span><span className="status-mark" />编辑草稿</span><span>Kindle Dashboard Protocol · v1</span></footer>
    {showSettings ? <SettingsModal runtime={runtime} onClose={() => setShowSettings(false)} onSave={saveSettings} /> : null}
    {showMessage ? <MessageComposer onClose={() => setShowMessage(false)} onSend={sendPushMessage} /> : null}
    {layerMenu ? <LayerMenu x={layerMenu.x} y={layerMenu.y} onMoveUp={() => reorderElement("up")} onMoveDown={() => reorderElement("down")} onBringToFront={() => reorderElement("front")} onSendToBack={() => reorderElement("back")} /> : null}
    {toast ? <div className="toast" role="status">{toast}</div> : null}
  </div>;

  function notify(reason: unknown) {
    const message = reason instanceof Error ? reason.message : String(reason);
    setToast(message); window.setTimeout(() => setToast((current) => current === message ? "" : current), 3600);
  }
}
