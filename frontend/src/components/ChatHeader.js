import React from 'react';
import { SidebarIcon } from './icons';

export default function ChatHeader({ sidebarOpen, onOpenSidebar }) {
  return (
    <header className="chat-header">
      <button
        className={`icon-button open-sidebar ${sidebarOpen ? 'hidden' : ''}`}
        onClick={onOpenSidebar}
        aria-label="Mở thanh bên"
        title="Mở thanh bên"
        tabIndex={sidebarOpen ? -1 : 0}
      >
        <SidebarIcon />
      </button>
      <div className="app-title">Chatbox</div>
    </header>
  );
}
