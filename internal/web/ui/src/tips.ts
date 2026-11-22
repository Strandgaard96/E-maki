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
    text: "The input E-maki listens on. A new mic shows up here within a few seconds. System default follows the computer's own choice. A picked mic that is unplugged falls back to the default until it is back. The bar is the level now; the line is where talk starts.",
    where: `A mic elsewhere on the network needs NET_MIC_BIND in .env, ${ENV_RESTART}`,
  },
  speech_threshold: {
    text: "Sound quieter than the threshold is skipped, because Whisper tends to invent words in silence.",
    where: "Segment length is segment_seconds in the profile, and needs a restart.",
  },
  min_speech_dbfs: {
    text: "Segments whose loudest moment is quieter than this are skipped and never transcribed. 0 dB is the loudest the mic can record, -80 dB is silence. Works best a little above a quiet room.",
  },
  languages: {
    text: "The languages spoken here. With none, Whisper guesses from all it knows. With one, it skips guessing. With several, a guess outside the list is done again in the first one.",
    where: `Separate Danish and Norwegian models: WHISPER_DA_NO=1 in .env, ${ENV_RESTART}`,
  },

  // Listening > Topics
  topic_provider: {
    text: "Which model reads the transcript and names the topic. On this machine nothing leaves the house. External sends the transcript window to that service.",
    where: `The models are set in .env (LLM_MODEL, TOPIC_API_URL, TOPIC_API_KEY, TOPIC_API_MODEL), ${ENV_RESTART}`,
  },
  min_confidence: {
    text: "The topic model says how sure it is of each topic. Topics below this get no picture. Higher gives fewer, clearer topics.",
  },
  window_segments: {
    text: "How much recent talk the topic model reads each time. Longer gives more context; shorter follows the talk faster.",
  },
  extract_every: {
    text: "How many new segments pass between two topic checks. More often reacts faster but keeps the model busier.",
  },
  topic_prompt: {
    text: "The instructions the topic model gets: what a good topic is and how to describe its picture. The privacy rules and answer format are always added after it.",
  },
  llm_temperature: {
    text: "How much the topic model's answers vary. 0 gives the same answer to the same talk. Default 0.2.",
  },

  // Listening > Privacy
  block_sensitive: {
    text: "Topics like these never get a picture. The topic model tags each topic; one tagged with a kind that is on here, or one it is unsure about, is held back.",
  },
  block_names: {
    text: "Holds back a picture whose description repeats a name heard in the room. It works on capital letters, so places and brands are held back too.",
  },
  never_paint: {
    text: "One word or phrase a line. A topic or description that contains one never becomes a picture. Whole words, any case, plurals too. The list is never logged.",
  },

  // System
  image_provider: {
    text: "Who draws the pictures. Automatic takes the first one that is set up, this machine first.",
    where: `Greyed-out ones need a key or model in .env (GEMINI_API_KEY, OPENAI_API_KEY, OPENROUTER_API_KEY, or LOCAL_IMAGES=1), ${ENV_RESTART}`,
  },
  test_picture: {
    text: "Draws one picture with the provider above and the current style. Not shown on the frame, not counted.",
  },
  daily_image_cap: {
    text: "The most new pictures in a day. After that, new topics wait for the next day. 0 makes no pictures. Test pictures don't count. With an online provider, each one is a paid call.",
  },
  keep_pictures: {
    text: "Saves the pictures on the frame and in the queue, so the frame looks the same after a restart. Never what was said. Turning it off deletes them.",
  },
  storage: {
    text: "Audio is never written to disk. What was said lives only in a short window in memory, gone when E-maki stops or listening is paused. The settings and password files hold no audio and no words.",
    where: "The full list is in PRIVACY.md.",
  },
  password: {
    text: "Without a password, anyone on your network can open this page. The frame's picture and buttons stay open, because e-ink frames can't sign in.",
    where: "PANEL_TOKEN in .env guards those.",
  },
  new_password: { text: "At least 8 characters. A few random words work well." },
  panel_token: {
    text: "Moves this page and the frame under a secret address.",
    where: `Set PANEL_TOKEN in .env, ${ENV_RESTART}`,
  },
  env: { text: "Set in .env next to docker-compose.yml, not here.", where: `Change one, ${ENV_RESTART}` },
  removed: { text: "These stay off the frame until the date shown, even when they come up again." },
  timeline: {
    text: "Saves a copy of every new picture, with its topic and description, for History. Never what was said.",
  },

  // Frame
  image_style: { text: "Used for new pictures. Tiles already on the frame keep their look." },
  custom_style: { text: "Flat colours in black, white, yellow, red, blue and green work best on the e-ink panel." },

  // Frame > Layout
  layout: {
    text: "Which topics are on the frame. Most talked about keeps the topics that come up most, and a quiet one leaves. One picture shows only the newest. Latest keeps the newest few, however much each was talked about.",
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
  diary_at: {
    text: "When Diary paints the topics it collected. Topics talked about at least twice wait for this time; the heaviest get pictures, within the daily limit.",
    where: "Uses the time zone in TZ in .env.",
  },
  new_every_minutes: { text: "The least time between two new pictures. Topics that come up faster wait in line." },
  rotate_minutes: {
    text: "With more saved pictures than fit, swaps in the ones not shown, this often. Makes no new pictures.",
  },
  quiet_hours: {
    text: "No new pictures go up between these times. Topics still collect and go up when quiet hours end.",
  },
  half_life_hours: {
    text: "How long it takes a topic nobody mentions to count half as much. One heard once leaves after about three times this; talking about it again keeps it.",
  },

  // Frame > Panel
  panel_size: { text: "Your e-ink screen's resolution. The picture is drawn at this size." },
  panel_inks: { text: "The colours your screen can show. Pictures are dithered to just these." },
  panel_rotation: { text: "Turns the picture for a frame that hangs sideways or upside down." },
  panel_poll_minutes: {
    text: "How long a battery frame (like the EE02) sleeps between checks for a new picture. Longer saves battery. Frames on mains power check on their own.",
  },
  panel_buttons: {
    text: "What the buttons on your display's driver board do. Works with an Inky on a Pi (A to D from the top) and a XIAO EE02 on its Arduino firmware (KEY3 is A, KEY2 is B, KEY1 is C). Any press wakes a XIAO and redraws it. The letter lights up here when a press arrives.",
  },
  panel_button: { text: "What this button does when pressed." },
} satisfies Record<string, TipText>;
