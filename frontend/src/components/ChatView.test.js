import { render } from '@testing-library/react';
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
