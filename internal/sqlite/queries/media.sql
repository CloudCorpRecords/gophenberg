-- SPDX-License-Identifier: Apache-2.0

-- name: CreateMedia :one
INSERT INTO core_media (
    media_type, file, title, alt_text, caption, description,
    mime_type, width, height, filesize, sizes, author_id, created_at, updated_at
)
VALUES (
    @media_type, @file, @title, @alt_text, @caption, @description,
    @mime_type, @width, @height, @filesize, @sizes, @author_id, @created_at, @updated_at
)
RETURNING id, media_type, file, title, alt_text, caption, description,
    mime_type, width, height, filesize, sizes, author_id, created_at, updated_at;

-- name: GetMedia :one
SELECT m.id, m.media_type, m.file, m.title, m.alt_text, m.caption, m.description,
    m.mime_type, m.width, m.height, m.filesize, m.sizes, m.author_id, m.created_at, m.updated_at
FROM core_media m
WHERE m.id = @id;

-- name: ListMediaByIDs :many
SELECT m.id, m.media_type, m.file, m.title, m.alt_text, m.caption, m.description,
    m.mime_type, m.width, m.height, m.filesize, m.sizes, m.author_id, m.created_at, m.updated_at
FROM core_media m
WHERE m.id IN (SELECT value FROM json_each(CAST(@ids AS TEXT)));

-- name: ListSavedMediaFiles :many
SELECT m.file
FROM core_media m
WHERE m.file IN (SELECT value FROM json_each(CAST(@files AS TEXT)));

-- name: ListMedia :many
WITH sort_order AS (
    SELECT CAST(@order_by AS TEXT) AS order_by, CAST(@order_dir AS TEXT) AS order_dir
)
SELECT sqlc.embed(m), u.name AS author_name
FROM core_media m
JOIN auth_users u ON u.id = m.author_id
CROSS JOIN sort_order s
WHERE (CAST(@media_type AS TEXT) = '' OR m.media_type = @media_type)
    AND (
        json_array_length(CAST(@mimes AS TEXT)) = 0
        OR EXISTS (
            SELECT 1 FROM json_each(CAST(@mimes AS TEXT)) AS prefix
            WHERE substr(m.mime_type, 1, length(prefix.value)) = prefix.value
        )
    )
    AND (
        json_array_length(CAST(@authors AS TEXT)) = 0
        OR m.author_id IN (SELECT value FROM json_each(CAST(@authors AS TEXT)))
    )
    AND m.author_id NOT IN (SELECT value FROM json_each(CAST(@excluded_authors AS TEXT)))
    AND (CAST(sqlc.narg(before) AS TIMESTAMP) IS NULL OR m.created_at < CAST(sqlc.narg(before) AS TIMESTAMP))
    AND (CAST(sqlc.narg(after) AS TIMESTAMP) IS NULL OR m.created_at > CAST(sqlc.narg(after) AS TIMESTAMP))
    AND (
        CAST(sqlc.narg(modified_before) AS TIMESTAMP) IS NULL
        OR m.updated_at < CAST(sqlc.narg(modified_before) AS TIMESTAMP)
    )
    AND (
        CAST(sqlc.narg(modified_after) AS TIMESTAMP) IS NULL
        OR m.updated_at > CAST(sqlc.narg(modified_after) AS TIMESTAMP)
    )
    AND (
        CAST(@search AS TEXT) = ''
        OR instr(casefold(m.title), casefold(CAST(@search AS TEXT))) > 0
        OR instr(casefold(m.file), casefold(CAST(@search AS TEXT))) > 0
        OR instr(casefold(m.caption), casefold(CAST(@search AS TEXT))) > 0
        OR instr(casefold(m.description), casefold(CAST(@search AS TEXT))) > 0
    )
ORDER BY
    CASE WHEN s.order_by = 'title' AND s.order_dir = 'asc' THEN casefold(m.title) END ASC,
    CASE WHEN s.order_by = 'title' AND s.order_dir <> 'asc' THEN casefold(m.title) END DESC,
    CASE WHEN s.order_by = 'author' AND s.order_dir = 'asc' THEN casefold(u.name) END ASC,
    CASE WHEN s.order_by = 'author' AND s.order_dir <> 'asc' THEN casefold(u.name) END DESC,
    CASE WHEN s.order_by = 'modified' AND s.order_dir = 'asc' THEN m.updated_at END ASC,
    CASE WHEN s.order_by = 'modified' AND s.order_dir <> 'asc' THEN m.updated_at END DESC,
    CASE WHEN s.order_by NOT IN ('title', 'author', 'modified') AND s.order_dir = 'asc'
        THEN m.created_at END ASC,
    CASE WHEN s.order_by NOT IN ('title', 'author', 'modified') AND s.order_dir <> 'asc'
        THEN m.created_at END DESC,
    m.created_at DESC,
    m.id DESC
LIMIT max(CAST(@row_limit AS INTEGER), 0) OFFSET @row_offset;

-- name: CountMedia :one
SELECT count(*)
FROM core_media m
WHERE (CAST(@media_type AS TEXT) = '' OR m.media_type = @media_type)
    AND (
        json_array_length(CAST(@mimes AS TEXT)) = 0
        OR EXISTS (
            SELECT 1 FROM json_each(CAST(@mimes AS TEXT)) AS prefix
            WHERE substr(m.mime_type, 1, length(prefix.value)) = prefix.value
        )
    )
    AND (
        json_array_length(CAST(@authors AS TEXT)) = 0
        OR m.author_id IN (SELECT value FROM json_each(CAST(@authors AS TEXT)))
    )
    AND m.author_id NOT IN (SELECT value FROM json_each(CAST(@excluded_authors AS TEXT)))
    AND (CAST(sqlc.narg(before) AS TIMESTAMP) IS NULL OR m.created_at < CAST(sqlc.narg(before) AS TIMESTAMP))
    AND (CAST(sqlc.narg(after) AS TIMESTAMP) IS NULL OR m.created_at > CAST(sqlc.narg(after) AS TIMESTAMP))
    AND (
        CAST(sqlc.narg(modified_before) AS TIMESTAMP) IS NULL
        OR m.updated_at < CAST(sqlc.narg(modified_before) AS TIMESTAMP)
    )
    AND (
        CAST(sqlc.narg(modified_after) AS TIMESTAMP) IS NULL
        OR m.updated_at > CAST(sqlc.narg(modified_after) AS TIMESTAMP)
    )
    AND (
        CAST(@search AS TEXT) = ''
        OR instr(casefold(m.title), casefold(CAST(@search AS TEXT))) > 0
        OR instr(casefold(m.file), casefold(CAST(@search AS TEXT))) > 0
        OR instr(casefold(m.caption), casefold(CAST(@search AS TEXT))) > 0
        OR instr(casefold(m.description), casefold(CAST(@search AS TEXT))) > 0
    );

-- name: UpdateMedia :one
UPDATE core_media
SET title = @title, alt_text = @alt_text, caption = @caption,
    description = @description, updated_at = @updated_at
WHERE id = @id AND updated_at = @expected_updated_at
RETURNING id, media_type, file, title, alt_text, caption, description,
    mime_type, width, height, filesize, sizes, author_id, created_at, updated_at;

-- name: DeleteMedia :one
DELETE FROM core_media
WHERE id = @id
RETURNING id, media_type, file, title, alt_text, caption, description,
    mime_type, width, height, filesize, sizes, author_id, created_at, updated_at;
