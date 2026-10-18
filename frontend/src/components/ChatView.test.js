import { fireEvent, render, screen } from '@testing-library/react';
import ChatView from './ChatView';

test('does not use the scroll result as an effect cleanup function', () => {
  const scrollIntoView = jest.fn(() => ({ animation: 'started' }));
  Element.prototype.scrollIntoView = scrollIntoView;

  const props = {
    input: '',
    loading: false,
    messages: [{ id: '1', role: 'user', content: 'Xin chào' }],
    onInputChange: jest.fn(),
    onSend: jest.fn(),
    onStop: jest.fn(),
  };

  const { rerender, unmount } = render(<ChatView {...props} />);
  rerender(<ChatView {...props} loading />);

  expect(scrollIntoView).toHaveBeenCalled();
  expect(() => unmount()).not.toThrow();
});

test('selects an allowed model in the composer', () => {
  const onModelChange = jest.fn();
  render(<ChatView input="" loading={false} messages={[]} models={[{ id: 'basic' }, { id: 'ag/gpt-oss-120b-medium', display_name: 'gpt-oss-120b' }]} selectedModel="basic" onModelChange={onModelChange} onInputChange={jest.fn()} onSend={jest.fn()} onStop={jest.fn()} />);
  fireEvent.click(screen.getByRole('button', { name: 'Chọn model' }));
  expect(screen.getByRole('menuitemradio', { name: 'basic' })).toHaveAttribute('aria-checked', 'true');
  fireEvent.click(screen.getByRole('menuitemradio', { name: 'gpt-oss-120b' }));
  expect(onModelChange).toHaveBeenCalledWith('ag/gpt-oss-120b-medium');
  expect(screen.queryByRole('menu')).not.toBeInTheDocument();
});
