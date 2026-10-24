# SvelteKit integration

## Server-side sync
Run stonefish (or a cdb server) and point SvelteKit at it:
```ts
// src/lib/db.ts
export const DB_URL = import.meta.env.VITE_DB_URL ?? 'http://localhost:5984';
```
Use `adapter.NewHTTPAdapter` on the Go side or fetch directly from SvelteKit server routes. When behind Traefik/Dokploy, trust proxy headers:
```
PROTOCOL_HEADER=x-forwarded-proto
HOST_HEADER=x-forwarded-host
```
Do not hardcode `ORIGIN=http://localhost:<port>` in production.

## Browser/offline (WASM)
Build the Rust core:
```sh
cd rust && cargo build --release --target wasm32-unknown-unknown
```
Load the `.wasm` in a SvelteKit client module for local collation/merge and offline storage. Add `wasm-bindgen` to expose functions to JS when needed.

## Recommended split
- Server: Go + SQLite (single binary, PocketBase style), cdb-compatible endpoints.
- Browser: Rust/WASM core for storage/replication helpers; JS shim for UI.

## Deployment (Dokploy)
- Use `expose: ["<container-port>"]`, not `ports:`.
- Declare env vars under `environment:` (no `env_file: .env`).
- Map domain to service + container port.
- Add container `HEALTHCHECK` + `/health` route.