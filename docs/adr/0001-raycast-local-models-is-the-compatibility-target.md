# 0001 — Raycast built-in Local Models is the compatibility target

## Status
Accepted (2026-07-06)

## Context
The project ports RobToMars/DeepSeek's `fake_ollama_server.py` (built for
PyCharm's Ollama integration) to Go. Every Ollama client probes a different
subset of the Ollama API: JetBrains needs only `/api/tags` + `/api/chat`;
the community Raycast "Ollama AI" extension expects `/api/generate`,
`/api/ps`, embeddings, and model-management endpoints; full Ollama emulation
is open-ended. Without a named target, endpoint scope creeps indefinitely.

## Decision
The facade targets exactly one client: Raycast's built-in Local Models
integration (Raycast ≥ 1.99.0). The endpoint surface is what that client
uses and nothing more: `GET /`, `GET /api/version`, `GET /api/tags`,
`POST /api/show`, `POST /api/chat` (streaming and non-streaming).

Tool calling is deliberately not supported: `/api/show` advertises
`capabilities: ["completion"]` (plus `"thinking"` for deepseek-reasoner),
so Raycast will not attempt AI Extensions with these models. Supporting
tools would require translating tool definitions and streamed tool-call
deltas between Ollama and OpenAI formats — roughly doubling the
translation code — and deepseek-reasoner's tool support upstream is
unreliable.

## Consequences
- Raycast chat, Quick AI, and commands work; Raycast AI Extensions do not.
- Other chat-only Ollama clients will likely work by accident, but breakage
  in them is not a bug.
- Adding `/api/generate` or tools later is additive, not a rework: the
  translation layer is the extension point.
