-- Public conversations share the direct Assistant model pipeline but are
-- invisible to the owner's private conversation list and scoped to a visitor.
ALTER TABLE assistant_conversations
    ADD COLUMN visitor_hash text NOT NULL DEFAULT '',
    ADD COLUMN share_token_revision bigint NOT NULL DEFAULT 0;

CREATE INDEX assistant_conversations_public_visitor
    ON assistant_conversations(id, assistant_id, visitor_hash, share_token_revision)
    WHERE visitor_hash <> '';
