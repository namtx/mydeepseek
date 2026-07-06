# 0003 — Listen on 127.0.0.1:11435, not the Ollama default 11434

## Status
Accepted (2026-07-06)

## Context
Raycast's built-in Local Models feature (the compatibility target per ADR
0001) does not treat every Ollama-shaped server the same way. Disassembling
`Raycast.app/Contents/MacOS/Raycast` (`strings` + Swift symbol names) shows
its host field has an `isDefault` check against the literal string
`127.0.0.1:11434`. When the field is left at that default, Raycast does not
query the port at all — it resolves the real Ollama.app by macOS bundle
identifier and spawns/manages its own `ollama serve` child process
("Raycast hosted Ollama server", `ollamaProcessTask`). If that app isn't
installed, Raycast reports "Ollama app is not installed", regardless of
anything already listening on 11434.

Only when the host field holds a *non-default* value does Raycast switch to
its "custom Ollama server" code path and make plain HTTP requests to it —
the only path this facade can serve, since it is not the real Ollama binary
and can't be resolved by bundle identifier.

## Decision
The facade's default listen address is `127.0.0.1:11435`. The user must
enter this exact host (or whatever `OLLAMA_HOST` is set to) into Raycast's
Local Models → Ollama Host field — never the literal default — so Raycast
takes the custom-server path.

## Consequences
- Works with Raycast without installing the real Ollama app.
- Any client that hardcodes port 11434 as a real-Ollama check would hit the
  same problem; if that surfaces for another client, the fix is the same
  (non-default host), not code changes here.
- `OLLAMA_HOST` remains user-overridable, so this is a default, not a
  hardcoded constraint.
