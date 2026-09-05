import type { DashboardElement } from "../types";

export type Alignment = "left" | "center" | "right" | "top" | "middle" | "bottom";

interface AlignmentToolbarProps {
  selected: DashboardElement | null;
  onAlign: (alignment: Alignment) => void;
}

const alignments: Array<{ value: Alignment; label: string; title: string }> = [
  { value: "left", label: "左", title: "左对齐" },
  { value: "center", label: "中", title: "水平居中" },
  { value: "right", label: "右", title: "右对齐" },
  { value: "top", label: "上", title: "顶部对齐" },
  { value: "middle", label: "中", title: "垂直居中" },
  { value: "bottom", label: "下", title: "底部对齐" }
];

export function AlignmentToolbar({ selected, onAlign }: AlignmentToolbarProps) {
  return (
    <div className="alignment-toolbar" aria-label="对齐">
      <span>对齐</span>
      {alignments.map((alignment) => (
        <button key={alignment.value} type="button" title={alignment.title} aria-label={alignment.title} disabled={!selected} onClick={() => onAlign(alignment.value)}>{alignment.label}</button>
      ))}
    </div>
  );
}

