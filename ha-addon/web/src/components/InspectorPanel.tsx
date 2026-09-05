import { useMemo } from "react";
import type { ChangeEvent } from "react";
import type { DashboardAction, DashboardBinding, DashboardElement, DashboardFrame, DashboardPage, DashboardStyle, EntitySummary, ServiceField, ServiceInfo } from "../types";

interface InspectorPanelProps {
  element: DashboardElement | null;
  pages: DashboardPage[];
  entities: EntitySummary[];
  services: ServiceInfo[];
  onChange: (element: DashboardElement) => void;
  onDelete: () => void;
  onDuplicate: () => void;
}

export function InspectorPanel({ element, pages, entities, services, onChange, onDelete, onDuplicate }: InspectorPanelProps) {
  if (!element) {
    return <aside className="inspector panel-card empty-inspector"><span className="inspector-orbit">✦</span><h2>选择一个组件</h2><p>在画布中选中组件，开始调整内容、样式和交互。</p></aside>;
  }
  const style = element.style || {};
  const patch = (changes: Partial<DashboardElement>) => onChange({ ...element, ...changes });
  const patchFrame = (changes: Partial<DashboardFrame>) => patch({ frame: { ...element.frame, ...changes } });
  const patchStyle = (changes: Partial<DashboardStyle>) => patch({ style: { ...style, ...changes } });
  const isImageButton = element.type === "image_button";
  const hasAction = element.type === "button" || isImageButton || Boolean(element.action);
  return (
    <aside className="inspector panel-card">
      <div className="inspector-header">
        <div><h2>{labelForType(element.type)}</h2></div>
        <span className="element-badge">{labelForType(element.type)}</span>
      </div>
      <div className="inspector-scroll">
        <section className="inspector-section">
          <div className="section-title"><span>内容</span><span className="section-rule" /></div>
          {element.type === "text" || element.type === "button" ? <label className="field full"><span>文本</span><textarea value={element.text || ""} rows={3} onChange={(event) => patch({ text: event.target.value })} /></label> : null}
          {element.type === "switch" || element.type === "climate" ? <label className="field full"><span>标题</span><input value={element.text || ""} onChange={(event) => patch({ text: event.target.value })} /></label> : null}
          {isImageButton ? <p className="field-help">图片本身就是点击区域，不显示额外文字、填充或边框。</p> : null}
          {element.type === "text" || element.type === "button" || element.type === "switch" || element.type === "climate" ? <BindingFields element={element} entities={entities} onChange={patch} allowField={element.type === "text" || element.type === "button"} /> : null}
          {element.type === "climate" ? <div className="field-grid"><NumberField label="温度步进" value={element.climate?.temperature_step || 0.5} onChange={(value) => patch({ climate: { ...(element.climate || {}), temperature_step: Math.max(0.1, value) } })} /></div> : null}
          {element.type === "image" || isImageButton ? <ImageFields element={element} onChange={patch} /> : null}
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
          {isImageButton ? <p className="field-help">图片按钮的外观由图片尺寸和适配方式决定。</p> : <>
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
          </>}
        </section>
        {hasAction ? <ActionFields key={element.id} action={element.action} pages={pages} entities={entities} services={services} onChange={(next) => patch({ action: next })} /> : null}
      </div>
      <div className="inspector-footer"><button type="button" className="footer-button" onClick={onDuplicate}>复制</button><button type="button" className="footer-button danger" onClick={onDelete}>删除组件</button></div>
    </aside>
  );
}

