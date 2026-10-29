import React, { useEffect, useRef, useState } from 'react';
import Auth from './Auth';
import { finishKeycloakLogin, logoutKeycloak } from './keycloak';
import ChatHeader from './components/ChatHeader';
import ChatView from './components/ChatView';
import Sidebar from './components/Sidebar';
import APIKeysPanel from './components/APIKeysPanel';
import AdminDashboard from './components/AdminDashboard';
import { api, streamChat } from './services/api';
import './App.css';

const modelStorageKey = (userId) => `chatbox:selected-model:${userId}`;

function savedModel(userId) {
  try { return window.localStorage.getItem(modelStorageKey(userId)) || ''; }
  catch (_) { return ''; }
}

function rememberModel(userId, modelId) {
  try { window.localStorage.setItem(modelStorageKey(userId), modelId); }
  catch (_) { /* Storage can be disabled in private browsing. */ }
}

function App() {
  const [currentUser, setCurrentUser] = useState(null);
  const [checkingSession, setCheckingSession] = useState(true);
  const [loginError, setLoginError] = useState('');
  const [conversations, setConversations] = useState([]);
  const [currentId, setCurrentId] = useState('');
  const [messages, setMessages] = useState([]);
  const [input, setInput] = useState('');
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');
  const [entitlement, setEntitlement] = useState(null);
  const [now, setNow] = useState(Date.now());
  const [selectedModel, setSelectedModel] = useState('');
  const [showAPIKeys, setShowAPIKeys] = useState(false);
  const [showAdmin, setShowAdmin] = useState(() => window.location.pathname === '/admin');
  const [sidebarOpen, setSidebarOpen] = useState(() => window.innerWidth > 768);
  const streamController = useRef(null);
  const sessionCheckStarted = useRef(false);

  const stopGenerating = () => streamController.current?.abort();
  const resetToLogin = () => {
    stopGenerating();
    setCurrentUser(null);
    setConversations([]);
    setCurrentId('');
    setMessages([]);
    setEntitlement(null);
    setSelectedModel('');
    setShowAPIKeys(false);
    setShowAdmin(false);
  };
  const handleApiError = (err) => {
    if (err.status === 401) resetToLogin();
    setError(err.message || 'Không thể kết nối đến máy chủ');
  };
  const fetchConversations = async () => {
    try {
      setConversations(await api('/api/conversations'));
    } catch (err) {
      handleApiError(err);
    }
  };
  const fetchEntitlement = async () => {
    try {
      const next = await api('/api/entitlement');
      setEntitlement(next);
      setSelectedModel((current) => {
        const preferred = current || savedModel(currentUser.id);
        return next.models.some((item) => item.id === preferred) ? preferred : next.models[0]?.id || '';
      });
    } catch (err) { handleApiError(err); }
  };

  // Check the existing cookie-backed session once when the app starts.
  useEffect(() => {
    if (sessionCheckStarted.current) return;
    sessionCheckStarted.current = true;
    if (window.location.pathname === '/auth/callback') {
      finishKeycloakLogin(api)
        .then((result) => setCurrentUser(result.user))
        .catch((err) => { setLoginError(err.message); resetToLogin(); })
        .finally(() => setCheckingSession(false));
      return;
    }
    api('/api/me')
      .then(setCurrentUser)
      .catch(resetToLogin)
      .finally(() => setCheckingSession(false));
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);
  useEffect(() => {
    const onNavigation = () => setShowAdmin(window.location.pathname === '/admin');
    window.addEventListener('popstate', onNavigation);
    return () => window.removeEventListener('popstate', onNavigation);
  }, []);
  const navigateAdmin = (open) => {
    window.history.pushState({}, '', open ? '/admin' : '/');
    setShowAdmin(open);
  };
  // Conversations refresh only when the authenticated user changes.
  useEffect(() => {
    if (currentUser) { fetchConversations(); fetchEntitlement(); }
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [currentUser]);
  useEffect(() => {
    if (!currentUser) return undefined;
    const timer = setInterval(() => {
      setNow(Date.now());
      fetchEntitlement();
    }, 30000);
    return () => clearInterval(timer);
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [currentUser]);
  const handleLogout = async () => {
    try {
      await api('/api/logout', { method: 'POST' });
    } catch (_) {
      // Clear the local session even when offline.
    } finally {
      resetToLogin();
      logoutKeycloak();
    }
  };
  const changeModel = (modelId) => {
    setSelectedModel(modelId);
    if (currentUser?.id) rememberModel(currentUser.id, modelId);
  };
  const closeSidebarOnMobile = () => {
    if (window.innerWidth <= 768) setSidebarOpen(false);
  };
  const selectConversation = async (id) => {
    stopGenerating();
    setError('');
    setCurrentId(id);
    closeSidebarOnMobile();
    try {
      setMessages(await api(`/api/history?id=${encodeURIComponent(id)}`));
    } catch (err) {
      handleApiError(err);
    }
  };
  const startNewChat = () => {
    stopGenerating();
    setCurrentId('');
    setMessages([]);
    setError('');
    closeSidebarOnMobile();
  };
  const renameConversation = async (id, title) => {
    const updated = await api(
      `/api/conversations?id=${encodeURIComponent(id)}`,
      { method: 'PATCH', body: JSON.stringify({ title }) },
    );
    setConversations((items) =>
      items.map((item) => (item.id === id ? updated : item)),
    );
  };
  const deleteConversation = async (id) => {
    if (id === currentId) stopGenerating();
    await api(`/api/conversations?id=${encodeURIComponent(id)}`, {
      method: 'DELETE',
    });
    setConversations((items) => items.filter((item) => item.id !== id));
    if (id === currentId) {
      setCurrentId('');
      setMessages([]);
      setError('');
    }
  };
  const sendMessage = async () => {
    if (!input.trim() || loading) return;
    const content = input.trim();
    setInput('');
    setError('');
    const pendingId = `stream-${Date.now()}`;
    setMessages((items) => [
      ...items,
      { role: 'user', content },
      { id: pendingId, role: 'assistant', content: '' },
    ]);
    setLoading(true);
    const controller = new AbortController();
    streamController.current = controller;
    let streamedContent = '';
    try {
      let conversationId = currentId;
      if (!conversationId) {
        const conversation = await api('/api/conversations', {
          method: 'POST',
          body: '{}',
        });
        conversationId = conversation.id;
        setCurrentId(conversationId);
      }
      const reply = await streamChat(
        { conversation_id: conversationId, content, model: selectedModel },
        {
          signal: controller.signal,
          onDelta: (delta) => {
            streamedContent += delta;
            setMessages((items) =>
              items.map((item) =>
                item.id === pendingId
                  ? { ...item, content: streamedContent }
                  : item,
              ),
            );
          },
        },
      );
      setMessages((items) =>
        items.map((item) => (item.id === pendingId ? reply : item)),
      );
    } catch (err) {
      if (err.name !== 'AbortError') {
        handleApiError(err);
        setMessages((items) =>
          items.map((item) =>
            item.id === pendingId
              ? {
                  ...item,
                  content: streamedContent || `❌ ${err.message}`,
                }
              : item,
          ),
        );
      } else if (!streamedContent) {
        setMessages((items) => items.filter((item) => item.id !== pendingId));
      }
    } finally {
      if (streamController.current === controller) streamController.current = null;
      setLoading(false);
      await fetchConversations();
      await fetchEntitlement();
    }
  };

  if (checkingSession) {
    return (
      <div className="session-loader">
        <span className="spinner" />
        Đang kiểm tra phiên đăng nhập...
      </div>
    );
  }
  if (!currentUser) {
    return <Auth error={loginError} />;
  }
  if (showAdmin) {
    if (currentUser.is_admin) return <AdminDashboard onBack={() => navigateAdmin(false)} onLogout={handleLogout} />;
    return <div className="admin-page"><div className="admin-denied"><h1>Không có quyền truy cập</h1><p>Tài khoản {currentUser.display_name || currentUser.username} chưa được cấp quyền quản trị.</p><button type="button" onClick={() => navigateAdmin(false)}>Về Chatbox</button></div></div>;
  }

  return (
    <div className="app-shell">
      <Sidebar
        conversations={conversations}
        currentId={currentId}
        currentUser={currentUser}
        plan={entitlement?.plan}
        creditBalance={entitlement?.balance}
        paidUntil={entitlement?.paid_until}
        creditResetAt={entitlement?.credit_reset_at}
        now={now}
        onManageKeys={() => setShowAPIKeys(true)}
        isAdmin={currentUser.is_admin}
        onOpenAdmin={() => navigateAdmin(true)}
        isOpen={sidebarOpen}
        onClose={() => setSidebarOpen(false)}
        onDelete={deleteConversation}
        onLogout={handleLogout}
        onNewChat={startNewChat}
        onRename={renameConversation}
        onSelect={selectConversation}
      />
      <main className="chat-main">
        <ChatHeader
          sidebarOpen={sidebarOpen}
          onOpenSidebar={() => setSidebarOpen(true)}
        />
        {error && (
          <div className="error-banner" role="alert">
            {error}
          </div>
        )}
        <ChatView
          input={input}
          loading={loading}
          messages={messages}
          models={entitlement?.models || []}
          selectedModel={selectedModel}
          onModelChange={changeModel}
          onInputChange={setInput}
          onSend={sendMessage}
          onStop={stopGenerating}
        />
      </main>
      {showAPIKeys && <APIKeysPanel models={entitlement?.models || []} onClose={() => setShowAPIKeys(false)} />}
    </div>
  );
}

export default App;
