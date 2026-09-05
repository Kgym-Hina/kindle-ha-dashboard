import { useState } from "react";
import type { ChangeEvent } from "react";
import type { DashboardAction, DashboardElement, DashboardFrame, DashboardPage, DashboardStyle, EntitySummary } from "../types";

interface InspectorPanelProps {
  element: DashboardElement | null;
  pages: DashboardPage[];
  entities: EntitySummary[];
  onChange: (element: DashboardElement) => void;
  onDelete: () => void;
  onDuplicate: () => void;
}

export function InspectorPanel({ element, pages, entities, onChange, onDelete, onDuplicate }: InspectorPanelProps) {
  if (!element) {
    return <aside className="inspector panel-card empty-inspector"><span className="inspector-orbit">✦</span><h2>选择一个组件</h2><p>在画布中选中组件，开始调整内容、样式和交互。</p></aside>;
  }
  const style = element.style || {};
  const patch = (changes: Partial<DashboardElement>) => onChange({ ...element, ...changes });
  const patchFrame = (changes: Partial<DashboardFrame>) => patch({ frame: { ...element.frame, ...changes } });
  const patchStyle = (changes: Partial<DashboardStyle>) => patch({ style: { ...style, ...changes } });
  const action = element.action;
  return (
    <aside className="inspector panel-card">
      <div className="inspector-header">
        <div><h2>{labelForType(element.type)}</h2></div>
        <span className="element-badge">{element.type}</span>
      </div>
      <div className="inspector-scroll">
        <section className="inspector-section">
          <div className="section-title"><span>内容</span><span className="section-rule" /></div>
          {(element.type === "text" || element.type === "button" || element.type === "image_button") ? <label className="field full"><span>文本</span><textarea value={element.text || ""} rows={3} onChange={(event) => patch({ text: event.target.value })} /></label> : null}
          {(element.type === "image" || element.type === "image_button") ? <ImageFields element={element} onChange={patch} /> : null}
        </section>
        <section className="inspector-section">
          <div className="section-title"><span>位置与尺寸</span><span className="section-rule" /></div>
          <div className="field-grid">
            <NumberField label="X" value={element.frame.x} onChange={(value) => patchFrame({ x: value })} />
            <NumberField label="Y" value={element.frame.y} onChange={(value) => patchFrame({ y: value })} />
            <NumberField label="W" value={element.frame.width} onChange={(value) => patchFrame({ width: Math.max(1, value) })} />
            <NumberField label="H" value={element.frame.height} onChange={(value) => patchFrame({ height: Math.max(1, value) })} />
          </div>
        </section>
        <section className="inspector-section">
          <div className="section-title"><span>外观</span><span className="section-rule" /></div>
          <div className="color-row">
            <ColorField label="文字" value={style.color || "#171717"} onChange={(value) => patchStyle({ color: value })} />
            {element.type !== "text" && element.type !== "line" ? <ColorField label="填充" value={style.fill || "#ffffff"} onChange={(value) => patchStyle({ fill: value })} /> : null}
            <ColorField label="描边" value={style.stroke || "#171717"} onChange={(value) => patchStyle({ stroke: value })} />
          </div>
          <div className="field-grid">
            <NumberField label="字号" value={style.font_size || 16} onChange={(value) => patchStyle({ font_size: Math.max(1, value) })} />
            <NumberField label="边框" value={style.border_width || 0} onChange={(value) => patchStyle({ border_width: Math.max(0, value) })} />
            <NumberField label="圆角" value={style.radius || 0} onChange={(value) => patchStyle({ radius: Math.max(0, value) })} />
            <SelectField label="对齐" value={style.align || "left"} options={["left", "center", "right"]} onChange={(value) => patchStyle({ align: value as DashboardStyle["align"] })} />
          </div>
          <label className="toggle-field"><input type="checkbox" checked={style.font_weight === "bold"} onChange={(event) => patchStyle({ font_weight: event.target.checked ? "bold" : "normal" })} /><span className="toggle-ui" /><span>粗体</span></label>
        </section>
        {(element.type === "button" || element.type === "image_button" || element.action) ? <ActionFields key={element.id} action={action} pages={pages} entities={entities} onChange={(next) => patch({ action: next })} /> : null}
      </div>
      <div className="inspector-footer"><button type="button" className="footer-button" onClick={onDuplicate}>复制</button><button type="button" className="footer-button danger" onClick={onDelete}>删除组件</button></div>
    </aside>
  );
}