function BindingFields({ element, entities, onChange, allowField }: { element: DashboardElement; entities: EntitySummary[]; onChange: (changes: Partial<DashboardElement>) => void; allowField: boolean }) {
  const binding = element.binding;
  const selectedEntity = entities.find((entity) => entity.entity_id === binding?.entity_id);
  const attributes = selectedEntity?.attribute_names || [];
  const patchBinding = (changes: Partial<DashboardBinding>) => {
    const next = { ...(binding || { entity_id: "" }), ...changes };
    if (!next.entity_id) {
      onChange({ binding: undefined });
      return;
    }
    onChange({ binding: next });
  };
  const insertValue = () => {
    if (!binding?.entity_id) return;
    const currentText = element.text || "";
    const text = currentText.includes("{value}") ? currentText : (!currentText.trim() || currentText === "新文本" ? "{value}" : currentText + " {value}");
    onChange({ text });
  };
  return (
    <div className="binding-fields">
      <div className="subsection-label">数据绑定</div>
      <label className="field full"><span>实体</span><select value={binding?.entity_id || ""} onChange={(event) => patchBinding({ entity_id: event.target.value })}><option value="">不绑定</option>{entities.map((entity) => <option key={entity.entity_id} value={entity.entity_id}>{entity.name} · {entity.entity_id}</option>)}</select></label>
      {allowField ? <label className="field full"><span>显示字段</span><select value={binding?.field || "state"} disabled={!binding?.entity_id} onChange={(event) => patchBinding({ field: event.target.value })}><option value="state">当前状态</option>{attributes.map((attribute) => <option key={attribute} value={attribute}>{attribute}</option>)}</select></label> : null}
      {allowField ? <div className="field-grid"><label className="field"><span>前缀</span><input value={binding?.prefix || ""} disabled={!binding?.entity_id} onChange={(event) => patchBinding({ prefix: event.target.value })} placeholder="例如 温度 " /></label><label className="field"><span>后缀</span><input value={binding?.suffix || ""} disabled={!binding?.entity_id} onChange={(event) => patchBinding({ suffix: event.target.value })} placeholder="例如 ℃" /></label></div> : null}
      {allowField ? <div className="field-grid binding-bottom"><NumberField label="小数位" value={binding?.decimals ?? 0} onChange={(value) => patchBinding({ decimals: Math.max(0, Math.min(6, Math.round(value))) })} /><button className="inline-action" type="button" disabled={!binding?.entity_id} onClick={insertValue}>插入值</button></div> : <p className="field-help">绑定后会自动读取实体状态，点击组件可执行内置控制。</p>}
      {allowField ? <p className="field-help">文本中使用 {"{value}"} 可放置实体值。</p> : null}
    </div>
  );
}

function ImageFields({ element, onChange }: { element: DashboardElement; onChange: (changes: Partial<DashboardElement>) => void }) {
  const image = element.image || { src: "", fit: "contain" as const };
  const handleUpload = (event: ChangeEvent<HTMLInputElement>) => {
    const file = event.target.files?.[0];
    if (!file || !["image/png", "image/jpeg", "image/gif"].includes(file.type)) return;
    const reader = new FileReader();
    reader.onload = () => onChange({ image: { ...image, src: String(reader.result) } });
    reader.readAsDataURL(file);
  };
  return <>
    <label className="field full"><span>图片地址</span><input value={image.src} onChange={(event) => onChange({ image: { ...image, src: event.target.value } })} placeholder="https://… 或选择本地图片" /></label>
    <div className="image-actions"><label className="upload-button">上传图片<input type="file" accept="image/png,image/jpeg,image/gif" onChange={handleUpload} /></label><SelectField label="适配" value={image.fit || "contain"} options={["contain", "cover", "stretch"]} onChange={(value) => onChange({ image: { ...image, fit: value as "contain" | "cover" | "stretch" } })} /></div>
  </>;
}

