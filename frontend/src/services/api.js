const API_URL = process.env.REACT_APP_API_URL || 'http://localhost:8080';
let accessToken = null;
let refreshInFlight = null;

export function setAccessToken(token) {
  accessToken = token || null;
}

export function clearAccessToken() {
  accessToken = null;
}

export async function refreshSession() {
  if (!refreshInFlight) {
    refreshInFlight = fetch(`${API_URL}/api/auth/refresh`, {
      method: 'POST',
      credentials: 'include',
      headers: { 'Content-Type': 'application/json' },
    }).then(async (response) => {
      if (!response.ok) {
        clearAccessToken();
        return null;
      }
      const data = await response.json();
      setAccessToken(data.access_token);
      return data;
    }).catch(() => {
      clearAccessToken();
      return null;
    }).finally(() => { refreshInFlight = null; });
  }
  return refreshInFlight;
}

async function authenticatedFetch(url, options = {}, retry = true) {
  const headers = { ...options.headers };
  if (accessToken) headers.Authorization = `Bearer ${accessToken}`;
  const response = await fetch(url, { ...options, headers, credentials: 'include' });
  if (response.status === 401 && retry && accessToken) {
    const session = await refreshSession();
    if (session?.access_token) return authenticatedFetch(url, options, false);
  }
  return response;
}

export async function api(path, options = {}) {
  const response = await authenticatedFetch(`${API_URL}${path}`, {
    ...options,
    headers: options.body
      ? { 'Content-Type': 'application/json', ...options.headers }
      : { ...options.headers },
  }, !path.startsWith('/api/auth/'));
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
  const options = {
    method: 'POST',
    signal,
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(payload),
  };
  const response = await authenticatedFetch(`${API_URL}/api/chat`, options);
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
