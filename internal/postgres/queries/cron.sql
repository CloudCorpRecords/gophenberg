-- SPDX-License-Identifier: Apache-2.0

-- name: ListCronSlots :many
SELECT name, next_run_at, leased_until
FROM core.cron_jobs;

-- name: TakeCronJob :one
INSERT INTO core.cron_jobs AS stored (name, next_run_at, leased_until, lease_token, last_started_at)
VALUES (@name, @taken_at, @leased_until, @lease_token, @taken_at)
ON CONFLICT (name) DO UPDATE
SET leased_until = excluded.leased_until,
    lease_token = excluded.lease_token,
    last_started_at = excluded.last_started_at
WHERE stored.next_run_at <= excluded.last_started_at
  AND (stored.leased_until IS NULL OR stored.leased_until <= excluded.last_started_at)
RETURNING next_run_at;

-- name: SettleCronJob :execrows
UPDATE core.cron_jobs
SET next_run_at = @next_run_at,
    leased_until = NULL,
    lease_token = NULL,
    last_finished_at = @finished_at,
    last_error = sqlc.narg('last_error')
WHERE name = @name AND lease_token = @lease_token;
