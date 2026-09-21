-- P14 notification: what was sent, and where a push should go.
--
-- Two tables, neither referencing another module's schema (2.5): a user id
-- here is an opaque string handed across a contract boundary, the same
-- convention every other module's migration follows.

CREATE TABLE IF NOT EXISTS notifications (
    id      TEXT PRIMARY KEY,
    user_id TEXT NOT NULL,

    title TEXT NOT NULL,
    body  TEXT NOT NULL,

    -- Which channel actually delivered it, or "none" when neither could.
    channel TEXT NOT NULL CHECK (channel IN ('push', 'sms', 'none')),
    status  TEXT NOT NULL CHECK (status IN ('sent', 'failed')),

    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- A user's own inbox: most recent first.
CREATE INDEX IF NOT EXISTS notifications_user_idx
    ON notifications (user_id, created_at DESC);

CREATE TABLE IF NOT EXISTS device_tokens (
    user_id  TEXT NOT NULL,
    -- 'ios' or 'android'. One row per user per platform: a reinstalled app
    -- registers a new token, replacing this row rather than adding another
    -- one nothing would ever clean up.
    platform TEXT NOT NULL CHECK (platform IN ('ios', 'android')),
    token    TEXT NOT NULL,

    registered_at TIMESTAMPTZ NOT NULL DEFAULT now(),

    PRIMARY KEY (user_id, platform)
);
