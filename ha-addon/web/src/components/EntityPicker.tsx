import { useEffect, useMemo, useRef, useState } from "react";
import type { EntitySummary } from "../types";

interface EntityPickerProps {
  entities: EntitySummary[];
  value?: string;
  onChange: (value: string) => void;
  allowedDomains?: string[];
  placeholder?: string;
  disabled?: boolean;
}

interface EntityGroup {
  key: string;
  label: string;
  items: EntitySummary[];
}

const DOMAIN_META: Record<string, { label: string; glyph: string }> = {
  light: { label: "灯光", glyph: "✦" },
  switch: { label: "开关", glyph: "◉" },
  climate: { label: "温控", glyph: "°" },
  sensor: { label: "传感器", glyph: "⌁" },
  binary_sensor: { label: "状态", glyph: "◌" },
  fan: { label: "风扇", glyph: "≋" },
  cover: { label: "窗帘", glyph: "▥" },
  media_player: { label: "媒体", glyph: "▶" },
  lock: { label: "门锁", glyph: "▣" },
  camera: { label: "摄像头", glyph: "▧" },
  input_boolean: { label: "输入开关", glyph: "◉" },
  input_number: { label: "输入数值", glyph: "↕" },
  number: { label: "数值", glyph: "#" },
  select: { label: "选择", glyph: "≡" },
  vacuum: { label: "清扫", glyph: "⌁" },
  scene: { label: "场景", glyph: "✧" },
  script: { label: "脚本", glyph: "↗" },
  automation: { label: "自动化", glyph: "⚙" }
};

const DOMAIN_ORDER = Object.keys(DOMAIN_META);

