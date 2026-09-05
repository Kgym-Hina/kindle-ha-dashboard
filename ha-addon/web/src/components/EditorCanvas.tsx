import { useEffect, useRef, useState } from "react";
import type { CSSProperties, MouseEvent as ReactMouseEvent, PointerEvent as ReactPointerEvent } from "react";
import type { DashboardElement, DashboardFrame, DashboardPage, EntitySummary } from "../types";

const navigationHeight = 72;
const minimumElementSize = 24;
type ResizeHandle = "nw" | "n" | "ne" | "e" | "se" | "s" | "sw" | "w";

interface EditorCanvasProps {
  page: DashboardPage;
  width: number;
  height: number;
  entities: EntitySummary[];
  selectedId: string | null;
  onSelect: (elementId: string) => void;
  onMove: (elementId: string, frame: DashboardFrame) => void;
  onResize: (elementId: string, frame: DashboardFrame) => void;
  onContextMenu: (elementId: string, x: number, y: number) => void;
}

interface MoveInteraction {
  kind: "move";
  elementId: string;
  startX: number;
  startY: number;
  frame: DashboardFrame;
}

interface ResizeInteraction {
  kind: "resize";
  elementId: string;
  handle: ResizeHandle;
  startX: number;
  startY: number;
  frame: DashboardFrame;
}

type Interaction = MoveInteraction | ResizeInteraction;

export function EditorCanvas({ page, width, height, entities, selectedId, onSelect, onMove, onResize, onContextMenu }: EditorCanvasProps) {
  const canvasRef = useRef<HTMLDivElement>(null);
  const [interaction, setInteraction] = useState<Interaction | null>(null);
  const contentHeight = Math.max(1, height - navigationHeight);

  useEffect(() => {
    if (!interaction) return;
    const update = (event: PointerEvent) => {
      const canvas = canvasRef.current;
      if (!canvas) return;
      const bounds = canvas.getBoundingClientRect();
      const scale = bounds.width / width;
      if (!Number.isFinite(scale) || scale <= 0) return;
      const deltaX = (event.clientX - interaction.startX) / scale;
      const deltaY = (event.clientY - interaction.startY) / scale;
      if (interaction.kind === "move") {
        onMove(interaction.elementId, {
          ...interaction.frame,
          x: clamp(Math.round(interaction.frame.x + deltaX), 0, Math.max(0, width - interaction.frame.width)),
          y: clamp(Math.round(interaction.frame.y + deltaY), 0, Math.max(0, contentHeight - interaction.frame.height))
        });
        return;
      }
      onResize(interaction.elementId, resizeFrame(interaction.frame, interaction.handle, deltaX, deltaY, width, contentHeight));
    };
    const stop = () => setInteraction(null);
    window.addEventListener("pointermove", update);
    window.addEventListener("pointerup", stop);
    window.addEventListener("pointercancel", stop);
    return () => {
      window.removeEventListener("pointermove", update);
      window.removeEventListener("pointerup", stop);
      window.removeEventListener("pointercancel", stop);
    };
  }, [contentHeight, interaction, onMove, onResize, width]);

  return (
    <div className="canvas-stage">
      <div className="canvas-ruler canvas-ruler-top"><span>0</span><span>150</span><span>300</span><span>450</span><span>600</span></div>
      <div className="canvas-ruler canvas-ruler-left"><span>0</span><span>200</span><span>400</span><span>600</span><span>800</span></div>
      <div
        className="editor-canvas"
        ref={canvasRef}
        style={{ width: `${width}px`, height: `${height}px`, background: page.background || "#fff" }}
        onPointerDown={(event) => { if (event.target === event.currentTarget) onSelect(""); }}
      >
        <div className="canvas-grid" aria-hidden="true" />
        <div className="canvas-content-boundary" aria-hidden="true" style={{ top: `${contentHeight}px` }} />
        {page.elements.map((element) => (
          <CanvasElement
            key={element.id}
            element={element}
            entities={entities}
            selected={element.id === selectedId}
            onSelect={() => onSelect(element.id)}
            onContextMenu={(event) => onContextMenu(element.id, event.clientX, event.clientY)}
            onStartMove={(event) => setInteraction({ kind: "move", elementId: element.id, startX: event.clientX, startY: event.clientY, frame: element.frame })}
            onStartResize={(event, handle) => setInteraction({ kind: "resize", elementId: element.id, handle, startX: event.clientX, startY: event.clientY, frame: element.frame })}
          />
        ))}
        <div className="canvas-navigation-preview" aria-hidden="true"><span>‹ 返回</span><span>主页</span><span>设置</span></div>
      </div>
      <div className="canvas-caption"><span className="caption-dot" /> KINDLE KT2 BASIC · 600 × 800</div>
    </div>
  );
}

interface CanvasElementProps {
  element: DashboardElement;
  entities: EntitySummary[];
  selected: boolean;
  onSelect: () => void;
  onContextMenu: (event: ReactMouseEvent<HTMLDivElement>) => void;
  onStartMove: (event: ReactPointerEvent<HTMLDivElement>) => void;
  onStartResize: (event: ReactPointerEvent<HTMLSpanElement>, handle: ResizeHandle) => void;
}

