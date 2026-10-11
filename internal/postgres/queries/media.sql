-- SPDX-License-Identifier: Apache-2.0

-- name: CreateMedia :one
INSERT INTO core.media (
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
FROM core.media m
WHERE m.id = @id;

-- name: ListMediaByIDs :many
SELECT m.id, m.media_type, m.file, m.title, m.alt_text, m.caption, m.description,
    m.mime_type, m.width, m.height, m.filesize, m.sizes, m.author_id, m.created_at, m.updated_at
FROM core.media m
WHERE m.id = ANY(@ids::bigint []);

-- name: ListSavedMediaFiles :many
SELECT m.file
FROM core.media m
WHERE m.file = ANY(@files::text []);

-- name: ListMedia :many
SELECT m.id, m.media_type, m.file, m.title, m.alt_text, m.caption, m.description,
    m.mime_type, m.width, m.height, m.filesize, m.sizes, m.author_id, m.created_at, m.updated_at,
    u.name AS author_name
FROM core.media m
JOIN auth.users u ON u.id = m.author_id
WHERE (@media_type::text = '' OR m.media_type = @media_type)
    AND (
        cardinality(@mimes::text[]) = 0
        OR EXISTS (
            SELECT 1 FROM unnest(@mimes::text[]) AS prefix
            WHERE m.mime_type LIKE prefix || '%'
        )
    )
    AND (cardinality(@authors::uuid[]) = 0 OR m.author_id = ANY(@authors::uuid[]))
    AND m.author_id <> ALL(@excluded_authors::uuid[])
    AND (sqlc.narg(before)::timestamptz IS NULL OR m.created_at < sqlc.narg(before)::timestamptz)
    AND (sqlc.narg(after)::timestamptz IS NULL OR m.created_at > sqlc.narg(after)::timestamptz)
    AND (sqlc.narg(modified_before)::timestamptz IS NULL OR m.updated_at < sqlc.narg(modified_before)::timestamptz)
    AND (sqlc.narg(modified_after)::timestamptz IS NULL OR m.updated_at > sqlc.narg(modified_after)::timestamptz)
    AND (
        @search::text = ''
        OR m.title ILIKE '%' || @search || '%'
        OR m.file ILIKE '%' || @search || '%'
        OR m.caption ILIKE '%' || @search || '%'
        OR m.description ILIKE '%' || @search || '%'
    )
ORDER BY
    CASE WHEN @order_by::text = 'title' AND @order_dir::text = 'asc' THEN m.title END ASC,
    CASE WHEN @order_by::text = 'title' AND @order_dir::text <> 'asc' THEN m.title END DESC,
    CASE WHEN @order_by::text = 'author' AND @order_dir::text = 'asc' THEN u.name END ASC,
    CASE WHEN @order_by::text = 'author' AND @order_dir::text <> 'asc' THEN u.name END DESC,
    CASE WHEN @order_by::text = 'modified' AND @order_dir::text = 'asc' THEN m.updated_at END ASC,
    CASE WHEN @order_by::text = 'modified' AND @order_dir::text <> 'asc' THEN m.updated_at END DESC,
    CASE WHEN @order_by::text NOT IN ('title', 'author', 'modified') AND @order_dir::text = 'asc'
        THEN m.created_at END ASC,
    CASE WHEN @order_by::text NOT IN ('title', 'author', 'modified') AND @order_dir::text <> 'asc'
        THEN m.created_at END DESC,
    m.created_at DESC,
    m.id DESC
LIMIT @row_limit OFFSET @row_offset;

-- name: CountMedia :one
SELECT count(*)
FROM core.media m
WHERE (@media_type::text = '' OR m.media_type = @media_type)
    AND (
        cardinality(@mimes::text[]) = 0
        OR EXISTS (
            SELECT 1 FROM unnest(@mimes::text[]) AS prefix
            WHERE m.mime_type LIKE prefix || '%'
        )
    )
    AND (cardinality(@authors::uuid[]) = 0 OR m.author_id = ANY(@authors::uuid[]))
    AND m.author_id <> ALL(@excluded_authors::uuid[])
    AND (sqlc.narg(before)::timestamptz IS NULL OR m.created_at < sqlc.narg(before)::timestamptz)
    AND (sqlc.narg(after)::timestamptz IS NULL OR m.created_at > sqlc.narg(after)::timestamptz)
    AND (sqlc.narg(modified_before)::timestamptz IS NULL OR m.updated_at < sqlc.narg(modified_before)::timestamptz)
    AND (sqlc.narg(modified_after)::timestamptz IS NULL OR m.updated_at > sqlc.narg(modified_after)::timestamptz)
    AND (
        @search::text = ''
        OR m.title ILIKE '%' || @search || '%'
        OR m.file ILIKE '%' || @search || '%'
        OR m.caption ILIKE '%' || @search || '%'
        OR m.description ILIKE '%' || @search || '%'
    );

-- name: UpdateMedia :one
UPDATE core.media AS m
SET title = @title, alt_text = @alt_text, caption = @caption,
    description = @description, updated_at = @updated_at
WHERE m.id = @id AND m.updated_at = @expected_updated_at
RETURNING m.id, m.media_type, m.file, m.title, m.alt_text, m.caption, m.description,
    m.mime_type, m.width, m.height, m.filesize, m.sizes, m.author_id, m.created_at, m.updated_at;

-- name: DeleteMedia :one
DELETE FROM core.media AS m
WHERE m.id = @id
RETURNING m.id, m.media_type, m.file, m.title, m.alt_text, m.caption, m.description,
    m.mime_type, m.width, m.height, m.filesize, m.sizes, m.author_id, m.created_at, m.updated_at;
