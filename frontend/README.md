# frontend

A React + TypeScript + Vite single-page app for the URL shortener. It shortens
URLs (with optional alias and expiry), lists created links (persisted in
`localStorage`), checks their live status, and tests redirects from the browser.

## Development

```bash
npm install
VITE_API_BASE=http://localhost:8080 npm run dev
```

The app is served on port 3000 and talks to the gateway at `VITE_API_BASE`
(default `http://localhost:8080`).

## Build

```bash
npm run build   # type-checks then builds into dist/
```

## Container

The Docker image builds the static site and serves it with nginx. `VITE_API_BASE`
is a build argument baked in at build time (set by Docker Compose).
