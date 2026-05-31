# hexpm-envoy-proxy

Tiny reverse proxy that fixes Erlang/OTP's `httpc` incompatibility with Envoy-based TLS inspection proxies.

## The problem

Erlang's `httpc` client sends an empty `te:` header on every request. Envoy-based egress proxies (commonly used in container environments with TLS inspection) reject this with HTTP 503:

```
upstream connect error or disconnect/reset before headers. reset reason: connection termination
```

This breaks `mix deps.get`, `mix local.hex`, and any Elixir/Erlang tool that uses `httpc` to reach hex.pm.

## How it works

This proxy listens on `127.0.0.1:8787` and forwards requests to hex.pm, stripping the problematic hop-by-hop headers (`te`, `host`, `connection`). Go's HTTP client makes clean upstream requests that pass through the egress proxy without issues.

- `/` → `https://repo.hex.pm` (package registry)
- `/builds/` → `https://builds.hex.pm` (hex installer downloads)

## Install

```bash
go install github.com/dvcrn/hexpm-envoy-proxy@latest
```

Or with mise:

```bash
mise use -g go:github.com/dvcrn/hexpm-envoy-proxy@latest
```

## Usage

Start the proxy, then set the Hex environment variables:

```bash
hexpm-envoy-proxy &
export HEX_MIRROR=http://127.0.0.1:8787
export HEX_BUILDS_URL=http://127.0.0.1:8787/builds
```

Then `mix local.hex`, `mix deps.get`, etc. work normally.

### Options

```
-addr string    listen address (default "127.0.0.1:8787")
-version        print version and exit
```

## Claude Code on the Web

Add the following to your **startup script** (SessionStart hook):

```bash
mise use -g go:github.com/dvcrn/hexpm-envoy-proxy@latest
if ! pgrep -f hexpm-envoy-proxy > /dev/null 2>&1; then
  nohup mise x -- hexpm-envoy-proxy > /tmp/hexpm-envoy-proxy.log 2>&1 &
  disown
  sleep 2
fi
# Wait for proxy to be ready (up to 5s)
for i in $(seq 1 10); do
  if curl -s -o /dev/null -w '' http://127.0.0.1:8787/ 2>/dev/null; then
    echo "hexpm-envoy-proxy is ready"
    break
  fi
  sleep 0.5
done
export HEX_MIRROR=http://127.0.0.1:8787
export HEX_BUILDS_URL=http://127.0.0.1:8787/builds

echo "export HEX_MIRROR=$HEX_MIRROR" >> ~/.bashrc
echo "export HEX_BUILDS_URL=$HEX_BUILDS_URL" >> ~/.bashrc
```

In addition to the startup script, set these **environment variables** in your environment configuration:

| Variable | Value |
|---|---|
| `HEX_MIRROR` | `http://127.0.0.1:8787` |
| `HEX_BUILDS_URL` | `http://127.0.0.1:8787/builds` |

Setting them as environment variables ensures they are available to all processes, not just those spawned by the startup script.

## Codex

For OpenAI Codex, add the same startup script above to **both** the **setup script** and the **maintenance script**. The setup script runs once when the environment is created, and the maintenance script runs on subsequent task executions — both need the proxy running.

Additionally, set these **environment variables** separately in the Codex environment configuration:

| Variable | Value |
|---|---|
| `HEX_MIRROR` | `http://127.0.0.1:8787` |
| `HEX_BUILDS_URL` | `http://127.0.0.1:8787/builds` |

## Generic CI / container startup

```bash
if ! pgrep -f hexpm-envoy-proxy > /dev/null 2>&1; then
  hexpm-envoy-proxy > /tmp/hexpm-envoy-proxy.log 2>&1 &
  sleep 1
fi
export HEX_MIRROR=http://127.0.0.1:8787
export HEX_BUILDS_URL=http://127.0.0.1:8787/builds
```
