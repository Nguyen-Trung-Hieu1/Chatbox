import React from 'react';

const Icon = ({ children, size = 20, ...props }) => (
  <svg
    aria-hidden="true"
    fill="none"
    height={size}
    viewBox="0 0 24 24"
    width={size}
    {...props}
  >
    {children}
  </svg>
);

export const ComposeIcon = (props) => (
  <Icon {...props}>
    <path
      d="M12 20h9M16.5 3.5a2.12 2.12 0 0 1 3 3L8 18l-4 1 1-4Z"
      stroke="currentColor"
      strokeLinecap="round"
      strokeLinejoin="round"
      strokeWidth="2"
    />
  </Icon>
);

export const MenuIcon = (props) => (
  <Icon {...props}>
    <path
      d="M4 6h16M4 12h16M4 18h16"
      stroke="currentColor"
      strokeLinecap="round"
      strokeWidth="2"
    />
  </Icon>
);

export const CloseIcon = (props) => (
  <Icon {...props}>
    <path
      d="m6 6 12 12M18 6 6 18"
      stroke="currentColor"
      strokeLinecap="round"
      strokeWidth="2"
    />
  </Icon>
);

export const SidebarIcon = (props) => (
  <Icon {...props}>
    <rect
      x="3"
      y="3"
      width="18"
      height="18"
      rx="3"
      stroke="currentColor"
      strokeWidth="1.8"
    />
    <path d="M9 3v18" stroke="currentColor" strokeWidth="1.8" />
  </Icon>
);

export const SendIcon = (props) => (
  <Icon {...props}>
    <path
      d="M12 19V5m0 0L6.5 10.5M12 5l5.5 5.5"
      stroke="currentColor"
      strokeLinecap="round"
      strokeLinejoin="round"
      strokeWidth="2.2"
    />
  </Icon>
);

export const StopIcon = (props) => (
  <Icon {...props}>
    <rect x="7" y="7" width="10" height="10" rx="1.5" fill="currentColor" />
  </Icon>
);

export const SearchIcon = (props) => (
  <Icon {...props}>
    <circle cx="11" cy="11" r="7" stroke="currentColor" strokeWidth="2" />
    <path
      d="m20 20-4-4"
      stroke="currentColor"
      strokeLinecap="round"
      strokeWidth="2"
    />
  </Icon>
);

export const MoreIcon = (props) => (
  <Icon {...props}>
    <circle cx="5" cy="12" fill="currentColor" r="1.5" />
    <circle cx="12" cy="12" fill="currentColor" r="1.5" />
    <circle cx="19" cy="12" fill="currentColor" r="1.5" />
  </Icon>
);

export const TrashIcon = (props) => (
  <Icon {...props}>
    <path
      d="M4 7h16M9 3h6l1 4H8l1-4Zm-2 4 1 14h8l1-14"
      stroke="currentColor"
      strokeLinecap="round"
      strokeLinejoin="round"
      strokeWidth="1.8"
    />
  </Icon>
);

export const CopyIcon = (props) => (
  <Icon {...props}>
    <rect x="8" y="8" width="11" height="11" rx="2" stroke="currentColor" strokeWidth="1.8" />
    <path
      d="M16 8V6a2 2 0 0 0-2-2H6a2 2 0 0 0-2 2v8a2 2 0 0 0 2 2h2"
      stroke="currentColor"
      strokeWidth="1.8"
    />
  </Icon>
);

export const CheckIcon = (props) => (
  <Icon {...props}>
    <path
      d="m5 12 4 4L19 6"
      stroke="currentColor"
      strokeLinecap="round"
      strokeLinejoin="round"
      strokeWidth="2"
    />
  </Icon>
);

export const CodeIcon = (props) => (
  <Icon {...props}>
    <path
      d="m8.5 8-4 4 4 4M15.5 8l4 4-4 4M14 5l-4 14"
      stroke="currentColor"
      strokeLinecap="round"
      strokeLinejoin="round"
      strokeWidth="1.8"
    />
  </Icon>
);
