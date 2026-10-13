-- SPDX-License-Identifier: Apache-2.0

-- +goose Up
CREATE TABLE core_content_types (
    key TEXT NOT NULL PRIMARY KEY,
    singular_label TEXT NOT NULL,
    plural_label TEXT NOT NULL,
    route_word TEXT NOT NULL,
    hierarchical BOOLEAN NOT NULL DEFAULT 0 CHECK (hierarchical IN (0, 1)),
    revisions BOOLEAN NOT NULL DEFAULT 1 CHECK (revisions IN (0, 1)),
    revision_cap INTEGER NOT NULL DEFAULT 0,
    page_kind TEXT NOT NULL CONSTRAINT content_types_page_kind_check CHECK (page_kind IN ('single', 'archive')),
    is_default BOOLEAN NOT NULL DEFAULT 0 CHECK (is_default IN (0, 1)),
    active BOOLEAN NOT NULL DEFAULT 1 CHECK (active IN (0, 1)),
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL,
    origin TEXT,
    description TEXT NOT NULL DEFAULT '',
    CONSTRAINT content_types_root_is_the_default CHECK (route_word <> '' OR is_default),
    CONSTRAINT content_types_default_stays_active CHECK (active OR NOT is_default)
);

CREATE UNIQUE INDEX core_content_types_one_default_idx ON core_content_types (is_default)
    WHERE is_default;

CREATE UNIQUE INDEX core_content_types_route_word_idx ON core_content_types (route_word)
    WHERE route_word <> '';

INSERT INTO core_content_types (
    key, singular_label, plural_label, route_word, hierarchical, revisions,
    revision_cap, page_kind, is_default, active, created_at, updated_at, description
)
VALUES (
    'post', 'Post', 'Posts', '', 0, 1,
    100, 'single', 1, 1,
    CAST(round(unixepoch('subsec') * 1000) AS INTEGER) * 1000,
    CAST(round(unixepoch('subsec') * 1000) AS INTEGER) * 1000,
    'Manage the posts on this site.'
);

CREATE TABLE core_content (
    id TEXT NOT NULL PRIMARY KEY,
    type TEXT NOT NULL CONSTRAINT content_type_fkey REFERENCES core_content_types (key) ON DELETE RESTRICT,
    status TEXT NOT NULL CONSTRAINT content_status_check CHECK (
        status IN ('draft', 'pending', 'private', 'scheduled', 'published', 'trash')
    ),
    slug TEXT NOT NULL,
    title TEXT NOT NULL DEFAULT '',
    content TEXT NOT NULL DEFAULT '',
    excerpt TEXT NOT NULL DEFAULT '',
    author_id TEXT NOT NULL CONSTRAINT content_author_id_fkey REFERENCES auth_users (id) ON DELETE RESTRICT,
    published_at INTEGER,
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL,
    parent_id TEXT CONSTRAINT content_parent_id_fkey REFERENCES core_content (id) ON DELETE RESTRICT,
    path TEXT NOT NULL,
    fields TEXT NOT NULL DEFAULT '{}' CHECK (json_valid(fields)),
    CONSTRAINT content_published_needs_date CHECK (
        status NOT IN ('scheduled', 'published') OR published_at IS NOT NULL
    ),
    CONSTRAINT content_path_is_addressable CHECK (path <> '')
);

CREATE UNIQUE INDEX core_content_path_unique ON core_content (path);

CREATE INDEX core_content_type_status_published_at_idx ON core_content (type, status, published_at DESC);

CREATE INDEX core_content_author_id_idx ON core_content (author_id);

CREATE INDEX core_content_parent_id_idx ON core_content (parent_id);

CREATE TABLE core_content_revisions (
    id TEXT NOT NULL PRIMARY KEY,
    content_id TEXT NOT NULL CONSTRAINT content_revisions_content_id_fkey
        REFERENCES core_content (id) ON DELETE CASCADE,
    kind TEXT NOT NULL CONSTRAINT content_revisions_kind_check CHECK (kind IN ('revision', 'autosave')),
    author_id TEXT NOT NULL CONSTRAINT content_revisions_author_id_fkey
        REFERENCES auth_users (id) ON DELETE RESTRICT,
    title TEXT NOT NULL DEFAULT '',
    content TEXT NOT NULL DEFAULT '',
    excerpt TEXT NOT NULL DEFAULT '',
    created_at INTEGER NOT NULL,
    fields TEXT NOT NULL DEFAULT '{}' CHECK (json_valid(fields))
);

CREATE INDEX core_content_revisions_content_id_created_at_idx
    ON core_content_revisions (content_id, created_at DESC);

CREATE INDEX core_content_revisions_author_id_idx ON core_content_revisions (author_id);

CREATE UNIQUE INDEX core_content_revisions_one_autosave_per_author_idx
    ON core_content_revisions (content_id, author_id) WHERE kind = 'autosave';

CREATE TABLE core_settings (
    key TEXT NOT NULL PRIMARY KEY,
    value TEXT NOT NULL
);

