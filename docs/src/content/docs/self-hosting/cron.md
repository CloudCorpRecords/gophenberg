---
title: Cron
description: Work that runs on a rhythm, how visits and the host's cron start it, and what to set.
---

Some work has to happen on a rhythm rather than when someone clicks:
sweeping expired sessions every hour, for example. Gophenberg calls
each piece of such work a cron job. A cron job has a name and an
interval, and the database keeps when it falls due next.

Nothing runs at an exact minute. On shared hosting no process stays
awake to watch a clock, because the host stops a quiet site after a
few minutes. So a job simply becomes due, and the first chance after
that runs it. WordPress works the same way with its scheduled events,
the ones it calls WP-Cron.

## Visits start them

After the server answers a request, it checks whether a job is due,
at most once every `GOPHENBERG_CRON_VISIT_PACE`, one minute unless
set. A job that is due then runs in the background. The visitor never
waits for it, because the page has already been answered.

Every request counts as a visit: a public page, the admin, a file, a
plugin's address. A site with no visitors runs no cron job, so a quiet
site that needs its jobs on time also wants the cron address below.

## The cron address

Most hosts also offer a cron of their own, a tool that runs a command
on a timetable you choose. The host's cron can run Gophenberg's cron
jobs too, by sending `POST /api/cron` with a secret. The address stays
off until you set `GOPHENBERG_CRON_SECRET` to at least 32 characters.
Make one with:

```sh
openssl rand -hex 32
```

The secret travels in a header, never in the address, so it stays out
of the host's access logs. Keep it out of the command as well. While a
command runs, other accounts on the same server may read its words in
the list of running processes. So the secret goes in a small file only
your account can read, and the command names that file.

On a cPanel plan:

1. Open cPanel's **File Manager**, go to your home folder, not
   `public_html`, and create a file named `gophenberg-cron-header`.
2. Edit the file so it holds this one line, with your own secret:

   ```text
   Authorization: Bearer PASTE_THE_SECRET
   ```

3. Choose **Permissions** on the file and set them to `600`, so only
   your account can read it.
4. Open cPanel's **Cron Jobs** screen, choose **Once Per Five Minutes**,
   and paste this command with your own address:

   ```sh
   curl -fsS -X POST -H @"$HOME/gophenberg-cron-header" https://example.com/api/cron > /dev/null
   ```

On a server of your own, create the same file so only you can read it:

```sh
install -m 600 /dev/null ~/gophenberg-cron-header
```

Write the `Authorization` line into it with an editor such as `nano`,
then add a crontab line that runs every minute:

```sh
* * * * * curl -fsS -X POST -H @"$HOME/gophenberg-cron-header" https://example.com/api/cron > /dev/null
```

When `GOPHENBERG_PUBLIC_URL` is set, call that public address. A call
to another address, such as `localhost`, needs
`-H "Host: example.com"` as well, or the server refuses it.

The call answers with how many jobs ran, failed, and were held by
another run, such as `{"ran":1,"failed":0,"held":0}`. A job that ran
but whose result the server could not save counts as failed, because
it runs again once its hold runs out. A caller that hangs up or times
out does not cut a job short.

What the other answers mean:

- **404** means the secret is unset, or the call sent a wrong secret
  or none. It reads exactly like any address the site does not have.
- **405** means the call used another method than `POST`. Once the
  address is on, a `GET` answers 405, which tells a stranger the
  address exists. That is harmless, because the secret guards it.
- **500** means the server could not read its cron jobs, usually
  because the database is unreachable or did not answer within
  `GOPHENBERG_CRON_LEASE`. `curl -f` turns it into an
  error, and cPanel mails that command's output to you.

When the host's cron calls the address, you can turn off the visit
trigger with `GOPHENBERG_CRON_ON_VISITS=false`. This is what
`DISABLE_WP_CRON` does in WordPress. The server refuses to start with
visits off and no secret, because then no cron job would ever run.

## What a job may do

Each run has a time limit, `GOPHENBERG_CRON_TIME_LIMIT`, 30 seconds
unless set. A job that fails, panics or runs past its limit is
recorded, and runs again at its next due time. A job that falls due
while the site is down runs once at the next chance, not once for
every time it missed.

While a run goes on, it holds its job for `GOPHENBERG_CRON_LEASE`,
one minute unless set, so two visits, or two copies of the server,
never run the same job at once. Two things follow:

- **A job must be safe to run twice.** When the host kills the site in
  the middle of a run, nothing records it, so the job runs again from
  the start once its hold runs out.
- **A job must stop when it is told to.** A job that ignores its stop
  can run on past its limit, and once its hold runs out another run may
  take the same job beside it.

When more than one server serves the site, keep their clocks in step,
for example with NTP. A server judges another's hold by its own clock.

The server also waits no longer than `GOPHENBERG_CRON_LEASE` for the
database to list the jobs or hand one over. When the database fails to
hand over a job or to record how it ended, the run stops there, and the
other jobs wait for the next check. A database that stops answering
delays the cron jobs, but later visits still check again.

A stop tells a running job to stop as soon as the server stops taking
requests, and gives it `GOPHENBERG_CRON_STOP_GRACE`, two seconds unless
set, to end and record itself, before the plugins stop. A job that ends
and records itself within that time runs again at the first check after
the restart. One that does not keeps its hold, and runs again once its
hold runs out.

## The job that ships

Gophenberg ships one cron job, `core/session-sweep`. Every
`GOPHENBERG_SESSION_SWEEP_INTERVAL`, one hour unless set, it deletes
expired sessions and expired invite, reset and confirmation links,
and the accounts an expired invite left behind.

It logs what it removed as `reaped expired sessions` and
`reaped expired tokens`, as before. A sweep that fails is now logged
at Warn, not Error, as `cron job failed` with `job=core/session-sweep`,
in place of the older `reap expired sessions` and
`reap expired tokens` lines.

When the server cannot read its cron jobs at all, usually because the
database is unreachable, no job runs and it logs `cron jobs unread` at
Error, with no `job` field. Update any alert that watched for the
older lines, or for the Error level alone, to watch both new lines.
