-- Migrate agents table from custom adapter fields to A2A protocol

-- Remove adapter-specific columns from agents
ALTER TABLE agents DROP COLUMN IF EXISTS adapter_type;
ALTER TABLE agents DROP COLUMN IF EXISTS adapter_config;
ALTER TABLE agents DROP COLUMN IF EXISTS auth_type;
ALTER TABLE agents DROP COLUMN IF EXISTS auth_config;
ALTER TABLE agents DROP COLUMN IF EXISTS input_schema;
ALTER TABLE agents DROP COLUMN IF EXISTS output_schema;
ALTER TABLE agents DROP COLUMN IF EXISTS config;

-- Add A2A-specific columns to agents
ALTER TABLE agents ADD COLUMN IF NOT EXISTS agent_card_url TEXT NOT NULL DEFAULT '';
ALTER TABLE agents ADD COLUMN IF NOT EXISTS agent_card JSONB NOT NULL DEFAULT '{}';
ALTER TABLE agents ADD COLUMN IF NOT EXISTS card_fetched_at TIMESTAMPTZ;
ALTER TABLE agents ADD COLUMN IF NOT EXISTS skills JSONB NOT NULL DEFAULT '[]';

-- Remove poll-specific columns from subtasks
ALTER TABLE subtasks DROP COLUMN IF EXISTS poll_job_id;
ALTER TABLE subtasks DROP COLUMN IF EXISTS poll_endpoint;

-- Add A2A task ID to subtasks
ALTER TABLE subtasks ADD COLUMN IF NOT EXISTS a2a_task_id TEXT NOT NULL DEFAULT '';
