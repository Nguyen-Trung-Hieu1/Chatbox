import React, { useEffect, useRef, useState } from 'react';
import LogoMark from './LogoMark';
import MessageContent from './MessageContent';
import { SendIcon, StopIcon } from './icons';

function Composer({ input, loading, models, selectedModel, onModelChange, onInputChange, onSend, onStop }) {
  const textareaRef = useRef(null);
  const modelMenuRef = useRef(null);
  const modelButtonRef = useRef(null);
  const [modelMenuOpen, setModelMenuOpen] = useState(false);
  const activeModel = models.find((model) => model.id === selectedModel) || models[0];
  const modelLabel = (model) => model.display_name || model.id.replace(/^ag\//, '');
  useEffect(() => {
    if (!modelMenuOpen) return undefined;
    const closeOnOutsideClick = (event) => {
      if (!modelMenuRef.current?.contains(event.target)) setModelMenuOpen(false);
    };
    const closeOnEscape = (event) => {
      if (event.key === 'Escape') {
        setModelMenuOpen(false);
        modelButtonRef.current?.focus();
      }
    };
    document.addEventListener('pointerdown', closeOnOutsideClick);
    document.addEventListener('keydown', closeOnEscape);
    return () => {
      document.removeEventListener('pointerdown', closeOnOutsideClick);
      document.removeEventListener('keydown', closeOnEscape);
    };
  }, [modelMenuOpen]);
  useEffect(() => {
    if (loading) setModelMenuOpen(false);
  }, [loading]);
  useEffect(() => {
    const el = textareaRef.current;
    if (el) {
      el.style.height = 'auto';
      el.style.height = `${Math.min(el.scrollHeight, 160)}px`;
    }
  }, [input]);
  const onKeyDown = (event) => {
    if (event.key === 'Enter' && !event.shiftKey) {
      event.preventDefault();
      onSend();
    }
  };
  return (
    <div className="composer-wrap">
      <div className="composer">
        <textarea
          ref={textareaRef}
          rows="1"
          value={input}
          onChange={(e) => onInputChange(e.target.value)}
          onKeyDown={onKeyDown}
          placeholder="Hỏi bất kỳ điều gì"
          disabled={loading}
          aria-label="Tin nhắn"
        />
        <div className="composer-tools">
          {models.length > 0 && (
            <div className="composer-model-wrap" ref={modelMenuRef}>
              <button
                ref={modelButtonRef}
                type="button"
                className={`composer-model ${modelMenuOpen ? 'open' : ''}`}
                aria-label="Chọn model"
                aria-haspopup="menu"
                aria-expanded={modelMenuOpen}
                onClick={() => setModelMenuOpen((open) => !open)}
                disabled={loading}
              >
                <span>{modelLabel(activeModel)}</span>
                <svg viewBox="0 0 16 16" aria-hidden="true"><path d="m4 6 4 4 4-4" /></svg>
              </button>
              {modelMenuOpen && (
                <div className="composer-model-menu" role="menu" aria-label="Danh sách model">
                  {models.map((model) => (
                    <button
                      type="button"
                      key={model.id}
                      className={`composer-model-item ${model.id === selectedModel ? 'selected' : ''}`}
                      role="menuitemradio"
                      aria-checked={model.id === selectedModel}
                      onClick={() => {
                        onModelChange(model.id);
                        setModelMenuOpen(false);
                        modelButtonRef.current?.focus();
                      }}
                    >
                      <span>{modelLabel(model)}</span>
                      {model.id === selectedModel && <span className="composer-model-check" aria-hidden="true">✓</span>}
                    </button>
                  ))}
                </div>
              )}
            </div>
          )}
          <button
            className={`send-button ${loading ? 'stop' : ''}`}
            onClick={loading ? onStop : onSend}
            disabled={!loading && !input.trim()}
            aria-label={loading ? 'Dừng tạo câu trả lời' : 'Gửi tin nhắn'}
          >
            {loading ? <StopIcon size={18} /> : <SendIcon size={19} />}
          </button>
        </div>
      </div>
      <p className="disclaimer">
        Chatbox có thể mắc lỗi. Kiểm tra thông tin quan trọng
      </p>
    </div>
  );
}

export default function ChatView({ input, loading, messages, models = [], selectedModel = '', onModelChange, onInputChange, onSend, onStop }) {
  const bottomRef = useRef(null);
  useEffect(() => {
    bottomRef.current?.scrollIntoView({ behavior: 'smooth' });
  }, [messages, loading]);
  return (
    <div className={`chat-view ${messages.length === 0 ? 'empty' : ''}`}>
      {!messages.length ? (
        <section className="welcome">
          <LogoMark size={34} />
          <h1>Tôi có thể giúp gì cho bạn?</h1>
        </section>
      ) : (
        <div className="message-scroll">
          <div className="message-column">
            {messages.map((message, index) => (
              <article
                className={`message-row ${message.role}`}
                key={message.id || index}
              >
                {message.role === 'assistant' && <LogoMark size={26} />}
                <div className="message-content">
                  {message.role === 'assistant' &&
                  !message.content &&
                  loading ? (
                    <span className="typing">
                      <i />
                      <i />
                      <i />
                    </span>
                  ) : message.role === 'assistant' ? (
                    <MessageContent content={message.content} />
                  ) : (
                    message.content
                  )}
                </div>
              </article>
            ))}
            <div ref={bottomRef} />
          </div>
        </div>
      )}
      <Composer
        input={input}
        loading={loading}
        models={models}
        selectedModel={selectedModel}
        onModelChange={onModelChange}
        onInputChange={onInputChange}
        onSend={onSend}
        onStop={onStop}
      />
    </div>
  );
}
