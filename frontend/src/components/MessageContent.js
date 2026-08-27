import React, { useState } from 'react';
import hljs from 'highlight.js/lib/core';
import bash from 'highlight.js/lib/languages/bash';
import c from 'highlight.js/lib/languages/c';
import cpp from 'highlight.js/lib/languages/cpp';
import csharp from 'highlight.js/lib/languages/csharp';
import css from 'highlight.js/lib/languages/css';
import go from 'highlight.js/lib/languages/go';
import java from 'highlight.js/lib/languages/java';
import javascript from 'highlight.js/lib/languages/javascript';
import json from 'highlight.js/lib/languages/json';
import python from 'highlight.js/lib/languages/python';
import sql from 'highlight.js/lib/languages/sql';
import typescript from 'highlight.js/lib/languages/typescript';
import xml from 'highlight.js/lib/languages/xml';
import ReactMarkdown from 'react-markdown';
import remarkGfm from 'remark-gfm';
import { CheckIcon, CodeIcon, CopyIcon } from './icons';

hljs.registerLanguage('bash', bash);
hljs.registerLanguage('c', c);
hljs.registerLanguage('cpp', cpp);
hljs.registerLanguage('csharp', csharp);
hljs.registerLanguage('css', css);
hljs.registerLanguage('go', go);
hljs.registerLanguage('java', java);
hljs.registerLanguage('javascript', javascript);
hljs.registerLanguage('json', json);
hljs.registerLanguage('python', python);
hljs.registerLanguage('sql', sql);
hljs.registerLanguage('typescript', typescript);
hljs.registerLanguage('xml', xml);

const languageNames = {
  js: 'JavaScript', javascript: 'JavaScript', ts: 'TypeScript', typescript: 'TypeScript',
  py: 'Python', python: 'Python', go: 'Go', java: 'Java', c: 'C', cpp: 'C++',
  cs: 'C#', csharp: 'C#', html: 'HTML', css: 'CSS', json: 'JSON', bash: 'Bash',
  sh: 'Shell', shell: 'Shell', sql: 'SQL', jsx: 'JSX', tsx: 'TSX',
};
const languageAliases = { js: 'javascript', jsx: 'javascript', ts: 'typescript', tsx: 'typescript', py: 'python', cs: 'csharp', html: 'xml', sh: 'bash', shell: 'bash' };

async function copyText(text) {
  if (navigator.clipboard?.writeText) return navigator.clipboard.writeText(text);
  const textarea = document.createElement('textarea');
  textarea.value = text; textarea.style.position = 'fixed'; textarea.style.opacity = '0';
  document.body.appendChild(textarea); textarea.select(); document.execCommand('copy'); textarea.remove();
}

function CodeBlock({ children }) {
  const [copyState, setCopyState] = useState('idle');
  const code = React.Children.toArray(children).find(React.isValidElement);
  if (!code) return <pre>{children}</pre>;
  const match = /language-([\w-]+)/.exec(code.props.className || '');
  const language = match?.[1] || '';
  const label = languageNames[language.toLowerCase()] || language || 'Code';
  const text = String(code.props.children || '').replace(/\n$/, '');
  const highlightLanguage = languageAliases[language.toLowerCase()] || language.toLowerCase();
  const highlighted = highlightLanguage && hljs.getLanguage(highlightLanguage)
    ? hljs.highlight(text, { language: highlightLanguage, ignoreIllegals: true }).value
    : hljs.highlightAuto(text).value;
  const copy = async () => {
    try {
      await copyText(text); setCopyState('copied');
      window.setTimeout(() => setCopyState('idle'), 1800);
    } catch (_) {
      setCopyState('error'); window.setTimeout(() => setCopyState('idle'), 1800);
    }
  };

  return <div className="code-block">
    <div className="code-header"><span className="code-language"><CodeIcon size={17} />{label}</span><button className={`copy-code ${copyState}`} onClick={copy} aria-label={copyState === 'copied' ? 'Đã sao chép mã nguồn' : 'Sao chép mã nguồn'} title={copyState === 'error' ? 'Không thể sao chép' : copyState === 'copied' ? 'Đã sao chép' : 'Sao chép'}>{copyState === 'copied' ? <CheckIcon size={21} /> : <CopyIcon size={21} />}</button></div>
    <pre><code className={code.props.className} dangerouslySetInnerHTML={{ __html: highlighted }} /></pre>
  </div>;
}

export default function MessageContent({ content }) {
  return <div className="markdown-body">
    <ReactMarkdown remarkPlugins={[remarkGfm]} components={{
      pre: CodeBlock,
      a: ({ children, ...props }) => <a {...props} target="_blank" rel="noreferrer">{children}</a>,
    }}>{content}</ReactMarkdown>
  </div>;
}
