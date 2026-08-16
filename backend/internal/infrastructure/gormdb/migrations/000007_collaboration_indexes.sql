CREATE INDEX session_messages_replay_index
    ON session_messages (session_id, id);

CREATE INDEX memory_candidates_task_index
    ON memory_candidates (coding_task_id, proposed_at DESC, id DESC);

CREATE INDEX agent_memories_agent_index
    ON agent_memories (agent_id, created_at DESC, id DESC);

CREATE INDEX workspace_write_leases_expiry_index
    ON workspace_write_leases (expires_at);
