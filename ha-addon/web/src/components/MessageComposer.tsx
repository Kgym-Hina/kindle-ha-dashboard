import { useState } from "react";
import type { KindleMessage } from "../types";

interface MessageComposerProps {
  onClose: () => void;
  onSend: (message: KindleMessage) => Promise<void>;
}

export function MessageComposer({ onClose, onSend }: MessageComposerProps) {
  const [title, setTitle] = useState("Home Assistant");
  const [message, setMessage] = useState("");
  const [device, setDevice] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const submit = async () => {
    if (!message.trim()) { setError("请输入消息内容"); return; }
    setBusy(true); setError("");
    try { await onSend({ title: title.trim() || "Home Assistant", message: message.trim(), target_device_id: device.trim() || undefined, timeout_ms: 10000 }); onClose(); }
    catch (reason) { setError(reason instanceof Error ? reason.message : "发送失败"); }
    finally { setBusy(false); }
  };
  return <div className="modal-backdrop" onMouseDown={(event) => { if (event.target === event.currentTarget) onClose(); }}><section className="modal-card message-modal"><div className="modal-heading"><div><h2>推送到 Kindle</h2></div><button type="button" className="close-button" onClick={onClose}>×</button></div><p className="modal-intro">发送后，目标 Kindle 会立即显示一张消息卡片。</p><label className="field full"><span>标题</span><input value={title} onChange={(event) => setTitle(event.target.value)} /></label><label className="field full"><span>消息</span><textarea rows={4} value={message} onChange={(event) => setMessage(event.target.value)} placeholder="输入要显示在 Kindle 上的内容" /></label><label className="field full"><span>指定设备（可选）</span><input value={device} onChange={(event) => setDevice(event.target.value)} placeholder="留空发送给所有 Kindle" /></label>{error ? <p className="form-error">{error}</p> : null}<div className="modal-actions"><button type="button" className="quiet-button" onClick={onClose}>取消</button><button type="button" className="publish-button" disabled={busy} onClick={submit}>{busy ? "发送中…" : "立即发送 ↗"}</button></div></section></div>;
}