export function EntityPicker({ entities, value = "", onChange, allowedDomains, placeholder = "选择实体", disabled = false }: EntityPickerProps) {
  const [open, setOpen] = useState(false);
  const [query, setQuery] = useState("");
  const [domainFilter, setDomainFilter] = useState("all");
  const [areaFilter, setAreaFilter] = useState("all");
  const searchRef = useRef<HTMLInputElement>(null);
  const allowed = useMemo(() => (allowedDomains || []).map((domain) => domain.toLowerCase()), [allowedDomains]);

  const available = useMemo(() => entities.filter((entity) => !allowed.length || allowed.includes(entity.domain) || entity.entity_id === value), [allowed, entities, value]);
  const selected = entities.find((entity) => entity.entity_id === value);
  const domains = useMemo(() => Array.from(new Set(available.map((entity) => entity.domain))).sort(compareDomains), [available]);
  const areas = useMemo(() => {
    const areaMap = new Map<string, string>();
    available.forEach((entity) => {
      const key = entity.area_id?.trim() || "unassigned";
      areaMap.set(key, areaLabel(entity));
    });
    return Array.from(areaMap.entries())
      .map(([key, label]) => ({ key, label }))
      .sort((a, b) => {
        if (a.key === "unassigned") return 1;
        if (b.key === "unassigned") return -1;
        return a.label.localeCompare(b.label, "zh-CN");
      });
  }, [available]);

  const filtered = useMemo(() => {
    const normalizedQuery = query.trim().toLocaleLowerCase();
    return available
      .filter((entity) => {
        const entityArea = entity.area_id?.trim() || "unassigned";
        if (domainFilter !== "all" && entity.domain !== domainFilter) return false;
        if (areaFilter !== "all" && entityArea !== areaFilter) return false;
        if (!normalizedQuery) return true;
        return [entity.name, entity.entity_id, entity.domain, domainInfo(entity.domain).label, entity.area_name || ""]
          .join(" ")
          .toLocaleLowerCase()
          .includes(normalizedQuery);
      })
      .sort((a, b) => a.name.localeCompare(b.name, "zh-CN") || a.entity_id.localeCompare(b.entity_id));
  }, [areaFilter, available, domainFilter, query]);

  const groups = useMemo(() => {
    const grouped = new Map<string, EntityGroup>();
    filtered.forEach((entity) => {
      const key = entity.area_id?.trim() || "unassigned";
      const current = grouped.get(key) || { key, label: areaLabel(entity), items: [] };
      current.items.push(entity);
      grouped.set(key, current);
    });
    return Array.from(grouped.values()).sort((a, b) => {
      if (a.key === "unassigned") return 1;
      if (b.key === "unassigned") return -1;
      return a.label.localeCompare(b.label, "zh-CN");
    });
  }, [filtered]);

  useEffect(() => {
    if (!open) return;
    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key === "Escape") setOpen(false);
    };
    const previousOverflow = window.document.body.style.overflow;
    window.document.body.style.overflow = "hidden";
    window.addEventListener("keydown", handleKeyDown);
    const focusTimer = window.setTimeout(() => searchRef.current?.focus(), 0);
    return () => {
      window.clearTimeout(focusTimer);
      window.document.body.style.overflow = previousOverflow;
      window.removeEventListener("keydown", handleKeyDown);
    };
  }, [open]);

  const openPicker = () => {
    if (disabled) return;
    setQuery("");
    setDomainFilter("all");
    setAreaFilter("all");
    setOpen(true);
  };

  const selectEntity = (entityId: string) => {
    onChange(entityId);
    setOpen(false);
  };

  const clearEntity = () => {
    onChange("");
    setOpen(false);
  };

  const triggerDomain = selected ? domainInfo(selected.domain) : null;
  return (
    <>
      <button type="button" className={"entity-picker-trigger" + (!selected && !value ? " is-empty" : "")} onClick={openPicker} disabled={disabled} aria-haspopup="dialog" aria-expanded={open}>
        <span className={"entity-domain-icon" + (!triggerDomain ? " is-empty" : "")}>{triggerDomain?.glyph || "＋"}</span>
        <span className="entity-picker-trigger-copy">
          <strong>{selected?.name || value || placeholder}</strong>
          <small>{selected ? domainInfo(selected.domain).label + " · " + selected.entity_id : value ? "当前实体未在列表中" : "点击打开实体选择器"}</small>
        </span>
        <span className="entity-picker-chevron">⌄</span>
      </button>
      {open ? (
        <div className="entity-picker-backdrop" onMouseDown={(event) => { if (event.target === event.currentTarget) setOpen(false); }}>
          <section className="entity-picker-dialog" role="dialog" aria-modal="true" aria-labelledby="entity-picker-title">
            <div className="modal-heading">
              <div><h2 id="entity-picker-title">选择实体</h2><span className="entity-picker-count">{filtered.length} 个可选实体</span></div>
              <button type="button" className="close-button" onClick={() => setOpen(false)} aria-label="关闭实体选择器">×</button>
            </div>
            <input ref={searchRef} className="entity-picker-search" value={query} onChange={(event) => setQuery(event.target.value)} placeholder="搜索名称、实体 ID 或房间" aria-label="搜索实体" />
            <div className="entity-picker-filters">
              <div className="entity-domain-filters" role="group" aria-label="按类型筛选">
                <button type="button" className={domainFilter === "all" ? "is-active" : ""} onClick={() => setDomainFilter("all")}>全部</button>
                {domains.map((domain) => <button type="button" key={domain} className={domainFilter === domain ? "is-active" : ""} onClick={() => setDomainFilter(domain)}><span>{domainInfo(domain).glyph}</span>{domainInfo(domain).label}</button>)}
              </div>
              <select className="entity-area-filter" value={areaFilter} onChange={(event) => setAreaFilter(event.target.value)} aria-label="按房间筛选">
                <option value="all">全部房间</option>
                {areas.map((area) => <option key={area.key} value={area.key}>{area.label}</option>)}
              </select>
            </div>
            {value ? <button type="button" className="entity-picker-clear" onClick={clearEntity}>清除当前绑定</button> : null}
            <div className="entity-picker-results">
              {groups.length > 0 ? groups.map((group) => (
                <section className="entity-group" key={group.key}>
                  <div className="entity-group-heading"><span>{group.label}</span><small>{group.items.length}</small></div>
                  {group.items.map((entity) => <EntityRow key={entity.entity_id} entity={entity} selected={entity.entity_id === value} onSelect={selectEntity} />)}
                </section>
              )) : <div className="entity-picker-empty"><span>⌕</span><strong>没有匹配的实体</strong><small>换一个关键词或筛选条件试试</small></div>}
            </div>
          </section>
        </div>
      ) : null}
    </>
  );
}

function EntityRow({ entity, selected, onSelect }: { entity: EntitySummary; selected: boolean; onSelect: (entityId: string) => void }) {
  const info = domainInfo(entity.domain);
  return (
    <button type="button" className={"entity-row" + (selected ? " is-selected" : "")} onClick={() => onSelect(entity.entity_id)}>
      <span className="entity-domain-icon">{info.glyph}</span>
      <span className="entity-row-copy"><strong>{entity.name}</strong><small>{info.label} · {entity.entity_id}</small></span>
      <span className="entity-row-state">{entity.state || "—"}{selected ? " ✓" : ""}</span>
    </button>
  );
}

function domainInfo(domain: string): { label: string; glyph: string } {
  return DOMAIN_META[domain] || { label: domain, glyph: "◌" };
}

function compareDomains(a: string, b: string): number {
  const aIndex = DOMAIN_ORDER.indexOf(a);
  const bIndex = DOMAIN_ORDER.indexOf(b);
  const aRank = aIndex < 0 ? DOMAIN_ORDER.length : aIndex;
  const bRank = bIndex < 0 ? DOMAIN_ORDER.length : bIndex;
  return aRank - bRank || domainInfo(a).label.localeCompare(domainInfo(b).label, "zh-CN");
}

function areaLabel(entity: EntitySummary): string {
  const areaName = entity.area_name?.trim();
  if (areaName) return areaName;
  return entity.area_id?.trim() || "未分配房间";
}
