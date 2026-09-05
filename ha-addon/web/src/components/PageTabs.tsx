import type { DashboardPage } from "../types";

interface PageTabsProps {
  pages: DashboardPage[];
  activeIndex: number;
  onSelect: (index: number) => void;
  onAdd: () => void;
}

export function PageTabs({ pages, activeIndex, onSelect, onAdd }: PageTabsProps) {
  return (
    <div className="page-tabs" role="tablist" aria-label="页面">
      {pages.map((page, index) => (
        <button key={page.id} type="button" role="tab" aria-selected={index === activeIndex} className={index === activeIndex ? "is-active" : ""} onClick={() => onSelect(index)}>
          <span className="page-index">{String(index + 1).padStart(2, "0")}</span>
          {page.name}
        </button>
      ))}
      <button className="add-page-button" type="button" onClick={onAdd} aria-label="添加页面">＋</button>
    </div>
  );
}

