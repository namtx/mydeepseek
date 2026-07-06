# Context: mydeepseek

Glossary of terms as used in this project. Keep implementation details out.

## Terms

**Facade (Fake Ollama Server)**
The HTTP server this project builds. It speaks the Ollama REST API on the wire
so that an Ollama *Client* can use it, while every request is actually served
by the *Upstream*. It holds no models and runs no inference.

**Client**
Any program that talks to the Facade believing it is Ollama. The supported
client is Raycast's built-in Local Models integration; anything else working
is incidental.

**Upstream**
The DeepSeek cloud API (OpenAI-compatible chat completions). The only source
of actual model output.

**Translation**
The two-way mapping the Facade performs per request: Ollama chat request →
Upstream request, and Upstream response (or stream) → Ollama response (or
NDJSON stream). Translation is lossy by design: parameters the Upstream does
not support are silently ignored.

**Capability**
A per-model tag (`completion`, `thinking`, `tools`, `vision`) the Facade
advertises so the Client knows what a model can do. Advertised capabilities:
`completion` for all models; `thinking` for deepseek-reasoner only. `tools`
and `vision` are deliberately not advertised.

**Thinking**
The reasoning text deepseek-reasoner produces before its answer
(`reasoning_content` upstream). Exposed to the Client as Ollama's native
`message.thinking` field, never inlined into the answer content.

**Model listing**
The fixed, hardcoded set of models the Facade advertises: `deepseek-chat` and
`deepseek-reasoner`. Model names pass through to the Upstream unchanged.
