// The JSON the Go service answers with (internal/web). Times are RFC 3339
// strings.

// Settings mirrors config.Settings.
export interface Settings {
  mode: string; // "diary" or "party"
  diary_at: string;
  mic_device: string;
  min_speech_dbfs: number;
  window_segments: number;
  extract_every: number;
  languages: string[];
  topic_prompt: string;
  llm_temperature: number | null;
  topic_provider: string;
  half_life_hours: number;
  min_confidence: number;
  daily_image_cap: number;
  block_sensitive: string[];
  block_names: boolean;
  never_paint: string[] | null;
  image_style: string;
  image_provider: string;
  layout: string;
  tiles: number;
  new_every_minutes: number;
  newest_big: boolean;
  quiet_from: string;
  quiet_to: string;
  hide_captions: boolean;
  arrange: string;
  spacing: string;
  shape: string;
  background: string;
  rotate_minutes: number;
  keep_pictures: boolean | null;
  timeline: boolean | null;
  paused: boolean;
  panel_buttons: Record<string, string> | null;
  panel_width: number;
  panel_height: number;
  panel_rotation: number;
  panel_poll_minutes: number;
}

export interface Tile {
  label: string;
  mentions: number;
  last_seen: string;
  pinned: boolean;
  prompt: string;
  made: string;
  style: string;
  rect: [number, number, number, number];
}

export interface Style {
  name: string;
  prompt: string;
}

export interface Provider {
  id: string;
  name: string;
  local: boolean;
  ready: boolean;
  setup: string;
}

export interface Service {
  state: "unknown" | "up" | "loading" | "down" | "failing";
  error?: string;
  last_at?: string;
  took?: number;
}

export interface Status {
  profile: string;
  topics: string;
  llm_url: string;
  topic_api: string;
  no_local_topics: boolean;
  speech: string;
  by_lang: Record<string, string> | null;
  segment: number;
  images: string;
  locked: boolean;
  saved: boolean;
  started: string;
  password: boolean;
}

export interface Activity {
  heard: number;
  quiet: number;
  topics: number;
  held_back: number;
  dropped: number;
  last_level: number;
  last_lang: string;
  last_at: string;
}

export interface State {
  settings: Settings;
  styles: Style[];
  default_style: string;
  categories: string[];
  languages: [string, string][];
  wall: Tile[];
  queue: Tile[];
  removed: { label: string; until: string }[];
  max_tiles: number;
  waiting: number;
  next_at?: string;
  images_today: number;
  collected: number;
  diary_next?: string;
  page: string;
  activity: Activity;
  default_prompt: string;
  providers: Provider[];
  status: Status;
  health: Record<string, Service>;
  warning?: string;
}

export interface Mic {
  live: boolean;
  devices: { name: string; default: boolean }[];
  active: string;
  level?: number;
  error?: string;
}

export interface Picture {
  id: string;
  label: string;
  prompt: string;
  style: string;
  made: string;
  w: number;
  h: number;
}

export interface Timeline {
  saved: boolean;
  on: boolean;
  row_px: number;
  pictures: Picture[] | null;
}
