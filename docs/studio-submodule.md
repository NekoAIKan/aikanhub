# Studio Submodule

`web/studio` is a git submodule for `git@github.com:NekoAIKan/studio.git`.
It tracks the `dev` branch and is pinned by this repository to a specific
commit so builds remain reproducible.

Use a submodule here instead of copying the studio source into this repository:
studio has its own package manager, build tool, tests, and release cadence.
Keeping it as a submodule makes the ownership boundary explicit while still
allowing this app to pin and review the exact studio revision it expects.

## Clone Or Refresh

After cloning this repository:

```bash
git submodule update --init --recursive
```

To move `web/studio` to the latest `dev` commit and record that pointer in this
repository:

```bash
git submodule update --remote --checkout web/studio
git status --short
git add .gitmodules web/studio
git commit -m "chore: update studio submodule"
```

## Deployment Shape

Studio is deployed as a separate frontend app. The main KittyVibe app only links
to `/studio/`; it does not own any `/studio` React routes.

For a same-host Caddy deployment:

1. Build the main app normally.
2. Build Studio with a `/studio/` asset base.
3. Route `/studio/*` to the Studio build or Studio frontend process.
4. Route `/v1/*` and the existing KittyVibe dashboard API paths to the
   KittyVibe backend.
5. Route Studio-owned app APIs to a Studio backend, mock API, or future
   compatibility adapter. Do not assume Studio's `/api/auth/me` style routes are
   the same as KittyVibe's dashboard API.

Production static build example:

```bash
cd web/studio
npm install
VITE_USE_MOCKS=false VITE_API_BASE_URL=/studio-api npm run build -- --base=/studio/
```

For a UI-only deployment before the Studio backend is bound, use
`VITE_USE_MOCKS=true` instead.

Caddy static file example:

```caddyfile
example.com {
  handle_path /studio/* {
    root * /srv/aikanhub/web/studio/apps/web/dist
    try_files {path} /index.html
    file_server
  }

  handle_path /studio-api/* {
    reverse_proxy 127.0.0.1:18000
  }

  handle /api/* {
    reverse_proxy 127.0.0.1:3000
  }

  handle /v1/* {
    reverse_proxy 127.0.0.1:3000
  }

  handle {
    reverse_proxy 127.0.0.1:5173
  }
}
```

`handle_path` is intentional: the browser requests `/studio/assets/...`, while
the Studio build writes files under `dist/assets/...`.

The `/studio-api/*` block is for Studio-owned endpoints such as
`/api/auth/me`, `/api/account/api-keys`, `/static`, and `/upload` when those are
served by the Studio backend or mock API. KittyVibe's platform generation API
stays on the main backend under `/v1/*`.

If the Studio backend returns absolute asset URLs like `/static/...` or
`/upload/...`, either configure it to return `/studio-api/static/...` and
`/studio-api/upload/...`, or add explicit Caddy routes for those prefixes. Avoid
stealing the main app's static paths unless that is the intended deployment.

If Studio is deployed on its own subdomain instead, build without the `/studio/`
base and point the top navigation URL at that host through `HeaderNavModules`:

```json
{
  "studio": {
    "enabled": true,
    "requireAuth": true,
    "href": "https://studio.example.com/"
  }
}
```

## Run Against This App

This app's local backend listens on port `3000` by default. Studio's Vite dev
server runs on port `5179`; point its proxy at the local app backend:

```bash
cd web/studio
npm install
VITE_USE_MOCKS=false VITE_API_BASE_URL= VITE_DEV_API_PROXY_TARGET=http://127.0.0.1:3000 npm run dev
```

Then open:

```text
http://127.0.0.1:5179/
```

For local path-prefix testing through Caddy, start Studio with the same base:

```bash
cd web/studio
VITE_USE_MOCKS=false npm run dev -- --base=/studio/
```

Before updating the pinned submodule commit, run the studio checks that match
the integration risk:

```bash
cd web/studio
npm run typecheck
npm run test
```

`npm run check:endpoints` additionally needs the studio Docker package fixture
at `web/studio/docker/app/assets`; skip it when that fixture is not checked out.
