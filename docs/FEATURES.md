# my-branch Features

Everything added on top of upstream stash. Features are surfaced in **Settings > Tasks** (AI sections), the **AI Chat**, the **scene/player pages**, the **Stats page**, and the **Front Page**.

## AI Chat & Search

- **AI Chat** (`/aiChat`): persistent sessions, image input, system prompt config, memories, and tools (create/merge tags, describe images/scenes, search, recommend).
- **Ask AI** on scene, image, and performer pages: opens the chat prefilled with context about the entity.
- **Library context (RAG)**: the chat is automatically augmented with the scenes whose embeddings are most similar to your message (title, details, transcript/summary). Toggle in the chat panel; `use_library_context` input.
- **`recommend_scene` tool**: tell the chat a vibe and it returns scene links, boosting scenes with detected moans 🔥.
- **Semantic search** (`Settings > Tasks`): text- or image-based embedding search over scenes/performers/images/galleries/studios/tags.
- **Similar items panel** on scene/image/performer pages: fading horizontal preview of the 2 closest items; click for a popup with batch actions (AI tag all similar, detect loops, add to group, merge performers).
- **Transcript search**: search scene audio transcripts (Settings > Tasks) with snippets.

## Tagging & Analysis

- **AI scene/image tagging**: performers, tags, title, details; modes: performers-only, fill-missing-only, create-missing performers/tags; batch from list selection.
- **AI performer tag**: career + details on the performer page.
- **Performer career analysis** (`AI Performer Career` job): active years, niches, studios, summary.
- **Scene segmentation**: markers with descriptions, tags, per-segment **performers** (per-actor timeline), and **intensity 1-10**.
- **Climax map**: intensity timeline in the markers panel — click a peak to seek.
- **Skip to the good part**: seeks the player to the highest-intensity marker; **Create highlight clip** cuts an mp4 around it (`<config>/clips/`).
- **AI mood tagging**: every scene classified into 2-4 moods (romantic, rough, goth, cosplay, amateur, milf, bdsm, anal, threesome, taboo, hardcore, sensual, humor, solo, lesbian, gangbang, cuckold, dirty talk) shown as card badges. Mood filter + sort in the scene sidebar; **Mood Groups** creates a group per mood.
- **Audio analysis**: whisper transcription with **timestamps**, summary, silence/music/speech/moans/ambient flags. **Moan leaderboard** and performer audio stats on the Stats page.
- **Steam score** 🔥/10 per scene (moans + silence + explicit tags): card badges, sidebar filter, and sort option.
- **Loop detection**: batch or per-scene "Detect Loop" (skip-tagged, fade-to-black proofed).
- **Smart collections**: AI groups/galleries built from tags, performers, studios, and embedding clusters.
- **AI file rename suggestions** with review/apply.
- **Media quality assessment** and **duplicate detection** (semantic) with merge actions.

## Review Flows (Settings > Tasks)

- **Performer merge suggestions**: embedding-similar pairs, review with Merge/Reject (Apply all / Reject all).
- **AI audit**: re-analyzes tagged scenes/images and lists discrepancies (empty title/details, missing performers) with apply/dismiss.
- **AI translation**: batch-translate titles/details into a configured language, review before applying.
- **Performer discovery**: finds recurring unknown performers in untagged scenes/images; review creates "Unknown Performer N" or merges into existing.
- All review dialogs have **Apply all / Reject all**.

## The Heaven Update

- **Performer session**: a Her session button builds a 30-minute afterglow queue of a performer's scenes.
- **Auto-advance on O**: logging an O during afterglow immediately advances to the next scene.
- **Who is she?**: a live chip shows the performer of the current segment; tap to open her page.
- **Transcend mode**: 60-minute build-up session of the steamiest scenes with afterglow, random edging, and a blind every third scene.
- **The Throne**: XP ranks (Novice Gooner to Heavenly) with progress on the scoreboard and hub.
- **Climax projection**: a live intensity bar tracking the playback position during afterglow.
- **Heavenly playlists**: saved session plans replayed from the builder or scoreboard.
- **The Pantheon**: longest session, total session time, and session count on the scoreboard.
- **Per-scene height**: a sparkle score (steam + O + moods) on scene cards.
- **The Oracle**: describe a session in words; the chat tool or the builder's Oracle box plans it for you.
- **Night Chapel**: a warm dim candlelight theme that deepens goon mode.

## The Ascension Update

- **Edging mode**: pause playback after a configurable interval (1-10 min, default 3) with a Continue overlay.
- **Blind goon**: play a random high-steam scene with the video blurred until it ends.
- **Vibe radio**: endless shuffled afterglow queue for a chosen mood.
- **Back for more**: the last ten played scenes on the front page.
- **Saved moments**: heart any marker; a /saved page collects them and can build a reel from them.
- **Session ordering**: build-up (peak at the end) or peak-first sessions.
- **Ritual builder**: a mood sequence that the session cycles through in order.
- **Watch-along chat**: chat opened from the player knows what you are watching and narrates along.
- **Goon Scoreboard** (/scoreboard): moaners, O board, streak, saved moments, finish-history timeline.
- **Achievements**: streaks, performer/play/saved milestones with progress bars on the front page.

## The Gooner Update

- **Goon mode** (`G` key): filters everything to steam ≥ 6 with a warm tint (settings toggle for the default).
- **Fap Roulette**: "Surprise me" button above the player — random scene from your queue or the library.
- **Afterglow mode**: reel playback — jumps each scene to its best moment and auto-advances after 45s (works without markers, just no seek). **Session recap** modal on exit (scenes, best moments, O count).
- **Session Builder** (Settings > Tasks): duration + performers + moods + steam floor + vibe → curated queue, start as an afterglow session, or **Create goon reel** (one mp4 of the best moments).
- **Daily Goon** widget on the front page: day-seeded steamy pick + streak counter.
- **For You hub**: Tonight's picks, Moaner of the week 👑, Fresh meat.
- **O board** on the Stats page: performers ranked by your logged O-scenes.
- **Quick-hide** (`H` key): blurs everything instantly for shared screens.

## Embeddings

- Text + **visual** embeddings for scenes, performers, images, galleries, studios, tags.
- **Stale-only refresh** (re-embed only what changed), `has_embedding` filter, embedding badges, and per-type counts in Settings > AI.
- Embedding model diagnostics in job logs.

## Other

- **Scheduled AI maintenance**: automatic stale-embedding refresh and audio analysis intervals (Settings > AI).
- **Job queue polish**: AI badge + AI-tasks-only filter.
- **Chat "Pick me a scene"** and library-aware answers.
