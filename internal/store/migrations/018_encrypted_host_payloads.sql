ALTER TABLE hosts ADD COLUMN encrypted_payload BLOB;

CREATE INDEX hosts_encrypted_payload_idx ON hosts(id);
