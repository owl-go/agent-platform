ALTER TABLE runs DROP CONSTRAINT runs_trigger_check;
ALTER TABLE runs ADD CONSTRAINT runs_trigger_check CHECK (trigger IN ('manual','scheduled','api','session_conversion','message_channel'));

CREATE TABLE workflow_message_channels (
 id uuid PRIMARY KEY,
 owner_user_id uuid NOT NULL REFERENCES users(id),
 workflow_id uuid NOT NULL REFERENCES workflows(id),
 provider text NOT NULL CHECK (provider IN ('telegram','discord','slack','dingtalk','feishu')),
 account_id text NOT NULL,
 binding_id text NOT NULL,
 tenant_id text NOT NULL DEFAULT '',
 configuration jsonb NOT NULL,
 credential_ciphertext bytea,
 config_version bigint NOT NULL CHECK (config_version > 0),
 version bigint NOT NULL CHECK (version > 0),
 generation bigint NOT NULL DEFAULT 1,
 enabled boolean NOT NULL DEFAULT false,
 validation_state text NOT NULL DEFAULT 'unverified' CHECK (validation_state IN ('unverified','testing','passed')),
 validation_code text NOT NULL DEFAULT '',
 validation_until timestamptz,
 health text NOT NULL DEFAULT 'disconnected',
 error_code text NOT NULL DEFAULT '',
 deleted_at timestamptz,
 send_after timestamptz NOT NULL DEFAULT now(),
 created_at timestamptz NOT NULL DEFAULT now()
);
-- Each bot's entire message stream has one binding, including disabled configs.
CREATE UNIQUE INDEX message_channel_account_binding ON workflow_message_channels(provider,binding_id) WHERE deleted_at IS NULL;
CREATE INDEX message_channels_owner ON workflow_message_channels(owner_user_id,workflow_id) WHERE deleted_at IS NULL;

CREATE TABLE message_channel_inbox (
 id uuid PRIMARY KEY,
 channel_id uuid NOT NULL REFERENCES workflow_message_channels(id),
 config_version bigint NOT NULL,
 generation bigint NOT NULL,
 event_id text NOT NULL,
 message_id text NOT NULL,
 chat_id text NOT NULL,
 sender_id text NOT NULL,
 message jsonb NOT NULL,
 reply_ciphertext bytea NOT NULL,
 state text NOT NULL DEFAULT 'received' CHECK (state IN ('received','admitted','rejected','ignored')),
 reason text NOT NULL DEFAULT '',
 run_id uuid REFERENCES runs(id),
 received_at timestamptz NOT NULL DEFAULT now(),
 UNIQUE(channel_id,event_id),
 UNIQUE(channel_id,chat_id,message_id)
);
CREATE INDEX channel_inbox_pending ON message_channel_inbox(received_at,id) WHERE state='received';
CREATE INDEX channel_inbox_sender_rate ON message_channel_inbox(channel_id,sender_id,received_at);
CREATE INDEX channel_inbox_channel_pending ON message_channel_inbox(channel_id) WHERE state='received';
CREATE UNIQUE INDEX channel_inbox_run ON message_channel_inbox(run_id) WHERE run_id IS NOT NULL;

CREATE TABLE message_channel_conversations (
 channel_id uuid NOT NULL REFERENCES workflow_message_channels(id),
 generation bigint NOT NULL,
 conversation_key text NOT NULL,
 run_id uuid NOT NULL REFERENCES runs(id),
 PRIMARY KEY(channel_id,generation,conversation_key)
);

CREATE TABLE message_channel_deliveries (
 id uuid PRIMARY KEY,
 channel_id uuid NOT NULL REFERENCES workflow_message_channels(id),
 inbox_id uuid NOT NULL REFERENCES message_channel_inbox(id),
 run_id uuid REFERENCES runs(id),
 kind text NOT NULL CHECK (kind IN ('answer','status','validation','waiting')),
 chunk integer NOT NULL CHECK (chunk > 0),
 payload text NOT NULL,
 config_version bigint NOT NULL,
 state text NOT NULL DEFAULT 'pending' CHECK (state IN ('pending','sending','sent','retry_wait','failed','expired','outcome_unknown','cancelled')),
 attempts integer NOT NULL DEFAULT 0,
 error_code text NOT NULL DEFAULT '',
 provider_message_id text NOT NULL DEFAULT '',
 lease uuid,
 lease_until timestamptz,
 retry_at timestamptz NOT NULL DEFAULT now(),
 deadline timestamptz NOT NULL DEFAULT now()+interval '24 hours',
 created_at timestamptz NOT NULL DEFAULT now(),
 UNIQUE(inbox_id,kind,chunk)
);
CREATE INDEX channel_delivery_pending ON message_channel_deliveries(retry_at,created_at,id) WHERE state IN ('pending','retry_wait');
CREATE INDEX channel_delivery_channel_pending ON message_channel_deliveries(channel_id) WHERE state IN ('pending','sending','retry_wait','outcome_unknown');

ALTER TABLE runs ADD COLUMN message_channel_id uuid REFERENCES workflow_message_channels(id);
ALTER TABLE runs ADD COLUMN message_channel_name text NOT NULL DEFAULT '';
CREATE INDEX runs_message_channel ON runs(message_channel_id) WHERE message_channel_id IS NOT NULL;

CREATE INDEX channel_inbox_retention ON message_channel_inbox(received_at) WHERE state<>'received';
CREATE INDEX channel_delivery_deadline ON message_channel_deliveries(deadline) WHERE state IN ('pending','retry_wait','outcome_unknown');
