---
title: Updates and backups
description: How to upgrade safely, what to back up, and how to restore it.
---

Two operational jobs: keeping Gophenberg current, and being able
to put it back after something goes wrong.

## Updating

Your compose file pins an exact version, so nothing updates behind
your back:

```sh
# 1. edit compose.yaml: change the image tag to the new version
docker compose pull
docker compose run --rm -T gophenberg check
docker compose up -d
```

`check` reads your settings with the new version while the old one
still serves, and names the ones it refuses, see
[what stops startup](/self-hosting/configuration/#what-stops-startup).
Migrations run automatically when the new version starts, and
`docker compose run --rm -T gophenberg migrate` applies them without
starting the server. Two copies that migrate at once never apply the
same migration twice, the second waits for the first and then finds
nothing left to do. While Gophenberg is below 1.0, read the release
notes first, since a release can change behavior.

Updating to %VERSION% from the release before this one runs two
migrations when the server starts, or when you run `migrate`. The
first gives every content type a short description. It fills the one
of Posts with *Manage the posts on this site.* and leaves the others
empty. Until you change it, the admin shows that sentence in the
language each person reads the admin in, as it does the names of the
built-in types. The second adds an empty table where the server keeps
its [cron jobs](/self-hosting/cron/). Neither changes anything else
you stored, and the database account needs no right it did not use
before. If a migration fails, the server stops at start, and the
older image still starts on that database. This release serves every
theme kit the release before it serves, so no theme needs rebuilding.
Back up the database before the first start on this release, as
before any update.

Coming from the release before this one, but not from its newest
patch, this release also runs themes built on `@gophenberg/astro`
%KIT_VERSION%, which its earlier releases turn away with
`kit_unsupported`. On the first of them, **Empty Trash** on any list
but Posts deleted the trashed posts for good. This release empties
only the trash of the type on screen.

Some things work differently after this update:

- The admin opens only for a role carrying `access_admin`. The
  `admin`, `editor` and `author` roles carry it. The new `member`
  role signs in but never enters the admin, see
  [roles](/guides/users/#roles). An account holding no role, or a
  role from a plugin that is gone, is now turned away the same way.
  Before you update, run `account:list` and give each such account a
  role, see
  [upgrading a site](/guides/users/#upgrading-a-site-that-ran-an-earlier-version).
- Each admin area opens only to the roles carrying its capability.
  The `admin`, `editor` and `author` roles see the same screens as on
  the release before this one. The themes, types and settings areas
  of the API refuse a role without the capability with
  `capability_missing`, where the release before this one answered
  `role_insufficient`. A member, or an account holding no role, gets
  `capability_missing` on every content, media and type route. A
  program that reads those codes needs updating.
- Any account whose role carries `manage_users` may now run the
  account commands. They take `member` and the roles plugins declare,
  and refuse a role beyond the reach of the account in `-as`. A role
  nobody declares exits with code 2 and lists the roles there are.
  `account:role` and `account:disable` refuse to act on the account
  in `-as`, on a dry run too. An `-as` made only of spaces now exits
  with code 2, as an empty one did. Every account command but
  `account:list` and `account:records` registers the plugins first,
  so a plugin that fails to register stops it. See
  [who may change an account](/self-hosting/commands/#who-may-change-an-account).
- A stop takes up to 25 seconds at the default settings, two more
  than the release before this one. It gives the running requests
  `GOPHENBERG_SHUTDOWN_GRACE` to finish, cancels the rest, gives a
  running cron job `GOPHENBERG_CRON_STOP_GRACE` to end, and then gives
  the plugins and the theme their own time. Docker kills the server
  after 10 seconds unless the compose file sets `stop_grace_period`,
  so set it above 25 seconds, as the compose file from
  [Install](/self-hosting/install/) does with `30s`. See
  [stopping the server](/self-hosting/configuration/#stopping-the-server).
- The hourly sweep of expired sessions and links is now a
  [cron job](/self-hosting/cron/), the only one this release ships.
  It runs after a visit is answered, or when the host's cron calls the
  cron address. On a host that stops a quiet site, it waits for the
  next visit, so set `GOPHENBERG_CRON_SECRET` and a host cron to keep
  it on time. The secret needs at least 32 characters, and a shorter
  one stops the server at start. A failed sweep logs `cron job failed`
  at Warn with `job=core/session-sweep`, and a database that cannot
  list the jobs logs `cron jobs unread` at Error. These replace the
  `reap expired sessions` and `reap expired tokens` lines, so update
  any alert that watched for them.
- Plugins draw from a share of the site's one database pool, and
  `GOPHENBERG_PLUGIN_DB_RESERVE` of its connections, 2 by default,
  always stay free for the site. The pool holds the larger of 4 and
  the number of CPUs, so the default reserve always leaves the plugins
  some. A reserve as large as the pool leaves them nothing, so the
  server refuses to start, even with no plugin. `check` names the
  problem before you update.
- The admin writes dates, times and numbers in one format for every
  reader, set by `GOPHENBERG_FORMAT_LOCALE`. Its default, `es-ES`,
  writes 4 October 2026 as 04/10/2026, a time as 14:05 and a number as
  1.234,56, also for a reader who reads the admin in English. The date
  column of a content list is the one exception, and follows the
  language each person reads the admin in. Set the locale, such as to
  `en-US` or `en-GB`, for another style. See
  [the admin lists](/self-hosting/configuration/#the-admin-lists).
- Twenty-nine settings are new, and none is required. They size the
  admin lists, time the confirmation messages and the cron jobs, size
  the plugins' database share, cap how many uploaded images the server
  decodes and resizes at once, pick how the site draws avatars, and
  tune a SQLite database. A value out of range or outside its list
  stops startup, so run `check` with your settings first. A PostgreSQL
  address ignores the six that start with `GOPHENBERG_SQLITE_`. See
  [configuration](/self-hosting/configuration/).
- This release can check and migrate a SQLite file, named in
  `GOPHENBERG_DATABASE_URL` as `sqlite:` and its absolute path, but
  cannot serve a site from one yet. The seven settings it reads there
  are `GOPHENBERG_DATABASE_MAX_CONNS` and the six that start with
  `GOPHENBERG_SQLITE_`. No older release opens a SQLite file. See
  [a SQLite database](/self-hosting/configuration/#a-sqlite-database).
- The admin's own address opens the list of the default content
  type, since the home screen is gone. The status tabs no longer show
  how many items each holds. The message after a trash no longer
  offers **Undo**, so a trashed post comes back from the Trash tab.
  **Empty Trash** now shows only to an admin or an editor. A new slug,
  a slug typed in the editor and a route word a type takes from its
  label lose their accents, so *Café* gives `cafe`. Slugs already
  saved keep their address. The breadcrumb at the bottom of the
  editor starts at the item's own type, such as Page, and no longer at
  Post on every type. See [the posts list](/guides/posts/).
- An avatar now sits before each author's name in a content list. By
  default it is a circle with the first letter of the name, which the
  site draws itself, so nothing is sent to another service.
  `GOPHENBERG_AVATARS` can show Gravatar pictures instead, or an
  author icon, and plugins that show authors follow it too. See
  [avatars](/self-hosting/configuration/#avatars).
- A definitions file exported by this release carries the description
  of each type. The release before this one refuses such a file with
  `body_unknown_attribute`, so move definitions only between sites on
  this release. A file with no descriptions still imports here, and
  leaves the descriptions on the site as they are. A type an import
  adds now lists after the site's other types.
- On a site that builds its own plugins, the SDK is now a Go module
  of its own, and a plugin requires it alone. A plugin role meant for
  the admin needs `access_admin`. The
  [versions of the plugin SDK](/extending/the-plugin-sdk/#versions)
  list every step to move a plugin. Building from source needs
  Go 1.27.2.
- On PostgreSQL, `GOPHENBERG_DATABASE_MAX_CONNS` now caps the site's
  one database pool. Left unset, the pool keeps the size it had, the
  larger of 4 and the number of CPUs. `GOPHENBERG_DATABASE_URL` takes
  no `pool_` setting, and
  [the PostgreSQL address](/self-hosting/configuration/#the-postgresql-address)
  lists every form it may not hold. Run `check` with the new image
  before you update. It names the setting and the reason without
  connecting, and it now stops at every `pool_` setting and names it.
  For example:

  ```text
  gophenberg: GOPHENBERG_DATABASE_URL: dbkit: refused database address: a ? or # comes before an @ ahead of the first /, write ? as %3F, # as %23 and @ as %40
  ```

  These forms already stopped `migrate` and `serve` before this
  update, so no running site holds them: any `pool_` setting, such as
  `pool_max_conns`, whatever its value, an `@` inside the user name or
  the password, a space inside a value, a `%` that starts no `%` code,
  `%00`, and a pair after the `?` with no `=` or with two.

  These forms worked before and now stop the server, so change them
  first. The likeliest come first:

  - A `?`, a `#` or another sign in the password or the user name,
    such as `^` or `"`. Only an unaccented letter, a digit, a
    semicolon and the signs `-._:~!$&'()*+,=%` may stay as they are.
  - A key set twice, such as `sslmode=disable` twice. `dbname` and
    `database` count as one key.
  - A space at the start or end of a value, such as
    `application_name= site`.
  - A `#` or a control character, such as a tab, anywhere else.
  - A `+` or a semicolon after the `?`.
  - `ssl` in place of `sslmode`.
  - A host list with an empty host, or with a port on some hosts only.
  - A database name that starts with `/`.
  - A backslash in a connection string written as `key=value` pairs.

  [The PostgreSQL address](/self-hosting/configuration/#the-postgresql-address)
  says how to write each one, mostly as its `%` code, such as `%23`
  for `#` and `%3F` for `?`. The release before this one reads the
  changed address too, so change the address first and update after.

  The update keeps every table and row. The core migrations keep their
  record in the same `goose_db_version` table, so none of them runs a
  second time.
- On PostgreSQL, `account:records -json` now gives each time in UTC,
  as the text output already does. When the PostgreSQL driver, pgx,
  cannot read the address in `GOPHENBERG_DATABASE_URL`, `account:list`
  and `account:records` now stop with this one line, which holds no
  part of the address and no longer repeats the driver's own text:

  ```text
  gophenberg: postgres: open the database: pgx cannot parse the address or one of its settings
  ```

  The other account commands check the address first and name the
  setting, as they did before.

  Errors from the command records now name their step after
  `postgres:`, such as `postgres: check the records:` when
  `account:records` or a `-yes -as` command cannot reach the database.
  A failed records step of `migrate` now reads
  `migrate records: postgres: apply the records schema:` followed by
  dbkit's own text. Update any script or alert that matches the old
  wording.

Updating from further back also runs the migrations of every release
you skip, so read each of their notes. Coming from five releases back
or older, write any `@` inside the password in
`GOPHENBERG_DATABASE_URL` as `%40`, or the server does not start. The
`%40` form works on those releases too, so change the address first
and update after.

Coming from two releases back or older, the update also runs the
migration the release before this one added. It adds the `gonsole`
schema, which keeps a record of each account change applied with
`-as`, so the database account needs the right to create a schema.
A run with no command lists the commands instead of serving, so a
setup that passes the image its own arguments, or runs the binary
itself, has to name `serve` first. `createadmin` is now
`account:create-admin`, and `grantrole` is now `account:grant-role`.
`seed` and the four account commands that change an account change
nothing until you add `-yes`, and those four need `-as`. Every start
deletes the files of an upload that a stop cut short, so run one
Gophenberg per media folder. A still picture over the pixel limit is
refused with `image_pixel_budget_exceeded`. A post in the trash is
read only, and moving a post to the trash deletes the unsaved work
parked on it, from every author. On a site that builds its own
plugins, commands such as `list` and `check` register every plugin
and stop it without starting it, so `Register` must open nothing, and
`Stop` must work when `Start` never ran.

Coming from three releases back or older, a browser that saves
anything from a page on another site is refused with
`request_cross_origin`, so a plugin form has to sit on this site's
own pages. Behind a reverse proxy, set `GOPHENBERG_PUBLIC_URL`, or set
`GOPHENBERG_TRUSTED_PROXIES` and let the proxy pass
`X-Forwarded-Proto`. Without one of them, a browser too old to say
where a request came from is refused on this site too.
**Move** also deletes the values a field leaves on the content its
new group does not reach, where those releases left them hidden.

If a type stopped nesting on an older release while its items still
sat inside other items, this release refuses to save those items.
Press **Let items nest** on that type. For a type a plugin declared,
move those items to the top level instead.

Coming from four releases back or older, the update also runs the
migration that three releases back added. On most sites it only adds
a database index, a rule that stops one container from holding two
fields of the same name, and changes no row. It never changes what
your items, revisions and autosaves store. The migration runs as one
step. If it fails, the server stops at start and the database stays
as it was, so the older image starts again.

The migration does more on a site where one container held two fields
of the same name. That can only happen where a release six back or
older moved a Section, a Repeater or a Flexible content field to
another group without the fields inside it. The migration keeps one
copy. It keeps the copy stored in the same group as the top-level
field above it, or else the older one. The other copy is deleted with
its label and settings. If both copies are the same kind, and two
relations point at the same type, the fields inside the removed copy
move into the copy that stays. Where that copy already holds a field
of the same name, those two are settled by the same rule. Otherwise
everything inside the removed copy is deleted too. Its values stay in
your items, and the public API still serves them, even where they do
not fit the field that stays. Saving such an item can be refused
until that value is cleared. Last, every field inside a container
moves into the group of the top-level field above it.

The server logs nothing about what the migration removes, and rolling
back does not bring it back. On such a site, press **Export
definitions** before you update to keep a record, and keep the backup
until you have checked those containers. Do not delete one of the two
copies by hand first, because on that release it clears the values
both copies read.

Rolling back to the release before this one deletes nothing. Put the
older image tag back, the newest patch of that release, and start it.
It runs on this release's database as it stands. Accounts keep
signing in, and the database password needs no change. Its newest
patch serves the same theme kits, so no theme needs rebuilding. An
older patch turns away a theme built on kit %KIT_VERSION% with
`kit_unsupported`, and the site shows the built-in pages. If
`GOPHENBERG_THEME` names that theme, the server does not start at
all. While you run it:

- It keeps the description of every type, but neither shows nor
  exports them. They show again once you update.
- Its import refuses a definitions file this release exported, with a
  message naming `description` that asks you to reload the page.
  Reloading does not help. Import a file it exported itself, or take
  every `description` out of the file first.
- Every account that signs in enters the admin again, members and
  accounts holding no role or a plugin's role included. Each of them
  can write and upload its own items, like an author. Disable the
  member accounts before you roll back if that matters. A plugin role
  carrying `manage_users` loses it, so only `admin` manages accounts,
  and `-role member` is refused.
- It has no cron address, so a host cron calling it gets 404, and
  `curl -f` reports a failure on every run. Pause the host's cron. It
  sweeps sessions on its own hourly timer, and leaves the cron jobs
  table unused.
- It ignores the settings this release added. An admin list shows 20
  items a page, image uploads are resized with no limit on how many
  at once, and dates and numbers follow each reader's language. A stop
  takes up to 23 seconds at the defaults. A plugin that uses
  `deps.DB`, `deps.Accounts`, `Needs` or `RoleProvider`, or requires
  the SDK module, does not build against it.
- Its PostgreSQL pool holds the larger of 4 and the number of CPUs.
- Its screens come back, with the home screen, the older lists with
  their counts and the **Undo** after a trash. An author sees
  **Empty Trash** again. The author column shows the name alone, with
  no avatar. The names of the built-in types show as stored again, and
  the editor's breadcrumb starts at Post on every type. New slugs,
  slugs typed in the editor and route words taken from a label keep
  their accents again, while the ones saved on this release stay
  plain.

Updating again runs no migration, and what you changed while rolled
back stays as it is. Rolling back never needs this release's
migrations run backwards. Doing it by hand forgets when each cron job
falls due, which is harmless, and deletes every type description for
good, so press **Export definitions** first.

Going back more than one release is not supported. To run an older
release, restore the backup you took just before you first updated
past it, with the media volume from the same day. Everything written
since that backup is lost.

You can instead start an older image on this release's database.
Before you do, write every sign in the database password that is not
a letter from a to z or a digit as its `%` code, such as `%23` for
`#` and `%3F` for `?`. The older image then starts, and starting
deletes nothing. Each older release also has the problems of every
newer one, the release before this one included, unless its own line
says otherwise:

- Two releases back knows three commands, `createadmin`, `grantrole`
  and `seed`. Any other word, `serve`, `check`, `migrate` and every
  `account:` command included, starts the full server and runs its
  migrations. Under `docker compose run`, that server keeps running
  until you stop it. Its `seed` stores the demo data at once, with no
  dry run, and its `grantrole` refuses `-as` and `-yes`. It keeps no
  record of the account changes it makes, and the records kept before
  stay. It reads every setting as it is written, spaces included, so
  take those spaces out first. It ignores `GOPHENBERG_PUBLIC_URL`, so
  another domain pointed at your server can again make a visitor's
  browser post to the site. A stop gives the requests and the plugins
  one window of 10 seconds. A stop in the middle of an upload leaves
  its files in the media folder. A post in the trash takes edits
  again, and two changes made at the same moment can slip past the
  checks on a page's place or a field's depth. Leaving the editor
  loses the unsaved words, and going Back to a post can show another
  post's content. Moving a group with a Linked from field needs that
  field deleted and declared again. It runs only themes built on theme
  kit 0.15.0 or older. A newer theme does not load, and the site shows
  the built-in pages instead. If `GOPHENBERG_THEME` names that theme,
  the server does not start.
- Three releases back accepts what a browser saves from a page on
  another site. Its **Move** leaves a field's values hidden on the
  content the new group does not reach, and updating again does not
  clean them up. Its imports delete the values of a field they move to
  another group, and some imports write part of the file before they
  refuse the rest. It lets you delete a relation inside a Section that
  a Linked from field reads, which leaves that field reading nothing.
  It refuses to delete a group holding both a relation and the Linked
  from field reading it. A site whose Linked from field reads a
  relation inside a Section cannot import its own export. It also lets
  a block delimiter inside an allowed attribute hide an event handler
  from the content sanitizer, which the newest patch of two releases
  back fixed.
- Four releases back cannot move a field into or out of a container,
  and fields a newer release moved stay where they are. It has no
  buttons to let a type nest or stop nesting, and it lets the API or
  an import turn nesting off while items still sit inside other items.
  After you update again, those items cannot be saved until you press
  **Let items nest**. A type a plugin declared, which newer releases
  keep nesting, turns flat when it starts and stays flat after you
  update again, so move its nested items to the top level. Deleting a
  field from a group that is turned off or **Shadowed** clears the
  values another active group serves at the same place.
- Five releases back ignores `GOPHENBERG_FIELD_DEPTH` and holds the
  limit at 32. It refuses a new field more than 32 deep, and any
  import file that holds one.
- Six releases back moves a Section, a Repeater or a Flexible
  content field to another group without the fields inside it. After
  such a move, adding a field to that container under the name of a
  field left behind fails with an internal error. Deleting the old
  group fails the same way. It also lets a shared cache keep a failed
  content API answer, and does not mark answers to a signed in account
  as private.
- Seven releases back does not know a relation inside a container. Its
  editor refuses to save a change to a container holding one, and a
  change sent through the API that leaves the relation out deletes its
  value. It keeps what an item points at in a separate list, while
  newer releases read the item itself, so a relation you change there
  is lost when you update again.
- Eight releases back does not know the newer field kinds. It shows
  their values but refuses a change to one. It runs only themes built
  on theme kit 0.12.0 or older. A newer theme does not load, and the
  site shows the built-in pages instead. If `GOPHENBERG_THEME` names
  that theme, the server does not start.
- Nine releases back knows only the six original kinds and does not
  know containers at all. It cannot create a field group. Deleting a
  field there also deletes every field of that name inside the group's
  containers, and deleting a group also deletes its fields that stand
  inside other groups' containers. It runs only themes built on theme
  kit 0.9.0.

Running the migrations backwards by hand, as nine releases back would
need, deletes definitions, and that is a one way trip. It deletes
every field whose kind is not one of the six, every field standing
inside another field, every field's settings, and the record of which
plugin declared what. It leaves every Gallery as a single Media field
holding a list of files it cannot read. The values stay in the
database, but an item holding a value for a deleted field cannot be
saved, not even with a new title, until a field of that name exists
again. The definitions do not come back if you migrate forward again,
and neither do the copies the container migration removed.

This release serves themes built on `@gophenberg/astro`
%KIT_VERSION%, and the older kits it still answers. Ask a site which
ones through [the handshake](/reference/content-api/), and rebuild
your theme against one of them before updating.
[Theme compatibility](/themes/compatibility/) explains how long a
built theme keeps working.

## What to back up

**The database**, which holds everything you wrote, and the record of
each account change applied with `-as`:

```sh
docker compose exec -T db pg_dump -U postgres -Fc gophenberg > gophenberg.dump
```

The `-T` is required: without it Docker attaches a terminal that
corrupts the binary dump, and you find out when the restore fails.

**Your compose file and any `.env` beside it**, the only place
your configuration and passwords exist.

**The media volume**, which holds every file you uploaded. The
database records what each file is called and where it lives, but
never the file itself, so a lost volume leaves a library of broken
pictures no dump can repair:

```sh
docker compose cp gophenberg:/media ./media-backup
```

Back it up whenever you back up the database. The two have to come
back together, or the library and the files disagree.

**The themes volume**, if you upload themes in the admin. An
uploaded theme exists only there, so a lost volume means
re-uploading every theme:

```sh
docker compose cp gophenberg:/themes ./themes-backup
```

Which theme is active is stored in the database, not in the
volume, so both have to come back for the site to look the same.
Themes you install by hand need no backup, they are rebuilt from
their source projects.

## Restoring

Restore into a database Gophenberg has never touched, because the
tables it creates make the restore fail. Starting the server creates
them, and so do `migrate`, `seed -yes` and `account:create-admin`, so
run none of them before the restore:

```sh
docker compose up -d db
docker compose exec -T db pg_restore -U postgres -d gophenberg < gophenberg.dump
docker compose up -d
```

Starting only the database first is what keeps Gophenberg out of
the way until the restore is done.
