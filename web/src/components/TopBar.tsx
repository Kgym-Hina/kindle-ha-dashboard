import type { RuntimeConfig } from "../types";

interface TopBarProps {
  runtime: RuntimeConfig | null;
  busy: boolean;
  onPublish: () => void;
  onSettings: () => void;
  onMessage: () => void;
  onExport: () => void;
}

export function TopBar({ runtime, busy, onPublish, onSettings, onMessage, onExport }: TopBarProps) {
  const connected = runtime?.ha_reachable;
  return (
    <header className="top-bar">
      <div className="brand-lockup">
        <div className="brand-mark" aria-hidden="true">K</div>
        <div>
          <h1>Kindle Dashboard</h1>
          <p>Home Assistant · E-ink studio</p>
        </div>
      </div>
      <div className="top-actions">
        <span className={`connection-pill ${connected ? "is-connected" : ""}`}>
          <span className="connection-dot" />
          {connected ? "HA 已连接" : "等待 HA"}
        </span>
        <button className="quiet-button" type="button" onClick={onMessage}>推送消息</button>
        <button className="quiet-button" type="button" onClick={onExport}>导出 JSON</button>
        <button className="quiet-button" type="button" onClick={onSettings}>设置</button>
        <button className="publish-button" type="button" onClick={onPublish} disabled={busy}>
          {busy ? "发布中…" : "发布到 HA"}
          <span aria-hidden="true">↗</span>
        </button>
      </div>
    </header>
  );
}

