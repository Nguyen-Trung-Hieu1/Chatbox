import { fireEvent, render, screen } from '@testing-library/react';
import Sidebar from './Sidebar';

const conversations = [
  { id: '1', title: 'Kế hoạch du lịch' },
  { id: '2', title: 'Học React' },
];

function renderSidebar(overrides = {}) {
  const props = {
    conversations, currentId: '1', currentUser: { username: 'tester' }, isOpen: true,
    onClose: jest.fn(), onDelete: jest.fn(), onLogout: jest.fn(), onNewChat: jest.fn(),
    onRename: jest.fn().mockResolvedValue(), onSelect: jest.fn(), ...overrides,
  };
  return { ...render(<Sidebar {...props} />), props };
}

test('filters conversation history by title', () => {
  renderSidebar();
  fireEvent.change(screen.getByLabelText('Tìm kiếm lịch sử trò chuyện'), { target: { value: 'react' } });
  expect(screen.getByText('Học React')).toBeInTheDocument();
  expect(screen.queryByText('Kế hoạch du lịch')).not.toBeInTheDocument();
});

test('opens conversation actions and asks before deleting', () => {
  const confirm = jest.spyOn(window, 'confirm').mockReturnValue(false);
  const { props } = renderSidebar();
  fireEvent.click(screen.getByLabelText('Tùy chọn cho Kế hoạch du lịch'));
  fireEvent.click(screen.getByText('Xóa'));
  expect(confirm).toHaveBeenCalled();
  expect(props.onDelete).not.toHaveBeenCalled();
  confirm.mockRestore();
});

test('shows the account plan below the username', () => {
  renderSidebar({ plan: 'paid', creditBalance: 9750 });
  expect(screen.getByText('tester')).toBeInTheDocument();
  expect(screen.getByText('Paid')).toBeInTheDocument();
  expect(screen.queryByText('9.750')).not.toBeInTheDocument();
  fireEvent.click(screen.getByText('Xem chi tiết gói'));
  expect(screen.getByText('9.750')).toBeInTheDocument();
});

test('shows the upgrade button for free accounts', () => {
  const { props } = renderSidebar({ plan: 'free', creditResetAt: '2026-10-20T02:00:00Z', now: Date.parse('2026-10-20T00:00:00Z'), onUpgrade: jest.fn() });
  fireEvent.click(screen.getByText('Xem chi tiết gói'));
  expect(screen.getByText('Reset credit sau')).toBeInTheDocument();
  expect(screen.getByText('2 giờ 0 phút')).toBeInTheDocument();
  fireEvent.click(screen.getByText('Nâng gói Paid (2 ngày)'));
  expect(props.onUpgrade).toHaveBeenCalledTimes(1);
});

test('shows remaining time and credit reset for paid accounts', () => {
  renderSidebar({ plan: 'paid', paidUntil: '2026-10-21T00:00:00Z', creditResetAt: '2026-10-20T02:00:00Z', now: Date.parse('2026-10-20T00:00:00Z') });
  expect(screen.queryByText('1 ngày 0 giờ 0 phút')).not.toBeInTheDocument();
  fireEvent.click(screen.getByText('Xem chi tiết gói'));
  expect(screen.getByText('1 ngày 0 giờ 0 phút')).toBeInTheDocument();
  expect(screen.getByText('Reset credit sau')).toBeInTheDocument();
  expect(screen.getByText('2 giờ 0 phút')).toBeInTheDocument();
  expect(screen.queryByText('Nâng gói Paid (2 ngày)')).not.toBeInTheDocument();
});
