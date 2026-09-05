import { useEffect, useRef, useState } from "react";
import type { CSSProperties, PointerEvent as ReactPointerEvent } from "react";
import type { DashboardElement, DashboardFrame, DashboardPage } from "../types";

interface EditorCanvasProps {
  page: DashboardPage;
  width: number;
  height: number;
  selectedId: string | null;
  onSelect: (elementId: string) => void;
  onMove: (elementId: string, frame: DashboardFrame) => void;
}

interface DragState {
  elementId: string;
  startX: number;
  startY: number;
  frame: DashboardFrame;
}

export function EditorCanvas({ page, width, height, selectedId, onSelect, onMove }: EditorCanvasProps) {
  const canvasRef = useRef<HTMLDivElement>(null);
  const [drag, setDrag] = useState<DragState | null>(null);

  useEffect(() => {
    if (!drag) return;
    const move = (event: PointerEvent) => {
      const canvas = canvasRef.current;
      if (!canvas) return;
      const bounds = canvas.getBoundingClientRect();
      const scale = bounds.width / width;
      const nextFrame = {
        ...drag.frame,
        x: clamp(Math.round(drag.frame.x + (event.clientX - drag.startX) / scale), 0, width - drag.frame.width),
        y: clamp(Math.round(drag.frame.y + (event.clientY - drag.startY) / scale), 0, height - drag.frame.height)
      };
      onMove(drag.elementId, nextFrame);
    };
    const stop = () => setDrag(null);
    window.addEventListener("pointermove", move);
    window.addEventListener("pointerup", stop, { once: true });
    return () => {
      window.removeEventListener("pointermove", move);
      window.removeEventListener("pointerup", stop);
    };
  }, [drag, height, onMove, width]);

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
        {page.elements.map((element) => (
          <CanvasElement
            key={element.id}
            element={element}
            selected={element.id === selectedId}
            onSelect={() => onSelect(element.id)}
            onStartDrag={(event) => setDrag({ elementId: element.id, startX: event.clientX, startY: event.clientY, frame: element.frame })}
          />
        ))}
      </div>
      <div className="canvas-caption"><span className="caption-dot" /> KINDLE KT2 BASIC · PORTRAIT</div>
    </div>
  );
}

interface CanvasElementProps {
  element: DashboardElement;
  selected: boolean;
  onSelect: () => void;
  onStartDrag: (event: ReactPointerEvent<HTMLDivElement>) => void;
}

function CanvasElement({ element, selected, onSelect, onStartDrag }: CanvasElementProps) {
  const frame = element.frame;
  const style = element.style || {};
  const visualStyle: CSSProperties = {
    left: frame.x,
    top: frame.y,
    width: frame.width,
    height: frame.height,
    color: style.color || "#171717",
    background: element.type === "line" ? "transparent" : style.fill || "transparent",
    border: element.type === "line" ? "none" : `${style.border_width ?? 0}px solid ${style.stroke || "transparent"}`,
    borderRadius: style.radius ?? 0,
    fontSize: style.font_size || 16,
    fontWeight: style.font_weight || "normal",
    textAlign: style.align || "left"
  };
  return (
    <div
      className={`canvas-element element-${element.type} ${selected ? "is-selected" : ""}`}
      style={visualStyle}
      onPointerDown={(event) => { event.stopPropagation(); onSelect(); onStartDrag(event); }}
      role="button"
      tabIndex={0}
      aria-label={element.text || element.type}
    >
      {element.type === "line" ? <span className="line-visual" style={{ background: style.stroke || style.color || "#171717", height: style.border_width || 2 }} /> : null}
      {element.type === "image" || element.type === "image_button" ? (
        element.image?.src ? <img src={element.image.src} alt="" style={{ objectFit: element.image.fit === "stretch" ? "fill" : element.image.fit || "contain" }} /> : <span className="empty-image">未设置图片</span>
      ) : null}
      {element.type === "text" || element.type === "button" || element.type === "image_button" ? <span className="element-text">{element.text}</span> : null}
      {selected ? <span className="resize-handle" aria-hidden="true" /> : null}
    </div>
  );
}

function clamp(value: number, min: number, max: number) {
  return Math.max(min, Math.min(max, value));
}
