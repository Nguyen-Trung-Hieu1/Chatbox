import React, { useEffect, useMemo, useState } from 'react';
import { api } from '../services/api';
import './AdminDashboard.css';

const number = (value) => new Intl.NumberFormat('vi-VN').format(value || 0);
const total = (day, prefix) => day[`${prefix}_input`] + day[`${prefix}_output`];

export default function AdminDashboard({ onBack, onLogout }) {
  const [users, setUsers] = useState([]);
  const [selectedId, setSelectedId] = useState('');
  const [report, setReport] = useState(null);
  const [query, setQuery] = useState('');
  const [loading, setLoading] = useState(true);
  const [reportLoading, setReportLoading] = useState(false);
  const [error, setError] = useState('');
  const [savingPlan, setSavingPlan] = useState(false);

  useEffect(() => {
    let active = true;
    api('/api/admin/users').then((items) => {
      if (!active) return;
      setUsers(items);
      setSelectedId((current) => current || items[0]?.id || '');
    }).catch((err) => { if (active) setError(err.message); })
      .finally(() => { if (active) setLoading(false); });
    return () => { active = false; };
  }, []);

  useEffect(() => {
    if (!selectedId) return undefined;
    let active = true;
    setReport(null);
    setReportLoading(true);
    setError('');
    api(`/api/admin/usage?user_id=${encodeURIComponent(selectedId)}`)
      .then((data) => { if (active) setReport(data); })
      .catch((err) => { if (active) setError(err.message); })
      .finally(() => { if (active) setReportLoading(false); });
    return () => { active = false; };
  }, [selectedId]);

  const filtered = useMemo(() => users.filter((item) => item.username.toLocaleLowerCase('vi').includes(query.trim().toLocaleLowerCase('vi'))), [users, query]);
  const selected = users.find((item) => item.id === selectedId);
  const changePlan = async () => {
    if (!selected || savingPlan) return;
    const nextPlan = selected.plan === 'paid' ? 'free' : 'paid';
    setSavingPlan(true);
    setError('');
    try {
      const account = await api('/api/admin/plan', {
        method: 'PATCH',
        body: JSON.stringify({ user_id: selected.id, plan: nextPlan }),
      });
      setUsers((current) => current.map((item) => item.id === account.id ? account : item));
    } catch (err) {
      setError(err.message);
    } finally {
      setSavingPlan(false);
    }
  };
  const days = report?.days || [];
  const webTokens = days.reduce((sum, day) => sum + total(day, 'web'), 0);
  const apiTokens = days.reduce((sum, day) => sum + total(day, 'api'), 0);
  const chartDays = days.slice(0, 14).reverse();
  const chartMax = Math.max(1, ...chartDays.map((day) => total(day, 'web') + total(day, 'api')));

  return (
    <div className="admin-page">
      <header className="admin-header">
        <div><span className="admin-eyebrow">CHATBOX</span><h1>Quản trị sử dụng</h1></div>
        <div className="admin-header-actions"><button type="button" onClick={onBack}>← Về Chatbox</button><button type="button" onClick={onLogout}>Đăng xuất</button></div>
      </header>
      <main className="admin-layout">
        <aside className="admin-users">
          <div className="admin-section-heading"><h2>Người dùng</h2><span>{number(users.length)}</span></div>
          <input aria-label="Tìm người dùng" value={query} onChange={(event) => setQuery(event.target.value)} placeholder="Tìm tên tài khoản" />
          {loading && <p className="admin-muted">Đang tải người dùng…</p>}
          {!loading && filtered.length === 0 && <p className="admin-muted">Không có người dùng phù hợp.</p>}
          <div className="admin-user-list">
            {filtered.map((item) => <button type="button" key={item.id} className={`admin-user ${item.id === selectedId ? 'active' : ''}`} onClick={() => setSelectedId(item.id)}>
              <span className="admin-user-name">{item.username}</span>
              <span className={`admin-plan ${item.plan}`}>{item.plan === 'paid' ? 'Paid' : 'Free'}</span>
            </button>)}
          </div>
        </aside>
        <section className="admin-detail">
          {error && <div className="admin-error" role="alert">{error}</div>}
          {!selected && !loading && <div className="admin-empty">Chọn một người dùng để xem thống kê.</div>}
          {selected && <>
            <div className="admin-detail-heading"><div><span className="admin-eyebrow">THỐNG KÊ TÀI KHOẢN</span><h2>{selected.username}</h2></div><span className={`admin-plan ${selected.plan}`}>{selected.plan === 'paid' ? 'Paid' : 'Free'}</span></div>
            <div className="admin-plan-control">
              <div><strong>Gói tài khoản</strong><small>{selected.plan === 'paid' ? 'Paid trong 2 ngày kể từ lúc chuyển gói; 2.000 credit mỗi 2 giờ' : 'Free; 1.000 credit mỗi 2 giờ'}</small></div>
              <label className="admin-plan-switch"><span>Free</span><input type="checkbox" role="switch" aria-label={`Gói Paid của ${selected.username}`} checked={selected.plan === 'paid'} disabled={savingPlan} onChange={changePlan} /><span className="admin-switch-track" aria-hidden="true" /><span>Paid</span></label>
            </div>
            {reportLoading && <p className="admin-muted">Đang tải thống kê…</p>}
            {report && <>
              <div className="admin-metrics">
                <div><span>Token Web</span><strong>{number(webTokens)}</strong><small>Chat trên trang web</small></div>
                <div><span>Token API</span><strong>{number(apiTokens)}</strong><small>Gọi qua API key</small></div>
                <div><span>Model dùng nhiều nhất</span><strong className="admin-model-name">{report.top_model?.id || 'Chưa có'}</strong><small>{report.top_model ? `${number(report.top_model.tokens)} token` : 'Chưa có lượt sử dụng'}</small></div>
              </div>
              {days.length === 0 ? <div className="admin-empty">Tài khoản này chưa có dữ liệu sử dụng.</div> : <>
                <div className="admin-chart-card"><div className="admin-section-heading"><h3>14 ngày có sử dụng gần nhất</h3><span>GMT+7</span></div>
                  <div className="admin-legend"><span><i className="web" />Web</span><span><i className="api" />API</span></div>
                  <div className="admin-chart">{chartDays.map((day) => <div className="admin-chart-column" key={day.date} title={`${day.date}: Web ${number(total(day, 'web'))}, API ${number(total(day, 'api'))}`}>
                    <div className="admin-chart-bar"><div className="api" style={{ height: `${100 * total(day, 'api') / chartMax}%` }} /><div className="web" style={{ height: `${100 * total(day, 'web') / chartMax}%` }} /></div>
                    <span>{day.date.slice(5)}</span>
                  </div>)}</div>
                </div>
                <div className="admin-table-card"><div className="admin-section-heading"><h3>Token theo ngày</h3><span>GMT+7</span></div>
                  <div className="admin-table-scroll"><table><thead><tr><th>Ngày</th><th>Web vào</th><th>Web ra</th><th>API vào</th><th>API ra</th><th>Tổng</th></tr></thead><tbody>
                    {days.map((day) => <tr key={day.date}><td>{day.date}</td><td>{number(day.web_input)}</td><td>{number(day.web_output)}</td><td>{number(day.api_input)}</td><td>{number(day.api_output)}</td><td>{number(total(day, 'web') + total(day, 'api'))}</td></tr>)}
                  </tbody></table></div>
                </div>
              </>}
              <p className="admin-footnote">Thống kê dựa trên tối đa {number(report.record_limit)} bản ghi sử dụng gần nhất được lưu trong ví. Web gồm cả token dùng để tóm tắt cuộc trò chuyện.</p>
            </>}
          </>}
        </section>
      </main>
    </div>
  );
}
