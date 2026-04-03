ALTER TABLE invocations
    MODIFY COLUMN status ENUM('submitted', 'assigned', 'working', 'completed', 'failed', 'cancelled')
    NOT NULL DEFAULT 'submitted';

ALTER TABLE invocations
    DROP COLUMN call_chain,
    DROP COLUMN output_ref,
    DROP COLUMN input_ref;

ALTER TABLE agents
    DROP COLUMN avatar_url;

ALTER TABLE users
    DROP COLUMN auth_provider,
    DROP COLUMN avatar_url;
