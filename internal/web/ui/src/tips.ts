// The hover help for each setting: what it does, and where else it can be
// set. A page setting starts from the profile (configs/*.json) and what is
// saved here wins over it; .env holds what needs a new start.

export interface TipText {
  text: string;
  where?: string;
}

// page is a setting this page saves, under its key in the profile.
const page = (text: string, key: string): TipText => ({
  text,
  where: `Also ${key} in the profile. What you save here wins.`,
});

const ENV_RESTART = "then run docker compose up -d.";

export const TIPS = {
  // Listening > Microphone
  mic_device: {
    text: "The input E-maki listens on. System default follows the computer's own choice. A picked mic that is unplugged falls back to the default until it is back.",
    where: `Also mic_device in the profile. A mic elsewhere on the network needs NET_MIC_BIND in .env, ${ENV_RESTART}`,
  },
  min_speech_dbfs: page(
    "Segments whose loudest moment is quieter than this are skipped and never transcribed. Raise it if words show up during silence, lower it if talk is missed.",
    "min_speech_dbfs",
  ),
  languages: {
    text: "With one language ticked, Whisper skips guessing. With several, a guess outside the list is done again in the first one.",
    where: `Also languages in the profile. Separate Danish and Norwegian models: WHISPER_DA_NO=1 in .env, ${ENV_RESTART}`,
  },

  // Listening > Topics
  topic_provider: {
    text: "Which model reads the transcript and names the topic. On this machine nothing leaves the house. External sends the transcript window to that service.",
    where: `Also topic_provider in the profile. The models are set in .env (LLM_MODEL, TOPIC_API_URL, TOPIC_API_KEY, TOPIC_API_MODEL), ${ENV_RESTART}`,
  },
  min_confidence: page(
    "The topic model says how sure it is of each topic. Topics below this get no picture.",
    "min_confidence",
  ),
  window_segments: page("How much recent talk the topic model reads each time, in audio segments.", "window_segments"),
  extract_every: page("How many new segments pass between two topic checks.", "extract_every"),
  topic_prompt: page(
    "The instructions the topic model gets: what a good topic is and how to describe its picture. The privacy rules and answer format are always added after it.",
    "topic_prompt",
  ),
  llm_temperature: page(
    "How much the topic model's answers vary. 0 gives the same answer to the same talk. Default 0.2.",
    "llm_temperature",
  ),

  // Listening > Privacy
  block_sensitive: page(
    "The topic model tags each topic. One tagged with a kind that is on here, or one it is unsure about, gets no picture.",
    "block_sensitive",
  ),
  block_names: page(
    "Holds back a picture whose description repeats a name heard in the room. It works on capital letters, so places and brands are held back too.",
    "block_names",
  ),
  never_paint: page(
    "Words that never become a picture. Whole words, any case, plurals too. The list is never logged.",
    "never_paint",
  ),

  // System
  image_provider: {
    text: "Who draws the pictures. Automatic takes the first one that is set up, this machine first.",
    where: `Also image_provider in the profile. Keys and models are set in .env (GEMINI_API_KEY, OPENAI_API_KEY, OPENROUTER_API_KEY, or LOCAL_IMAGES=1), ${ENV_RESTART}`,
  },
  daily_image_cap: page(
    "The most new pictures in a day. After that, new topics wait for the next day. 0 makes no pictures. Test pictures don't count.",
    "daily_image_cap",
  ),
  keep_pictures: page(
    "Saves the pictures on the frame and in the queue, so the frame looks the same after a restart. Never what was said.",
    "keep_pictures",
  ),
  timeline: page(
    "Saves a copy of every new picture, with its topic and description, for History. Never what was said.",
    "timeline",
  ),

  // Frame > Layout
  layout: page(
    "How the pictures share the frame. Collage keeps the most talked-about topics. One picture shows only the newest. Rolling keeps the newest few.",
    "layout",
  ),
  tiles: page("The most pictures on the frame at once.", "tiles"),
  arrange: page(
    "Newest big gives the first picture the big tile. Cascade makes each one a little smaller than the one before. Mosaic mixes sizes. Same size puts them in even rows.",
    "arrange",
  ),
  spacing: page("The gap between the pictures and around the edge of the frame.", "spacing"),
  shape: page("The outline of each picture. Arch has a round top.", "shape"),
  background: page("The colour behind and between the pictures.", "background"),
  hide_captions: page("The topic's name under each picture.", "hide_captions"),

  // Frame > Timing
  mode: page(
    "Diary waits for topics to come up twice and paints the top ones once a day. Continuous paints each new topic straight away.",
    "mode (diary or party)",
  ),
  diary_at: page("When Diary paints the topics it collected. Uses the time zone in TZ in .env.", "diary_at"),
  new_every_minutes: page(
    "The least time between two new pictures. Topics that come up faster wait in line.",
    "new_every_minutes",
  ),
  rotate_minutes: page(
    "With more saved pictures than fit, swaps in the ones not shown, this often. Makes no new pictures.",
    "rotate_minutes",
  ),
  quiet_hours: page(
    "No new pictures go up between these times. Topics still collect and go up when quiet hours end.",
    "quiet_from and quiet_to",
  ),
  half_life_hours: page(
    "How long it takes a picture to shrink to half its size when nobody talks about it.",
    "half_life_hours",
  ),

  // Frame > Panel
  panel_size: page(
    "Your e-ink screen's resolution. The picture is drawn at this size.",
    "panel_width and panel_height",
  ),
  panel_rotation: page("Turns the picture for a frame that hangs sideways or upside down.", "panel_rotation"),
  panel_poll_minutes: page(
    "How long a battery frame sleeps between checks for a new picture. Longer saves battery.",
    "panel_poll_minutes",
  ),
  panel_buttons: page(
    "How many buttons your frame has. Only frames whose software reports a press can use them, like the Inky client on a Pi.",
    "panel_buttons",
  ),
  panel_button: page("What this button does when pressed.", "panel_buttons"),
} satisfies Record<string, TipText>;
