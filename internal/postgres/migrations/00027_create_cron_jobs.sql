-- SPDX-License-Identifier: Apache-2.0

-- +goose Up
CREATE TABLE core.cron_jobs (
    name text NOT NULL,
    next_run_at timestamptz NOT NULL,
    leased_until timestamptz,
    lease_token text,
    last_started_at timestamptz,
    last_finished_at timestamptz,
    last_error text,
    CONSTRAINT cron_jobs_pkey PRIMARY KEY (name),
    CONSTRAINT cron_jobs_lease_whole CHECK ((leased_until IS NULL) = (lease_token IS NULL))
);

-- +goose Down
DROP TABLE core.cron_jobs;
