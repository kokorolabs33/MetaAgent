-- Create conversations table
CREATE TABLE IF NOT EXISTS conversations (
    id         TEXT PRIMARY KEY,
    title      TEXT NOT NULL DEFAULT '',
    created_by TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_conversations_user ON conversations(created_by, updated_at DESC);

-- Add conversation_id to tasks
ALTER TABLE tasks ADD COLUMN IF NOT EXISTS conversation_id TEXT NOT NULL DEFAULT '';
CREATE INDEX IF NOT EXISTS idx_tasks_conversation ON tasks(conversation_id, created_at);

-- Messages: drop task_id FK (messages now belong to conversations, task_id is optional)
ALTER TABLE messages DROP CONSTRAINT IF EXISTS messages_task_id_fkey;
ALTER TABLE messages ALTER COLUMN task_id DROP NOT NULL;
ALTER TABLE messages ALTER COLUMN task_id SET DEFAULT '';

-- Add conversation_id to messages
ALTER TABLE messages ADD COLUMN IF NOT EXISTS conversation_id TEXT NOT NULL DEFAULT '';
CREATE INDEX IF NOT EXISTS idx_messages_conversation ON messages(conversation_id, created_at);

-- Add conversation_id to events (SSE streams per conversation)
ALTER TABLE events ADD COLUMN IF NOT EXISTS conversation_id TEXT NOT NULL DEFAULT '';
CREATE INDEX IF NOT EXISTS idx_events_conversation ON events(conversation_id, created_at);
