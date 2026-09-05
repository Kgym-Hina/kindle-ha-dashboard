import { useCallback, useEffect, useMemo, useState } from "react";
import { EditorCanvas } from "./components/EditorCanvas";
import { InspectorPanel } from "./components/InspectorPanel";
import { MessageComposer } from "./components/MessageComposer";
import { PageTabs } from "./components/PageTabs";
import { Palette } from "./components/Palette";
import { SettingsModal } from "./components/SettingsModal";
import { TargetBar } from "./components/TargetBar";
import { TopBar } from "./components/TopBar";
import { getDashboard, getEntities, getRuntimeConfig, getZones, publishDashboard, saveRuntimeConfig, sendMessage } from "./lib/api";
import { createDocument, createElement, patchElement } from "./lib/document";
import type { DashboardDocument, DashboardElement, DashboardFrame, DashboardMode, DashboardTarget, EntitySummary, KindleMessage, RuntimeConfig, ZoneInfo } from "./types";
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
  const [runtime, setRuntime] = useState<RuntimeConfig | null>(null);
  const [busy, setBusy] = useState(false);
  const [loading, setLoading] = useState(true);
  const [toast, setToast] = useState("");
  const [showSettings, setShowSettings] = useState(false);
  const [showMessage, setShowMessage] = useState(false);

  const page = document.pages[activePage] || document.pages[0]!;
  const selectedElement = useMemo(() => page?.elements.find((element) => element.id === selectedId) || null, [page, selectedId]);

  const refreshMeta = useCallback(async () => {
    const [runtimeResult, zonesResult, entitiesResult] = await Promise.allSettled([getRuntimeConfig(), getZones(), getEntities()]);
    if (runtimeResult.status === "fulfilled") setRuntime(runtimeResult.value);
    if (zonesResult.status === "fulfilled") setZones(zonesResult.value);
    if (entitiesResult.status === "fulfilled") setEntities(entitiesResult.value);
  }, []);

  useEffect(() => {
    refreshMeta().catch((reason) => notify(reason));
  }, [refreshMeta]);

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
    const duplicate: DashboardElement = { ...selectedElement, id: `${selectedElement.type}-${Date.now()}`, frame: { ...selectedElement.frame, x: Math.min(canvasWidth - selectedElement.frame.width, selectedElement.frame.x + 20), y: Math.min(canvasHeight - selectedElement.frame.height, selectedElement.frame.y + 20) } };
    updateCurrentPage({ ...page, elements: [...page.elements, duplicate] }); setSelectedId(duplicate.id);
  };

  const addPage = () => {
    const index = document.pages.length + 1;
    const nextPage = { id: `page-${Date.now()}`, name: `Page ${index}`, background: document.target.background, elements: [] };
    setDocument((current) => ({ ...current, pages: [...current.pages, nextPage] })); setActivePage(index - 1); setSelectedId(null);
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
          <PageTabs pages={document.pages} activeIndex={activePage} onSelect={(index) => { setActivePage(index); setSelectedId(null); }} onAdd={addPage} />
          <div className="canvas-card panel-card">
            <div className="canvas-card-header"><div><h2>{page?.name || "页面"}</h2></div><div className="canvas-meta"><span>黑白预览</span><span className="meta-divider" /><span>{page?.elements.length || 0} 个组件</span></div></div>
            <div className="canvas-wrap">{loading ? <div className="canvas-loading">载入界面…</div> : <EditorCanvas page={page} width={canvasWidth} height={canvasHeight} selectedId={selectedId} onSelect={(id) => setSelectedId(id || null)} onMove={moveElement} />}</div>
            <div className="canvas-card-footer"><span>拖拽组件定位 · 右侧面板微调</span><span>revision {document.revision}</span></div>
          </div>
        </section>
        <InspectorPanel element={selectedElement} pages={document.pages} entities={entities} onChange={changeElement} onDelete={deleteElement} onDuplicate={duplicateElement} />
      </div>
    </main>
    <footer className="status-bar"><span><span className="status-mark" />编辑草稿</span><span>Kindle Dashboard Protocol · v1</span></footer>
    {showSettings ? <SettingsModal runtime={runtime} onClose={() => setShowSettings(false)} onSave={saveSettings} /> : null}
    {showMessage ? <MessageComposer onClose={() => setShowMessage(false)} onSend={sendPushMessage} /> : null}
    {toast ? <div className="toast" role="status">{toast}</div> : null}
  </div>;

  function notify(reason: unknown) {
    const message = reason instanceof Error ? reason.message : String(reason);
    setToast(message); window.setTimeout(() => setToast((current) => current === message ? "" : current), 3600);
  }
}
