ALTER TABLE session_messages
    ADD COLUMN evidence jsonb NOT NULL DEFAULT '[]'::jsonb,
    ADD CONSTRAINT session_messages_evidence_array CHECK (jsonb_typeof(evidence) = 'array');

ALTER TABLE runs
    ADD COLUMN evidence jsonb NOT NULL DEFAULT '[]'::jsonb,
    ADD CONSTRAINT runs_evidence_array CHECK (jsonb_typeof(evidence) = 'array');

COMMENT ON COLUMN session_messages.evidence IS 'Bounded platform-derived execution Evidence; never raw tool output, prompts, replies, credentials, or provider payloads.';
COMMENT ON COLUMN runs.evidence IS 'Bounded platform-derived execution Evidence; never raw tool output, prompts, replies, credentials, or provider payloads.';