function CanvasElement({ element, entities, selected, onSelect, onContextMenu, onStartMove, onStartResize }: CanvasElementProps) {
  const frame = element.frame;
  const style = element.style || {};
  const isImageButton = element.type === "image_button";
  const visualStyle: CSSProperties = {
    left: frame.x,
    top: frame.y,
    width: frame.width,
    height: frame.height,
    color: style.color || "#171717",
    background: isImageButton || element.type === "line" ? "transparent" : style.fill || "transparent",
    border: isImageButton || element.type === "line" ? "none" : `${style.border_width ?? 0}px solid ${style.stroke || "transparent"}`,
    borderRadius: isImageButton ? 0 : style.radius ?? 0,
    fontSize: style.font_size || 16,
    fontWeight: style.font_weight || "normal",
    textAlign: textAlignForPreview(style.align),
    justifyContent: element.type === "switch" || element.type === "climate" ? "flex-start" : "center"
  };
  const entity = element.binding ? entities.find((candidate) => candidate.entity_id === element.binding?.entity_id) : undefined;
  const entityValue = entity ? entity.state : "未绑定";
  const switchIsOn = Boolean(entity && isOnState(entity.state));
  const switchLabel = entity ? (switchIsOn ? "已开启" : "已关闭") : "未绑定";
  const climateValue = entity ? formatClimatePreview(entity) : "未绑定";
  return (
    <div
      className={`canvas-element element-${element.type} ${selected ? "is-selected" : ""}`}
      style={visualStyle}
      onPointerDown={(event) => {
        if (event.button !== 0) return;
        event.stopPropagation();
        onSelect();
        onStartMove(event);
      }}
      onContextMenu={(event) => { event.preventDefault(); event.stopPropagation(); onContextMenu(event); }}
      role="button"
      tabIndex={0}
      aria-label={element.text || labelForType(element.type)}
    >
      {element.type === "line" ? <span className="line-visual" style={{ background: style.stroke || style.color || "#171717", height: style.border_width || 2 }} /> : null}
      {element.type === "image" || element.type === "image_button" ? (
        element.image?.src ? <img src={element.image.src} alt="" style={{ objectFit: imageObjectFit(element.image.fit) }} /> : <span className="empty-image">未设置图片</span>
      ) : null}
      {element.type === "text" || element.type === "button" ? <span className="element-text">{element.text}</span> : null}
      {element.type === "switch" ? <span className="switch-preview-copy"><strong>{element.text || "开关"}</strong><small>{switchLabel}</small></span> : null}
      {element.type === "switch" ? <span className={`switch-preview ${switchIsOn ? "is-on" : ""}`} aria-label={entityValue}><span /></span> : null}
      {element.type === "climate" ? <><span className="climate-preview-heading"><strong>{element.text || "温控"}</strong><small>{entity ? entity.state : "未绑定"}</small></span><span className="climate-preview-reading"><strong>{entity ? entity.state : "—"}</strong><small>{climateValue}</small></span><span className="climate-preview-controls"><span>− 调低</span><span>模式</span><span>＋ 调高</span></span></> : null}
      {selected ? <ResizeHandles onStartResize={onStartResize} /> : null}
    </div>
  );
}

function ResizeHandles({ onStartResize }: { onStartResize: (event: ReactPointerEvent<HTMLSpanElement>, handle: ResizeHandle) => void }) {
  const handles: ResizeHandle[] = ["nw", "n", "ne", "e", "se", "s", "sw", "w"];
  return <>{handles.map((handle) => <span key={handle} className={`resize-handle resize-${handle}`} onPointerDown={(event) => { event.preventDefault(); event.stopPropagation(); onStartResize(event, handle); }} aria-hidden="true" />)}</>;
}

function resizeFrame(frame: DashboardFrame, handle: ResizeHandle, deltaX: number, deltaY: number, canvasWidth: number, canvasHeight: number): DashboardFrame {
  let left = frame.x;
  let top = frame.y;
  let right = frame.x + frame.width;
  let bottom = frame.y + frame.height;
  if (handle.includes("w")) left += deltaX;
  if (handle.includes("e")) right += deltaX;
  if (handle.includes("n")) top += deltaY;
  if (handle.includes("s")) bottom += deltaY;
  if (right - left < minimumElementSize) {
    if (handle.includes("w")) left = right - minimumElementSize;
    else right = left + minimumElementSize;
  }
  if (bottom - top < minimumElementSize) {
    if (handle.includes("n")) top = bottom - minimumElementSize;
    else bottom = top + minimumElementSize;
  }
  left = clamp(left, 0, canvasWidth - minimumElementSize);
  top = clamp(top, 0, canvasHeight - minimumElementSize);
  right = clamp(right, left + minimumElementSize, canvasWidth);
  bottom = clamp(bottom, top + minimumElementSize, canvasHeight);
  return { x: Math.round(left), y: Math.round(top), width: Math.round(right - left), height: Math.round(bottom - top) };
}

function formatClimatePreview(entity: EntitySummary): string {
  const state = entity.state || "—";
  return `${state} · ${entity.name}`;
}

function imageObjectFit(fit: "contain" | "cover" | "stretch" | undefined): CSSProperties["objectFit"] {
  if (fit === "cover") return "cover";
  if (fit === "stretch") return "fill";
  return "contain";
}

function textAlignForPreview(align: "left" | "center" | "right" | undefined): CSSProperties["textAlign"] {
  if (align === "center") return "center";
  if (align === "right") return "right";
  return "left";
}

function isOnState(value: string): boolean {
  return ["on", "true", "1", "yes"].includes(value.trim().toLowerCase());
}

function labelForType(type: DashboardElement["type"]): string {
  return { text: "文本", button: "按钮", image_button: "图片按钮", image: "图片", rect: "卡片", line: "分隔线", switch: "开关", climate: "温控" }[type];
}

function clamp(value: number, min: number, max: number) {
  return Math.max(min, Math.min(max, value));
}