CREATE TABLE core_media (
    id INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,
    media_type TEXT NOT NULL CONSTRAINT media_media_type_check CHECK (media_type IN ('image', 'file')),
    file TEXT NOT NULL,
    title TEXT NOT NULL DEFAULT '',
    alt_text TEXT NOT NULL DEFAULT '',
    caption TEXT NOT NULL DEFAULT '',
    description TEXT NOT NULL DEFAULT '',
    mime_type TEXT NOT NULL,
    width INTEGER NOT NULL DEFAULT 0,
    height INTEGER NOT NULL DEFAULT 0,
    filesize INTEGER NOT NULL DEFAULT 0,
    sizes TEXT NOT NULL DEFAULT '{}' CHECK (json_valid(sizes)),
    author_id TEXT NOT NULL CONSTRAINT media_author_id_fkey REFERENCES auth_users (id) ON DELETE RESTRICT,
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL
);

CREATE INDEX core_media_created_at_idx ON core_media (created_at DESC, id DESC);

CREATE INDEX core_media_media_type_idx ON core_media (media_type);

CREATE INDEX core_media_author_id_idx ON core_media (author_id);

CREATE TABLE core_user_settings (
    user_id TEXT NOT NULL,
    key TEXT NOT NULL,
    value TEXT NOT NULL,
    CONSTRAINT user_settings_pkey PRIMARY KEY (user_id, key)
);

CREATE TABLE core_field_groups (
    id INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,
    title TEXT NOT NULL,
    location TEXT NOT NULL DEFAULT '[]',
    position INTEGER NOT NULL DEFAULT 0,
    active BOOLEAN NOT NULL DEFAULT 1 CHECK (active IN (0, 1)),
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL,
    origin TEXT,
    key TEXT NOT NULL,
    CONSTRAINT field_groups_location_is_array CHECK (json_valid(location) AND json_type(location) = 'array')
);

CREATE UNIQUE INDEX core_field_groups_key_unique ON core_field_groups (key);

CREATE TABLE core_content_fields (
    id INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,
    key TEXT NOT NULL,
    label TEXT NOT NULL,
    kind TEXT NOT NULL,
    relates_to TEXT CONSTRAINT content_fields_relates_to_fkey REFERENCES core_content_types (key),
    many BOOLEAN NOT NULL DEFAULT 0 CHECK (many IN (0, 1)),
    required BOOLEAN NOT NULL DEFAULT 0 CHECK (required IN (0, 1)),
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL,
    position INTEGER NOT NULL DEFAULT 0,
    group_id INTEGER NOT NULL CONSTRAINT content_fields_group_fkey
        REFERENCES core_field_groups (id) ON DELETE RESTRICT,
    settings TEXT NOT NULL DEFAULT '{}' CHECK (json_valid(settings)),
    parent_field_id INTEGER CONSTRAINT content_fields_parent_fkey
        REFERENCES core_content_fields (id) ON DELETE CASCADE,
    depth INTEGER NOT NULL DEFAULT 0,
    origin TEXT,
    CONSTRAINT content_fields_relation_target CHECK ((kind = 'relation') = (relates_to IS NOT NULL)),
    CONSTRAINT content_fields_many_relational CHECK (many = 0 OR kind IN ('relation', 'media'))
);

CREATE UNIQUE INDEX core_content_fields_scope_key_unique
    ON core_content_fields (group_id, coalesce(parent_field_id, 0), key);

CREATE UNIQUE INDEX core_content_fields_container_key_unique
    ON core_content_fields (parent_field_id, key) WHERE parent_field_id IS NOT NULL;

CREATE TABLE core_content_relations (
    from_id TEXT NOT NULL CONSTRAINT content_relations_from_id_fkey
        REFERENCES core_content (id) ON DELETE CASCADE,
    field_id INTEGER NOT NULL CONSTRAINT content_relations_field_id_fkey
        REFERENCES core_content_fields (id) ON DELETE CASCADE,
    to_id TEXT NOT NULL CONSTRAINT content_relations_to_id_fkey
        REFERENCES core_content (id) ON DELETE CASCADE,
    position INTEGER NOT NULL DEFAULT 1,
    sort_at INTEGER NOT NULL,
    visible BOOLEAN NOT NULL DEFAULT 0 CHECK (visible IN (0, 1)),
    CONSTRAINT content_relations_pkey PRIMARY KEY (from_id, field_id, to_id)
);

CREATE INDEX core_content_relations_target_idx
    ON core_content_relations (to_id, sort_at DESC, from_id, field_id)
    WHERE visible;

-- +goose Down
DROP TABLE core_content_relations;

DROP TABLE core_content_fields;

DROP TABLE core_field_groups;

DROP TABLE core_user_settings;

DROP TABLE core_media;

DROP TABLE core_settings;

DROP TABLE core_content_revisions;

DROP TABLE core_content;

DROP TABLE core_content_types;
