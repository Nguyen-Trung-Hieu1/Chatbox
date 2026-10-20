import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import App from './App';
import { api } from './services/api';

jest.mock('./services/api', () => ({ api: jest.fn(), streamChat: jest.fn() }));

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
