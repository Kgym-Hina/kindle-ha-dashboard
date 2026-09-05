import type { DashboardElement } from "../types";

interface PaletteProps {
  onAdd: (type: DashboardElement["type"]) => void;
}

const tools: Array<{ type: DashboardElement["type"]; label: string; icon: string; description: string }> = [
  { type: "text", label: "文本", icon: "T", description: "标题与说明" },
  { type: "button", label: "按钮", icon: "↗", description: "文字交互" },
  { type: "image_button", label: "图片按钮", icon: "▧", description: "图片本身可点击" },
  { type: "image", label: "图片", icon: "▨", description: "导入图像" },
  { type: "rect", label: "卡片", icon: "□", description: "背景容器" },
  { type: "line", label: "分隔线", icon: "—", description: "视觉分组" },
  { type: "switch", label: "开关", icon: "◉", description: "绑定开关实体" },
  { type: "climate", label: "温控", icon: "℃", description: "温度与模式控制" }
];

export function Palette({ onAdd }: PaletteProps) {
  return (
    <aside className="palette panel-card">
      <div className="panel-heading">
        <div>
          <h2>组件</h2>
        </div>
        <span className="panel-count">{tools.length}</span>
      </div>
      <div className="palette-list">
        {tools.map((tool) => (
          <button key={tool.type} className="palette-item" type="button" onClick={() => onAdd(tool.type)}>
            <span className="palette-icon">{tool.icon}</span>
            <span className="palette-copy"><strong>{tool.label}</strong><small>{tool.description}</small></span>
            <span className="palette-plus">＋</span>
          </button>
        ))}
      </div>
      <div className="palette-tip"><span>⌘</span><span>拖拽画布中的组件来调整位置</span></div>
    </aside>
  );
}
