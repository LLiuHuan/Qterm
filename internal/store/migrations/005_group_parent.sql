ALTER TABLE groups ADD COLUMN parent_id TEXT REFERENCES groups(id) ON UPDATE CASCADE ON DELETE RESTRICT;
CREATE INDEX groups_parent_id_idx ON groups(parent_id);
