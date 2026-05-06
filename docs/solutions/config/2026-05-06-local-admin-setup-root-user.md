---
title: Local admin setup when root login is not initialized
date: 2026-05-06
category: config
tags: [admin, setup, sqlite, local-development, authentication]
severity: medium
time_to_resolve: 20 minutes
---

## Problem

Local development showed the setup/login screen rejecting the commonly assumed
default password with:

```text
Password must be at least 8 characters long
```

The expected default `root / 123456` did not work, and the current SQLite DB had
only a `root` user with `role = 1`, not an administrator. `/api/setup` reported
that setup was still incomplete because no role-100 root user existed.

## Root Cause

There are two different first-run paths:

- `model.createRootAccountIfNeed()` can auto-create a fallback user named
  `root` with password `123456` when the database has no users.
- The setup wizard and `/api/setup` create the real root administrator with
  `role = 100`, but they require a password of at least 8 characters.

Because the local database already contained a normal `root` account
(`role = 1`), there was no usable admin credential to discover. `RootUserExists()`
checks for `role = 100`, so setup was still allowed, but the username `root` was
already taken.

## Solution

Use `/api/setup` to create a real administrator with a new username and an
8-character-or-longer password:

```bash
curl -s -X POST http://localhost:3000/api/setup \
  -H 'Content-Type: application/json' \
  -d '{
    "username": "admin",
    "password": "admin123456",
    "confirmPassword": "admin123456",
    "SelfUseModeEnabled": false,
    "DemoSiteEnabled": false
  }' | jq .
```

Then verify the DB has a role-100 account:

```bash
sqlite3 /Users/randomradio/src/aikanhub/data/one-api.db \
  "select id, username, role, status from users order by id;"
```

Expected result: the new `admin` user has `role = 100` and can log in at
`http://localhost:3000`.

## Prevention

When diagnosing local admin login problems, check setup state before guessing
credentials:

```bash
curl -s http://localhost:3000/api/setup | jq .
sqlite3 /Users/randomradio/src/aikanhub/data/one-api.db \
  "select id, username, role, status from users order by id;"
```

Treat any `role = 1` account as a normal user, even if its username is `root`.
Only `role = 100` is the root administrator.

## Key Insight

The default `root / 123456` log line is a backend fallback, not a reliable
modern local setup credential. If the setup wizard says root is not initialized,
create a role-100 admin through `/api/setup`; if a normal `root` username already
exists, choose a different admin username.
