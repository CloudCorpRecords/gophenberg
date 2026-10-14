-- SPDX-License-Identifier: Apache-2.0

-- +goose Up
CREATE TABLE core_cron_jobs (
    name TEXT NOT NULL,
    next_run_at INTEGER NOT NULL,
    leased_until INTEGER,
    lease_token TEXT,
    last_started_at INTEGER,
    last_finished_at INTEGER,
    last_error TEXT,
    CONSTRAINT cron_jobs_pkey PRIMARY KEY (name),
    CONSTRAINT cron_jobs_lease_whole CHECK ((leased_until IS NULL) = (lease_token IS NULL))
);

-- +goose Down
DROP TABLE core_cron_jobs;
