import { useEffect, useState } from "react";
import type { RuntimeConfig } from "../types";

interface SettingsModalProps {
  runtime: RuntimeConfig | null;
  onClose: () => void;
  onSave: (haUrl: string, token: string) => Promise<void>;
}

export function SettingsModal({ runtime, onClose, onSave }: SettingsModalProps) {
  const [haUrl, setHaUrl] = useState(runtime?.ha_url || "");
  const [token, setToken] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  useEffect(() => setHaUrl(runtime?.ha_url || ""), [runtime?.ha_url]);
  const submit = async () => {
    if (!haUrl.trim()) { setError("请输入 Home Assistant 地址"); return; }
    setBusy(true); setError("");
    try { await onSave(haUrl.trim(), token); onClose(); } catch (reason) { setError(reason instanceof Error ? reason.message : "保存失败"); } finally { setBusy(false); }
  };
  return <div className="modal-backdrop" onMouseDown={(event) => { if (event.target === event.currentTarget) onClose(); }}><section className="modal-card"><div className="modal-heading"><div><h2>Home Assistant 设置</h2></div><button type="button" className="close-button" onClick={onClose}>×</button></div><p className="modal-intro">配置编辑器与 Home Assistant 的连接。Add-on 内运行时可以直接使用 Supervisor 凭据。</p><label className="field full"><span>Home Assistant 地址</span><input value={haUrl} onChange={(event) => setHaUrl(event.target.value)} placeholder="http://homeassistant.local:8123" /></label><label className="field full"><span>Long-Lived Access Token</span><input type="password" value={token} onChange={(event) => setToken(event.target.value)} placeholder={runtime?.token_configured ? "已保存，留空保持不变" : "粘贴长期访问令牌"} /></label><div className="settings-status"><span className={`status-bullet ${runtime?.ha_reachable ? "is-good" : ""}`} />{runtime?.ha_reachable ? "连接正常" : "尚未连接"}<span className="status-detail">{runtime?.auth_mode === "supervisor" ? "Supervisor" : runtime?.auth_mode === "token" ? "Access Token" : "未配置凭据"}</span></div>{error ? <p className="form-error">{error}</p> : null}<div className="modal-actions"><button type="button" className="quiet-button" onClick={onClose}>取消</button><button type="button" className="publish-button" disabled={busy} onClick={submit}>{busy ? "保存中…" : "保存设置"}</button></div></section></div>;
}
