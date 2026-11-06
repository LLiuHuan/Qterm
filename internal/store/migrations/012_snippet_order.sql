ALTER TABLE snippets ADD COLUMN sort_order INTEGER NOT NULL DEFAULT 0;
WITH ranked AS (
 SELECT id, ROW_NUMBER() OVER (PARTITION BY snippet_group_id ORDER BY name COLLATE NOCASE,id) - 1 AS position FROM snippets
)
UPDATE snippets SET sort_order=(SELECT position FROM ranked WHERE ranked.id=snippets.id);
CREATE INDEX snippets_sort_idx ON snippets(snippet_group_id,sort_order);
