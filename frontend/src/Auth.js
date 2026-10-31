import React, { useState } from 'react';
import LogoMark from './components/LogoMark';
import { startKeycloakLogin } from './keycloak';

export default function Auth({ error = '', onLegacyLogin }) {
  const [message, setMessage] = useState(error);
  const [showLegacy, setShowLegacy] = useState(false);
  const [username, setUsername] = useState('');
  const [password, setPassword] = useState('');
  const [busy, setBusy] = useState(false);
  const start = async (register) => {
    setMessage('');
    try { await startKeycloakLogin(register); }
    catch (err) { setMessage(err.message || 'Không thể mở trang đăng nhập'); }
  };
  return (
    <main className="auth-page">
      <div className="auth-logo"><LogoMark /><span>Chatbox</span></div>
      <section className="auth-card">
        <h1>Chào mừng bạn đến Chatbox</h1>
        {message && <div className="auth-message" role="alert">{message}</div>}
        <div className="auth-actions">
          <button className="auth-submit" type="button" onClick={() => start(false)}>Đăng nhập</button>
          <button className="auth-secondary" type="button" onClick={() => start(true)}>Tạo tài khoản</button>
          <button className="auth-secondary" type="button" onClick={() => { setMessage(''); setShowLegacy((value) => !value); }}>Đăng nhập bằng tài khoản cũ</button>
        </div>
        {showLegacy && <form className="legacy-login-form" onSubmit={async (event) => {
          event.preventDefault();
          if (!onLegacyLogin || busy) return;
          setBusy(true);
          setMessage('');
          try { await onLegacyLogin(username, password); }
          catch (err) { setMessage(err.message || 'Không thể đăng nhập tài khoản cũ'); }
          finally { setBusy(false); }
        }}>
          <label htmlFor="legacy-username">Tên đăng nhập cũ</label>
          <input id="legacy-username" autoComplete="username" value={username} onChange={(event) => setUsername(event.target.value)} required />
          <label htmlFor="legacy-password">Mật khẩu cũ</label>
          <input id="legacy-password" type="password" autoComplete="current-password" value={password} onChange={(event) => setPassword(event.target.value)} required />
          <button className="auth-submit" type="submit" disabled={busy}>{busy ? 'Đang đăng nhập...' : 'Vào Chatbox'}</button>
        </form>}
        <p className="auth-help">Quên mật khẩu? Chọn Đăng nhập, rồi bấm “Forgot password?” trên trang Keycloak.</p>
      </section>
      <p className="auth-footer">Bằng việc tiếp tục, bạn đồng ý với Điều khoản sử dụng và Chính sách quyền riêng tư.</p>
    </main>
  );
}
