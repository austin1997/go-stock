# MCP OAuth in Web / Docker deployments

Set `WEB_PUBLIC_ORIGIN` in the application's environment to the **browser-facing origin**, for example `https://stocks.example.com`. The callback is always:

```
https://stocks.example.com/api/mcp/oauth/callback
```

For local Docker development with the existing published port, use `WEB_PUBLIC_ORIGIN=http://localhost:8080` and open the application at that same origin. No additional callback ports need publishing. For remote access, use HTTPS (typically a reverse proxy forwarding to the existing container port 8080); HTTP is accepted only for literal loopback IP addresses or `localhost`. The value must contain no credentials, query, fragment, or path prefix. A trailing `/` is allowed. Missing/invalid configuration fails OAuth startup explicitly; it does not fall back to a container loopback listener.

The origin is configuration, not request input. `Host`, `Forwarded`, and `X-Forwarded-*` never determine registration or redirect URIs. Configure your proxy to preserve the `/api/mcp/oauth/callback` route and avoid logging callback query strings. Register/allow this exact callback at providers that require allowlisting; providers permitting only native loopback redirects cannot support this Web flow without provider-side changes.

The callback requires an active go-stock session for the same user who started the flow. Open the application at the configured origin before starting authorization so its session cookie is available on the returning top-level GET redirect. Logging out, account disabling, using another browser profile, or returning to another hostname prevents completion; log in and restart authorization. `form_post` callbacks are not supported.

Pending flows are process-local, bound to user ID + MCP server ID, a cryptographically random state, and a unique S256 PKCE verifier. State is claimed atomically before exchanging the code. A flow expires after three minutes; denial, successful completion, and failed exchanges cannot be replayed. Starting again replaces the user's pending flow for that server; an older exchange cannot overwrite the newer flow. Tokens are encrypted in the initiating tenant's database and never returned in the callback page. A restart loses pending flows; multiple replicas require sticky routing for the start request and callback (shared/distributed flow storage is not implemented).

Desktop mode retains the existing `http://localhost:18963`–`18968` listener and does not require `WEB_PUBLIC_ORIGIN`.

Regression tests use only local HTTP fixtures and temporary SQLite databases:

- `go test -buildvcs=false ./backend/data -run '^TestMCP(Web|Desktop)OAuth' -count=1`
- Root Web integration: select production Go files with `go list -buildvcs=false -tags goweb -f '{{join .GoFiles " "}}' .`, then `go test -buildvcs=false -tags goweb $files web_mcp_oauth_test.go -run '^TestWebMCPOAuth' -count=1`.

The selected-file root command avoids unrelated pre-existing `app_test.go` compile failures; it exercises the real Web mux, authentication sessions, discovery, registration, PKCE token exchange, and two isolated tenants with the same MCP server ID. External provider compatibility and real reverse-proxy/TLS/browser deployment remain deployment checks.