function ActionFields({ action, pages, entities, services, onChange }: { action?: DashboardAction; pages: DashboardPage[]; entities: EntitySummary[]; services: ServiceInfo[]; onChange: (action: DashboardAction | undefined) => void }) {
  const type = action?.type || "none";
  const currentServiceValue = action?.domain && action.service ? action.domain + "." + action.service : "";
  const currentService = services.find((service) => service.domain === action?.domain && service.service === action?.service);
  const selectedServiceValue = currentServiceValue || (services[0] ? services[0].domain + "." + services[0].service : "");
  const serviceOptions = useMemo(() => services.map((service) => ({ value: service.domain + "." + service.service, label: (service.name || service.service) + " · " + service.domain + "." + service.service })), [services]);
  const setType = (next: string) => {
    if (next === "none") {
      onChange(undefined);
      return;
    }
    if (next === "navigate_page") onChange({ type: "navigate_page", page_id: pages[0]?.id || "home" });
    if (next === "call_service") {
      const first = services[0];
      onChange({ type: "call_service", domain: first?.domain || "light", service: first?.service || "turn_on", service_data: {} });
    }
    if (next === "show_message") onChange({ type: "show_message", title: "Kindle", message: "按钮已按下", timeout_ms: 6000 });
  };
  const setService = (value: string) => {
    const separator = value.indexOf(".");
    if (separator < 1) return;
    onChange({ ...(action || { type: "call_service" }), type: "call_service", domain: value.slice(0, separator), service: value.slice(separator + 1), service_data: action?.service_data || {} });
  };
  return <section className="inspector-section">
    <div className="section-title"><span>触摸动作</span><span className="section-rule" /></div>
    <label className="field full"><span>点击后</span><select value={type} onChange={(event) => setType(event.target.value)}><option value="none">无动作</option><option value="call_service">调用 HA 服务</option><option value="navigate_page">切换页面</option><option value="show_message">弹出消息</option></select></label>
    {action?.type === "call_service" ? <>
      {services.length > 0 ? <label className="field full"><span>服务</span><select value={selectedServiceValue} onChange={(event) => setService(event.target.value)}>{!serviceOptions.some((option) => option.value === currentServiceValue) && currentServiceValue ? <option value={currentServiceValue}>{currentServiceValue}</option> : null}{serviceOptions.map((option) => <option key={option.value} value={option.value}>{option.label}</option>)}</select></label> : <p className="field-help">暂时无法读取 HA 服务列表，请先检查连接设置。</p>}
      {currentService ? <div className="service-fields">{Object.entries(currentService.fields || {}).map(([name, field]) => <ServiceFieldControl key={name} name={name} field={field} value={action.service_data?.[name]} entities={entities} onChange={(value) => updateServiceField(action, name, value, onChange)} />)}</div> : null}
      {currentService && Object.keys(currentService.fields || {}).length === 0 ? <p className="field-help">此服务不需要额外参数。</p> : null}
    </> : null}
    {action?.type === "navigate_page" ? <label className="field full"><span>目标页面</span><select value={action.page_id || ""} onChange={(event) => onChange({ ...action, page_id: event.target.value })}>{pages.map((page, index) => <option key={page.id} value={page.id}>{pageDepth(pages, index) ? "└ ".repeat(pageDepth(pages, index)) : ""}{page.name}</option>)}</select></label> : null}
    {action?.type === "show_message" ? <><label className="field full"><span>标题</span><input value={action.title || ""} onChange={(event) => onChange({ ...action, title: event.target.value })} /></label><label className="field full"><span>消息</span><textarea value={action.message || ""} rows={3} onChange={(event) => onChange({ ...action, message: event.target.value })} /></label><NumberField label="停留毫秒" value={action.timeout_ms || 6000} onChange={(value) => onChange({ ...action, timeout_ms: Math.max(0, value) })} /></> : null}
  </section>;
}

