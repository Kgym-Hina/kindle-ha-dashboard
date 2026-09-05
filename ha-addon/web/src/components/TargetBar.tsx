import type { DashboardMode, ZoneInfo } from "../types";

interface TargetBarProps {
  mode: DashboardMode;
  zoneId: string | null;
  zones: ZoneInfo[];
  onModeChange: (mode: DashboardMode) => void;
  onZoneChange: (zoneId: string | null) => void;
  onRefreshZones: () => void;
}

export function TargetBar({ mode, zoneId, zones, onModeChange, onZoneChange, onRefreshZones }: TargetBarProps) {
  return (
    <section className="target-bar panel-card">
      <div className="target-heading">
        <strong className="target-title">显示目标</strong>
        <span className="target-size">600 × 800 px</span>
      </div>
      <div className="target-controls">
        <div className="segmented-control" role="group" aria-label="发布目标">
          <button type="button" className={mode === "portable" ? "is-active" : ""} onClick={() => onModeChange("portable")}>Portable</button>
          <button type="button" className={mode === "zone" ? "is-active" : ""} onClick={() => onModeChange("zone")}>房间</button>
        </div>
        {mode === "zone" ? (
          <div className="zone-select-wrap">
            <select value={zoneId || ""} onChange={(event) => onZoneChange(event.target.value || null)} aria-label="选择房间">
              <option value="">选择 Home Assistant 房间</option>
              {zones.map((zone) => <option key={zone.zone_id} value={zone.zone_id}>{zone.name}</option>)}
            </select>
            <button className="icon-button" type="button" onClick={onRefreshZones} aria-label="刷新房间" title="刷新房间">↻</button>
          </div>
        ) : (
          <span className="target-note">所有未匹配房间的 Kindle 使用这份界面</span>
        )}
      </div>
    </section>
  );
}
