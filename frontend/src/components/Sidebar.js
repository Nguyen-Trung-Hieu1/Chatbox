import React, { useEffect, useMemo, useRef, useState } from 'react';
import LogoMark from './LogoMark';
import { ComposeIcon, MoreIcon, SearchIcon, SidebarIcon, TrashIcon } from './icons';

export default function Sidebar({
  conversations,
  currentId,
  currentUser,
  isOpen,
  onClose,
  onDelete,
  onLogout,
  onNewChat,
  onRename,
  onSelect,
}) {
  const [query, setQuery] = useState('');
  const [menuId, setMenuId] = useState('');
  const [editingId, setEditingId] = useState('');
  const [draftTitle, setDraftTitle] = useState('');
  const [actionError, setActionError] = useState('');
  const menuRef = useRef(null);
  const filtered = useMemo(() => {
    const normalized = query.trim().toLocaleLowerCase('vi');
    return normalized
      ? conversations.filter((item) =>
          (item.title || '').toLocaleLowerCase('vi').includes(normalized),
        )
      : conversations;
  }, [conversations, query]);

  useEffect(() => {
    const closeMenu = (event) => {
      if (menuRef.current && !menuRef.current.contains(event.target)) {
        setMenuId('');
      }
    };
    document.addEventListener('mousedown', closeMenu);
    return () => document.removeEventListener('mousedown', closeMenu);
  }, []);

  const beginRename = (item) => {
    setMenuId('');
    setEditingId(item.id);
    setDraftTitle(item.title || '');
    setActionError('');
  };
  const saveRename = async (id) => {
    const title = draftTitle.trim();
    if (!title) {
      setActionError('Tên cuộc trò chuyện không được để trống.');
      return;
    }
    try {
      await onRename(id, title);
      setEditingId('');
      setActionError('');
    } catch (err) {
      setActionError(err.message);
    }
  };
  const removeConversation = async (item) => {
    setMenuId('');
    if (
      !window.confirm(
        `Xóa cuộc trò chuyện “${item.title || 'Cuộc trò chuyện mới'}”? Hành động này không thể hoàn tác.`,
      )
    ) {
      return;
    }
    try {
      await onDelete(item.id);
      setActionError('');
    } catch (err) {
      setActionError(err.message);
    }
  };

  return (
    <>
      <div
        className={`sidebar-backdrop ${isOpen ? 'visible' : ''}`}
        onClick={onClose}
      />
      <aside className={`sidebar ${isOpen ? 'open' : ''}`}>
        <div className="sidebar-top">
          <button className="brand-button" onClick={onNewChat}>
            <LogoMark size={25} />
            <span>Chatbox</span>
          </button>
          <button
            className="icon-button sidebar-close"
            onClick={onClose}
            aria-label="Thu gọn thanh bên"
            title="Thu gọn thanh bên"
          >
            <SidebarIcon />
          </button>
        </div>
        <nav className="sidebar-actions" aria-label="Điều hướng chính">
          <button onClick={onNewChat}>
            <ComposeIcon size={18} />
            <span>Đoạn chat mới</span>
          </button>
        </nav>
        <div className="conversation-search">
          <SearchIcon size={17} />
          <input
            value={query}
            onChange={(event) => setQuery(event.target.value)}
            placeholder="Tìm đoạn chat"
            aria-label="Tìm kiếm lịch sử trò chuyện"
          />
          {query && (
            <button onClick={() => setQuery('')} aria-label="Xóa tìm kiếm">
              ×
            </button>
          )}
        </div>
        <div className="history-label">Đoạn chat</div>
        {actionError && (
          <div className="sidebar-error" role="alert">
            {actionError}
          </div>
        )}
        <div className="conversation-list">
          {!conversations.length && (
            <p className="empty-history">
              Các cuộc trò chuyện gần đây sẽ xuất hiện tại đây.
            </p>
          )}
          {!!conversations.length && !filtered.length && (
            <p className="empty-history">
              Không tìm thấy cuộc trò chuyện phù hợp.
            </p>
          )}
          {filtered.map((item) => (
            <div
              className={`conversation-entry ${item.id === currentId ? 'active' : ''}`}
              key={item.id}
            >
              {editingId === item.id ? (
                <input
                  className="rename-input"
                  value={draftTitle}
                  autoFocus
                  maxLength={100}
                  onChange={(event) => setDraftTitle(event.target.value)}
                  onBlur={() => saveRename(item.id)}
                  onKeyDown={(event) => {
                    if (event.key === 'Enter') {
                      event.preventDefault();
                      saveRename(item.id);
                    }
                    if (event.key === 'Escape') {
                      setEditingId('');
                      setActionError('');
                    }
                  }}
                />
              ) : (
                <button
                  className="conversation-select"
                  onClick={() => onSelect(item.id)}
                >
                  <span>{item.title || 'Cuộc trò chuyện mới'}</span>
                </button>
              )}
              {editingId !== item.id && (
                <div
                  className="conversation-menu-wrap"
                  ref={menuId === item.id ? menuRef : null}
                >
                  <button
                    className="conversation-more"
                    onClick={() =>
                      setMenuId(menuId === item.id ? '' : item.id)
                    }
                    aria-label={`Tùy chọn cho ${item.title}`}
                  >
                    <MoreIcon size={17} />
                  </button>
                  {menuId === item.id && (
                    <div className="conversation-menu">
                      <button onClick={() => beginRename(item)}>
                        <ComposeIcon size={16} />
                        Đổi tên
                      </button>
                      <button
                        className="danger"
                        onClick={() => removeConversation(item)}
                      >
                        <TrashIcon size={16} />
                        Xóa
                      </button>
                    </div>
                  )}
                </div>
              )}
            </div>
          ))}
        </div>
        <div className="account-wrap">
          <div className="account-row">
            <span className="avatar">
              {currentUser.username.charAt(0).toUpperCase()}
            </span>
            <span className="account-name">{currentUser.username}</span>
            <button className="logout-button" onClick={onLogout}>
              Logout
            </button>
          </div>
        </div>
      </aside>
    </>
  );
}
