# Decision Models (Ollama System One)

Demonstrates `client.decide()`: one request to a decision model that
classifies the state, estimates a yes/no probability, and scores it against
an ordered rubric — with per-question probabilities and confidence. Decision
models answer; they never generate text.

## Prerequisites

- Ollama v0.35.0+ running locally (default `http://localhost:11434`)
- A decision model pulled: `ollama pull clef-flash` (also: `clef`, `nimble`, `tev1`)

## Run

```sh
../../bin/scriptling example.py
# or explicitly:
../../bin/scriptling example.py http://localhost:11434 clef-flash
```

## What it shows

- `choice` — pick from 2–26 named options, with each option's probability
- `noul` — a yes/no probability (0–1)
- `score` — a probability-weighted position on an ordered rubric
- `usage` — input/output token counts for the request

Only Ollama clients support decision models; other providers return an error
naming the provider.

## See Also

- [AI Client reference — client.decide()](https://scriptling.ai/reference/libraries/ai/client/)
