import type { DashboardElement, EntitySummary } from "../types";

const fallbackModes = ["auto", "heat", "cool"];
const modeLabels: Record<string, string> = {
  auto: "自动",
  heat: "制热",
  cool: "制冷",
  dry: "除湿",
  fan_only: "送风",
  heat_cool: "自动"
};

export function ClimatePreview({ element, entity }: { element: DashboardElement; entity?: EntitySummary }) {
  const currentMode = (entity?.hvac_mode || entity?.state || "").trim().toLowerCase();
  const isOff = currentMode === "off" || entity?.state.trim().toLowerCase() === "off";
  const modes = (entity?.hvac_modes?.length ? entity.hvac_modes : fallbackModes)
    .map((mode) => mode.trim().toLowerCase())
    .filter((mode, index, values) => mode && mode !== "off" && values.indexOf(mode) === index)
    .slice(0, 6);

  return (
    <div className="climate-preview-shell">
      <div className="climate-preview-header">
        <span className="climate-preview-title"><span className="climate-preview-symbol">°</span><strong>{element.text || entity?.name || "温控"}</strong></span>
        <span className={`climate-preview-power ${isOff ? "is-off" : "is-on"}`}><span className="climate-preview-power-dot" />{isOff ? "开机" : "关机"}</span>
      </div>
      <div className="climate-preview-reading">
        <div className="climate-preview-metric"><small>室温</small><strong>{formatTemperature(entity?.current_temperature)}</strong></div>
        <div className="climate-preview-metric is-target"><small>目标温度</small><strong>{formatTemperature(entity?.temperature)}</strong></div>
      </div>
      <div className="climate-preview-temp-controls"><span>−</span><small>调温</small><span>＋</span></div>
      <div className="climate-preview-modes" style={{ gridTemplateColumns: `repeat(${Math.max(modes.length, 1)}, minmax(0, 1fr))` }}>
        {modes.length > 0 ? modes.map((mode) => <span key={mode} className={mode === currentMode ? "is-active" : ""}>{modeLabels[mode] || mode}</span>) : <span>暂无模式</span>}
      </div>
    </div>
  );
}

function formatTemperature(value: number | string | undefined): string {
  if (value === undefined || value === null || String(value).trim() === "") return "—";
  const text = String(value);
  return text.endsWith("°") ? text : `${text}°`;
}
