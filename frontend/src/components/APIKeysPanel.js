import React, { useEffect, useState } from 'react';
import { api } from '../services/api';

export default function APIKeysPanel({ onClose, models = [] }) {
  const [keys, setKeys] = useState([]);
  const [name, setName] = useState('');
  const [newKey, setNewKey] = useState('');
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  const apiOrigin = process.env.REACT_APP_API_URL || 'http://localhost:8080';
  const baseURL = `${new URL(apiOrigin.replace(/\/$/, ''), window.location.origin).href.replace(/\/$/, '')}/v1`;

  useEffect(() => {
    api('/api/keys').then(setKeys).catch((err) => setError(err.message));
  }, []);

  useEffect(() => {
    const onEscape = (event) => { if (event.key === 'Escape') onClose(); };
    document.addEventListener('keydown', onEscape);
    return () => document.removeEventListener('keydown', onEscape);
  }, [onClose]);

  const createKey = async (event) => {
    event.preventDefault();
    if (!name.trim() || busy) return;
    setBusy(true);
    setError('');
    try {
      const created = await api('/api/keys', { method: 'POST', body: JSON.stringify({ name: name.trim() }) });
      setKeys((current) => [created.item, ...current]);
      setNewKey(created.key);
      setName('');
    } catch (err) {
      setError(err.message);
    } finally {
      setBusy(false);
    }
  };

  const revokeKey = async (item) => {
    if (!window.confirm(`Thu hồi API key “${item.name}”? IDE đang dùng key này sẽ không gọi được nữa.`)) return;
    setBusy(true);
    setError('');
    try {
      await api(`/api/keys?id=${encodeURIComponent(item.id)}`, { method: 'DELETE' });
      setKeys((current) => current.map((key) => key.id === item.id ? { ...key, revoked_at: new Date().toISOString() } : key));
      setNewKey('');
    } catch (err) {
      setError(err.message);
    } finally {
      setBusy(false);
    }
  };

  return (
    <div className="api-keys-overlay" onMouseDown={(event) => { if (event.target === event.currentTarget) onClose(); }}>
      <section className="api-keys-panel" role="dialog" aria-modal="true" aria-label="Quản lý API key">
        <div className="api-keys-heading">
          <div><h2>API keys</h2><p>Dùng Chatbox trong IDE hoặc ứng dụng hỗ trợ OpenAI API.</p></div>
          <button type="button" onClick={onClose} aria-label="Đóng">×</button>
        </div>
        <div className="api-keys-base"><span>Base URL</span><code>{baseURL}</code></div>
        <div className="api-keys-models"><span>Model ID được phép dùng</span>{models.map((model) => <code key={model.id}>{model.id}</code>)}</div>
        <form className="api-keys-create" onSubmit={createKey}>
          <label htmlFor="api-key-name">Tên key</label>
          <div><input id="api-key-name" value={name} maxLength={80} onChange={(event) => setName(event.target.value)} placeholder="Ví dụ: VS Code" /><button type="submit" disabled={busy || !name.trim()}>Tạo key</button></div>
        </form>
        {newKey && <div className="api-keys-new" role="status"><strong>Sao chép key ngay — key chỉ hiện một lần.</strong><code>{newKey}</code><button type="button" onClick={() => navigator.clipboard.writeText(newKey).catch(() => setError('Không thể sao chép tự động. Hãy chọn và sao chép key ở trên.'))}>Sao chép</button></div>}
        {error && <p className="api-keys-error" role="alert">{error}</p>}
        <h3>Key đã tạo</h3>
        <div className="api-keys-list">
          {keys.length === 0 && <p>Chưa có API key.</p>}
          {keys.map((item) => <div className="api-keys-item" key={item.id}><div><strong>{item.name}</strong><span>{item.prefix}… · {item.revoked_at ? 'Đã thu hồi' : 'Đang hoạt động'}</span></div>{!item.revoked_at && <button type="button" disabled={busy} onClick={() => revokeKey(item)}>Thu hồi</button>}</div>)}
        </div>
      </section>
    </div>
  );
}