function ImageFields({ element, onChange }: { element: DashboardElement; onChange: (changes: Partial<DashboardElement>) => void }) {
  const image = element.image || { src: "", fit: "contain" as const };
  const source = image.src;
  const handleUpload = (event: ChangeEvent<HTMLInputElement>) => {
    const file = event.target.files?.[0];
    if (!file || !["image/png", "image/jpeg", "image/gif"].includes(file.type)) return;
    const reader = new FileReader();
    reader.onload = () => onChange({ image: { ...image, src: String(reader.result) } });
    reader.readAsDataURL(file);
  };
  return <>
    <label className="field full"><span>图片地址</span><input value={source} onChange={(event) => onChange({ image: { ...image, src: event.target.value } })} placeholder="https://… 或选择本地图片" /></label>
    <div className="image-actions"><label className="upload-button">上传图片<input type="file" accept="image/png,image/jpeg,image/gif" onChange={handleUpload} /></label><SelectField label="适配" value={image.fit || "contain"} options={["contain", "cover", "stretch"]} onChange={(value) => onChange({ image: { ...image, fit: value as "contain" | "cover" | "stretch" } })} /></div>
  </>;
}

function ActionFields({ action, pages, entities, onChange }: { action?: DashboardAction; pages: DashboardPage[]; entities: EntitySummary[]; onChange: (action: DashboardAction | undefined) => void }) {
  const [serviceDataText, setServiceDataText] = useState(action?.service_data ? JSON.stringify(action.service_data, null, 2) : "{}");
  const type = action?.type || "none";
  const setType = (next: string) => {
    if (next === "none") { onChange(undefined); return; }
    if (next === "navigate_page") onChange({ type: "navigate_page", page_id: pages[0]?.id || "home" });
    if (next === "call_service") onChange({ type: "call_service", domain: "light", service: "turn_on", service_data: {} });
    if (next === "show_message") onChange({ type: "show_message", title: "Kindle", message: "按钮已按下", timeout_ms: 6000 });
  };
  return <section className="inspector-section">
    <div className="section-title"><span>触摸动作</span><span className="section-rule" /></div>
    <label className="field full"><span>点击后</span><select value={type} onChange={(event) => setType(event.target.value)}><option value="none">无动作</option><option value="call_service">调用 HA 服务</option><option value="navigate_page">切换页面</option><option value="show_message">弹出消息</option></select></label>
    {action?.type === "call_service" ? <>
      <div className="field-grid"><label className="field"><span>域</span><input value={action.domain || ""} onChange={(event) => onChange({ ...action, domain: event.target.value })} /></label><label className="field"><span>服务</span><input value={action.service || ""} onChange={(event) => onChange({ ...action, service: event.target.value })} /></label></div>
      <label className="field full"><span>实体</span><select value={typeof action.service_data?.entity_id === "string" ? action.service_data.entity_id : ""} onChange={(event) => { const serviceData = { ...(action.service_data || {}), entity_id: event.target.value }; setServiceDataText(JSON.stringify(serviceData, null, 2)); onChange({ ...action, service_data: serviceData }); }}><option value="">手动填写实体</option>{entities.map((entity) => <option key={entity.entity_id} value={entity.entity_id}>{entity.name} · {entity.entity_id}</option>)}</select></label>
      <label className="field full"><span>服务数据 JSON</span><textarea value={serviceDataText} rows={4} onChange={(event) => { setServiceDataText(event.target.value); try { onChange({ ...action, service_data: JSON.parse(event.target.value) }); } catch { /* keep the editor responsive while typing */ } }} /></label>
    </> : null}
    {action?.type === "navigate_page" ? <label className="field full"><span>目标页面</span><select value={action.page_id || ""} onChange={(event) => onChange({ ...action, page_id: event.target.value })}>{pages.map((page) => <option key={page.id} value={page.id}>{page.name}</option>)}</select></label> : null}
    {action?.type === "show_message" ? <><label className="field full"><span>标题</span><input value={action.title || ""} onChange={(event) => onChange({ ...action, title: event.target.value })} /></label><label className="field full"><span>消息</span><textarea value={action.message || ""} rows={3} onChange={(event) => onChange({ ...action, message: event.target.value })} /></label><NumberField label="停留毫秒" value={action.timeout_ms || 6000} onChange={(value) => onChange({ ...action, timeout_ms: Math.max(0, value) })} /></> : null}
  </section>;
}

function NumberField({ label, value, onChange }: { label: string; value: number; onChange: (value: number) => void }) {
  return <label className="field"><span>{label}</span><input type="number" value={value} onChange={(event) => onChange(Number(event.target.value) || 0)} /></label>;
}

function ColorField({ label, value, onChange }: { label: string; value: string; onChange: (value: string) => void }) {
  return <label className="color-field"><span>{label}</span><span className="color-input"><input type="color" value={value} onChange={(event) => onChange(event.target.value)} /><input value={value} onChange={(event) => onChange(event.target.value)} /></span></label>;
}

function SelectField({ label, value, options, onChange }: { label: string; value: string; options: string[]; onChange: (value: string) => void }) {
  return <label className="field"><span>{label}</span><select value={value} onChange={(event) => onChange(event.target.value)}>{options.map((option) => <option key={option} value={option}>{option}</option>)}</select></label>;
}

function labelForType(type: DashboardElement["type"]): string {
  return { text: "文本", button: "文字按钮", image_button: "图片按钮", image: "图片", rect: "卡片", line: "分隔线" }[type];
}
