import React, { useState } from 'react';
import LogoMark from './components/LogoMark';
import { startKeycloakLogin } from './keycloak';

export default function Auth({ error = '' }) {
  const [message, setMessage] = useState(error);
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
        </div>
        <p className="auth-help">Quên mật khẩu? Chọn Đăng nhập, rồi bấm “Forgot password?” trên trang Keycloak.</p>
      </section>
      <p className="auth-footer">Bằng việc tiếp tục, bạn đồng ý với Điều khoản sử dụng và Chính sách quyền riêng tư.</p>
    </main>
  );
}
