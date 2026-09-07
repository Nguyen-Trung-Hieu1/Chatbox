import React, { useEffect, useRef, useState } from 'react';
import Auth from './Auth';
import ChatHeader from './components/ChatHeader';
import ChatView from './components/ChatView';
import Sidebar from './components/Sidebar';
import { api, streamChat } from './services/api';
import './App.css';

function App() {
  const [currentUser, setCurrentUser] = useState(null);
  const [checkingSession, setCheckingSession] = useState(true);
  const [conversations, setConversations] = useState([]);
  const [currentId, setCurrentId] = useState('');
  const [messages, setMessages] = useState([]);
  const [input, setInput] = useState('');
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');
  const [sidebarOpen, setSidebarOpen] = useState(() => window.innerWidth > 768);
  const streamController = useRef(null);

  const stopGenerating = () => streamController.current?.abort();
  const resetToLogin = () => {
    stopGenerating();
    setCurrentUser(null);
    setConversations([]);
    setCurrentId('');
    setMessages([]);
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

  // Check the existing cookie-backed session once when the app starts.
  // eslint-disable-next-line react-hooks/exhaustive-deps
  useEffect(() => {
    api('/api/me')
      .then(setCurrentUser)
      .catch(resetToLogin)
      .finally(() => setCheckingSession(false));
  }, []);
  // Conversations refresh only when the authenticated user changes.
  // eslint-disable-next-line react-hooks/exhaustive-deps
  useEffect(() => {
    if (currentUser) fetchConversations();
  }, [currentUser]);

  const handleLogout = async () => {
    try {
      await api('/api/logout', { method: 'POST' });
    } catch (_) {
      // Clear the local session even when offline.
    } finally {
      resetToLogin();
    }
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
        { conversation_id: conversationId, content },
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
    return <Auth onLoginSuccess={setCurrentUser} api={api} />;
  }

  return (
    <div className="app-shell">
      <Sidebar
        conversations={conversations}
        currentId={currentId}
        currentUser={currentUser}
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
          onInputChange={setInput}
          onSend={sendMessage}
          onStop={stopGenerating}
        />
      </main>
    </div>
  );
}

export default App;