function ServiceFieldControl({ name, field, value, entities, onChange }: { name: string; field: ServiceField; value: unknown; entities: EntitySummary[]; onChange: (value: unknown) => void }) {
  const selector = field.selector || {};
  const entitySelector = asRecord(selector.entity);
  const currentValue = value !== undefined ? value : field.default;
  const label = (field.name || humanize(name)) + (field.required ? " *" : "");
  if (entitySelector) {
    const rawDomains = entitySelector.domain;
    const domains = Array.isArray(rawDomains) ? rawDomains.filter((item): item is string => typeof item === "string") : [];
    const available = domains.length ? entities.filter((entity) => domains.includes(entity.domain)) : entities;
    const current = typeof currentValue === "string" ? currentValue : "";
    const hasCurrent = available.some((entity) => entity.entity_id === current);
    return <div className="service-field"><label className="field"><span>{label}</span>{available.length > 0 && (!current || hasCurrent) ? <select value={current} onChange={(event) => onChange(event.target.value)}><option value="">选择实体</option>{available.map((entity) => <option key={entity.entity_id} value={entity.entity_id}>{entity.name} · {entity.entity_id}</option>)}</select> : <input value={current} onChange={(event) => onChange(event.target.value)} placeholder="输入实体 ID" />}</label>{field.description ? <small>{field.description}</small> : null}</div>;
  }
  const booleanSelector = selector.boolean !== undefined;
  if (booleanSelector) {
    return <div className="service-field"><label className="toggle-field service-toggle"><input type="checkbox" checked={currentValue === true || currentValue === "true"} onChange={(event) => onChange(event.target.checked)} /><span className="toggle-ui" /><span>{label}</span></label>{field.description ? <small>{field.description}</small> : null}</div>;
  }
  const numberSelector = asRecord(selector.number);
  if (numberSelector) {
    const min = numberValue(numberSelector, "min");
    const max = numberValue(numberSelector, "max");
    const step = numberValue(numberSelector, "step");
    return <div className="service-field"><label className="field"><span>{label}</span><input type="number" value={currentValue === undefined ? "" : String(currentValue)} min={min} max={max} step={step ?? "any"} onChange={(event) => onChange(event.target.value === "" ? undefined : Number(event.target.value))} /></label>{field.description ? <small>{field.description}</small> : null}</div>;
  }
  const selectSelector = asRecord(selector.select);
  if (selectSelector) {
    const rawOptions = Array.isArray(selectSelector.options) ? selectSelector.options : [];
    const options = rawOptions.map((option) => {
      if (typeof option === "string") return { value: option, label: option };
      const record = asRecord(option);
      const optionValue = record && typeof record.value === "string" ? record.value : "";
      const optionLabel = record && typeof record.label === "string" ? record.label : optionValue;
      return { value: optionValue, label: optionLabel };
    }).filter((option) => option.value);
    return <div className="service-field"><label className="field"><span>{label}</span><select value={currentValue === undefined ? "" : String(currentValue)} onChange={(event) => onChange(event.target.value)}><option value="">请选择</option>{options.map((option) => <option key={option.value} value={option.value}>{option.label}</option>)}</select></label>{field.description ? <small>{field.description}</small> : null}</div>;
  }
  return <div className="service-field"><label className="field"><span>{label}</span><input value={currentValue === undefined ? "" : String(currentValue)} onChange={(event) => onChange(event.target.value === "" ? undefined : event.target.value)} /></label>{field.description ? <small>{field.description}</small> : null}</div>;
}

function updateServiceField(action: DashboardAction, name: string, value: unknown, onChange: (action: DashboardAction | undefined) => void) {
  const serviceData = { ...(action.service_data || {}) };
  if (value === undefined || value === "") delete serviceData[name];
  else serviceData[name] = value;
  onChange({ ...action, service_data: serviceData });
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

function asRecord(value: unknown): Record<string, unknown> | null {
  return typeof value === "object" && value !== null && !Array.isArray(value) ? value as Record<string, unknown> : null;
}

function numberValue(value: Record<string, unknown>, key: string): number | undefined {
  return typeof value[key] === "number" && Number.isFinite(value[key]) ? value[key] as number : undefined;
}

function humanize(value: string): string {
  return value.replace(/_/g, " ").replace(/\b\w/g, (character) => character.toUpperCase());
}

function pageDepth(pages: DashboardPage[], index: number): number {
  let depth = 0;
  let parentId = pages[index]?.parent_id?.trim();
  const visited = new Set<string>();
  while (parentId && !visited.has(parentId)) {
    visited.add(parentId);
    const parentIndex = pages.findIndex((page) => page.id === parentId);
    if (parentIndex < 0) break;
    depth += 1;
    parentId = pages[parentIndex].parent_id?.trim();
  }
  return depth;
}

function labelForType(type: DashboardElement["type"]): string {
  return { text: "文本", button: "文字按钮", image_button: "图片按钮", image: "图片", rect: "卡片", line: "分隔线", switch: "开关", climate: "温控" }[type];
}
