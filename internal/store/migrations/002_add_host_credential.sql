ALTER TABLE hosts ADD COLUMN credential_id TEXT NOT NULL DEFAULT '';

CREATE INDEX hosts_credential_id_idx ON hosts(credential_id);
