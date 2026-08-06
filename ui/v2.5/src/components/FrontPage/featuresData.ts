export interface IFeatureEntry {
  id: string;
  category: string;
  name: string;
  emoji: string;
  description: string;
  location: string;
  howTo: string;
}

export const FEATURE_CATEGORIES = [
  "AI Chat & Search",
  "Tagging & Analysis",
  "Review Flows",
  "The Gooner Update",
  "The Ascension Update",
  "The Heaven Update",
  "Embeddings & Jobs",
] as const;

export const FEATURES: IFeatureEntry[] = [
  // --- AI Chat & Search ---
  {
    id: "chat",
    category: "AI Chat & Search",
    name: "AI Chat",
    emoji: "🤖",
    description:
      "A full chat with the configured AI model: sessions, image input, memories, and tools that can edit tags, describe content, search, and recommend.",
    location: "Sidebar → AI Chat (or /aiChat)",
    howTo:
      "Open AI Chat, type any question or request, and press send. Attach an image with the camera button.",
  },
  {
    id: "ask-ai",
    category: "AI Chat & Search",
    name: "Ask AI",
    emoji: "💬",
    description:
      "Opens the chat prefilled with context about the scene, image, or performer you are looking at.",
    location: "Scene / image / performer pages",
    howTo:
      "Use the Ask AI menu item (scene/image) or the Ask AI button on the performer page.",
  },
  {
    id: "library-context",
    category: "AI Chat & Search",
    name: "Library context (RAG)",
    emoji: "📚",
    description:
      "The chat is automatically augmented with the scenes whose embeddings are most similar to your message (title, details, transcript).",
    location: "AI Chat panel",
    howTo:
      "Leave the Use library context checkbox on, then ask questions about your collection, e.g. 'what happens in the scene with the red room?'",
  },
  {
    id: "recommend-scene",
    category: "AI Chat & Search",
    name: "recommend_scene tool",
    emoji: "🔥",
    description:
      "Tell the chat a vibe and it returns scene links, boosting scenes with detected moans.",
    location: "AI Chat",
    howTo:
      "Ask something like 'pick me a scene with brunettes and bondage' and the chat uses the tool automatically.",
  },
  {
    id: "plan-session",
    category: "AI Chat & Search",
    name: "plan_session tool",
    emoji: "🗓️",
    description:
      "The chat plans a session from natural language: duration, moods, ordering, and steam floor, then lists matching scenes.",
    location: "AI Chat",
    howTo:
      "Ask e.g. 'plan a 40 minute session, sensual then rough, ending intense'.",
  },
  {
    id: "semantic-search",
    category: "AI Chat & Search",
    name: "Semantic search",
    emoji: "🔎",
    description:
      "Text- or image-based embedding search across scenes, performers, images, galleries, studios, and tags.",
    location: "Settings → Tasks (AI section)",
    howTo:
      "Press Semantic Search…, type a phrase (or drop an image), choose entity types, and search.",
  },
  {
    id: "similar-items",
    category: "AI Chat & Search",
    name: "Similar items",
    emoji: "🧲",
    description:
      "A fading preview shows the two closest items; the popup shows all similar items with batch actions.",
    location: "Scene / image / performer detail pages",
    howTo:
      "Click the Similar items strip. In the popup you can AI tag all similar, detect loops, add to a group, or merge performers.",
  },
  {
    id: "transcript-search",
    category: "AI Chat & Search",
    name: "Transcript search",
    emoji: "🎙️",
    description:
      "Searches the transcribed audio of your scenes and shows matching scenes with snippets.",
    location: "Settings → Tasks (AI section)",
    howTo:
      "Run AI Audio Analysis first, then Transcript Search… and type a phrase.",
  },

  // --- Tagging & Analysis ---
  {
    id: "ai-tag",
    category: "Tagging & Analysis",
    name: "AI scene / image tagging",
    emoji: "🏷️",
    description:
      "Vision-based tagging: performers, tags, title, and details, with performers-only and fill-missing modes.",
    location: "Scene / image pages and Settings → Tasks",
    howTo:
      "Use AI Tag in the scene/image menus, the batch action in the list selection, or the settings job for the whole library.",
  },
  {
    id: "segmentation",
    category: "Tagging & Analysis",
    name: "Scene segmentation",
    emoji: "🎬",
    description:
      "Breaks scenes into markers with descriptions, tags, per-segment performers, and an intensity rating (1-10).",
    location: "Settings → Tasks → AI Segment Scenes",
    howTo:
      "Run the job; markers appear in the scene's Markers tab with a climax map.",
  },
  {
    id: "climax-map",
    category: "Tagging & Analysis",
    name: "Climax map & good part",
    emoji: "⛰️",
    description:
      "An intensity timeline of the markers; click a peak to seek, or jump straight to the best moment.",
    location: "Scene page → Markers tab",
    howTo: "Click Skip to the good part, or click any peak on the climax map.",
  },
  {
    id: "highlight-clip",
    category: "Tagging & Analysis",
    name: "Highlight clips",
    emoji: "✂️",
    description:
      "Cuts an mp4 around a marker into <config>/clips/ for saving or sharing.",
    location: "Scene page → Markers tab",
    howTo: "Press Create highlight clip next to the best part button.",
  },
  {
    id: "moods",
    category: "Tagging & Analysis",
    name: "AI mood tagging",
    emoji: "🎭",
    description:
      "Classifies every scene into 2-4 moods shown as card badges, with sidebar filters and mood groups.",
    location: "Settings → Tasks → Tag Moods; scene sidebar filters",
    howTo:
      "Run Tag Moods, then filter or sort by Moods/Steam in the scene list, or create a Mood Group.",
  },
  {
    id: "audio-analysis",
    category: "Tagging & Analysis",
    name: "Audio analysis",
    emoji: "📊",
    description:
      "Whisper transcription with timestamps, summaries, and silence/music/speech/moans flags.",
    location: "Scene page → audio panel; Settings → Tasks",
    howTo:
      "Run AI Analyze Audio (or schedule it), then open a scene to see the transcript and analysis.",
  },
  {
    id: "steam",
    category: "Tagging & Analysis",
    name: "Steam score",
    emoji: "🔥",
    description:
      "A 0-10 score per scene from moans, silence, and explicit tags; shown on cards and usable as a filter and sort.",
    location: "Scene cards, scene sidebar",
    howTo:
      "Filter by Steam score in the sidebar or sort by it; badges show on cards.",
  },
  {
    id: "height",
    category: "Tagging & Analysis",
    name: "Per-scene height",
    emoji: "✨",
    description:
      "A 0-25 score adding your O history and mood bonuses on top of steam.",
    location: "Scene cards",
    howTo: "Nothing to do — the sparkle badge appears on cards.",
  },
  {
    id: "loop",
    category: "Tagging & Analysis",
    name: "Loop detection",
    emoji: "🔁",
    description:
      "Detects seamlessly looping videos (batch or per scene) and tags them.",
    location: "Scene page, scene list selection, Settings → Tasks",
    howTo:
      "Use Detect loops in the scene menu or list selection; already-tagged scenes are skipped.",
  },
  {
    id: "collections",
    category: "Tagging & Analysis",
    name: "Smart collections",
    emoji: "🗃️",
    description:
      "AI-built groups and galleries from tags, performers, studios, and embedding clusters.",
    location: "Settings → Tasks → AI Smart Collections",
    howTo: "Run the job; groups appear in the Groups page.",
  },
  {
    id: "duplicates",
    category: "Tagging & Analysis",
    name: "Duplicate detection",
    emoji: "👯",
    description:
      "Semantic duplicate groups with one-click merge actions for scenes and performers.",
    location: "Settings → Tasks → Duplicate Detection",
    howTo:
      "Detect groups, then press Merge into first on scene/performer groups.",
  },
  {
    id: "file-rename",
    category: "Tagging & Analysis",
    name: "AI file rename",
    emoji: "📝",
    description: "Suggests filename renames based on the AI analysis.",
    location: "Settings → Tasks",
    howTo: "Generate suggestions, review, and apply.",
  },

  // --- Review Flows ---
  {
    id: "merge-suggest",
    category: "Review Flows",
    name: "Performer merge suggestions",
    emoji: "🔀",
    description:
      "Finds performers whose images look alike and suggests merges with a confidence score.",
    location: "Settings → Tasks → Review Suggestions",
    howTo:
      "Review pairs and press Merge or Reject (or Apply all / Reject all).",
  },
  {
    id: "audit",
    category: "Review Flows",
    name: "AI audit",
    emoji: "🩺",
    description:
      "Re-analyzes tagged scenes/images and lists discrepancies: empty titles/details, missing performers.",
    location: "Settings → Tasks → Run Audit / Review Findings",
    howTo:
      "Run the audit, then apply or dismiss each finding (Apply all works too).",
  },
  {
    id: "translation",
    category: "Review Flows",
    name: "AI translation",
    emoji: "🌍",
    description:
      "Batch-translates titles/details into the language configured in Settings → AI.",
    location: "Settings → Tasks",
    howTo:
      "Set a translation language in AI settings, run the job, and review before applying.",
  },
  {
    id: "discovery",
    category: "Review Flows",
    name: "Performer discovery",
    emoji: "🕵️",
    description:
      "Finds recurring unknown performers in untagged scenes/images and builds review candidates.",
    location: "Settings → Tasks → Discover Performers / Review Candidates",
    howTo:
      "Run discovery, then create an Unknown Performer, merge into an existing one, or reject.",
  },

  // --- The Gooner Update ---
  {
    id: "goon-mode",
    category: "The Gooner Update",
    name: "Goon mode",
    emoji: "🌶️",
    description:
      "Filters the scene list to steam 6+ with a warm tint. Press G to toggle anywhere.",
    location: "Global (G key) + Settings → Interface",
    howTo: "Press G, or enable the default in Settings → Interface.",
  },
  {
    id: "roulette",
    category: "The Gooner Update",
    name: "Fap Roulette",
    emoji: "🎲",
    description:
      "Surprise me plays a random scene from your queue or the library.",
    location: "Scene player (roulette bar)",
    howTo: "Press Surprise me above the player.",
  },
  {
    id: "afterglow",
    category: "The Gooner Update",
    name: "Afterglow reel",
    emoji: "⚡",
    description:
      "Jumps each scene to its best moment and auto-advances after 45 seconds (or when you finish).",
    location: "Scene player",
    howTo:
      "Toggle Afterglow; the state survives reloads. Works without markers (plays from the start).",
  },
  {
    id: "recap",
    category: "The Gooner Update",
    name: "Session recap",
    emoji: "📜",
    description:
      "After an afterglow session, a recap shows scenes played, best moments, and O count.",
    location: "Scene player",
    howTo: "Toggle Afterglow off to see the recap.",
  },
  {
    id: "session-builder",
    category: "The Gooner Update",
    name: "Session Builder",
    emoji: "🧩",
    description:
      "Builds a curated queue by duration, performers, moods, steam floor, and vibe; start it or save it as a plan.",
    location: "Settings → Tasks and the For You hub button",
    howTo:
      "Adjust the sliders and filters, press Build, then Start session or Save plan.",
  },
  {
    id: "goon-reel",
    category: "The Gooner Update",
    name: "Goon reel",
    emoji: "📼",
    description:
      "Renders one mp4 from the best-moment clips of your plan or saved moments.",
    location: "Session builder preview; /saved page",
    howTo:
      "Build a plan and press Create goon reel, or use Create reel from saved.",
  },
  {
    id: "daily-goon",
    category: "The Gooner Update",
    name: "Daily Goon",
    emoji: "📅",
    description:
      "A day-seeded steamy pick with a streak counter on the front page.",
    location: "Front page (top)",
    howTo: "Check in daily; the streak grows.",
  },
  {
    id: "for-you",
    category: "The Gooner Update",
    name: "For You hub",
    emoji: "🏠",
    description:
      "Tonight's picks, Moaner of the week, Back for more, achievements, and the throne on the front page.",
    location: "Front page",
    howTo: "Open the front page.",
  },
  {
    id: "o-board",
    category: "The Gooner Update",
    name: "O board",
    emoji: "💦",
    description: "Performers ranked by the scenes you logged an O for.",
    location: "Stats page and Goon Scoreboard",
    howTo: "Log O's on scenes; the board updates.",
  },
  {
    id: "quick-hide",
    category: "The Gooner Update",
    name: "Quick-hide",
    emoji: "🙈",
    description:
      "Blurs all media and hides titles instantly for shared screens.",
    location: "Global (H key)",
    howTo: "Press H.",
  },

  // --- The Ascension Update ---
  {
    id: "edging",
    category: "The Ascension Update",
    name: "Edging mode",
    emoji: "⏳",
    description:
      "Pauses playback after a random interval (0.5-10 min by default, or a fixed interval) with a Continue overlay.",
    location: "Scene player",
    howTo:
      "Toggle Edging, pick an interval; Continue resumes playback. Each pause counts toward the Edge pauses achievement.",
  },
  {
    id: "blind",
    category: "The Ascension Update",
    name: "Blind goon",
    emoji: "🕶️",
    description:
      "Plays a random high-steam scene blurred until it ends — mystery box mode.",
    location: "Scene player",
    howTo: "Toggle Blind; the reveal comes on completion.",
  },
  {
    id: "radio",
    category: "The Ascension Update",
    name: "Vibe radio",
    emoji: "📻",
    description: "An endless shuffled afterglow queue of a chosen mood.",
    location: "Scene player (Radio dropdown)",
    howTo: "Pick a mood from the Radio menu; playback starts automatically.",
  },
  {
    id: "saved-moments",
    category: "The Ascension Update",
    name: "Saved moments",
    emoji: "❤️",
    description:
      "Heart any marker; the /saved page collects them and can build a reel.",
    location: "Scene markers (hearts) + /saved page",
    howTo:
      "Tap the heart on a climax-map peak, then visit /saved (linked from the scoreboard).",
  },
  {
    id: "watch-along",
    category: "The Ascension Update",
    name: "Watch-along chat",
    emoji: "👀",
    description:
      "The chat knows what you are watching and narrates along with each message.",
    location: "Scene player (Chat button)",
    howTo:
      "Press Chat and ask anything — your messages carry the current scene context.",
  },
  {
    id: "scoreboard",
    category: "The Ascension Update",
    name: "Goon Scoreboard",
    emoji: "🏅",
    description:
      "One page with moaners, O board, streak, saved moments, session stats, finish-history timeline, and saved plans.",
    location: "/scoreboard (For You hub button)",
    howTo: "Open the scoreboard from the For You hub.",
  },

  // --- The Heaven Update ---
  {
    id: "performer-session",
    category: "The Heaven Update",
    name: "Performer session",
    emoji: "👸",
    description: "A 30-minute afterglow queue of a performer's scenes.",
    location: "Performer page",
    howTo: "Press Her session next to AI Tag.",
  },
  {
    id: "auto-advance",
    category: "The Heaven Update",
    name: "Auto-advance on O",
    emoji: "⚡",
    description: "Logging an O during afterglow immediately advances.",
    location: "Scene player (during afterglow)",
    howTo: "Just log an O mid-session.",
  },
  {
    id: "who-is-she",
    category: "The Heaven Update",
    name: "Who is she?",
    emoji: "🤔",
    description:
      "A live chip shows the performer of the current segment during playback.",
    location: "Scene player",
    howTo: "Tap the chip to open her page.",
  },
  {
    id: "transcend",
    category: "The Heaven Update",
    name: "Transcend mode",
    emoji: "🕊️",
    description:
      "The everything-button: 60-minute build-up session of the steamiest scenes with afterglow, random edging, and a blind every third scene.",
    location: "Scene player",
    howTo: "Press Transcend and hold on.",
  },
  {
    id: "throne",
    category: "The Heaven Update",
    name: "The Throne",
    emoji: "👑",
    description:
      "XP ranks from scenes, O's, streaks, sessions, and session time — from Novice Gooner to the mystery rank.",
    location: "Scoreboard + For You hub",
    howTo: "Keep going; the XP climbs automatically.",
  },
  {
    id: "projection",
    category: "The Heaven Update",
    name: "Climax projection",
    emoji: "📈",
    description:
      "A live intensity bar tracks the playback position during afterglow.",
    location: "Scene player (during afterglow)",
    howTo: "Turn on Afterglow and watch the bar.",
  },
  {
    id: "playlists",
    category: "The Heaven Update",
    name: "Heavenly playlists",
    emoji: "📚",
    description: "Saved session plans replayed from the builder or scoreboard.",
    location: "Session builder + scoreboard",
    howTo: "Build a plan, type a name, press Save plan; replay it later.",
  },
  {
    id: "oracle",
    category: "The Heaven Update",
    name: "The Oracle",
    emoji: "✨",
    description:
      "Describe a session in words and the builder (or the chat) plans it for you.",
    location: "Session builder + AI Chat",
    howTo:
      "Type e.g. '40 minutes, sensual then rough, ending intense' in the Oracle box and press the button.",
  },
  {
    id: "night-chapel",
    category: "The Heaven Update",
    name: "Night Chapel",
    emoji: "🕯️",
    description: "A warm, dim candlelight theme that deepens goon mode.",
    location: "Settings → Interface",
    howTo: "Toggle Night Chapel in the interface settings.",
  },

  // --- Embeddings & Jobs ---
  {
    id: "embeddings",
    category: "Embeddings & Jobs",
    name: "Embeddings",
    emoji: "🧠",
    description:
      "Text and visual embeddings for scenes, performers, images, galleries, studios, and tags, with stale-only refresh.",
    location: "Settings → Tasks → Generate Embeddings; Settings → AI",
    howTo:
      "Generate embeddings once; use the stale-only option to refresh what changed.",
  },
  {
    id: "scheduled",
    category: "Embeddings & Jobs",
    name: "Scheduled AI maintenance",
    emoji: "⏰",
    description:
      "Automatic stale-embedding refresh and audio analysis on intervals.",
    location: "Settings → AI",
    howTo: "Set intervals in hours in the AI settings panel.",
  },
  {
    id: "job-queue",
    category: "Embeddings & Jobs",
    name: "Job queue",
    emoji: "🧰",
    description: "AI jobs are tagged with a robot badge and can be filtered.",
    location: "Settings → Tasks (queue)",
    howTo: "Tick AI tasks only to see AI jobs.",
  },
];
