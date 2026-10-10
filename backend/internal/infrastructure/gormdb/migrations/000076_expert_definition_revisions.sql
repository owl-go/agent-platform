CREATE TABLE expert_definition_revisions (
    expert_id uuid NOT NULL,
    version bigint NOT NULL CHECK (version > 0),
    owner_user_id uuid NOT NULL,
    definition jsonb NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (expert_id, version)
);

INSERT INTO expert_definition_revisions(expert_id, version, owner_user_id, definition)
SELECT id, version, owner_user_id, to_jsonb(experts) FROM experts;

CREATE FUNCTION preserve_expert_definition_revision() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF TG_OP = 'UPDATE' AND NEW.version = OLD.version THEN RETURN NEW; END IF;
    INSERT INTO expert_definition_revisions(expert_id, version, owner_user_id, definition)
    VALUES (NEW.id, NEW.version, NEW.owner_user_id, to_jsonb(NEW));
    RETURN NEW;
END;
$$;
CREATE TRIGGER expert_definition_revision AFTER INSERT OR UPDATE ON experts
FOR EACH ROW EXECUTE FUNCTION preserve_expert_definition_revision();

CREATE FUNCTION reject_definition_revision_mutation() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    RAISE EXCEPTION 'definition revisions are immutable' USING ERRCODE = '23514';
END;
$$;
CREATE TRIGGER immutable_expert_definition_revision BEFORE UPDATE OR DELETE ON expert_definition_revisions
FOR EACH ROW EXECUTE FUNCTION reject_definition_revision_mutation();
