const API_URL = process.env.REACT_APP_API_URL || 'http://localhost:8080';

export async function api(path, options = {}) {
  const response = await fetch(`${API_URL}${path}`, {
    ...options,
    credentials: 'include',
    headers: options.body
      ? { 'Content-Type': 'application/json', ...options.headers }
      : options.headers,
  });
  const data =
    response.status === 204 ? {} : await response.json().catch(() => ({}));
  if (!response.ok) {
    const error = new Error(data.error || 'Đã có lỗi xảy ra');
    error.status = response.status;
    throw error;
  }
  return data;
}

export async function streamChat(payload, { signal, onDelta }) {
  const response = await fetch(`${API_URL}/api/chat`, {
    method: 'POST',
    credentials: 'include',
    signal,
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(payload),
  });
  if (!response.ok) {
    const data = await response.json().catch(() => ({}));
    const error = new Error(data.error || 'Đã có lỗi xảy ra');
    error.status = response.status;
    throw error;
  }
  if (!response.body) throw new Error('Trình duyệt không hỗ trợ streaming');
  const reader = response.body.getReader();
  const decoder = new TextDecoder();
  let buffer = '';
  let completedMessage = null;
  while (true) {
    const { value, done } = await reader.read();
    buffer += decoder.decode(value || new Uint8Array(), { stream: !done });
    const lines = buffer.split('\n');
    buffer = lines.pop() || '';
    for (const line of lines) {
      if (!line.trim()) continue;
      const event = JSON.parse(line);
      if (event.type === 'delta') onDelta(event.content);
      if (event.type === 'error') throw new Error(event.error || 'Luồng trả lời bị gián đoạn');
      if (event.type === 'done') completedMessage = event.message;
    }
    if (done) break;
  }
  if (!completedMessage) throw new Error('Luồng trả lời kết thúc không hoàn chỉnh');
  return completedMessage;
}
