import React from 'react';
import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import App from './App';
import Auth from './Auth';
import { api } from './services/api';
import { finishKeycloakLogin } from './keycloak';

jest.mock('./services/api', () => ({ api: jest.fn(), streamChat: jest.fn() }));
jest.mock('./keycloak', () => ({ finishKeycloakLogin: jest.fn(), logoutKeycloak: jest.fn() }));

beforeEach(() => {
  window.localStorage.clear();
  api.mockImplementation((path) => {
    if (path === '/api/me') return Promise.resolve({ id: 'user-1', username: 'tester' });
    if (path === '/api/conversations') return Promise.resolve([]);
    if (path === '/api/entitlement') return Promise.resolve({
      plan: 'paid', balance: 2000, models: [
        { id: 'ag/basic', display_name: 'Basic' },
        { id: 'cx/gpt-5.6-sol', display_name: 'Sol' },
      ],
    });
    return Promise.resolve({});
  });
});

test('restores the selected model after remounting', async () => {
  const first = render(<App />);
  fireEvent.click(await screen.findByRole('button', { name: 'Chọn model' }));
  fireEvent.click(screen.getByRole('menuitemradio', { name: 'Sol' }));
  expect(window.localStorage.getItem('chatbox:selected-model:user-1')).toBe('cx/gpt-5.6-sol');
  first.unmount();

  render(<App />);
  await waitFor(() => expect(screen.getByRole('button', { name: 'Chọn model' })).toHaveTextContent('Sol'));
});

test('explains why a regular account cannot open /admin', async () => {
  window.history.replaceState({}, '', '/admin');
  render(<App />);
  expect(await screen.findByText('Không có quyền truy cập')).toBeInTheDocument();
  expect(screen.getByText('Tài khoản tester chưa được cấp quyền quản trị.')).toBeInTheDocument();
  window.history.replaceState({}, '', '/');
});

test('handles a Keycloak callback only once in React StrictMode', async () => {
  window.history.replaceState({}, '', '/auth/callback?code=sample');
  finishKeycloakLogin.mockResolvedValue({ user: { id: 'user-1', username: 'tester' } });
  render(<React.StrictMode><App /></React.StrictMode>);
  await screen.findByText('tester');
  expect(finishKeycloakLogin).toHaveBeenCalledTimes(1);
  expect(api).not.toHaveBeenCalledWith('/api/me');
  window.history.replaceState({}, '', '/');
});

test('shows only the Keycloak sign-in options', () => {
  render(<Auth />);
  expect(screen.getByRole('button', { name: 'Đăng nhập' })).toBeInTheDocument();
  expect(screen.getByRole('button', { name: 'Tạo tài khoản' })).toBeInTheDocument();
  expect(screen.queryByText('Đăng nhập tài khoản cũ')).not.toBeInTheDocument();
});
