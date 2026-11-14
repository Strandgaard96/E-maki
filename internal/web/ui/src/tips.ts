// The hover help for each setting: what it does, and for a choice that
// depends on .env, what to set there. Each setting lives in one place: these
// only on the settings page, the rest only in .env.

export interface TipText {
  text: string;
  where?: string;
}

const ENV_RESTART = "then run docker compose up -d.";

export const TIPS = {
  // Listening > Microphone
  mic_device: {
    text: "The input E-maki listens on. System default follows the computer's own choice. A picked mic that is unplugged falls back to the default until it is back.",
    where: `A mic elsewhere on the network needs NET_MIC_BIND in .env, ${ENV_RESTART}`,
  },
  min_speech_dbfs: {
    text: "Segments whose loudest moment is quieter than this are skipped and never transcribed. Raise it if words show up during silence, lower it if talk is missed.",
  },
  languages: {
    text: "With one language ticked, Whisper skips guessing. With several, a guess outside the list is done again in the first one.",
    where: `Separate Danish and Norwegian models: WHISPER_DA_NO=1 in .env, ${ENV_RESTART}`,
  },

  // Listening > Topics
  topic_provider: {
    text: "Which model reads the transcript and names the topic. On this machine nothing leaves the house. External sends the transcript window to that service.",
    where: `The models are set in .env (LLM_MODEL, TOPIC_API_URL, TOPIC_API_KEY, TOPIC_API_MODEL), ${ENV_RESTART}`,
  },
  min_confidence: { text: "The topic model says how sure it is of each topic. Topics below this get no picture." },
  window_segments: { text: "How much recent talk the topic model reads each time, in audio segments." },
  extract_every: { text: "How many new segments pass between two topic checks." },
  topic_prompt: {
    text: "The instructions the topic model gets: what a good topic is and how to describe its picture. The privacy rules and answer format are always added after it.",
  },
  llm_temperature: {
    text: "How much the topic model's answers vary. 0 gives the same answer to the same talk. Default 0.2.",
  },

  // Listening > Privacy
  block_sensitive: {
    text: "The topic model tags each topic. One tagged with a kind that is on here, or one it is unsure about, gets no picture.",
  },
  block_names: {
    text: "Holds back a picture whose description repeats a name heard in the room. It works on capital letters, so places and brands are held back too.",
  },
  never_paint: {
    text: "Words that never become a picture. Whole words, any case, plurals too. The list is never logged.",
  },

  // System
  image_provider: {
    text: "Who draws the pictures. Automatic takes the first one that is set up, this machine first.",
    where: `Keys and models are set in .env (GEMINI_API_KEY, OPENAI_API_KEY, OPENROUTER_API_KEY, or LOCAL_IMAGES=1), ${ENV_RESTART}`,
  },
  daily_image_cap: {
    text: "The most new pictures in a day. After that, new topics wait for the next day. 0 makes no pictures. Test pictures don't count.",
  },
  keep_pictures: {
    text: "Saves the pictures on the frame and in the queue, so the frame looks the same after a restart. Never what was said.",
  },
  timeline: {
    text: "Saves a copy of every new picture, with its topic and description, for History. Never what was said.",
  },

  // Frame > Layout
  layout: {
    text: "How the pictures share the frame. Collage keeps the most talked-about topics. One picture shows only the newest. Rolling keeps the newest few.",
  },
  tiles: { text: "The most pictures on the frame at once." },
  arrange: {
    text: "Newest big gives the first picture the big tile. Cascade makes each one a little smaller than the one before. Mosaic mixes sizes. Same size puts them in even rows.",
  },
  spacing: { text: "The gap between the pictures and around the edge of the frame." },
  shape: { text: "The outline of each picture. Arch has a round top." },
  background: { text: "The colour behind and between the pictures." },
  hide_captions: { text: "The topic's name under each picture. Off gives the pictures more room." },

  // Frame > Timing
  mode: {
    text: "Diary waits for topics to come up twice and paints the top ones once a day. Continuous paints each new topic straight away.",
  },
  diary_at: { text: "When Diary paints the topics it collected. Uses the time zone in TZ in .env." },
  new_every_minutes: { text: "The least time between two new pictures. Topics that come up faster wait in line." },
  rotate_minutes: {
    text: "With more saved pictures than fit, swaps in the ones not shown, this often. Makes no new pictures.",
  },
  quiet_hours: {
    text: "No new pictures go up between these times. Topics still collect and go up when quiet hours end.",
  },
  half_life_hours: { text: "How long it takes a picture to shrink to half its size when nobody talks about it." },

  // Frame > Panel
  panel_size: { text: "Your e-ink screen's resolution. The picture is drawn at this size." },
  panel_rotation: { text: "Turns the picture for a frame that hangs sideways or upside down." },
  panel_poll_minutes: {
    text: "How long a battery frame sleeps between checks for a new picture. Longer saves battery.",
  },
  panel_buttons: {
    text: "How many buttons your frame has. Only frames whose software reports a press can use them, like the Inky client on a Pi.",
  },
  panel_button: { text: "What this button does when pressed." },
} satisfies Record<string, TipText>;
