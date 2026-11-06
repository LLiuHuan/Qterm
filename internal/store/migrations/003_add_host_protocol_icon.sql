ALTER TABLE hosts ADD COLUMN protocol TEXT NOT NULL DEFAULT 'ssh';
ALTER TABLE hosts ADD COLUMN icon TEXT NOT NULL DEFAULT '';

CREATE INDEX hosts_protocol_idx ON hosts(protocol);
