# CLAUDE.md

Behavioral guidelines to reduce common LLM coding mistakes, merged with E-maki-specific instructions.

**Tradeoff:** These guidelines bias toward caution over speed. For trivial tasks, use judgment.

## 1. Think Before Coding

**Don't assume. Don't hide confusion. Surface tradeoffs.**

Before implementing:

- State your assumptions explicitly. If uncertain, ask.
- If multiple interpretations exist, present them - don't pick silently.
- If a simpler approach exists, say so. Push back when warranted.
- If something is unclear, stop. Name what's confusing. Ask.

## 2. Simplicity First

**Minimum code that solves the problem. Nothing speculative.**

- No features beyond what was asked.
- No abstractions for single-use code.
- No "flexibility" or "configurability" that wasn't requested.
- No error handling for impossible scenarios.
- If you write 200 lines and it could be 50, rewrite it.

Ask yourself: "Would a senior engineer say this is overcomplicated?" If yes, simplify.

## 3. Surgical Changes

**Touch only what you must. Clean up only your own mess.**

When editing existing code:

- Don't "improve" adjacent code, comments, or formatting.
- Don't refactor things that aren't broken.
- Match existing style, even if you'd do it differently.
- If you notice unrelated dead code, mention it - don't delete it.

When your changes create orphans:

- Remove imports/variables/functions that YOUR changes made unused.
- Don't remove pre-existing dead code unless asked.

The test: Every changed line should trace directly to the user's request.

## 4. Goal-Driven Execution

**Define success criteria. Loop until verified.**

Transform tasks into verifiable goals:

- "Add validation" → "Write tests for invalid inputs, then make them pass"
- "Fix the bug" → "Write a test that reproduces it, then make it pass"
- "Refactor X" → "Ensure tests pass before and after"

For multi-step tasks, state a brief plan:

```text
1. [Step] → verify: [check]
2. [Step] → verify: [check]
3. [Step] → verify: [check]
```

Strong success criteria let you loop independently. Weak criteria ("make it work") require constant clarification.

---

## 5. E-maki project context

E-maki listens to a room, works out what people talk about, and paints those topics as a living collage on a 13.3" Spectra 6 colour e-ink frame. How it works, privacy and modes: see `README.md`.

**Privacy rules (non-negotiable):**

- Audio is never written to disk. It lives only in RAM buffers.
- Transcript text is never written to disk, logged, or sent off the local network. It exists only in an in-memory rolling window. The one exception is the external topic model, which the user must pick on the settings page (never a default or automatic choice) and which the page warns about while it is on.
- The only data that may leave the house is the final English topic / image prompt sent to the image API.
- Tests and logs must not contain real transcripts. Use synthetic fixtures.
- Any change that touches what is stored, logged or sent over the network needs an explicit note in the PR.

**Architecture:**

- Go service (single binary) does capture, pipeline, change gate, topic memory, collage, dithering and the web/panel API.
- Model work runs in sidecars reached over HTTP: speech-to-text via whisper.cpp `whisper-server` (or an OpenAI-compatible faster-whisper server), topic LLM via Ollama / llama-server (OpenAI-compatible chat API). Hardware is chosen later, so never hard-code a device; differences belong in config profiles.
- Languages: Danish, Norwegian and English, often mixed. Danish needs a fine-tune (Edda), Norwegian uses NB-Whisper. Topic output is always English.
- The panel is driven by Fugleramme's existing clients via its protocol: `GET /frame.png` (6-colour paletted PNG, ETag/304), `GET /frame.e6`, `POST /press`. Ported Fugleramme code (MIT) must keep its copyright notice in `third_party/` / file headers.
- Two modes: diary (default, slow/overnight) and party (live, guests over).

**Conventions:**

- Go 1.27+, standard library first; add a dependency only when it clearly saves work.
- `go vet ./...` and `go test ./...` must pass before pushing.
- Model and API calls are behind small interfaces with fakes, so tests run offline without sidecars.

**These guidelines are working if:** fewer unnecessary changes in diffs, fewer rewrites due to overcomplication, and clarifying questions come before implementation rather than after mistakes.
