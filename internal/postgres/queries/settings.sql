-- SPDX-License-Identifier: Apache-2.0

-- name: GetSetting :one
SELECT s.value FROM core.settings s WHERE s.key = @key;

-- name: SetSetting :exec
INSERT INTO core.settings (key, value)
VALUES (@key, @value)
ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value;

-- name: LockSettings :exec
SELECT pg_advisory_xact_lock(hashtext('core.settings'));
