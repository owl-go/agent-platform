ALTER TABLE workflow_message_channels DROP CONSTRAINT workflow_message_channels_provider_check;
ALTER TABLE workflow_message_channels ADD CONSTRAINT workflow_message_channels_provider_check
 CHECK (provider IN ('telegram','discord','slack','dingtalk','feishu','matrix','whatsapp','signal','wecom','wechat','qqbot','bluebubbles','yuanbao'));

CREATE TABLE message_channel_receive_cursors (
 channel_id uuid PRIMARY KEY REFERENCES workflow_message_channels(id),
 config_version bigint NOT NULL CHECK (config_version > 0),
 ciphertext bytea NOT NULL,
 updated_at timestamptz NOT NULL DEFAULT now()
);
