import React, { useState } from 'react';
import LogoMark from './components/LogoMark';

export default function Auth({ onLoginSuccess, api }) {
  const [mode, setMode] = useState('login');
  const [username, setUsername] = useState('');
  const [password, setPassword] = useState('');
  const [message, setMessage] = useState('');
  const [submitting, setSubmitting] = useState(false);
  const changeMode = (next) => {
    setMode(next);
    setUsername('');
    setPassword('');
    setMessage('');
  };
  const submit = async (event) => {
    event.preventDefault();
    setMessage('');
    if (!username.trim() || !password) {
      return setMessage('Vui lòng điền đầy đủ thông tin');
    }
    setSubmitting(true);
    try {
      if (mode === 'login') {
        const data = await api('/api/login', { method: 'POST', body: JSON.stringify({ username, password }) });
        onLoginSuccess(data.user);
      } else {
        await api('/api/register', { method: 'POST', body: JSON.stringify({ username, password }) });
        setPassword('');
        setMode('login');
        setMessage('Đăng ký thành công. Hãy đăng nhập.');
      }
    } catch (err) {
      setMessage(err.message || 'Không thể kết nối đến máy chủ');
    } finally {
      setSubmitting(false);
    }
  };
  const success = message.startsWith('Đăng ký thành công');
  return (
    <main className="auth-page">
      <div className="auth-logo">
        <LogoMark />
        <span>Chatbox</span>
      </div>
      <section className="auth-card">
        <h1>
          {mode === 'login' ? 'Chào mừng bạn trở lại' : 'Tạo tài khoản'}
        </h1>
        <div className="auth-tabs" role="tablist">
          <button
            className={mode === 'login' ? 'active' : ''}
            onClick={() => changeMode('login')}
            type="button"
          >
            Đăng nhập
          </button>
          <button
            className={mode === 'register' ? 'active' : ''}
            onClick={() => changeMode('register')}
            type="button"
          >
            Đăng ký
          </button>
        </div>
        {message && (
          <div
            className={`auth-message ${success ? 'success' : ''}`}
            role="alert"
          >
            {message}
          </div>
        )}
        <form onSubmit={submit} autoComplete="off">
          <label htmlFor="username">Tên đăng nhập</label>
          <input
            id="username"
            name="chatbox-username"
            autoComplete="off"
            placeholder="Tên đăng nhập"
            value={username}
            onChange={(e) => setUsername(e.target.value)}
          />
          <label htmlFor="password">Mật khẩu</label>
          <input
            id="password"
            name="chatbox-password"
            type="password"
            autoComplete="new-password"
            placeholder="Mật khẩu"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
          />
          <button
            className="auth-submit"
            type="submit"
            disabled={submitting}
          >
            {submitting
              ? 'Đang xử lý...'
              : mode === 'login'
                ? 'Tiếp tục'
                : 'Tạo tài khoản'}
          </button>
        </form>
      </section>
      <p className="auth-footer">
        Bằng việc tiếp tục, bạn đồng ý với Điều khoản sử dụng và Chính sách quyền
        riêng tư.
      </p>
    </main>
  );
}
