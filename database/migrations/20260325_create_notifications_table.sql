-- Migration: Create notifications table
CREATE TABLE IF NOT EXISTS notifications (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     UUID REFERENCES users(id) ON DELETE CASCADE,
    role_target VARCHAR(50),
    type        VARCHAR(50) NOT NULL, -- 'transaction' | 'payment' | 'system' | 'capacity'
    title       VARCHAR(255) NOT NULL,
    body        TEXT NOT NULL,
    metadata    JSONB,
    is_read     BOOLEAN NOT NULL DEFAULT false,
    read_at     TIMESTAMPTZ,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_notifications_user_id ON notifications (user_id);
CREATE INDEX IF NOT EXISTS idx_notifications_unread ON notifications (user_id, is_read) WHERE is_read = false;
