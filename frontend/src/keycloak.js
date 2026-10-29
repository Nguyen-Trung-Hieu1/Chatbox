const issuer = (process.env.REACT_APP_KEYCLOAK_ISSUER || 'https://auth.chatbox.cyou/realms/chatbox').replace(/\/$/, '');
const clientId = process.env.REACT_APP_KEYCLOAK_CLIENT_ID || 'chatbox-web';
const callbackURL = `${window.location.origin}/auth/callback`;
const pendingKey = 'chatbox:oidc:pending';

function randomURLSafe(length = 32) {
  const bytes = new Uint8Array(length);
  window.crypto.getRandomValues(bytes);
  return btoa(String.fromCharCode(...bytes)).replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/, '');
}

function base64URL(bytes) {
  return btoa(String.fromCharCode(...new Uint8Array(bytes))).replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/, '');
}

export async function startKeycloakLogin(register = false) {
  const state = randomURLSafe();
  const nonce = randomURLSafe();
  const verifier = randomURLSafe(48);
  const digest = await window.crypto.subtle.digest('SHA-256', new TextEncoder().encode(verifier));
  const challenge = base64URL(digest);
  window.sessionStorage.setItem(pendingKey, JSON.stringify({ state, nonce, verifier, created: Date.now() }));
  const endpoint = register ? 'registrations' : 'auth';
  const url = new URL(`${issuer}/protocol/openid-connect/${endpoint}`);
  Object.entries({ client_id: clientId, redirect_uri: callbackURL, response_type: 'code', scope: 'openid profile email', state, nonce, code_challenge: challenge, code_challenge_method: 'S256' })
    .forEach(([key, value]) => url.searchParams.set(key, value));
  window.location.assign(url.toString());
}

export async function finishKeycloakLogin(api) {
  const params = new URLSearchParams(window.location.search);
  const raw = window.sessionStorage.getItem(pendingKey);
  window.sessionStorage.removeItem(pendingKey);
  window.history.replaceState({}, '', '/');
  if (!raw || params.has('error')) throw new Error('Đăng nhập Keycloak không thành công. Vui lòng thử lại.');
  const pending = JSON.parse(raw);
  if (params.get('state') !== pending.state || Date.now() - pending.created > 5 * 60 * 1000 || !params.get('code')) {
    throw new Error('Phiên đăng nhập không hợp lệ hoặc đã hết hạn.');
  }
  const response = await fetch(`${issuer}/protocol/openid-connect/token`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
    body: new URLSearchParams({ grant_type: 'authorization_code', client_id: clientId, redirect_uri: callbackURL, code: params.get('code'), code_verifier: pending.verifier }),
  });
  if (!response.ok) throw new Error('Không thể hoàn tất đăng nhập Keycloak.');
  const tokens = await response.json();
  if (!tokens.id_token) throw new Error('Keycloak không trả về ID token.');
  return api('/api/keycloak/login', { method: 'POST', body: JSON.stringify({ id_token: tokens.id_token, nonce: pending.nonce }) });
}

export function logoutKeycloak() {
  const url = new URL(`${issuer}/protocol/openid-connect/logout`);
  url.searchParams.set('client_id', clientId);
  url.searchParams.set('post_logout_redirect_uri', `${window.location.origin}/`);
  window.location.assign(url.toString());
}
