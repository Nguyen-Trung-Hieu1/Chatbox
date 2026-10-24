import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import AdminDashboard from './AdminDashboard';
import { api } from '../services/api';

jest.mock('../services/api', () => ({ api: jest.fn() }));

test('shows user plan and separate Web/API token totals', async () => {
  api.mockImplementation((path) => {
    if (path === '/api/admin/users') return Promise.resolve([{ id: 'u1', username: 'Gaudau', plan: 'paid', balance: 2000 }]);
    if (path === '/api/admin/usage?user_id=u1') return Promise.resolve({
      record_limit: 1000,
      top_model: { id: 'cx/gpt-5.6-sol', tokens: 180 },
      days: [{ date: '2026-10-23', web_input: 100, web_output: 20, api_input: 50, api_output: 10 }],
    });
    return Promise.reject(new Error('Unexpected API path'));
  });
  render(<AdminDashboard onBack={jest.fn()} onLogout={jest.fn()} />);
  expect((await screen.findAllByText('Gaudau')).length).toBeGreaterThan(0);
  expect(await screen.findByText('cx/gpt-5.6-sol')).toBeInTheDocument();
  expect(screen.getByText('120')).toBeInTheDocument();
  expect(screen.getByText('60')).toBeInTheDocument();
  fireEvent.change(screen.getByLabelText('Tìm người dùng'), { target: { value: 'không có' } });
  expect(screen.getByText('Không có người dùng phù hợp.')).toBeInTheDocument();
});

test('admin can switch a user from Free to Paid', async () => {
  api.mockImplementation((path, options) => {
    if (path === '/api/admin/users') return Promise.resolve([{ id: 'u1', username: 'Gaudau', plan: 'free', balance: 1000 }]);
    if (path === '/api/admin/usage?user_id=u1') return Promise.resolve({ days: [], record_limit: 1000 });
    if (path === '/api/admin/plan' && options.method === 'PATCH') return Promise.resolve({ id: 'u1', username: 'Gaudau', plan: 'paid', balance: 2000 });
    return Promise.reject(new Error('Unexpected API path'));
  });
  render(<AdminDashboard onBack={jest.fn()} onLogout={jest.fn()} />);
  fireEvent.click(await screen.findByRole('switch', { name: 'Gói Paid của Gaudau' }));
  await waitFor(() => expect(api).toHaveBeenCalledWith('/api/admin/plan', {
    method: 'PATCH', body: JSON.stringify({ user_id: 'u1', plan: 'paid' }),
  }));
  expect(await screen.findByText(/Paid trong 2 ngày/)).toBeInTheDocument();
});
