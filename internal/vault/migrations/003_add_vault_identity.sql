CREATE TABLE vault_identity (
    singleton INTEGER PRIMARY KEY CHECK (singleton = 1),
    envelope BLOB NOT NULL
);
