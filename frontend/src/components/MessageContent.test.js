import { act, fireEvent, render, screen } from '@testing-library/react';
import MessageContent from './MessageContent';

test('renders fenced code with a language label and copy action', async () => {
  const writeText = jest.fn().mockResolvedValue();
  Object.defineProperty(navigator, 'clipboard', { configurable: true, value: { writeText } });
  render(<MessageContent content={'Đoạn mã:\n\n```js\nconst greeting = "Xin chào";\n```'} />);

  expect(screen.getByText('JavaScript')).toBeInTheDocument();
  expect(document.querySelector('.hljs-keyword')).toHaveTextContent('const');
  await act(async () => { fireEvent.click(screen.getByRole('button', { name: 'Sao chép mã nguồn' })); });
  expect(writeText).toHaveBeenCalledWith('const greeting = "Xin chào";');
  expect(screen.getByRole('button', { name: 'Đã sao chép mã nguồn' })).toBeInTheDocument();
});

test('does not render raw HTML from an assistant message', () => {
  const { container } = render(<MessageContent content={'<script>alert("xss")</script>'} />);
  expect(container.querySelector('script')).not.toBeInTheDocument();
  expect(screen.getByText(/script/)).toBeInTheDocument();
});
