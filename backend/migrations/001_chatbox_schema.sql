-- Chatbox data in PostgreSQL. Keycloak owns a separate schema in the same DB.
-- This file creates empty tables only; it does not copy MongoDB data.
-- Keep MongoDB ObjectID values as 24-character strings so existing IDs remain valid.

CREATE SCHEMA IF NOT EXISTS chatbox;

CREATE TABLE IF NOT EXISTS chatbox.users (
    id            varchar(24) PRIMARY KEY,
    username      text NOT NULL UNIQUE,
    display_name  text NOT NULL DEFAULT '',
    email         text NOT NULL DEFAULT '',
    first_name    text NOT NULL DEFAULT '',
    last_name     text NOT NULL DEFAULT '',
    keycloak_sub  text UNIQUE,
    is_admin      boolean NOT NULL DEFAULT false, -- legacy field; Keycloak role governs admin access
    created_at    timestamptz NOT NULL
);

CREATE TABLE IF NOT EXISTS chatbox.conversations (
    id          varchar(24) PRIMARY KEY,
    owner_id    varchar(24) NOT NULL REFERENCES chatbox.users(id),
    title       text NOT NULL,
    summary     text NOT NULL DEFAULT '',
    created_at  timestamptz NOT NULL,
    updated_at  timestamptz NOT NULL
);

CREATE INDEX IF NOT EXISTS conversations_owner_updated_idx
    ON chatbox.conversations (owner_id, updated_at DESC);

CREATE TABLE IF NOT EXISTS chatbox.messages (
    id               varchar(24) PRIMARY KEY,
    conversation_id  varchar(24) NOT NULL REFERENCES chatbox.conversations(id),
    role             text NOT NULL,
    content          text NOT NULL,
    summarized       boolean NOT NULL DEFAULT false,
    created_at       timestamptz NOT NULL
);

CREATE INDEX IF NOT EXISTS messages_conversation_created_idx
    ON chatbox.messages (conversation_id, created_at, id);

-- MongoDB wallet _id equals the user's _id. Plan is a local copy of the
-- Keycloak role, while balance and reset time remain Chatbox-owned data.
CREATE TABLE IF NOT EXISTS chatbox.wallets (
    user_id          varchar(24) PRIMARY KEY REFERENCES chatbox.users(id),
    plan             text NOT NULL DEFAULT 'free',
    plan_source      text,
    balance          bigint NOT NULL,
    paid_until       timestamptz,
    credit_reset_at  timestamptz
);

-- Each MongoDB wallet embeds a usage array. Preserve array order with position.
CREATE TABLE IF NOT EXISTS chatbox.usage_records (
    user_id        varchar(24) NOT NULL REFERENCES chatbox.wallets(user_id),
    position       bigint NOT NULL,
    model          text NOT NULL,
    kind           text NOT NULL,
    input_tokens   integer NOT NULL,
    output_tokens  integer NOT NULL,
    credits        bigint NOT NULL,
    reported       boolean NOT NULL,
    status         text NOT NULL,
    created_at     timestamptz NOT NULL,
    PRIMARY KEY (user_id, position)
);

CREATE TABLE IF NOT EXISTS chatbox.api_keys (
    id            varchar(24) PRIMARY KEY,
    user_id       varchar(24) NOT NULL REFERENCES chatbox.users(id),
    name          text NOT NULL,
    prefix        text NOT NULL,
    hash          text NOT NULL UNIQUE,
    created_at    timestamptz NOT NULL,
    last_used_at  timestamptz,
    revoked_at    timestamptz
);

CREATE INDEX IF NOT EXISTS api_keys_user_created_idx
    ON chatbox.api_keys (user_id, created_at DESC);
