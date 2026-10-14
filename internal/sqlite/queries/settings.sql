-- SPDX-License-Identifier: Apache-2.0

-- name: GetSetting :one
SELECT s.value FROM core_settings s WHERE s.key = @key;

-- name: SetSetting :exec
INSERT INTO core_settings (key, value)
VALUES (@key, @value)
ON CONFLICT (key) DO UPDATE SET value = excluded.value;
