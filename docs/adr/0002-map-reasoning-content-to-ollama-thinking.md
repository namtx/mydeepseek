# 0002 — Map DeepSeek reasoning_content to Ollama's message.thinking

## Status
Accepted (2026-07-06)

## Context
`deepseek-reasoner` streams `delta.reasoning_content` before any
`delta.content`. The Python original only read `delta.content`, so reasoner
streams logged errors and the client saw nothing for the whole thinking
phase. Three ways to fix it: drop reasoning, inline it as `<think>` tags in
the answer text (old R1-GGUF convention), or map it to Ollama's native
`message.thinking` field introduced for thinking models.

## Decision
Translate `reasoning_content` → `message.thinking` and advertise the
`thinking` capability for deepseek-reasoner in `/api/show`. Reasoning text
never appears in `message.content`.

## Consequences
- Raycast renders reasoning as a proper collapsible section, and the stream
  is visibly alive during the thinking phase.
- Clients that predate `message.thinking` simply ignore the field and see
  only the final answer — acceptable, since Raycast is the target (ADR 0001).
- The facade must handle chunks where `content` is absent and only
  `reasoning_content` is present without erroring (the original's bug).
