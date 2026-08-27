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
