import { useEffect, useState } from "react";
import type { DashboardPage } from "../types";

interface PageTabsProps {
  pages: DashboardPage[];
  activeIndex: number;
  onSelect: (index: number) => void;
  onAddRoot: () => void;
  onAddChild: (parentIndex: number) => void;
  onRename: (index: number, name: string) => void;
}

interface PageEntry {
  page: DashboardPage;
  index: number;
  depth: number;
}

export function PageTabs({ pages, activeIndex, onSelect, onAddRoot, onAddChild, onRename }: PageTabsProps) {
  const entries = flattenPages(pages);
  const activePage = pages[activeIndex];
  const [draftName, setDraftName] = useState(activePage?.name || "");
  useEffect(() => setDraftName(activePage?.name || ""), [activePage?.id, activePage?.name]);
  const commitName = () => {
    if (!activePage) return;
    const nextName = draftName.trim();
    if (!nextName) {
      setDraftName(activePage.name);
      return;
    }
    if (nextName !== activePage.name) onRename(activeIndex, nextName);
  };
  return (
    <div className="page-tabs" role="tablist" aria-label="页面">
      <div className="page-tab-list">
        {entries.map(({ page, index, depth }) => (
          <button key={page.id} type="button" role="tab" aria-selected={index === activeIndex} className={index === activeIndex ? "is-active" : ""} style={{ paddingLeft: 12 + depth * 15 }} onClick={() => onSelect(index)}>
            <span className="page-index">{String(index + 1).padStart(2, "0")}</span>
            <span>{page.name}</span>
          </button>
        ))}
      </div>
      <div className="page-tab-actions">
        {activePage ? <label className="page-name-editor"><span>页面名称</span><input aria-label="页面名称" value={draftName} onChange={(event) => setDraftName(event.target.value)} onBlur={commitName} onKeyDown={(event) => { if (event.key === "Enter" || event.key === "Escape") { if (event.key === "Escape") setDraftName(activePage.name); event.currentTarget.blur(); } }} /></label> : null}
        <button className="add-page-button" type="button" onClick={onAddRoot} aria-label="添加页面" title="添加页面">＋</button>
        <button className="add-child-button" type="button" onClick={() => onAddChild(activeIndex)} disabled={activeIndex < 0} title="添加子页">＋ 子页</button>
      </div>
    </div>
  );
}

function flattenPages(pages: DashboardPage[]): PageEntry[] {
  const children = new Map<string, number[]>();
  const roots: number[] = [];
  pages.forEach((page, index) => {
    const parentId = page.parent_id?.trim();
    if (!parentId) {
      roots.push(index);
      return;
    }
    const list = children.get(parentId) || [];
    list.push(index);
    children.set(parentId, list);
  });
  const entries: PageEntry[] = [];
  const visited = new Set<number>();
  const append = (index: number, depth: number) => {
    if (visited.has(index)) return;
    visited.add(index);
    entries.push({ page: pages[index], index, depth });
    for (const childIndex of children.get(pages[index].id) || []) append(childIndex, depth + 1);
  };
  roots.forEach((index) => append(index, 0));
  pages.forEach((_, index) => append(index, 0));
  return entries;
}
