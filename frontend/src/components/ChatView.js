import React, { useEffect, useRef } from 'react';
import LogoMark from './LogoMark';
import MessageContent from './MessageContent';
import { SendIcon, StopIcon } from './icons';

function Composer({ input, loading, onInputChange, onSend, onStop }) {
  const textareaRef = useRef(null);
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
        Chatbox có thể mắc lỗi. Đây là bản thử nghiệm Docker.
      </p>
    </div>
  );
}

export default function ChatView({ input, loading, messages, onInputChange, onSend, onStop }) {
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
        onInputChange={onInputChange}
        onSend={onSend}
        onStop={onStop}
      />
    </div>
  );
}
