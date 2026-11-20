// E-maki's settings page: Preact (TSX), styled with Tailwind and Basecoat.
// `npm run build` bundles it into ../static/app.js, which the Go binary embeds.

import justifiedLayout from "justified-layout";
import {
  Activity,
  Ban,
  Brush,
  CircleQuestionMark,
  Clock,
  Frame as FrameIcon,
  HardDrive,
  ImagePlus,
  Images,
  KeyRound,
  Languages as LanguagesIcon,
  LayoutGrid,
  ListOrdered,
  Lock,
  type LucideIcon,
  MessagesSquare,
  Mic as MicIcon,
  NotebookPen,
  Palette,
  Pencil,
  ScrollText,
  Server,
  Settings as SettingsIcon,
  ShieldCheck,
  SlidersHorizontal,
  Tablet,
  Volume2,
  Zap,
} from "lucide-preact";
import PhotoSwipe from "photoswipe";
import type { ComponentChildren, JSX, RefObject, TargetedEvent } from "preact";
import { createContext, render } from "preact";
import { useCallback, useContext, useEffect, useId, useLayoutEffect, useRef, useState } from "preact/hooks";
import { TIPS, type TipText } from "./tips";
import type {
  Service as Health,
  Mic,
  Picture,
  Provider,
  Settings,
  State,
  Tile,
  Timeline as TimelineData,
} from "./types";

// Basecoat's dark theme hangs on a class; follow the system setting.
const dark = matchMedia("(prefers-color-scheme: dark)");
const theme = () => document.documentElement.classList.toggle("dark", dark.matches);
dark.addEventListener("change", theme);
theme();

const CATEGORIES: Record<string, [string, string]> = {
  health: ["Health", "Illness, doctors, bodies"],
  money: ["Money", "Salaries, debt, what things cost"],
  relationship: ["Relationships", "Dating, break-ups, family rows"],
  confidential_work: ["Work secrets", "Clients, deals, anything under NDA"],
};

const HALF_LIVES: [number, string][] = [
  [6, "6 hours"],
  [12, "12 hours"],
  [24, "1 day"],
  [48, "2 days"],
  [96, "4 days"],
  [168, "1 week"],
  [336, "2 weeks"],
];
const POLLS = [1, 2, 5, 10, 15, 30, 60];
const ROTATIONS = [0, 90, 180, 270];

async function api<T = State>(path: string, method = "GET", body?: unknown): Promise<T> {
  const r = await fetch(path, {
    method,
    cache: "no-store",
    headers: body ? { "Content-Type": "application/json" } : {},
    body: body ? JSON.stringify(body) : undefined,
  });
  if (r.status === 401 && path !== "api/login") {
    throw Object.assign(new Error("Sign in first."), { signin: true });
  }
  if (!r.ok) {
    const text = (await r.text()).trim() || r.statusText;
    if (r.status < 500 || r.status === 501) throw new Error(text);
    // A fault on E-maki's side or a provider's: say what failed in plain words
    // and keep the technical part in detail.
    const head = text.indexOf(": ");
    const message = head > 0 ? text.slice(0, head) + "." : "Something went wrong. Try again.";
    throw Object.assign(new Error(message), { detail: text });
  }
  return r.status === 204 ? (null as T) : r.json();
}

// ApiError is what api throws: signin is set when the page needs a password,
// detail holds the technical text behind a short message.
type ApiError = Error & { signin?: boolean; detail?: string };

const same = (a: unknown, b: unknown) => JSON.stringify(a) === JSON.stringify(b);
// focus moves to an element once, when it appears (a stable ref callback).
const focus = (e: HTMLElement | null) => e?.focus();

function when(iso: string) {
  const d = new Date(iso);
  const today = new Date().toDateString() === d.toDateString();
  const time = d.toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" });
  return today ? `today at ${time}` : `${d.toLocaleDateString([], { weekday: "long" })} at ${time}`;
}

function App() {
  const [state, setState] = useState<State | null>(null);
  const [draft, setDraft] = useState<Settings | null>(null);
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");
  const [saving, setSaving] = useState(false);
  const [tab, setTab] = useState(TABS.find(([k]) => location.hash.startsWith("#" + k))?.[0] ?? "frame");
  const [signin, setSignin] = useState(false);
  const [hot, setHot] = useState(""); // the tile pointed at, in the list or in the picture
  const [redrawing, setRedrawing] = useState(""); // the picture being redrawn
  // The Frame tab's preview sticks right under the header, whose height changes when it wraps.
  const header = useRef<HTMLElement>(null);
  useEffect(() => {
    if (!header.current || !window.ResizeObserver) return;
    const ro = new ResizeObserver(([e]) =>
      document.documentElement.style.setProperty("--header-h", e.borderBoxSize[0].blockSize + "px"),
    );
    ro.observe(header.current);
    return () => ro.disconnect();
  }, [!!state]);
  const [jump, setJump] = useState(0); // remounts a section tab on the section in the address
  const [view, setView] = useState("now"); // Pictures: on the frame now, or the history

  const load = useCallback(async () => {
    try {
      const s = await api("api/state");
      setState(s);
      setDraft((d) => d ?? s.settings);
      setSignin(false);
      setError("");
    } catch (e) {
      if (e.signin) setSignin(true);
      else setError(e instanceof TypeError ? "Can't reach E-maki. Is it still running?" : e.message);
    }
  }, []);

  useEffect(() => {
    load();
    const t = setInterval(load, 15000);
    return () => clearInterval(t);
  }, [load]);

  if (signin) return <SignIn onDone={load} />;
  if (!state || !draft) return <Loading error={error} />;

  // Pausing and the timeline switch apply at once, so they are never part of the draft.
  const pending = { ...draft, paused: state.settings.paused, timeline: state.settings.timeline };
  const changed = (Object.keys(pending) as (keyof Settings)[]).filter((k) => !same(pending[k], state.settings[k]));
  const dirty = changed.length > 0;
  const where: Record<string, boolean> = {
    frame: changed.some((k) => !EMAKI_KEYS.includes(k) && !SYSTEM_KEYS.includes(k)),
    listening: changed.some((k) => EMAKI_KEYS.includes(k)),
    system: changed.some((k) => SYSTEM_KEYS.includes(k)),
  };
  const set = (patch: Partial<Settings>) => setDraft({ ...draft, ...patch });
  const show = (t: string) => {
    setTab(t);
    history.replaceState(null, "", "#" + t);
  };
  // The status in the header leads to the servers that are not ready.
  const openSystem = (e: Event) => {
    e.preventDefault();
    history.replaceState(null, "", "#system/status");
    setTab("system");
    setJump((n) => n + 1);
  };

  const call = async (path: string, body: unknown, done?: string) => {
    try {
      const s = await api(path, "POST", body);
      setState(s);
      setError("");
      if (done) setNotice(done);
    } catch (e) {
      if (e.signin) return setSignin(true);
      setError(e.message);
      load();
    }
  };

  // A redraw waits for the image provider, which can take a while.
  const redraw = async (label: string) => {
    setRedrawing(label);
    await call("api/redraw", { label }, "Redrawn.");
    setRedrawing("");
  };

  const save = async () => {
    setSaving(true);
    try {
      const s = await api("api/settings", "PUT", pending);
      setState(s);
      setDraft(s.settings);
      setNotice(
        s.warning || (s.status.saved ? "Saved and in use now." : "In use now. These last until E-maki restarts."),
      );
      setError("");
    } catch (e) {
      if (e.signin) setSignin(true);
      else setError("Not saved: " + e.message);
    }
    setSaving(false);
  };

  const paused = state.settings.paused;
  const waiting = notReady(state.health);
  const Status = waiting ? "a" : "span";
  const togglePause = () => call("api/pause", { paused: !paused }, paused ? "Listening again." : "Listening paused.");

  return (
    <>
      <header ref={header} class="sticky top-0 z-10 border-b bg-background">
        <div class="mx-auto flex max-w-6xl flex-wrap items-center gap-x-6 gap-y-3 px-4 py-3">
          <Brand />
          <div class="tabs fixed inset-x-0 bottom-0 z-20 border-t bg-background px-2 pt-2 pb-[max(0.5rem,env(safe-area-inset-bottom))] sm:static sm:z-auto sm:w-auto sm:border-0 sm:bg-transparent sm:p-0">
            <nav
              role="tablist"
              aria-orientation="horizontal"
              aria-label="Sections"
              class="w-full sm:w-auto"
              onKeyDown={(e) => {
                if (e.key !== "ArrowLeft" && e.key !== "ArrowRight") return;
                const i = TABS.findIndex(([k]) => k === tab);
                const other = TABS[(i + (e.key === "ArrowRight" ? 1 : TABS.length - 1)) % TABS.length][0];
                show(other);
                document.getElementById("tab-" + other)?.focus();
              }}
            >
              {TABS.map(([k, name, Icon]) => (
                <button
                  type="button"
                  role="tab"
                  id={"tab-" + k}
                  data-area={k}
                  aria-selected={tab === k}
                  aria-controls={"panel-" + k}
                  tabindex={tab === k ? 0 : -1}
                  class="relative flex-col gap-0.5 text-xs sm:flex-row sm:gap-1.5 sm:text-sm"
                  onClick={() => show(k)}
                >
                  <Icon class="size-5 text-area sm:size-4" aria-hidden="true" />
                  {name}
                  <span
                    class={
                      "absolute top-1 right-0.5 size-1.5 rounded-full bg-ink-yellow " + (where[k] ? "" : "invisible")
                    }
                    role="img"
                    aria-label={where[k] ? "has unsaved changes" : undefined}
                    aria-hidden={!where[k]}
                  />
                </button>
              ))}
            </nav>
          </div>
          <div class="ms-auto flex items-center gap-3">
            <span role="status">
              <Status
                href={waiting ? "#system/status" : undefined}
                onClick={waiting ? openSystem : undefined}
                class={
                  "flex items-center gap-2 text-sm font-medium " + (waiting ? "underline-offset-4 hover:underline" : "")
                }
              >
                <span
                  aria-hidden="true"
                  class={
                    "size-2.5 rounded-full " +
                    (paused || waiting ? "bg-ink-yellow" : "bg-ink-green ring-4 ring-ink-green/20")
                  }
                />
                {paused ? "Paused" : waiting || "Listening"}
              </Status>
            </span>
            <button type="button" class="btn" data-variant="outline" onClick={togglePause}>
              {paused ? "Resume" : "Pause"}
            </button>
          </div>
        </div>
      </header>
      {error && (
        <div class="mx-auto max-w-6xl px-4 pt-4">
          <div class="alert" data-variant="destructive" role="alert">
            <section>{error}</section>
          </div>
        </div>
      )}
      {tab === "frame" ? (
        <FrameTab state={state} draft={draft} pending={pending} set={set} />
      ) : tab === "pictures" ? (
        view === "history" ? (
          <>
            <PicturesSwitch view={view} setView={setView} />
            <Timeline onState={setState} onSignedOut={() => setSignin(true)} />
          </>
        ) : (
          <>
            <PicturesSwitch view={view} setView={setView} />
            <main
              id="panel-pictures"
              // @ts-expect-error: ARIA in HTML allows <main> no role but "main"; kept as it was.
              role="tabpanel"
              aria-labelledby="tab-pictures"
              data-area="pictures"
              class="mx-auto grid max-w-6xl gap-6 px-4 pt-4 pb-6 lg:grid-cols-2 lg:items-start"
            >
              <Wall
                state={state}
                hot={hot}
                onHot={setHot}
                onTile={(a: string, label: string, to?: string) => call("api/tiles/" + a, { label, to })}
                onClear={() => call("api/clear", {}, "Frame cleared.")}
                redrawing={redrawing}
                onRedraw={redraw}
              />
              <div class="grid gap-6">
                <Queue
                  state={state}
                  onLine={(a: string, label: string, done?: string) => call("api/queue/" + a, { label }, done)}
                  redrawing={redrawing}
                  onRedraw={redraw}
                />
                <Removed
                  removed={state.removed}
                  onAllow={(label: string) => call("api/removed/allow", { label }, "It can come back now.")}
                />
              </div>
            </main>
          </>
        )
      ) : tab === "listening" ? (
        <SectionTab
          key={"l" + jump}
          id="listening"
          sections={LISTENING}
          state={state}
          draft={draft}
          set={set}
          paused={paused}
          onState={setState}
          onSignedOut={() => setSignin(true)}
        />
      ) : (
        <SectionTab
          key={"s" + jump}
          id="system"
          sections={SYSTEM}
          state={state}
          draft={draft}
          set={set}
          paused={paused}
          onState={setState}
          onSignedOut={() => setSignin(true)}
        />
      )}
      <SaveBar
        dirty={dirty}
        saving={saving}
        notice={notice}
        what={`${changed.length === 1 ? "1 unsaved change" : changed.length + " unsaved changes"} on ${[where.frame && "Frame", where.listening && "Listening", where.system && "System"].filter(Boolean).join(" and ")}.`}
        onSave={save}
        onDiscard={() => setDraft(state.settings)}
        onDone={() => setNotice("")}
      />
    </>
  );
}

type Set = (patch: Partial<Settings>) => void;
type DraftProps = { draft: Settings; set: Set };
type StateProps = DraftProps & { state: State };

const TABS: [string, string, LucideIcon][] = [
  ["frame", "Frame", FrameIcon],
  ["pictures", "Pictures", Images],
  ["listening", "Listening", MicIcon],
  ["system", "System", SettingsIcon],
];

// Settings that change the picture itself, which the frame can show before saving.
const PICTURE_KEYS: (keyof Settings)[] = [
  "layout",
  "tiles",
  "hide_captions",
  "arrange",
  "spacing",
  "background",
  "shape",
  "rotate_minutes",
  "half_life_hours",
  "panel_width",
  "panel_height",
  "panel_rotation",
];
// Settings edited on the Listening and System tabs; the rest are on Frame.
const SYSTEM_KEYS: (keyof Settings)[] = ["image_provider", "daily_image_cap", "keep_pictures"];
// The settings each section holds, for its unsaved dot.
const SECTION_KEYS: Record<string, (keyof Settings)[]> = {
  layout: ["layout", "tiles", "arrange", "spacing", "shape", "background", "hide_captions"],
  style: ["image_style"],
  timing: ["mode", "new_every_minutes", "rotate_minutes", "quiet_from", "quiet_to", "half_life_hours", "diary_at"],
  panel: ["panel_width", "panel_height", "panel_rotation", "panel_buttons", "panel_poll_minutes"],
  listening: ["mic_device", "min_speech_dbfs", "languages"],
  topics: ["topic_provider", "min_confidence", "window_segments", "extract_every", "topic_prompt", "llm_temperature"],
  privacy: ["block_sensitive", "block_names", "never_paint"],
  pictures: ["image_provider", "daily_image_cap"],
  storage: ["keep_pictures"],
};
const EMAKI_KEYS: (keyof Settings)[] = [
  "mic_device",
  "min_speech_dbfs",
  "languages",
  "window_segments",
  "extract_every",
  "topic_prompt",
  "llm_temperature",
  "min_confidence",
  "topic_provider",
  "block_sensitive",
  "block_names",
  "never_paint",
];

const FRAME_SECTIONS: [string, string, LucideIcon][] = [
  ["layout", "Layout", LayoutGrid],
  ["style", "Style", Palette],
  ["timing", "Timing", Clock],
  ["panel", "Panel", Tablet],
];

// FrameTab is the frame as large as fits, with its settings in a side panel
// of tabs beside it. The page itself does not scroll on a wide screen; only
// the panel does. On a phone the frame sticks on top and the panel follows.
function FrameTab({ state, draft, pending, set }: StateProps & { pending: Settings }) {
  const initial = FRAME_SECTIONS.find(([k]) => location.hash === "#frame/" + k)?.[0] ?? "layout";
  const [section, setSection] = useState(initial);
  const go = (k: string) => {
    setSection(k);
    history.replaceState(null, "", "#frame/" + k);
  };
  const unsaved = (k: string) => SECTION_KEYS[k].some((key) => !same(draft[key], state.settings[key]));
  return (
    <main
      id="panel-frame"
      // @ts-expect-error: ARIA in HTML allows <main> no role but "main"; kept as it was.
      role="tabpanel"
      aria-labelledby="tab-frame"
      data-area="frame"
      class="grid pb-24 md:h-[calc(100dvh-var(--header-h,4rem))] md:grid-cols-[minmax(0,1fr)_24rem] md:grid-rows-[minmax(0,1fr)] md:pb-0 lg:grid-cols-[minmax(0,1fr)_28rem]"
    >
      <section
        aria-label="The frame"
        class="sticky top-(--header-h,4rem) z-[5] border-b bg-background px-4 py-3 md:static md:grid md:place-items-center md:border-0 md:bg-muted/40 md:p-8"
      >
        <Frame
          page={state.page}
          wall={state.wall}
          draft={pending}
          summary={summary(state, pending)}
          changed={PICTURE_KEYS.some((k) => !same(pending[k], state.settings[k]))}
        />
      </section>
      <aside aria-label="Frame settings" class="flex min-h-0 flex-col bg-background md:border-l">
        <nav aria-label="Frame sections" class="flex border-b px-2">
          {FRAME_SECTIONS.map(([k, name, Icon]) => (
            <button
              type="button"
              aria-current={section === k ? "page" : undefined}
              onClick={() => go(k)}
              class={
                "-mb-px flex flex-1 items-center justify-center gap-1.5 border-b-2 px-2 py-3 text-sm font-medium transition-colors " +
                (section === k
                  ? "border-area text-foreground"
                  : "border-transparent text-muted-foreground hover:text-foreground")
              }
            >
              <Icon class="size-4 shrink-0 text-area" aria-hidden="true" />
              {name}
              {unsaved(k) && (
                <span
                  class="size-1.5 shrink-0 rounded-full bg-ink-yellow"
                  role="img"
                  aria-label="has unsaved changes"
                />
              )}
            </button>
          ))}
        </nav>
        <div class="relative min-h-0 flex-1 overflow-y-auto p-4 md:p-5 md:pb-24">
          <Bare.Provider value={true}>
            {section === "layout" && <Layout draft={draft} set={set} />}
            {section === "style" && <Look draft={draft} set={set} styles={state.styles} />}
            {section === "timing" && <Timing state={state} draft={draft} set={set} />}
            {section === "panel" && <Hanging draft={draft} set={set} />}
          </Bare.Provider>
        </div>
      </aside>
    </main>
  );
}

function summary(s: State, d: Settings) {
  const layout = (
    {
      collage: `Most talked about, up to ${d.tiles}`,
      single: "One picture",
      rolling: `The latest ${d.tiles}`,
    } as Record<string, string>
  )[d.layout];
  const when =
    d.mode === "diary"
      ? `painted once a day at ${d.diary_at}`
      : d.new_every_minutes > 0
        ? `a new picture at most every ${d.new_every_minutes} min`
        : "new pictures straight away";
  const quiet = d.quiet_from ? `, none ${d.quiet_from} to ${d.quiet_to}` : "";
  const turn = ROTATE.find(([v]) => v === d.rotate_minutes);
  const rotate =
    d.rotate_minutes > 0 ? `, rotating ${turn ? String(turn[1]).toLowerCase() : `every ${d.rotate_minutes} min`}` : "";
  // A topic heard once leaves at a tenth of its weight: about 3.3 half-lives.
  const gone = Math.round(d.half_life_hours * 3.3);
  const fade =
    d.layout === "collage"
      ? `. A quiet topic leaves after about ${gone >= 48 ? Math.round(gone / 24) + " days" : gone + " hours"}`
      : "";
  return `${layout}, ${when}${quiet}${rotate}${fade}. ${s.wall.length} on the frame now.`;
}

// The six inks of the panel, the brand mark.
const INKS = ["#1d1c21", "#f4f4f0", "#e2bd2f", "#b23a3a", "#2f3c7e", "#2e6a4b"];

function Brand() {
  return (
    <div class="flex items-center gap-3">
      <span
        class="rounded-md bg-neutral-900 p-[3px] shadow-sm ring-1 ring-black/20 dark:ring-white/25"
        aria-hidden="true"
      >
        <span class="grid grid-cols-3 gap-0.5 rounded-[2px] bg-[#fbfbf8] p-[3px]">
          {INKS.map((c) => (
            <i class="size-2 rounded-[1px] ring-1 ring-black/15" style={{ background: c }} />
          ))}
        </span>
      </span>
      <h1 class="text-xl font-semibold tracking-tight">E-maki</h1>
    </div>
  );
}

// Frame shows the layout the settings give: the pictures on the frame, with
// sample pictures in the places they do not fill yet, and unsaved layout
// changes applied. Before the first topic it is all sample pictures.
function Frame({
  page,
  draft,
  changed,
  wall,
  summary,
}: {
  page: string;
  draft: Settings;
  changed: boolean;
  wall: Tile[];
  summary: string;
}) {
  const empty = wall.length === 0;
  const mark = changed || empty;
  const src =
    "api/preview.png?" + new URLSearchParams([...PICTURE_KEYS.map((k) => [k, String(draft[k])]), ["v", page]]);
  const pic = usePicture(src);
  const alt = empty
    ? "Preview pictures in the chosen layout"
    : changed
      ? "The frame as it would look with your unsaved changes"
      : "The layout with sample pictures in the empty places";
  const badge = empty ? "Preview" : changed ? "Not saved yet" : "";
  const [w, h] =
    draft.panel_rotation % 180 ? [draft.panel_height, draft.panel_width] : [draft.panel_width, draft.panel_height];
  // On a phone the picture is kept to a third of the screen's height, so the settings show under it;
  // on a wide screen it is as big as fits the height under the header.
  return (
    <figure
      class="mx-auto grid w-full gap-2 max-md:max-w-[calc(30dvh*var(--ratio))] md:max-w-[calc((100dvh-var(--header-h,4rem)-9rem)*var(--ratio))]"
      style={{ "--ratio": w / h }}
      aria-busy={pic.loading}
    >
      <div class={"bezel relative " + (mark ? "outline-2 outline-offset-2 outline-ink-yellow" : "")}>
        <div class="relative" style={{ aspectRatio: aspect(draft) }}>
          {pic.prev && <img class="absolute inset-0 size-full object-contain" src={pic.prev} alt="" />}
          {pic.cur && <img key={pic.cur} class="fade-in relative size-full object-contain" src={pic.cur} alt={alt} />}
        </div>
        <span class="absolute top-1 right-1 flex items-center gap-2 sm:top-2 sm:right-2">
          <span
            class={"loader size-4 " + (pic.slow ? "" : "invisible")}
            role="status"
            aria-label={pic.slow ? "Drawing the preview" : undefined}
          />
          {badge && (
            <span class="rounded-full bg-ink-yellow px-2.5 py-0.5 text-xs font-medium text-neutral-900 shadow-sm">
              {badge}
            </span>
          )}
        </span>
      </div>
      <figcaption class="flex items-start justify-between gap-3 text-sm text-muted-foreground">
        <span>{summary}</span>
        <a
          class="shrink-0 underline underline-offset-4 hover:text-foreground max-md:hidden"
          href="view"
          target="_blank"
          rel="noopener"
        >
          Open full screen
        </a>
      </figcaption>
    </figure>
  );
}

// aspect is the page's shape as the viewer sees it, for CSS aspect-ratio.
const aspect = (d: Settings) =>
  d.panel_rotation % 180 ? `${d.panel_height} / ${d.panel_width}` : `${d.panel_width} / ${d.panel_height}`;

// usePicture loads a new picture before showing it, so the old one stays up
// until the new one is ready and then fades over it. A spinner shows only
// if drawing takes longer than a tenth of a second.
function usePicture(src: string) {
  const [pic, setPic] = useState({ cur: "", prev: "" });
  const [slow, setSlow] = useState(false);
  useEffect(() => {
    if (src === pic.cur) return;
    let on = true;
    const t = setTimeout(() => on && setSlow(true), 100);
    const im = new Image();
    const done = (ok: boolean) => {
      if (!on) return;
      clearTimeout(t);
      setSlow(false);
      if (ok) setPic((p) => ({ cur: src, prev: p.cur }));
    };
    im.onload = () => done(true);
    im.onerror = () => done(false);
    im.src = src;
    return () => {
      on = false;
      clearTimeout(t);
    };
  }, [src]);
  return { ...pic, slow, loading: src !== pic.cur };
}

// Loading holds the page's shape while the first state loads, so nothing
// jumps when it arrives. It stays blank for the first tenth of a second.
function Loading({ error }: { error: string }) {
  const [show, setShow] = useState(false);
  useEffect(() => {
    const t = setTimeout(() => setShow(true), 100);
    return () => clearTimeout(t);
  }, []);
  return (
    <>
      <header class="border-b">
        <div class="mx-auto flex max-w-6xl flex-wrap items-center gap-x-6 gap-y-3 px-4 py-3">
          <Brand />
          <span class={"skeleton order-3 h-9 w-full sm:order-none sm:w-56 " + (show ? "" : "invisible")} />
          <span class={"skeleton ms-auto h-9 w-36 " + (show ? "" : "invisible")} />
        </div>
      </header>
      {error ? (
        <p class="p-8 text-center text-muted-foreground" role="alert">
          {error}
        </p>
      ) : (
        <div
          aria-busy="true"
          aria-label="Loading"
          class={
            "mx-auto grid max-w-6xl gap-6 px-4 py-6 lg:grid-cols-[minmax(0,1.1fr)_minmax(0,1fr)] " +
            (show ? "fade-in" : "invisible")
          }
        >
          <div class="grid content-start gap-6">
            <span class="skeleton h-5 w-56" />
            <span class="skeleton h-10 w-72" />
            <div class="bezel">
              <div />
            </div>
          </div>
          <div class="grid content-start gap-6">
            <span class="skeleton h-72" />
            <span class="skeleton h-48" />
          </div>
        </div>
      )}
    </>
  );
}

const WALL_LEDE: Record<string, TipText> = {
  collage: {
    text: "The topics talked about most, the newest biggest. A topic that stops coming up fades and leaves; pin one to keep it.",
  },
  single: { text: "Only the newest topic is on the frame. Pin it to keep it there for good." },
  rolling: {
    text: "The newest topics, the newest biggest. The oldest drops off when a new one comes in; pin one to keep it.",
  },
};

function Wall({
  state,
  hot,
  onHot,
  onTile,
  onClear,
  redrawing,
  onRedraw,
}: {
  state: State;
  hot: string;
  onHot: (label: string) => void;
  onTile: (action: string, label: string, to?: string) => void;
  onClear: () => void;
  redrawing: string;
  onRedraw: (label: string) => void;
}) {
  const { wall, max_tiles: max } = state;
  const [asking, setAsking] = useState("");
  const [renaming, setRenaming] = useState("");
  return (
    <Card title="On the frame" icon={FrameIcon} tip={WALL_LEDE[state.settings.layout]}>
      {wall.length === 0 ? (
        <p class="rounded-lg border border-dashed p-4 text-center text-muted-foreground">
          No tiles yet. Topics show up here as soon as they are on the frame.
        </p>
      ) : (
        <ol class="grid gap-2">
          {wall.map((t, i) => (
            <li
              key={t.label}
              onMouseEnter={() => onHot(t.label)}
              onMouseLeave={() => onHot("")}
              class={
                "flex flex-wrap items-center gap-3 rounded-lg border p-3 transition-shadow " +
                (t.pinned ? "border-ink-yellow bg-ink-yellow/10 " : "") +
                (hot === t.label ? "ring-2 ring-primary" : "")
              }
            >
              <span
                class={
                  "grid size-7 shrink-0 place-content-center rounded-md text-xs font-semibold " +
                  (i === 0 ? "bg-primary text-primary-foreground" : "bg-muted text-muted-foreground")
                }
                aria-label={i === 0 ? "Big tile" : "Tile " + (i + 1)}
              >
                {i + 1}
              </span>
              <i
                class="h-12 w-16 shrink-0 rounded-sm bg-[#e9e1ca] ring-1 ring-black/15"
                aria-hidden="true"
                style={thumb(t.rect, state.page)}
              />
              <div class="min-w-0 flex-1 basis-40">
                {renaming === t.label ? (
                  <form
                    class="flex flex-wrap items-center gap-2"
                    onSubmit={(e) => {
                      e.preventDefault();
                      setRenaming("");
                      onTile("rename", t.label, (e.currentTarget.elements.namedItem("to") as HTMLInputElement).value);
                    }}
                    onKeyDown={(e) => e.key === "Escape" && setRenaming("")}
                  >
                    <input
                      name="to"
                      class="input min-w-0 flex-1"
                      aria-label="New caption"
                      value={t.label}
                      maxlength={120}
                      required
                      ref={focus}
                    />
                    <button type="submit" class="btn" data-size="sm">
                      Save
                    </button>
                    <button
                      type="button"
                      class="btn"
                      data-size="sm"
                      data-variant="ghost"
                      onClick={() => setRenaming("")}
                    >
                      Cancel
                    </button>
                  </form>
                ) : (
                  <p class="text-lg font-semibold leading-snug tracking-tight break-words">{t.label}</p>
                )}
                <p class="text-muted-foreground">
                  {t.pinned ? "Pinned. " : ""}
                  {"Came up "}
                  {t.mentions === 1 ? "once" : t.mentions + " times"}
                  {", last "}
                  {when(t.last_seen)}
                </p>
                <Origin t={t} />
              </div>
              {asking === t.label ? (
                <div
                  class="flex flex-wrap items-center gap-2"
                  role="group"
                  aria-label="Confirm remove"
                  onKeyDown={(e) => e.key === "Escape" && setAsking("")}
                >
                  <span>Remove “{t.label}” and keep it off the frame for a week?</span>
                  <button
                    type="button"
                    class="btn"
                    data-size="sm"
                    data-variant="destructive"
                    onClick={() => {
                      setAsking("");
                      onTile("remove", t.label);
                    }}
                  >
                    Remove
                  </button>
                  <button
                    type="button"
                    class="btn"
                    data-size="sm"
                    data-variant="ghost"
                    ref={focus}
                    onClick={() => setAsking("")}
                  >
                    Keep
                  </button>
                </div>
              ) : (
                <div class="flex flex-wrap gap-2">
                  <button
                    type="button"
                    class={"btn " + (t.pinned ? "bg-ink-yellow text-neutral-900 hover:bg-ink-yellow/80" : "")}
                    data-size="sm"
                    data-variant={t.pinned ? undefined : "outline"}
                    aria-pressed={t.pinned}
                    onClick={() => onTile(t.pinned ? "unpin" : "pin", t.label)}
                  >
                    <PinIcon /> {t.pinned ? "Pinned" : "Pin"}
                  </button>
                  <Redraw label={t.label} redrawing={redrawing} onRedraw={onRedraw} />
                  <button
                    type="button"
                    class="btn"
                    data-size="sm"
                    data-variant="ghost"
                    onClick={() => setRenaming(t.label)}
                  >
                    Rename
                  </button>
                  <button
                    type="button"
                    class="btn"
                    data-size="sm"
                    data-variant="ghost"
                    onClick={() => setAsking(t.label)}
                  >
                    Remove
                  </button>
                </div>
              )}
            </li>
          ))}
        </ol>
      )}
      <div class="flex flex-wrap items-center justify-between gap-2">
        <p class="text-muted-foreground">
          {wall.length > 0 && wall.length < max ? `Room for ${max - wall.length} more.` : ""}
        </p>
        {wall.some((t) => !t.pinned) &&
          (asking === "*" ? (
            <div
              class="flex flex-wrap items-center gap-2"
              role="group"
              aria-label="Confirm clear"
              onKeyDown={(e) => e.key === "Escape" && setAsking("")}
            >
              <span>
                {"Take "}
                {wall.filter((t) => !t.pinned).length}
                {" unpinned pictures off the frame?"}
              </span>
              <button
                type="button"
                class="btn"
                data-size="sm"
                data-variant="destructive"
                onClick={() => {
                  setAsking("");
                  onClear();
                }}
              >
                Clear the frame
              </button>
              <button
                type="button"
                class="btn"
                data-size="sm"
                data-variant="ghost"
                ref={focus}
                onClick={() => setAsking("")}
              >
                Keep
              </button>
            </div>
          ) : (
            <button type="button" class="btn" data-size="sm" data-variant="outline" onClick={() => setAsking("*")}>
              Clear the frame
            </button>
          ))}
      </div>
    </Card>
  );
}

// Redraw makes a new picture from the same description, in the current
// style. One at a time: the others wait while one is drawn.
function Redraw({
  label,
  redrawing,
  onRedraw,
}: {
  label: string;
  redrawing: string;
  onRedraw: (label: string) => void;
}) {
  const busy = redrawing === label;
  return (
    <button
      type="button"
      class="btn"
      data-size="sm"
      data-variant="outline"
      disabled={redrawing !== ""}
      aria-label={"Redraw " + label}
      onClick={() => onRedraw(label)}
    >
      {busy && <span class="loader size-3.5" aria-hidden="true" />}
      {busy ? "Redrawing…" : "Redraw"}
    </button>
  );
}

// Origin is the description the picture was made from (what the image API
// got, never what was said), with its style and when it was made.
function Origin({ t }: { t: Tile }) {
  if (!t.prompt) return null;
  return (
    <p class="text-sm text-muted-foreground">
      <span class="block italic">{t.prompt}</span>
      <span class="block">
        {t.style ? `${t.style}, ` : ""}
        {when(t.made)}
      </span>
    </p>
  );
}

// Queue lists the new pictures waiting for their turn on the frame, the next
// one first, so they can be moved up, put up now or dropped.
function Queue({
  state,
  onLine,
  redrawing,
  onRedraw,
}: {
  state: State;
  onLine: (action: string, label: string, done?: string) => void;
  redrawing: string;
  onRedraw: (label: string) => void;
}) {
  const { queue, next_at: nextAt, settings } = state;
  const [asking, setAsking] = useState("");
  if (queue.length === 0) return null;
  const why =
    settings.quiet_from && settings.new_every_minutes === 0
      ? "Quiet hours hold them back."
      : `One goes up at most every ${settings.new_every_minutes >= 60 ? settings.new_every_minutes / 60 + " h" : settings.new_every_minutes + " min"}${settings.quiet_from ? ", never in quiet hours" : ""}.`;
  return (
    <Card title="Queue" icon={ListOrdered} intro={`${why} Next one ${when(nextAt ?? "")}.`}>
      <ol class="grid gap-2">
        {queue.map((t, i) => (
          <li key={t.label} class="flex flex-wrap items-center gap-3 rounded-lg border border-dashed p-3">
            <span
              class="grid size-7 shrink-0 place-content-center rounded-md bg-muted text-xs font-semibold text-muted-foreground"
              aria-label={"Place " + (i + 1) + " in line"}
            >
              {i + 1}
            </span>
            <img
              class="h-12 w-16 shrink-0 rounded-sm object-cover ring-1 ring-black/15"
              alt=""
              loading="lazy"
              src={"api/picture.png?label=" + encodeURIComponent(t.label) + "&made=" + encodeURIComponent(t.made)}
            />
            <div class="min-w-0 flex-1 basis-40">
              <p class="text-lg font-semibold leading-snug tracking-tight break-words">{t.label}</p>
              <Origin t={t} />
            </div>
            {asking === t.label ? (
              <div
                class="flex flex-wrap items-center gap-2"
                role="group"
                aria-label="Confirm drop"
                onKeyDown={(e) => e.key === "Escape" && setAsking("")}
              >
                <span>Drop “{t.label}” and keep the topic off the frame for a week?</span>
                <button
                  type="button"
                  class="btn"
                  data-size="sm"
                  data-variant="destructive"
                  onClick={() => {
                    setAsking("");
                    onLine("drop", t.label, "Picture dropped.");
                  }}
                >
                  Drop
                </button>
                <button
                  type="button"
                  class="btn"
                  data-size="sm"
                  data-variant="ghost"
                  ref={focus}
                  onClick={() => setAsking("")}
                >
                  Keep
                </button>
              </div>
            ) : (
              <div class="flex flex-wrap gap-2">
                <button
                  type="button"
                  class="btn"
                  data-size="sm"
                  onClick={() => onLine("now", t.label, "Put up on the frame.")}
                >
                  Put up now
                </button>
                {i > 0 && (
                  <button
                    type="button"
                    class="btn"
                    data-size="sm"
                    data-variant="outline"
                    aria-label={"Move " + t.label + " up"}
                    onClick={() => onLine("up", t.label)}
                  >
                    Move up
                  </button>
                )}
                <Redraw label={t.label} redrawing={redrawing} onRedraw={onRedraw} />
                <button
                  type="button"
                  class="btn"
                  data-size="sm"
                  data-variant="ghost"
                  onClick={() => setAsking(t.label)}
                >
                  Drop
                </button>
              </div>
            )}
          </li>
        ))}
      </ol>
    </Card>
  );
}

// Timeline shows every saved picture, newest first, grouped by day in
// justified rows (Flickr's justified-layout), with a date scrubber at the
// side and a full-screen viewer (PhotoSwipe) that shows each picture's
// topic, description, style and time.
type Day = {
  key: string;
  date: Date;
  pics: Picture[];
  layout: { containerHeight: number; boxes: { left: number; top: number; width: number; height: number }[] };
  first: number;
};

function Timeline({ onState, onSignedOut }: { onState: (s: State) => void; onSignedOut: () => void }) {
  const [data, setData] = useState<TimelineData | null>(null);
  const [error, setError] = useState("");
  const [asking, setAsking] = useState(false);
  const [width, setWidth] = useState(0);
  const box = useRef<HTMLDivElement>(null);

  const fail = (e: ApiError) => (e.signin ? onSignedOut() : setError(e.message));
  const load = useCallback(
    () =>
      api<TimelineData>("api/timeline").then((d) => {
        setData(d);
        setError("");
      }, fail),
    [],
  );
  useEffect(() => {
    load();
    const t = setInterval(load, 30000);
    return () => clearInterval(t);
  }, [load]);
  useEffect(() => {
    if (!box.current) return;
    const ro = new ResizeObserver(([e]) => setWidth(Math.floor(e.contentRect.width)));
    ro.observe(box.current);
    // Day headings and the scrubber sit just under the page header, however tall it is.
    const head = document.querySelector("header");
    const ho = new ResizeObserver(() =>
      document.documentElement.style.setProperty("--head", (head as HTMLElement).offsetHeight + "px"),
    );
    if (head) ho.observe(head);
    return () => {
      ro.disconnect();
      ho.disconnect();
    };
  }, [data !== null]);

  const keep = async (on: boolean) => {
    try {
      onState(await api("api/timeline/keep", "POST", { on }));
      load();
    } catch (e) {
      fail(e);
    }
  };
  const remove = async () => {
    setAsking(false);
    try {
      setData(await api<TimelineData>("api/timeline/delete", "POST", {}));
    } catch (e) {
      fail(e);
    }
  };

  const pics = data?.pictures ?? [];
  const days: Day[] = [];
  for (const p of pics) {
    const k = new Date(p.made).toDateString();
    if (days.at(-1)?.key !== k)
      days.push({ key: k, date: new Date(p.made), pics: [], layout: { containerHeight: 0, boxes: [] }, first: 0 });
    days[days.length - 1].pics.push(p);
  }
  const row = data?.row_px || 220;
  let at = 0;
  for (const d of days) {
    d.layout = width
      ? justifiedLayout(
          d.pics.map((p) => p.w / p.h),
          {
            containerWidth: width,
            containerPadding: 0,
            boxSpacing: 4,
            targetRowHeight: width < 640 ? Math.round(row * 0.55) : row,
          },
        )
      : { boxes: [], containerHeight: 0 };
    d.first = at;
    at += d.pics.length;
  }

  const open = (index: number) => {
    const pswp = new PhotoSwipe({
      index,
      bgOpacity: 1,
      showHideAnimationType: "fade",
      loop: false,
      dataSource: pics.map((p) => ({
        src: "api/timeline/" + p.id,
        msrc: "api/timeline/" + p.id + "?size=thumb",
        width: p.w,
        height: p.h,
        alt: p.label,
        pic: p,
      })),
    });
    pswp.on("uiRegister", () =>
      pswp.ui?.registerElement({
        name: "info",
        appendTo: "root",
        order: 9,
        onInit: (el: HTMLElement) => {
          el.className = "pswp-info";
          const show = () => {
            const p = pswp.currSlide?.data.pic as Picture | undefined;
            if (!p) return;
            const line = (tag: string, cls: string, text: string) =>
              Object.assign(document.createElement(tag), { className: cls, textContent: text });
            el.replaceChildren(
              line("p", "font-semibold text-lg", p.label),
              line("p", "italic", p.prompt),
              line("p", "opacity-75", [p.style, longDate(new Date(p.made), true)].filter(Boolean).join(", ")),
            );
          };
          pswp.on("change", show);
          show();
        },
      }),
    );
    pswp.init();
  };

  return (
    <main
      id="panel-pictures"
      // @ts-expect-error: ARIA in HTML allows <main> no role but "main"; kept as it was.
      role="tabpanel"
      aria-labelledby="tab-pictures"
      class="mx-auto grid max-w-6xl gap-4 pt-4 pb-6 ps-4 pe-10 sm:pe-20"
    >
      <div class="flex flex-wrap items-end justify-between gap-3">
        <div class="grid gap-1">
          <h2 class="text-lg font-semibold tracking-tight">History</h2>
          <p class="text-muted-foreground">
            {pics.length === 1 ? "1 picture" : pics.length + " pictures"}
            {days.length > 1 ? ` over ${days.length} days` : ""}
          </p>
        </div>
        {data?.saved && (
          <div class="flex flex-wrap items-center gap-3">
            <label class="flex items-center gap-2 text-sm font-medium">
              <input
                type="checkbox"
                role="switch"
                class="input"
                checked={data.on}
                onChange={(e) => keep(e.currentTarget.checked)}
              />
              Save new pictures
              <Tip tip={TIPS.timeline} name="Save new pictures" />
            </label>
            {pics.length > 0 &&
              (asking ? (
                <div
                  class="flex flex-wrap items-center gap-2"
                  role="group"
                  aria-label="Confirm delete"
                  onKeyDown={(e) => e.key === "Escape" && setAsking(false)}
                >
                  <span>
                    {"Delete all "}
                    {pics.length}
                    {" saved pictures?"}
                  </span>
                  <button type="button" class="btn" data-size="sm" data-variant="destructive" onClick={remove}>
                    Delete all
                  </button>
                  <button
                    type="button"
                    class="btn"
                    data-size="sm"
                    data-variant="ghost"
                    ref={focus}
                    onClick={() => setAsking(false)}
                  >
                    Keep
                  </button>
                </div>
              ) : (
                <button type="button" class="btn" data-size="sm" data-variant="outline" onClick={() => setAsking(true)}>
                  Delete all
                </button>
              ))}
          </div>
        )}
      </div>
      {error && (
        <div class="alert" data-variant="destructive" role="alert">
          <section>{error}</section>
        </div>
      )}
      {data && !data.saved && <Note>The timeline needs a settings file: start E-maki with -settings.</Note>}
      {data?.saved && pics.length === 0 && (
        <p class="rounded-lg border border-dashed p-8 text-center text-muted-foreground">
          {data.on ? "No pictures yet. Each new picture lands here, day by day." : "Saving is off."}
        </p>
      )}
      <div ref={box} class="grid gap-8">
        {days.map((d) => (
          <section key={d.key} data-day={d.key} aria-label={longDate(d.date)}>
            <h3 class="sticky top-(--head) z-[5] -mx-1 mb-2 bg-background/90 px-1 py-1 font-semibold backdrop-blur-sm">
              {longDate(d.date)}
              <span class="ms-2 text-sm font-normal text-muted-foreground">{d.pics.length}</span>
            </h3>
            <div class="relative" style={{ height: d.layout.containerHeight + "px" }}>
              {d.layout.boxes.map((b, i) => {
                const p = d.pics[i];
                return (
                  <button
                    type="button"
                    key={p.id}
                    data-made={p.made}
                    onClick={() => open(d.first + i)}
                    class="absolute overflow-hidden rounded-sm bg-muted focus-visible:ring-2 focus-visible:ring-ring focus-visible:outline-none"
                    style={{ left: b.left + "px", top: b.top + "px", width: b.width + "px", height: b.height + "px" }}
                    aria-label={`${p.label}, ${new Date(p.made).toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" })}`}
                  >
                    <img
                      class="size-full object-cover transition-transform duration-200 hover:scale-[1.03]"
                      src={"api/timeline/" + p.id + "?size=thumb"}
                      alt=""
                      loading="lazy"
                      decoding="async"
                    />
                  </button>
                );
              })}
            </div>
          </section>
        ))}
      </div>
      {pics.length > 0 && <Scrubber count={pics.length} box={box} width={width} />}
    </main>
  );
}

const headHeight = () => document.querySelector("header")?.offsetHeight ?? 0;

function longDate(d: Date, time?: boolean) {
  const now = new Date();
  const yesterday = new Date(now.getFullYear(), now.getMonth(), now.getDate() - 1);
  let day =
    d.toDateString() === now.toDateString()
      ? "Today"
      : d.toDateString() === yesterday.toDateString()
        ? "Yesterday"
        : d.toLocaleDateString([], {
            weekday: "long",
            day: "numeric",
            month: "long",
            year: d.getFullYear() === now.getFullYear() ? undefined : "numeric",
          });
  if (time)
    day =
      day.replace(/^(Today|Yesterday)$/, (w) => w.toLowerCase()) +
      " at " +
      d.toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" });
  return day;
}

// Scrubber is the date bar beside the timeline, as in Immich: drag or click
// it to jump to a day; its marks sit where their days are in the page, and
// pointing at it names the day and time of the pictures there.
function Scrubber({ count, box, width }: { count: number; box: RefObject<HTMLDivElement | null>; width: number }) {
  const bar = useRef<HTMLElement>(null);
  const [marks, setMarks] = useState<{ key: string; at: number; short: string; long: string }[]>([]);
  const [times, setTimes] = useState<{ at: number; long: string }[]>([]); // where each picture starts, with when it was made
  const [pos, setPos] = useState(0);
  const [hover, setHover] = useState<[number, string | undefined] | null>(null); // [fraction, day label] while pointing
  const span = () => {
    const r = (box.current as HTMLDivElement).getBoundingClientRect();
    return { top: r.top + scrollY - headHeight(), height: Math.max(1, r.height) };
  };
  useEffect(() => {
    const measure = () => {
      if (!box.current) return;
      const { top, height } = span();
      const at = (el: Element) => (el.getBoundingClientRect().top + scrollY - headHeight() - top) / height;
      setTimes(
        [...box.current.querySelectorAll<HTMLElement>("[data-made]")]
          .map((el) => ({ at: at(el), long: longDate(new Date(el.dataset.made as string), true) }))
          .sort((a, b) => a.at - b.at),
      );
      setMarks(
        [...box.current.querySelectorAll<HTMLElement>("[data-day]")].map((el) => {
          const day = el.dataset.day as string;
          return {
            key: day,
            at: at(el),
            short: new Date(day).toLocaleDateString([], { day: "numeric", month: "short" }),
            long: longDate(new Date(day)),
          };
        }),
      );
    };
    const scrolled = () => {
      const { top, height } = span();
      setPos(Math.min(1, Math.max(0, (scrollY - top) / height)));
    };
    measure();
    scrolled();
    addEventListener("scroll", scrolled, { passive: true });
    return () => removeEventListener("scroll", scrolled);
  }, [count, width]);
  const frac = (e: PointerEvent) => {
    const r = (bar.current as HTMLElement).getBoundingClientRect();
    return Math.min(1, Math.max(0, (e.clientY - r.top) / r.height));
  };
  const timeAt = (f: number) => times.filter((m) => m.at <= f + 0.001).at(-1) ?? times[0];
  const go = (f: number) => {
    const { top, height } = span();
    scrollTo({ top: top + f * height });
  };
  const move = (e: PointerEvent) => {
    const f = frac(e);
    setHover([f, timeAt(f)?.long]);
    if (e.buttons) go(f);
  };
  // Marks closer than 1.5rem to the one above are left out, so labels never overlap.
  const h = bar.current?.getBoundingClientRect().height ?? 0;
  let last = -Infinity;
  const shown = marks.filter((m) => {
    if (m.at * h - last < 24) return false;
    last = m.at * h;
    return true;
  });
  return (
    <nav
      aria-label="Jump to a day"
      ref={bar}
      class="fixed top-[calc(var(--head)+1rem)] right-0 bottom-24 z-[6] w-9 cursor-ns-resize sm:right-1 sm:w-16 touch-none select-none sm:bottom-6"
      onPointerDown={(e) => {
        e.currentTarget.setPointerCapture(e.pointerId);
        go(frac(e));
      }}
      onPointerMove={move}
      onPointerLeave={() => setHover(null)}
    >
      {shown.map((m) => (
        <button
          type="button"
          key={m.key}
          tabindex={-1}
          aria-label={m.long}
          class="absolute right-1 hidden text-xs tabular-nums sm:block text-muted-foreground hover:text-foreground"
          style={{ top: m.at * 100 + "%" }}
          onClick={() => go(m.at)}
        >
          {m.short}
        </button>
      ))}
      <span aria-hidden="true" class="absolute inset-y-0 right-1 w-px bg-border" />
      <span aria-hidden="true" class="absolute right-0 h-0.5 w-8 rounded bg-primary" style={{ top: pos * 100 + "%" }} />
      {hover && (
        <span
          aria-hidden="true"
          class="absolute right-full me-1 -translate-y-1/2 whitespace-nowrap rounded-md bg-popover px-2 py-1 text-sm shadow ring-1 ring-border"
          style={{ top: hover[0] * 100 + "%" }}
        >
          {hover[1]}
        </span>
      )}
    </nav>
  );
}

// Removed lists the topics taken off the frame that are kept away for a
// while, so one removed by mistake can come back.
function Removed({ removed, onAllow }: { removed: State["removed"]; onAllow: (label: string) => void }) {
  if (!removed?.length) return null;
  return (
    <Card title="Removed topics" icon={Ban} tip={TIPS.removed}>
      <ul class="grid gap-2">
        {removed.map((r) => (
          <li key={r.label} class="flex flex-wrap items-center gap-3 rounded-lg border p-3">
            <div class="min-w-0 flex-1 basis-40">
              <p class="font-medium break-words">{r.label}</p>
              <p class="text-sm text-muted-foreground">
                {"Kept away until "}
                {new Date(r.until).toLocaleDateString(undefined, { weekday: "short", day: "numeric", month: "short" })}
              </p>
            </div>
            <button type="button" class="btn" data-size="sm" data-variant="outline" onClick={() => onAllow(r.label)}>
              Allow again
            </button>
          </li>
        ))}
      </ul>
    </Card>
  );
}

// Logs shows E-maki's own recent log, newest at the bottom, from memory.
type Line = { n: number; text: string };

function Logs() {
  const [lines, setLines] = useState<Line[]>([]);
  const [filter, setFilter] = useState("");
  const [follow, setFollow] = useState(true);
  const [error, setError] = useState("");
  const [copied, setCopied] = useState(false);
  const box = useRef<HTMLDivElement>(null);
  useEffect(() => {
    let on = true,
      last = 0;
    const get = async () => {
      try {
        const add = await api<Line[]>("api/logs?after=" + last);
        if (!on || add.length === 0) return;
        last = add[add.length - 1].n;
        setLines((l) => [...l, ...add].slice(-1000));
        setError("");
      } catch (e) {
        if (on) setError(e.message);
      }
    };
    get();
    const t = setInterval(get, 2000);
    return () => {
      on = false;
      clearInterval(t);
    };
  }, []);
  useEffect(() => {
    if (follow && box.current) box.current.scrollTop = box.current.scrollHeight;
  }, [lines, follow, filter]);
  const f = filter.trim().toLowerCase();
  const shown = f ? lines.filter((l) => l.text.toLowerCase().includes(f)) : lines;
  const tone = (t: string) =>
    /error|fail|can't|cannot|refused|warning/i.test(t)
      ? "text-destructive"
      : /rejected|held back|dropped|skipped/i.test(t)
        ? "text-muted-foreground"
        : "";
  return (
    <section class="grid min-w-0 gap-4" aria-label="Logs">
      <div class="flex flex-wrap items-end justify-between gap-3">
        <div class="grid gap-1">
          <h2 class="text-lg font-semibold tracking-tight">Logs</h2>
          <p class="max-w-prose text-muted-foreground">
            {"The last "}
            {lines.length}
            {
              " lines E-maki wrote, kept in memory only. Loudness, timings, topics and picture descriptions, never what was said."
            }
          </p>
        </div>
        <div class="flex flex-wrap items-center gap-3">
          <label class="flex items-center gap-2 text-sm font-medium">
            Filter
            <input
              class="input w-56"
              type="search"
              placeholder="topic, microphone"
              value={filter}
              onInput={(e) => setFilter(e.currentTarget.value)}
            />
          </label>
          <label class="flex items-center gap-2 text-sm">
            <input
              type="checkbox"
              role="switch"
              class="input"
              checked={follow}
              onChange={(e) => setFollow(e.currentTarget.checked)}
            />
            Follow new lines
          </label>
          {navigator.clipboard && (
            <button
              type="button"
              class="btn"
              data-size="sm"
              data-variant="outline"
              disabled={shown.length === 0}
              onClick={async () => {
                try {
                  await navigator.clipboard.writeText(shown.map((l) => l.text).join("\n") + "\n");
                  setCopied(true);
                  setTimeout(() => setCopied(false), 2000);
                } catch (e) {
                  setError("Can't copy: " + e.message);
                }
              }}
            >
              {copied ? "Copied" : "Copy"}
            </button>
          )}
          <a class="btn" data-size="sm" data-variant="outline" href="api/logs.txt" download>
            Download
          </a>
        </div>
      </div>
      {error && (
        <div class="alert" data-variant="destructive" role="alert">
          <section>{error}</section>
        </div>
      )}
      <div
        ref={box}
        class="h-[65dvh] overflow-auto rounded-xl border bg-muted/40 p-3 font-mono text-sm leading-relaxed"
        role="log"
        aria-live="polite"
      >
        {shown.length === 0 ? (
          <p class="p-4 text-center text-muted-foreground">
            {lines.length === 0 ? "Nothing logged yet." : "No line matches the filter."}
          </p>
        ) : (
          shown.map((l) => (
            <div key={l.n} class={"whitespace-pre-wrap break-words " + tone(l.text)}>
              {l.text}
            </div>
          ))
        )}
      </div>
    </section>
  );
}

// thumb crops a tile out of the frame's picture, as a CSS background.
function thumb([x, y, w, h]: Tile["rect"], page: string) {
  const at = (pos: number, size: number) => (size >= 1 ? 0 : (pos / (1 - size)) * 100) + "%";
  return {
    backgroundImage: `url(preview.png?v=${encodeURIComponent(page)})`,
    backgroundSize: `${100 / w}% ${100 / h}%`,
    backgroundPosition: `${at(x, w)} ${at(y, h)}`,
  };
}

function PinIcon() {
  return (
    <svg viewBox="0 0 16 16" width="14" height="14" aria-hidden="true">
      <path d="M5 1.5h6l-1 4.5 2.5 2.5V10h-9V8.5L6 6zM7.4 10h1.2v4l-.6 1.2-.6-1.2z" fill="currentColor" />
    </svg>
  );
}

// Tip is the small "?" after a setting's name: what the setting does and
// where else it is set. It opens on hover, on keyboard focus and on a tap,
// and Escape or a tap elsewhere closes it. It sits inside the setting's
// <label>, so it is a focusable span rather than a button (a button would
// take the label from the input) and keeps its clicks to itself.
function Tip({ tip, name }: { tip: TipText; name: string }) {
  const id = useId();
  const box = useRef<HTMLSpanElement>(null);
  const [hover, setHover] = useState(false);
  const [open, setOpen] = useState(false);
  const pop = useRef<HTMLSpanElement>(null);
  const shown = hover || open;
  const close = () => {
    setOpen(false);
    setHover(false);
  };
  // The box is fixed to the window, so a card's rounded corners can't clip
  // it: under the "i", or above it near the bottom, and inside the edges.
  const place = () => {
    const el = pop.current;
    const at = box.current?.getBoundingClientRect();
    if (!el || !at) return;
    const { width, height } = el.getBoundingClientRect();
    const vw = document.documentElement.clientWidth;
    const below = at.bottom + 6 + height <= window.innerHeight - 8;
    el.style.left = Math.max(16, Math.min(at.left, vw - 16 - width)) + "px";
    el.style.top = (below ? at.bottom + 6 : Math.max(8, at.top - 6 - height)) + "px";
  };
  useLayoutEffect(() => {
    if (!shown) return;
    place();
    const away = (e: PointerEvent) => !box.current?.contains(e.target as Node) && setOpen(false);
    document.addEventListener("pointerdown", away);
    window.addEventListener("scroll", place, { capture: true, passive: true });
    window.addEventListener("resize", place);
    return () => {
      document.removeEventListener("pointerdown", away);
      window.removeEventListener("scroll", place, { capture: true });
      window.removeEventListener("resize", place);
    };
  }, [shown]);
  return (
    // The click handler only stops a click on the tip from reaching the
    // label's input; the trigger inside does the work and has its keys.
    // biome-ignore lint/a11y/noStaticElementInteractions: see above
    // biome-ignore lint/a11y/useKeyWithClickEvents: see above
    <span
      ref={box}
      class="relative ms-1.5 inline-flex align-[-2px] font-normal"
      onPointerEnter={(e) => e.pointerType === "mouse" && setHover(true)}
      onPointerLeave={() => setHover(false)}
      onClick={(e) => e.preventDefault()}
    >
      <span
        role="button"
        tabIndex={0}
        aria-label={"About " + name}
        aria-describedby={id}
        class="cursor-help rounded-full text-muted-foreground hover:text-foreground focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring"
        onClick={(e) => {
          e.stopPropagation();
          e.preventDefault();
          setOpen((o) => !o);
        }}
        onFocus={(e) => e.currentTarget.matches(":focus-visible") && setOpen(true)}
        onBlur={() => setOpen(false)}
        onKeyDown={(e) => {
          if (e.key === "Escape") close();
          if (e.key === "Enter" || e.key === " ") {
            e.preventDefault();
            setOpen((o) => !o);
          }
        }}
      >
        <CircleQuestionMark class="size-[15px]" aria-hidden="true" />
      </span>
      <span
        id={id}
        ref={pop}
        role="tooltip"
        class={
          "fixed z-50 w-max max-w-[min(20rem,calc(100vw-2rem))] rounded-md border bg-popover px-3 py-2 text-left text-sm leading-snug tracking-normal text-popover-foreground shadow-md " +
          (shown ? "" : "hidden")
        }
      >
        {tip.text}
        {tip.where && <span class="mt-1.5 block text-xs text-muted-foreground">{tip.where}</span>}
      </span>
    </span>
  );
}

// Card is a group of settings under a title with an icon in its tab's
// colour. The intro line is for what is true right now; help goes in tip.
// Inside the Frame tab's side panel the tab already names the card, so the
// card drops its frame and title and keeps only its help.
const Bare = createContext(false);

function Card({
  title,
  icon: Icon,
  intro,
  tip,
  children,
}: {
  title: string;
  icon?: LucideIcon;
  intro?: ComponentChildren;
  tip?: TipText;
  children: ComponentChildren;
}) {
  if (useContext(Bare))
    return (
      <section class="grid gap-5" aria-label={title}>
        {tip && (
          <span class="-mb-3 font-medium">
            {title}
            <Tip tip={tip} name={title} />
          </span>
        )}
        {intro && <p class="text-sm text-muted-foreground">{intro}</p>}
        {children}
      </section>
    );
  return (
    <section class="card">
      <header>
        <h2 class="flex items-center gap-2.5 text-lg font-semibold tracking-tight">
          {Icon && (
            <span class="grid size-8 shrink-0 place-items-center rounded-lg bg-area/12 text-area">
              <Icon class="size-[18px]" aria-hidden="true" />
            </span>
          )}
          <span>
            {title}
            {tip && <Tip tip={tip} name={title} />}
          </span>
        </h2>
        {intro && <p>{intro}</p>}
      </header>
      <section class="grid gap-5">{children}</section>
    </section>
  );
}

// Field is a settings row: the label and its help on the left, the control
// (and its value) on the right. On a phone, or with stack, they stack.
function Field({
  label,
  value,
  help,
  stack,
  tip,
  children,
}: {
  label: string;
  value?: string;
  help?: ComponentChildren;
  stack?: boolean;
  tip?: TipText;
  children: ComponentChildren;
}) {
  return (
    <label
      class={"grid gap-2 " + (stack ? "" : "sm:grid-cols-[minmax(0,2fr)_minmax(0,3fr)] sm:items-center sm:gap-x-8")}
    >
      <span class="grid gap-0.5">
        <span class="font-medium">
          {label}
          {tip && <Tip tip={tip} name={label} />}
        </span>
        {help && <small class="text-muted-foreground">{help}</small>}
      </span>
      <span class="flex items-center gap-3">
        <span class="min-w-0 flex-1">{children}</span>
        {value !== undefined && <span class="shrink-0 text-sm tabular-nums text-muted-foreground">{value}</span>}
      </span>
    </label>
  );
}

// Range is a slider whose filled part follows its value (Basecoat reads --slider-value).
function Range(props: {
  min: string;
  max: string;
  step: string;
  value: number;
  onInput: (e: TargetedEvent<HTMLInputElement>) => void;
}) {
  const { min, max, value } = props;
  return (
    <input
      class="input w-full"
      type="range"
      {...props}
      style={{ "--slider-value": ((value - +min) / (+max - +min)) * 100 + "%" }}
    />
  );
}

// Toggle is an on/off switch with a title and an explanation.
function Toggle({
  checked,
  onChange,
  title,
  text,
  disabled,
  tip,
}: {
  checked: boolean;
  onChange: (on: boolean) => void;
  title: string;
  text?: string;
  disabled?: boolean;
  tip?: TipText;
}) {
  return (
    <label class="flex items-start justify-between gap-4">
      <span class="grid gap-0.5">
        <span class="font-medium">
          {title}
          {tip && <Tip tip={tip} name={title} />}
        </span>
        {text && <small class="text-muted-foreground">{text}</small>}
      </span>
      <input
        type="checkbox"
        role="switch"
        class="input mt-0.5"
        checked={checked}
        disabled={disabled}
        onChange={(e) => onChange(e.currentTarget.checked)}
      />
    </label>
  );
}

// Choice is one option in a group of big radio cards.
function Choice({
  name,
  checked,
  onChange,
  children,
}: {
  name: string;
  checked: boolean;
  onChange: () => void;
  children: ComponentChildren;
}) {
  return (
    <label class="flex cursor-pointer gap-3 rounded-lg border p-3 transition-colors hover:bg-accent has-checked:border-primary has-checked:bg-primary/5 has-checked:ring-1 has-checked:ring-primary has-focus-visible:ring-2 has-focus-visible:ring-ring">
      <input type="radio" name={name} class="sr-only" checked={checked} onChange={onChange} />
      {children}
    </label>
  );
}

function Segments<T>({
  name,
  options,
  value,
  onChange,
  label,
  inline,
  tip,
}: {
  name: string;
  options: [T, string][];
  value: T;
  onChange: (v: T) => void;
  label: string;
  inline?: boolean;
  tip?: TipText;
}) {
  return (
    <div class="grid gap-2" role="radiogroup" aria-label={label}>
      {!inline && (
        <span class="font-medium">
          {label}
          {tip && <Tip tip={tip} name={label} />}
        </span>
      )}
      <div class="inline-flex w-fit rounded-lg bg-muted p-1">
        {options.map(([v, text]) => (
          <label class="tap cursor-pointer rounded-md px-3 py-1.5 text-sm sm:px-4 font-medium text-muted-foreground has-checked:bg-background has-checked:text-foreground has-checked:shadow-sm has-focus-visible:ring-2 has-focus-visible:ring-ring">
            <input type="radio" name={name} class="sr-only" checked={value === v} onChange={() => onChange(v)} />
            {text}
          </label>
        ))}
      </div>
    </div>
  );
}

function Note({ tone, children }: { tone?: string; children: ComponentChildren }) {
  const color =
    (
      { ok: "border-ink-green/40 bg-ink-green/10", warn: "border-ink-yellow/60 bg-ink-yellow/15" } as Record<
        string,
        string
      >
    )[tone ?? ""] || "bg-muted/50";
  return (
    <p class={"rounded-lg border px-3 py-2 " + color} role={tone ? "status" : undefined}>
      {children}
    </p>
  );
}

// Stays marks a choice that keeps everything on this machine.
function Stays() {
  return (
    <span class="flex items-center gap-1.5 text-ink-green">
      <ShieldCheck class="size-3.5 shrink-0" aria-hidden="true" />
      Stays on this machine
    </span>
  );
}

const LISTENING: [string, string, LucideIcon][] = [
  ["listening", "Microphone", MicIcon],
  ["topics", "Topics", MessagesSquare],
  ["privacy", "Privacy", ShieldCheck],
];
const SYSTEM: [string, string, LucideIcon][] = [
  ["status", "Status", Activity],
  ["pictures", "Image provider", ImagePlus],
  ["storage", "Storage", HardDrive],
  ["security", "Security", KeyRound],
  ["logs", "Logs", ScrollText],
];

function PicturesSwitch({ view, setView }: { view: string; setView: (v: string) => void }) {
  return (
    <div class="mx-auto max-w-6xl px-4 pt-6">
      <Segments
        inline
        name="view"
        label="Pictures"
        value={view}
        onChange={setView}
        options={[
          ["now", "On the frame"],
          ["history", "History"],
        ]}
      />
    </div>
  );
}

// SectionTab is a tab of settings with a list of its sections at the side
// (a scrolling row on a phone), one section shown at a time.
function SectionTab({
  id,
  sections,
  state,
  draft,
  set,
  paused,
  onState,
  onSignedOut,
}: StateProps & {
  id: string;
  sections: [string, string, LucideIcon][];
  paused: boolean;
  onState: (s: State) => void;
  onSignedOut: () => void;
}) {
  const initial = sections.find(([k]) => location.hash === "#" + id + "/" + k)?.[0] ?? sections[0][0];
  const [section, setSection] = useState(initial);
  const go = (k: string) => {
    setSection(k);
    history.replaceState(null, "", "#" + id + "/" + k);
  };
  // On a phone the list is one scrolling row: keep the open section in view.
  const nav = useRef<HTMLElement>(null);
  useEffect(
    () => nav.current?.querySelector("[aria-current]")?.scrollIntoView({ block: "nearest", inline: "nearest" }),
    [section],
  );
  const unsaved = (k: string) => (SECTION_KEYS[k] || []).some((key) => !same(draft[key], state.settings[key]));
  return (
    <main
      id={"panel-" + id}
      // @ts-expect-error: ARIA in HTML allows <main> no role but "main"; kept as it was.
      role="tabpanel"
      aria-labelledby={"tab-" + id}
      data-area={id}
      class="mx-auto grid max-w-6xl gap-6 px-4 py-6 md:grid-cols-[15rem_minmax(0,1fr)] md:items-start"
    >
      <nav
        ref={nav}
        aria-label={id[0].toUpperCase() + id.slice(1) + " sections"}
        class="-mx-4 flex gap-1 overflow-x-auto px-4 md:sticky md:top-20 md:mx-0 md:flex-col md:overflow-visible md:px-0"
      >
        {sections.map(([k, name, Icon]) => (
          <button
            type="button"
            aria-current={section === k ? "page" : undefined}
            onClick={() => go(k)}
            class={
              "flex shrink-0 items-center gap-2.5 rounded-lg px-3 py-2 text-left text-sm font-medium transition-colors " +
              (section === k ? "bg-area/12" : "hover:bg-accent")
            }
          >
            <Icon class="size-4 shrink-0 text-area" aria-hidden="true" />
            {name}
            {unsaved(k) && (
              <span class="size-1.5 shrink-0 rounded-full bg-ink-yellow" role="img" aria-label="has unsaved changes" />
            )}
          </button>
        ))}
      </nav>
      <div class={"grid gap-6 " + (section === "logs" ? "min-w-0" : "max-w-3xl")}>
        {section === "listening" && <Listening state={state} draft={draft} set={set} paused={paused} />}
        {section === "topics" && <Topics state={state} draft={draft} set={set} />}
        {section === "privacy" && <Privacy draft={draft} set={set} />}
        {section === "pictures" && <Pictures state={state} draft={draft} set={set} />}
        {section === "storage" && (
          <Storage draft={draft} set={set} saved={state.status.saved} topicAPI={state.status.topic_api} />
        )}
        {section === "logs" && <Logs />}
        {section === "security" && <Security state={state} onState={onState} onSignedOut={onSignedOut} />}
        {section === "status" && <System state={state} />}
      </div>
    </main>
  );
}

function SignIn({ onDone }: { onDone: () => void }) {
  const [password, setPassword] = useState("");
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  const submit = async (e: Event) => {
    e.preventDefault();
    setBusy(true);
    try {
      await api("api/login", "POST", { password });
      onDone();
    } catch (err) {
      setError(err.message);
    }
    setBusy(false);
  };
  return (
    <main class="grid min-h-dvh place-items-center p-4">
      <form class="card w-full max-w-sm" onSubmit={submit}>
        <header>
          <Brand />
          <p>This page is locked with a password. The frame keeps showing its picture either way.</p>
        </header>
        <section class="grid gap-4">
          <Field label="Password">
            <input
              class="input"
              type="password"
              autocomplete="current-password"
              autofocus
              value={password}
              onInput={(e) => setPassword(e.currentTarget.value)}
            />
          </Field>
          {error && (
            <div class="alert" data-variant="destructive" role="alert">
              <section>{error}</section>
            </div>
          )}
          <button class="btn w-full" type="submit" disabled={busy || !password}>
            {busy ? "Signing in…" : "Sign in"}
          </button>
          <p class="text-muted-foreground">
            {"Forgot it? Delete "}
            <code>password.json</code>
            {" next to the settings file (in docker: the "}
            <code>emaki-data</code>
            {" volume) and restart E-maki."}
          </p>
        </section>
      </form>
    </main>
  );
}

function Listening({ state, draft, set, paused }: StateProps & { paused: boolean }) {
  const a = state.activity;
  // Go sends the zero time (year 1) until a first segment is heard.
  const level = a.last_at && !a.last_at.startsWith("0001-") ? a.last_level : null;
  return (
    <>
      <Microphone draft={draft} set={set} paused={paused} />
      <Card title="Speech threshold" icon={Volume2} tip={TIPS.speech_threshold}>
        <LevelMeter level={level} threshold={draft.min_speech_dbfs} />
        <Field
          label="Threshold"
          tip={TIPS.min_speech_dbfs}
          value={draft.min_speech_dbfs + " dB"}
          help={
            level === null
              ? "Nothing heard yet"
              : `Last heard: ${Math.round(level)} dB, ${when(a.last_at)}${a.last_lang ? `, in ${langName(a.last_lang)}` : ""}`
          }
        >
          <Range
            min="-80"
            max="-10"
            step="1"
            value={draft.min_speech_dbfs}
            onInput={(e) => set({ min_speech_dbfs: +e.currentTarget.value })}
          />
        </Field>
      </Card>
      <Languages state={state} draft={draft} set={set} />
    </>
  );
}

// Languages lists the ones ticked and those with their own speech server;
// any other language Whisper knows can be added from the list.
function Languages({ state, draft, set }: StateProps) {
  const on = draft.languages;
  const toggle = (l: string) => set({ languages: on.includes(l) ? on.filter((x) => x !== l) : [...on, l] });
  const own = state.status.by_lang || {};
  const name = (code: string) => {
    const n = langName(code);
    return n === code ? (state.languages.find(([c]) => c === code)?.[1] ?? code) : n;
  };
  const rows = [...on, ...Object.keys(own).filter((l) => !on.includes(l))];
  const rest = state.languages
    .map(([c]) => c)
    .filter((c) => !rows.includes(c))
    .sort((x, y) => name(x).localeCompare(name(y)));
  return (
    <Card title="Languages" icon={LanguagesIcon} tip={TIPS.languages}>
      {rows.map((l) => (
        <Toggle
          checked={on.includes(l)}
          onChange={() => toggle(l)}
          title={name(l)}
          text={own[l] ? "Own speech model" : ""}
        />
      ))}
      <select
        class="select"
        value=""
        aria-label="Add a language"
        onChange={(e) => e.currentTarget.value && toggle(e.currentTarget.value)}
      >
        <option value="">Add a language</option>
        {rest.map((c) => (
          <option value={c}>{name(c)}</option>
        ))}
      </select>
    </Card>
  );
}

// Microphone lists the inputs the machine offers, marks the one E-maki
// listens on and shows its loudness live. Only names and a level come from
// the server, never audio.
function Microphone({ draft, set, paused }: DraftProps & { paused: boolean }) {
  const [mic, setMic] = useState<Mic | null>(null);
  useEffect(() => {
    let on = true;
    const get = () =>
      api<Mic>("api/mic")
        .then((m) => on && setMic(m))
        .catch(() => {});
    get();
    const t = setInterval(get, 1000);
    return () => {
      on = false;
      clearInterval(t);
    };
  }, []);
  if (!mic) {
    return (
      <Card title="Microphone" icon={MicIcon} tip={TIPS.mic_device}>
        <div class="grid gap-2" aria-busy="true" aria-label="Looking for inputs">
          <span class="skeleton h-16" />
          <span class="skeleton h-16" />
        </div>
      </Card>
    );
  }
  const pick = draft.mic_device;
  const fallback = mic.devices.find((d) => d.default);
  const rows: [string, string, string][] = [
    ["", "System default", fallback ? fallback.name : "Whatever the system picks"],
    ...mic.devices.map((d): [string, string, string] => [
      d.name,
      d.name,
      d.default ? "The system default right now" : "",
    ]),
  ];
  const missing = pick !== "" && !mic.devices.some((d) => d.name === pick);
  if (missing) rows.push([pick, pick, "Not connected. E-maki uses the default until it is back."]);
  // The row that is listening: the picked one if it is open, else the default's own row.
  const inUse = (v: string) =>
    mic.live &&
    mic.active !== "" &&
    (v === mic.active || (v === "" && pick === "" && !mic.devices.some((d) => d.name === mic.active)));
  const pos = (db: number) => Math.max(0, Math.min(100, ((db + 80) / 80) * 100));
  return (
    <Card title="Microphone" icon={MicIcon} tip={TIPS.mic_device}>
      {!mic.live && <Note>Playing a recording, not a mic. Your pick is used next time.</Note>}
      {mic.error && <Note tone="warn">{mic.error}</Note>}
      {mic.devices.length === 0 && !mic.error && (
        <Note tone="warn">No inputs found. In docker, check that the sound server is passed into the container.</Note>
      )}
      <div class="grid gap-2" role="radiogroup" aria-label="Microphone">
        {rows.map(([v, name, text]) => (
          <Choice name="mic" checked={pick === v} onChange={() => set({ mic_device: v })}>
            <span class="grid min-w-0 flex-1 gap-1">
              <span class="flex flex-wrap items-center gap-2 font-medium">
                <span class="break-words">{name}</span>
                {inUse(v) && (
                  <span class="rounded-full bg-ink-green px-2 py-0.5 text-xs text-white dark:text-neutral-900">
                    {paused ? "In use, paused" : "Listening"}
                  </span>
                )}
              </span>
              {text && (
                <small class={missing && v === pick ? "text-destructive" : "text-muted-foreground"}>{text}</small>
              )}
              {inUse(v) && mic.level != null && (
                <span
                  class="relative mt-1 block h-2 rounded-full bg-muted"
                  role="img"
                  aria-label={`Level now ${Math.round(mic.level)} dB`}
                >
                  <i
                    class="absolute inset-0 origin-left rounded-full bg-ink-green transition-transform duration-200"
                    style={{ transform: `scaleX(${pos(mic.level) / 100})` }}
                  />
                  <b
                    class="absolute -inset-y-1 w-0.5 rounded bg-primary"
                    style={{ left: pos(draft.min_speech_dbfs) + "%" }}
                  />
                </span>
              )}
            </span>
          </Choice>
        ))}
      </div>
    </Card>
  );
}

function langName(code: string) {
  try {
    return new Intl.DisplayNames(["en"], { type: "language" }).of(code) ?? code;
  } catch {
    return code;
  }
}

// LevelMeter shows the last segment's loudness against the speech line, on a -80 to 0 dBFS scale.
function LevelMeter({ level, threshold }: { level: number | null; threshold: number }) {
  const pos = (db: number) => Math.max(0, Math.min(100, ((db + 80) / 80) * 100));
  return (
    <div class="grid gap-1">
      <div
        class="relative h-3 rounded-full bg-muted"
        role="img"
        aria-label={level === null ? "No level yet" : `Last heard ${Math.round(level)} dB, threshold ${threshold} dB`}
      >
        <i
          class={
            "absolute inset-0 origin-left rounded-full transition-transform duration-200 " +
            (level !== null && level >= threshold ? "bg-ink-green" : "bg-muted-foreground/40")
          }
          style={{ transform: `scaleX(${level === null ? 0 : pos(level) / 100})` }}
        />
        <b class="absolute -inset-y-1 w-0.5 rounded bg-primary" style={{ left: pos(threshold) + "%" }} />
      </div>
      <div class="flex justify-between text-xs text-muted-foreground" aria-hidden="true">
        <span>Silent</span>
        <span>Loud</span>
      </div>
    </div>
  );
}

// diaryNote says what diary mode has collected so far, and when it paints.
function diaryNote(s: State) {
  if (!s.diary_next) return "";
  const n = s.collected === 1 ? "1 topic" : `${s.collected} topics`;
  return `${n} so far, painted ${when(s.diary_next)}`;
}

function Topics({ state, draft, set }: StateProps) {
  const seconds = state.status.segment;
  const minutes = (n: number) => {
    const m = (n * seconds) / 60;
    return m >= 1 ? `${Math.round(m * 10) / 10} min` : `${n * seconds} s`;
  };
  const temp = draft.llm_temperature ?? 0.2;
  const st = state.status;
  const external = draft.topic_provider === "external";
  return (
    <>
      <Card title="Topic model" icon={MessagesSquare}>
        <Field
          label="Provider"
          tip={TIPS.topic_provider}
          help={
            external ? (
              ""
            ) : st.no_local_topics ? (
              "The local topic model is off, so no topics are found." + (st.topic_api ? " Pick the external one." : "")
            ) : (
              <Stays />
            )
          }
        >
          <select
            class="select"
            value={external ? "external" : "local"}
            onChange={(e) => set({ topic_provider: e.currentTarget.value })}
          >
            <option value="local" disabled={st.no_local_topics}>
              {"On this machine: "}
              {st.no_local_topics ? "off" : st.topics}
            </option>
            {st.topic_api && (
              <option value="external">
                {"External: "}
                {st.topic_api}
              </option>
            )}
          </select>
        </Field>
        {external && (
          <Note tone="warn">
            <strong class="font-semibold">What is said in the room leaves your network.</strong> {"Every "}
            {minutes(draft.extract_every)}
            {", the last "}
            {minutes(draft.window_segments)}
            {" of transcript is sent to "}
            {st.topic_api}
            {
              ". The privacy filters still decide which pictures get made, but the service has already read the text. Tell the people in the room."
            }
          </Note>
        )}
      </Card>
      <Card title="Topics" icon={SlidersHorizontal}>
        <Field
          label="Minimum confidence"
          tip={TIPS.min_confidence}
          value={Math.round(draft.min_confidence * 100) + "%"}
        >
          <Range
            min="0"
            max="1"
            step="0.05"
            value={draft.min_confidence}
            onInput={(e) => set({ min_confidence: +e.currentTarget.value })}
          />
        </Field>
      </Card>
      <Card title="Transcript window" icon={ScrollText}>
        <Field
          label="Window length"
          tip={TIPS.window_segments}
          value={`${draft.window_segments} segments, ${minutes(draft.window_segments)}`}
        >
          <Range
            min="1"
            max="40"
            step="1"
            value={draft.window_segments}
            onInput={(e) => set({ window_segments: +e.currentTarget.value })}
          />
        </Field>
        <Field
          label="Topic check interval"
          tip={TIPS.extract_every}
          value={`${draft.extract_every} segments, ${minutes(draft.extract_every)}`}
        >
          <Range
            min="1"
            max="20"
            step="1"
            value={draft.extract_every}
            onInput={(e) => set({ extract_every: +e.currentTarget.value })}
          />
        </Field>
      </Card>
      <Card title="Topic prompt" icon={Pencil} tip={TIPS.topic_prompt}>
        <details open={!!draft.topic_prompt}>
          <summary class="cursor-pointer font-medium">
            {draft.topic_prompt ? "Custom prompt" : "Built-in prompt"}
          </summary>
          <textarea
            class="textarea mt-3 font-mono text-xs"
            rows={9}
            maxlength={8000}
            aria-label="Topic prompt"
            value={draft.topic_prompt || state.default_prompt}
            onInput={(e) =>
              set({ topic_prompt: e.currentTarget.value === state.default_prompt ? "" : e.currentTarget.value })
            }
          />
        </details>
        {draft.topic_prompt && (
          <div>
            <button
              type="button"
              class="btn"
              data-size="sm"
              data-variant="outline"
              onClick={() => set({ topic_prompt: "" })}
            >
              Reset to built-in prompt
            </button>
          </div>
        )}
        <Field label="Temperature" tip={TIPS.llm_temperature} value={temp.toFixed(1)}>
          <Range
            min="0"
            max="1.5"
            step="0.1"
            value={temp}
            onInput={(e) => set({ llm_temperature: +e.currentTarget.value })}
          />
        </Field>
      </Card>
    </>
  );
}

// provider is the image provider image_provider picks: "" is the first one
// that is set up, as in Go's imagegen.Pick.
function provider(providers: Provider[] = [], id: string) {
  return providers.find((p) => p.ready && (id === "" || p.id === id));
}

function Pictures({ state, draft, set }: StateProps) {
  const cap = draft.daily_image_cap;
  const today = state.images_today;
  const providers = state.providers || [];
  const ready = providers.filter((p) => p.ready);
  const auto = ready[0];
  const chosen = provider(providers, draft.image_provider);
  const missing = providers.filter((p) => !p.ready);
  return (
    <>
      <Card title="Image provider" icon={ImagePlus}>
        <Field
          label="Provider"
          tip={TIPS.image_provider}
          help={chosen?.id === "placeholder" || chosen?.local ? <Stays /> : "Sends only the picture description"}
        >
          <select
            class="select w-full"
            value={draft.image_provider}
            onChange={(e) => set({ image_provider: e.currentTarget.value })}
          >
            <option value="">
              {"Automatic: "}
              {auto?.name}
            </option>
            {ready.map((p) => (
              <option value={p.id}>{p.name}</option>
            ))}
            {missing.map((p) => (
              <option disabled>
                {p.id === "local" ? "On this machine" : p.name}
                {": "}
                {p.setup}
              </option>
            ))}
          </select>
        </Field>
        <Field label="Daily limit" tip={TIPS.daily_image_cap} help={`${today} made today`}>
          <input
            class="input w-32"
            type="number"
            min="0"
            max="1000"
            value={cap}
            onInput={(e) => set({ daily_image_cap: Math.max(0, parseInt(e.currentTarget.value, 10) || 0) })}
          />
        </Field>
        {cap > 0 && (
          <div class="progress" role="img" aria-label={`${today} of ${cap} pictures used today`}>
            <span style={{ width: Math.min(100, (today / cap) * 100) + "%" }} />
          </div>
        )}
      </Card>
      <TestPicture provider={draft.image_provider} style={draft.image_style} name={chosen?.name} />
    </>
  );
}

// TestPicture makes one picture from a fixed description with the provider
// picked above, saved or not. It goes nowhere near the frame and does not
// count towards the daily limit.
type Test = { busy?: boolean; image?: string; took?: number; prompt?: string; error?: string; detail?: string };

function TestPicture({ provider, style, name }: { provider: string; style: string; name?: string }) {
  const [test, setTest] = useState<Test | null>(null); // { busy } | { image, took } | { error }
  const run = async () => {
    setTest({ busy: true });
    try {
      const r = await api<Test>("api/test-picture", "POST", { provider, style });
      setTest({ image: r.image, took: r.took, prompt: r.prompt });
    } catch (e) {
      setTest({ error: e.message, detail: e.detail });
    }
  };
  return (
    <Card title="Test picture" icon={Brush} tip={TIPS.test_picture}>
      <div>
        <button type="button" class="btn" data-variant="outline" disabled={test?.busy} onClick={run}>
          {test?.busy && <span class="loader size-4" aria-hidden="true" />}
          {test?.busy ? "Drawing…" : "Make a test picture"}
        </button>
      </div>
      <div aria-live="polite">
        {test?.busy && <span class="skeleton block aspect-4/3 w-full max-w-sm" aria-label={"Waiting for " + name} />}
        {test?.image && (
          <figure class="grid gap-2">
            <img
              class="fade-in w-full max-w-sm rounded-md ring-1 ring-black/15"
              src={test.image}
              alt={"Test picture: " + test.prompt}
            />
            <figcaption class="text-sm text-muted-foreground">
              <span class="block italic">{test.prompt}</span>
              {name}
              {" answered in "}
              {secs(test.took ?? 0)}. Dithered as the frame shows it.
            </figcaption>
          </figure>
        )}
        {test?.error && (
          <div class="alert" data-variant="destructive" role="alert">
            <section>
              {test.error}
              {test.detail && (
                <details class="mt-1 text-sm">
                  <summary class="cursor-pointer">Details</summary>
                  <code class="block break-words text-xs">{test.detail}</code>
                </details>
              )}
            </section>
          </div>
        )}
      </div>
    </Card>
  );
}

function Security({
  state,
  onState,
  onSignedOut,
}: {
  state: State;
  onState: (s: State) => void;
  onSignedOut: () => void;
}) {
  const has = state.status.password;
  const [current, setCurrent] = useState("");
  const [next, setNext] = useState("");
  const [msg, setMsg] = useState(["", ""]);
  const submit = async (password: string) => {
    if (has && !current) return setMsg(["warn", "Enter your current password first."]);
    if (password && password.length < 8) return setMsg(["warn", "Use at least 8 characters."]);
    try {
      const s = await api("api/password", "POST", { current, password });
      onState(s);
      setCurrent("");
      setNext("");
      setMsg([
        "ok",
        password ? "Password saved. Other browsers are signed out." : "Password removed. The page is open again.",
      ]);
    } catch (e) {
      setMsg(["warn", e.message]);
    }
  };
  const signOut = async () => {
    await api("api/logout", "POST", {});
    onSignedOut();
  };
  return (
    <>
      <Card title="Page password" icon={KeyRound} tip={TIPS.password}>
        <Note tone={has ? "ok" : "warn"}>
          {has
            ? "This page is locked with a password."
            : "No password yet. Anyone on your network can change the frame."}
        </Note>
        <form
          class="grid gap-4"
          onSubmit={(e) => {
            e.preventDefault();
            submit(next);
          }}
        >
          {has && (
            <Field label="Current password">
              <input
                class="input"
                type="password"
                autocomplete="current-password"
                value={current}
                onInput={(e) => setCurrent(e.currentTarget.value)}
              />
            </Field>
          )}
          <Field label={has ? "New password" : "Password"} tip={TIPS.new_password}>
            <input
              class="input"
              type="password"
              autocomplete="new-password"
              minlength={8}
              value={next}
              onInput={(e) => setNext(e.currentTarget.value)}
            />
          </Field>
          {msg[1] && <Note tone={msg[0]}>{msg[1]}</Note>}
          <div class="flex flex-wrap gap-2">
            <button class="btn" type="submit">
              {has ? "Change password" : "Set password"}
            </button>
            {has && (
              <button type="button" class="btn" data-variant="outline" onClick={() => submit("")}>
                Remove password
              </button>
            )}
            {has && (
              <button type="button" class="btn" data-variant="ghost" onClick={signOut}>
                Sign out
              </button>
            )}
          </div>
        </form>
      </Card>
      <Card title="Panel token" icon={Lock} tip={TIPS.panel_token}>
        <p class="flex items-center gap-2 font-medium">
          <i
            class={"size-2.5 rounded-full " + (state.status.locked ? "bg-ink-green" : "bg-muted-foreground/40")}
            aria-hidden="true"
          />
          {state.status.locked ? "On" : "Off"}
        </p>
      </Card>
    </>
  );
}

const TILE_COUNTS = [2, 3, 4, 5, 6, 7, 8, 9];

function Layout({ draft, set }: DraftProps) {
  const layouts: [string, string][] = [
    ["collage", "Most talked about"],
    ["single", "One picture"],
    ["rolling", "Latest"],
  ];
  return (
    <Card title="Layout" icon={LayoutGrid} tip={TIPS.layout}>
      <div class="grid grid-cols-3 gap-2 sm:gap-3" role="radiogroup" aria-label="Layout">
        {layouts.map(([v, name]) => (
          <Choice name="layout" checked={draft.layout === v} onChange={() => set({ layout: v })}>
            <span class="grid gap-2">
              <LayoutIcon kind={v} on={draft.layout === v} />
              <span class="text-sm font-medium sm:text-base">{name}</span>
            </span>
          </Choice>
        ))}
      </div>
      {draft.layout !== "single" && (
        <Segments
          name="tiles"
          value={draft.tiles}
          onChange={(n) => set({ tiles: n })}
          label={draft.layout === "rolling" ? "Number of pictures" : "Maximum pictures"}
          tip={TIPS.tiles}
          options={TILE_COUNTS.map((n): [number, string] => [n, String(n)])}
        />
      )}
      {draft.layout !== "single" && (
        <Segments
          name="arrange"
          label="Arrangement"
          tip={TIPS.arrange}
          value={draft.arrange}
          onChange={(v) => set({ arrange: v })}
          options={[
            ["hero", "Newest big"],
            ["cascade", "Cascade"],
            ["mosaic", "Mosaic"],
            ["even", "Same size"],
          ]}
        />
      )}
      <Segments
        name="spacing"
        label="Spacing"
        tip={TIPS.spacing}
        value={draft.spacing}
        onChange={(v) => set({ spacing: v })}
        options={[
          ["none", "None"],
          ["thin", "Thin"],
          ["normal", "Normal"],
          ["wide", "Wide"],
        ]}
      />
      <Segments
        name="shape"
        label="Shape"
        tip={TIPS.shape}
        value={draft.shape}
        onChange={(v) => set({ shape: v })}
        options={[
          ["rectangle", "Rectangle"],
          ["rounded", "Rounded"],
          ["circle", "Circle"],
          ["arch", "Arch"],
        ]}
      />
      <Segments
        name="background"
        label="Background"
        tip={TIPS.background}
        value={draft.background}
        onChange={(v) => set({ background: v })}
        options={[
          ["paper", "Paper"],
          ["white", "White"],
          ["black", "Black"],
        ]}
      />
      <Toggle
        checked={!draft.hide_captions}
        onChange={(v) => set({ hide_captions: !v })}
        title="Captions"
        tip={TIPS.hide_captions}
      />
    </Card>
  );
}

// Small drawings of the three layouts: bars for the topics talked about
// most, one picture, and a row with the newest first for the latest.
function LayoutIcon({ kind, on }: { kind: string; on: boolean }) {
  const hi = on ? "fill-primary" : "fill-muted-foreground/50";
  const lo = "fill-muted-foreground/30";
  const shapes: [number, number, number, number, string][] = (
    {
      collage: [
        [1, 5, 9, 27, hi],
        [12, 11, 9, 21, lo],
        [23, 17, 9, 15, lo],
        [34, 24, 9, 8, lo],
      ],
      single: [[1, 1, 42, 31, hi]],
      rolling: [
        [1, 6, 18, 21, hi],
        [21, 9, 10, 15, lo],
        [33, 11, 7, 11, lo],
        [42, 13, 1, 7, lo],
      ],
    } as Record<string, [number, number, number, number, string][]>
  )[kind];
  return (
    <svg class="h-9 w-12" viewBox="0 0 44 33" aria-hidden="true">
      {shapes.map(([x, y, w, h, c]) => (
        <rect x={x} y={y} width={w} height={h} rx="1.5" class={c} />
      ))}
    </svg>
  );
}

const EVERY: [number, string][] = [
  [0, "Straight away"],
  [5, "5 minutes"],
  [15, "15 minutes"],
  [30, "30 minutes"],
  [60, "1 hour"],
  [120, "2 hours"],
  [240, "4 hours"],
];

const ROTATE: [number, string][] = [
  [0, "Off"],
  [60, "Every hour"],
  [180, "Every 3 hours"],
  [360, "Every 6 hours"],
  [720, "Every 12 hours"],
  [1440, "Every day"],
];

function Timing({ state, draft, set }: StateProps) {
  const modes: [string, string, LucideIcon][] = [
    ["diary", "Diary", NotebookPen],
    ["party", "Continuous", Zap],
  ];
  const quiet = draft.quiet_from !== "";
  return (
    <Card title="Timing" icon={Clock} tip={TIPS.mode}>
      <div class="grid grid-cols-2 gap-2 sm:gap-3" role="radiogroup" aria-label="Mode">
        {modes.map(([v, name, Icon]) => (
          <Choice name="mode" checked={draft.mode === v} onChange={() => set({ mode: v })}>
            <span class="flex items-center gap-2 font-medium">
              <Icon
                class={"size-5 " + (draft.mode === v ? "text-primary" : "text-muted-foreground")}
                aria-hidden="true"
              />
              {name}
            </span>
          </Choice>
        ))}
      </div>
      {draft.mode === "diary" && (
        <Field label="Paint at" help={diaryNote(state)} tip={TIPS.diary_at}>
          <input
            class="input w-32"
            type="time"
            value={draft.diary_at}
            onChange={(e) => set({ diary_at: e.currentTarget.value || "07:00" })}
          />
        </Field>
      )}
      {draft.mode === "diary" ? (
        ""
      ) : (
        <Field label="Picture interval" tip={TIPS.new_every_minutes}>
          <select
            class="select w-full"
            value={draft.new_every_minutes}
            onChange={(e) => set({ new_every_minutes: +e.currentTarget.value })}
          >
            {withCurrent(EVERY, draft.new_every_minutes, (v) => v + " minutes").map(([v, t]) => (
              <option value={v}>{t}</option>
            ))}
          </select>
        </Field>
      )}
      <Field label="Rotate" tip={TIPS.rotate_minutes}>
        <select
          class="select w-full"
          value={draft.rotate_minutes}
          onChange={(e) => set({ rotate_minutes: +e.currentTarget.value })}
        >
          {withCurrent(ROTATE, draft.rotate_minutes, (v) => v + " minutes").map(([v, t]) => (
            <option value={v}>{t}</option>
          ))}
        </select>
      </Field>
      <div class="grid gap-3">
        <Toggle
          checked={quiet}
          title="Quiet hours"
          tip={TIPS.quiet_hours}
          onChange={(v) => set(v ? { quiet_from: "22:00", quiet_to: "07:00" } : { quiet_from: "", quiet_to: "" })}
        />
        {quiet && (
          <div class="flex flex-wrap items-center gap-3">
            <label class="flex items-center gap-2">
              {"From "}
              <input
                class="input w-32"
                type="time"
                value={draft.quiet_from}
                onChange={(e) => set({ quiet_from: e.currentTarget.value })}
              />
            </label>
            <label class="flex items-center gap-2">
              {"until "}
              <input
                class="input w-32"
                type="time"
                value={draft.quiet_to}
                onChange={(e) => set({ quiet_to: e.currentTarget.value })}
              />
            </label>
          </div>
        )}
      </div>
      {draft.layout === "collage" && (
        <Field label="Fade time" tip={TIPS.half_life_hours}>
          <select
            class="select w-full"
            value={draft.half_life_hours}
            onChange={(e) => set({ half_life_hours: +e.currentTarget.value })}
          >
            {withCurrent(HALF_LIVES, draft.half_life_hours, (v) => v + " hours").map(([v, t]) => (
              <option value={v}>{t}</option>
            ))}
          </select>
        </Field>
      )}
    </Card>
  );
}

function Privacy({ draft, set }: DraftProps) {
  const blocked = draft.block_sensitive;
  const toggle = (c: string) =>
    set({ block_sensitive: blocked.includes(c) ? blocked.filter((x) => x !== c) : [...blocked, c] });
  return (
    <Card title="Blocked topics" icon={ShieldCheck} tip={TIPS.block_sensitive}>
      {Object.entries(CATEGORIES).map(([c, [name, text]]) => (
        <Toggle checked={blocked.includes(c)} onChange={() => toggle(c)} title={name} text={text} />
      ))}
      <Toggle
        checked={draft.block_names}
        onChange={(v) => set({ block_names: v })}
        title="Spoken names"
        tip={TIPS.block_names}
      />
      <NeverPaint value={draft.never_paint} onChange={(v) => set({ never_paint: v })} />
      {blocked.length === 0 && !draft.block_names && !(draft.never_paint || []).length && (
        <Note tone="warn">Nothing is held back: every topic can become a picture.</Note>
      )}
    </Card>
  );
}

function Storage({ draft, set, saved, topicAPI }: DraftProps & { saved: boolean; topicAPI: string }) {
  const external = draft.topic_provider === "external";
  return (
    <Card title="Storage" icon={HardDrive} tip={TIPS.storage}>
      <Note tone="ok">
        <span class="flex items-center gap-2">
          <ShieldCheck class="size-4 shrink-0 text-ink-green" aria-hidden="true" />
          Nothing is recorded
        </span>
      </Note>
      {external && (
        <Note tone="warn">
          {"The external topic model is on: that window is also sent to "}
          {topicAPI}. Switch back under Listening, Topics.
        </Note>
      )}
      <Toggle
        checked={draft.keep_pictures !== false}
        onChange={(v) => set({ keep_pictures: v })}
        disabled={!saved}
        title="Keep pictures over a restart"
        tip={TIPS.keep_pictures}
        text={saved ? "" : "Needs a settings file: start E-maki with -settings."}
      />
    </Card>
  );
}

// NeverPaint edits the never-paint list, one word or phrase a line. The text
// is kept as typed so a new line survives until something is written on it.
function NeverPaint({ value, onChange }: { value: string[] | null; onChange: (v: string[] | null) => void }) {
  const list = (t: string) =>
    t
      .split("\n")
      .map((w) => w.trim())
      .filter(Boolean);
  const [text, setText] = useState((value || []).join("\n"));
  useEffect(() => {
    if (!same(list(text), value || [])) setText((value || []).join("\n"));
  }, [value]);
  return (
    <Field stack label="Never paint" tip={TIPS.never_paint}>
      <textarea
        class="textarea"
        rows={4}
        maxlength={20000}
        placeholder={"For example:\nwork\nAnna"}
        value={text}
        onInput={(e) => {
          setText(e.currentTarget.value);
          const l = list(e.currentTarget.value);
          onChange(l.length ? l : null);
        }}
      />
    </Field>
  );
}

// Picture style moods, in the order the grid shows them.
const MOODS: [string, string][] = [
  ["", "All"],
  ["cool", "Cool"],
  ["relaxed", "Relaxed"],
  ["somber", "Somber"],
];

function Look({ draft, set, styles }: DraftProps & { styles: State["styles"] }) {
  // image_style holds a preset's name; older settings hold its prompt, and empty is the first preset.
  const style = draft.image_style || styles[0].name;
  const preset = styles.find((s) => s.name.toLowerCase() === style.toLowerCase() || s.prompt === style);
  const [custom, setCustom] = useState(!preset);
  const [full, setFull] = useState(false);
  const [mood, setMood] = useState("");
  const [query, setQuery] = useState("");
  const q = query.trim().toLowerCase();
  // Presets in mood order; a profile's own styles, without a mood, last.
  const order = (m?: string) => {
    const i = MOODS.findIndex(([v]) => v && v === m);
    return i < 0 ? MOODS.length : i;
  };
  const shown = styles
    .filter((s) => (!mood || s.mood === mood) && (s.name + " " + s.prompt).toLowerCase().includes(q))
    .sort((a, b) => order(a.mood) - order(b.mood));
  const tile =
    "tap grid cursor-pointer content-start gap-1 rounded-lg border p-1.5 text-sm font-medium transition-colors hover:bg-accent has-checked:border-primary has-checked:bg-primary/5 has-checked:ring-1 has-checked:ring-primary has-focus-visible:ring-2 has-focus-visible:ring-ring";
  return (
    <Card title="Picture style" icon={Palette} tip={TIPS.image_style}>
      <div class="flex flex-wrap items-center gap-2">
        <input
          type="search"
          class="input w-auto min-w-0 flex-1 basis-48"
          placeholder="Search styles"
          aria-label="Search styles"
          value={query}
          onInput={(e) => setQuery(e.currentTarget.value)}
        />
        <Segments name="mood" inline label="Mood" value={mood} onChange={setMood} options={MOODS} />
      </div>
      <div
        class="grid grid-cols-[repeat(auto-fill,minmax(6rem,1fr))] gap-2"
        role="radiogroup"
        aria-label="Picture style"
      >
        {shown.map((s) => (
          <label class={tile} key={s.name}>
            <input
              type="radio"
              name="style"
              class="sr-only"
              checked={!custom && preset === s}
              onChange={() => {
                setCustom(false);
                set({ image_style: s.name });
              }}
            />
            <img
              src={`ui/styles/${s.name.toLowerCase().replaceAll(" ", "-")}.png`}
              alt=""
              loading="lazy"
              class="aspect-4/3 w-full rounded bg-muted object-cover [image-rendering:pixelated]"
              onError={(e) => {
                e.currentTarget.style.visibility = "hidden";
              }}
            />
            <span class="truncate px-0.5">{s.name}</span>
          </label>
        ))}
        <label class={tile}>
          <input
            type="radio"
            name="style"
            class="sr-only"
            checked={custom}
            onChange={() => {
              setCustom(true);
              if (preset) set({ image_style: preset.prompt });
            }}
          />
          <span class="grid aspect-4/3 w-full place-items-center rounded bg-muted text-muted-foreground">Aa</span>
          <span class="truncate px-0.5">Your own</span>
        </label>
      </div>
      {!shown.length && <p class="text-muted-foreground">No style matches "{query.trim()}".</p>}
      <div class="grid">
        {[
          ...styles.map((s): [string, boolean, JSX.Element] => [
            s.name,
            !custom && preset === s,
            <div class="grid justify-items-start gap-1 rounded-lg bg-muted p-3">
              <p class={"italic text-muted-foreground " + (full ? "" : "line-clamp-3")}>{s.prompt}</p>
              <button
                type="button"
                class="text-sm font-medium underline-offset-4 hover:underline"
                aria-expanded={full}
                onClick={() => setFull(!full)}
              >
                {full ? "Show less" : "Show the whole prompt"}
              </button>
            </div>,
          ]),
          [
            "",
            custom,
            <Field stack label="Custom style" tip={TIPS.custom_style}>
              <textarea
                class="textarea"
                rows={4}
                maxlength={1000}
                placeholder="For example: woodcut print with thick black lines, flat yellow and blue"
                value={draft.image_style}
                onInput={(e) => set({ image_style: e.currentTarget.value })}
              />
            </Field>,
          ] as [string, boolean, JSX.Element],
        ].map(([k, on, el]) => on && <div key={k}>{el}</div>)}
      </div>
    </Card>
  );
}

// The Inky Impression panels Fugleramme knows, by their resolution (landscape).
const PANELS: [string, string][] = [
  ["1600x1200", '13.3" (1600 × 1200)'],
  ["800x480", '7.3" (800 × 480)'],
  ["600x448", '5.7" (600 × 448)'],
  ["640x400", '4" (640 × 400)'],
];

// withSize keeps a panel size set in the profile selectable.
function withSize(d: Settings): [string, string][] {
  const v = d.panel_width + "x" + d.panel_height;
  return PANELS.some(([p]) => p === v) ? PANELS : [...PANELS, [v, `${d.panel_width} × ${d.panel_height}`]];
}

// withCurrent keeps a value set in the profile selectable when it is not one of the options.
function withCurrent(options: [number, string][], value: number, label: (v: number) => string) {
  return options.some(([v]) => v === value)
    ? options
    : [...options, [value, label(value)] as [number, string]].sort((a, b) => a[0] - b[0]);
}

function Hanging({ draft, set }: DraftProps) {
  return (
    <Card title="Panel" icon={Tablet}>
      <details class="group">
        <summary class="flex cursor-pointer items-center justify-between gap-3 text-muted-foreground">
          <span>{panelSummary(draft)}</span>
          <span class="shrink-0 text-sm font-medium text-foreground group-open:hidden">Change</span>
        </summary>
        <div class="grid gap-5 pt-5">
          <Field label="Size" tip={TIPS.panel_size}>
            <select
              class="select w-full"
              value={draft.panel_width + "x" + draft.panel_height}
              onChange={(e) => {
                const [w, h] = e.currentTarget.value.split("x").map(Number);
                set({ panel_width: w, panel_height: h });
              }}
            >
              {withSize(draft).map(([v, t]) => (
                <option value={v}>{t}</option>
              ))}
            </select>
          </Field>
          <div class="grid gap-2" role="radiogroup" aria-label="Rotation">
            <span class="font-medium">
              Rotation
              <Tip tip={TIPS.panel_rotation} name="Rotation" />
            </span>
            <div class="flex flex-wrap gap-2">
              {ROTATIONS.map((v) => (
                <label class="grid w-16 cursor-pointer justify-items-center gap-1 rounded-lg border p-2 hover:bg-accent has-checked:border-primary has-checked:ring-1 has-checked:ring-primary has-focus-visible:ring-2 has-focus-visible:ring-ring">
                  <input
                    type="radio"
                    name="rotation"
                    class="sr-only"
                    checked={draft.panel_rotation === v}
                    onChange={() => set({ panel_rotation: v })}
                  />
                  <span
                    class="grid h-6 w-8 place-content-center rounded-sm border-2 border-foreground/70 text-xs"
                    style={{ transform: `rotate(${v}deg)` }}
                    aria-hidden="true"
                  >
                    A
                  </span>
                  <small>{v}°</small>
                </label>
              ))}
            </div>
          </div>
          {hasE6(draft) && (
            <Field label="Refresh interval" tip={TIPS.panel_poll_minutes}>
              <select
                class="select w-full"
                value={draft.panel_poll_minutes}
                onChange={(e) => set({ panel_poll_minutes: +e.currentTarget.value })}
              >
                {withCurrent(
                  POLLS.map((m): [number, string] => [m, m === 1 ? "minute" : m + " minutes"]),
                  draft.panel_poll_minutes,
                  (v) => v + " minutes",
                ).map(([v, t]) => (
                  <option value={v}>{t}</option>
                ))}
              </select>
            </Field>
          )}
          <div class="grid gap-3">
            <Field
              label="Buttons"
              tip={TIPS.panel_buttons}
              help={
                <a
                  class="underline underline-offset-4 hover:text-foreground"
                  href="https://github.com/Strandgaard96/E-maki/blob/main/docs/hardware.md#any-device"
                  target="_blank"
                  rel="noopener"
                >
                  How a frame sends a press
                </a>
              }
            >
              <select
                class="select w-full"
                value={Object.keys(draft.panel_buttons || {}).length}
                onChange={(e) => set({ panel_buttons: buttons(draft.panel_buttons, +e.currentTarget.value) })}
              >
                {["", ...BUTTON_LETTERS].map((_, n) => (
                  <option value={n}>{n === 0 ? "None" : n}</option>
                ))}
              </select>
            </Field>
            {Object.keys(draft.panel_buttons || {})
              .sort()
              .map((b) => (
                <Field label={"Button " + b} tip={TIPS.panel_button}>
                  <select
                    class="select w-full"
                    value={draft.panel_buttons?.[b]}
                    onChange={(e) => set({ panel_buttons: { ...draft.panel_buttons, [b]: e.currentTarget.value } })}
                  >
                    {BUTTON_ACTIONS.map(([v, t]) => (
                      <option value={v}>{t}</option>
                    ))}
                  </select>
                </Field>
              ))}
          </div>
        </div>
      </details>
    </Card>
  );
}

// panelSummary is the Panel card's folded line: size, rotation and buttons.
function panelSummary(d: Settings) {
  const size =
    withSize(d).find(([v]) => v === d.panel_width + "x" + d.panel_height)?.[1] ??
    `${d.panel_width} × ${d.panel_height}`;
  const n = Object.keys(d.panel_buttons || {}).length;
  return (
    [size, d.panel_rotation ? `turned ${d.panel_rotation}°` : "not turned", n ? `${n} buttons` : "no buttons"].join(
      ", ",
    ) + "."
  );
}

// hasE6 is whether the panel gets /frame.e6, the only place the refresh interval goes.
const hasE6 = (d: Settings) => ["800x480", "1600x1200"].includes(d.panel_width + "x" + d.panel_height);

// The letters a frame's buttons can send (config.Buttons), and the first n of
// them with the actions they had, new ones as on an Inky.
const BUTTON_LETTERS = ["A", "B", "C", "D", "E", "F", "G", "H"];
const INKY_BUTTONS: Record<string, string> = { A: "pause", B: "remove", C: "pin", D: "next" };
function buttons(old: Settings["panel_buttons"], n: number) {
  return Object.fromEntries(BUTTON_LETTERS.slice(0, n).map((b) => [b, old?.[b] ?? INKY_BUTTONS[b] ?? ""]));
}

// What a frame button can do (panel_buttons).
const BUTTON_ACTIONS = [
  ["pause", "Pause or resume listening"],
  ["remove", "Remove the last picture"],
  ["pin", "Pin the last picture"],
  ["next", "Put the next picture up"],
  ["", "Nothing"],
];

function Facts({ rows }: { rows: [string, string][] }) {
  return (
    <dl class="grid gap-x-6 gap-y-2 sm:grid-cols-[max-content_1fr]">
      {rows.map(([k, v]) => (
        <>
          <dt class="text-muted-foreground">{k}</dt>
          <dd class="break-words">{v}</dd>
        </>
      ))}
    </dl>
  );
}

function System({ state }: { state: State }) {
  const st = state.status;
  const a = state.activity;
  const langs = Object.entries(st.by_lang || {});
  const h = state.health;
  return (
    <>
      <Card title="Since start" icon={Activity}>
        <div class="grid grid-cols-2 gap-3 sm:grid-cols-4">
          {[
            [a.heard, "segments transcribed"],
            [a.quiet, "too quiet, skipped"],
            [a.topics, "topics found"],
            [a.held_back, "held back for privacy"],
            ...(a.dropped ? [[a.dropped, "dropped, topic model too slow"]] : []),
          ].map(([n, text]) => (
            <div class="rounded-lg border p-3">
              <b class="block text-2xl font-semibold tabular-nums">{n}</b>
              <small class="text-muted-foreground">{text}</small>
            </div>
          ))}
        </div>
      </Card>
      <Card title="Services" icon={Server}>
        <div class="grid gap-3">
          <Service name="Speech to text" where={st.speech} s={h.speech} />
          <Service
            name="Topic model"
            s={h.topics}
            where={
              state.settings.topic_provider === "external" && st.topic_api
                ? `${st.topic_api} (external)`
                : st.no_local_topics
                  ? "None (NO_LOCAL_TOPICS in .env)"
                  : `${st.topics} at ${st.llm_url}`
            }
          />
          <Service name="Image provider" where={st.images} s={h.images} idle="No picture made yet" />
        </div>
        <Facts
          rows={[
            ["Profile", st.profile],
            ...langs.map(([l, u]): [string, string] => ["Speech to text, " + langName(l), u]),
            ["Running since", when(st.started)],
            [
              "Settings",
              st.saved ? "Saved, kept after a restart" : "Last until restart (start with -settings to keep them)",
            ],
          ]}
        />
      </Card>
      <Card title="Set in .env" icon={SettingsIcon} tip={TIPS.env}>
        <dl class="grid gap-x-6 gap-y-2 sm:grid-cols-[max-content_1fr_max-content]">
          {(st.env || []).map((r) => (
            <>
              <dt class="text-muted-foreground">{r.label}</dt>
              <dd class="break-words">{r.value}</dd>
              <dd class="mb-2 sm:mb-0">
                <code class="text-xs">{r.var}</code>
              </dd>
            </>
          ))}
        </dl>
      </Card>
    </>
  );
}

// HEALTH is each service state's dot and words.
const HEALTH: Record<string, [string, string]> = {
  up: ["bg-ink-green", "Answering"],
  loading: ["bg-ink-yellow", "Loading its model"],
  down: ["bg-destructive", "Not answering"],
  failing: ["bg-destructive", "Last call failed"],
  unknown: ["bg-muted-foreground/40", "Not used yet"],
};

const secs = (s: number) => (s < 10 ? s.toFixed(1) : Math.round(s)) + " s";

// notReady names the models that do not answer yet, "" when both do.
function notReady(h: State["health"]) {
  const no = (s?: Health) => s && (s.state === "down" || s.state === "loading");
  if (no(h.speech) && no(h.topics)) return "Models not ready";
  if (no(h.speech)) return "Speech model not ready";
  if (no(h.topics)) return "Topic model not ready";
  return "";
}

// Service is one model or provider: whether it answers, and how its last call went.
function Service({ name, where, s, idle }: { name: string; where: string; s: Health; idle?: string }) {
  const [dot, word] = HEALTH[s.state] || HEALTH.unknown;
  const last = s.last_at
    ? s.error
      ? `Failed ${when(s.last_at)}`
      : `Last answer in ${secs(s.took ?? 0)}, ${when(s.last_at)}`
    : "";
  const detail =
    s.state === "down"
      ? "On a first start it may still be downloading its model."
      : s.state === "unknown"
        ? idle || ""
        : "";
  return (
    <div class="grid gap-1 rounded-lg border p-3">
      <div class="flex flex-wrap items-center justify-between gap-x-4 gap-y-1">
        <span class="font-medium">{name}</span>
        <span class="flex items-center gap-2 text-sm font-medium">
          <i class={"size-2 rounded-full " + dot} aria-hidden="true" />
          {word}
        </span>
      </div>
      {where && <small class="break-words text-muted-foreground">{where}</small>}
      {(last || detail) && <small class="text-muted-foreground">{[last, detail].filter(Boolean).join(". ")}</small>}
      {s.error && (
        <details class="text-sm">
          <summary class="cursor-pointer text-muted-foreground">Details</summary>
          <code class="mt-1 block break-words text-xs text-destructive">{s.error}</code>
        </details>
      )}
    </div>
  );
}

function SaveBar({
  dirty,
  saving,
  notice,
  what,
  onSave,
  onDiscard,
  onDone,
}: {
  dirty: boolean;
  saving: boolean;
  notice: string;
  what: string;
  onSave: () => void;
  onDiscard: () => void;
  onDone: () => void;
}) {
  // Closing or reloading the page would drop the changes: let the browser ask first.
  useEffect(() => {
    if (!dirty) return;
    const ask = (e: BeforeUnloadEvent) => {
      e.preventDefault();
      e.returnValue = "";
    };
    addEventListener("beforeunload", ask);
    return () => removeEventListener("beforeunload", ask);
  }, [dirty]);
  useEffect(() => {
    if (!notice) return;
    const t = setTimeout(onDone, 5000);
    return () => clearTimeout(t);
  }, [notice]);
  if (!dirty && !notice) return null;
  return (
    <div
      class="fixed inset-x-0 bottom-20 z-20 flex justify-center px-4 sm:bottom-4"
      role="region"
      aria-label="Unsaved changes"
    >
      <div class="rise-in flex flex-wrap items-center gap-3 rounded-xl border bg-popover px-4 py-3 text-sm text-popover-foreground shadow-lg">
        {dirty ? (
          <>
            <span role="status">{what}</span>
            <button type="button" class="btn" data-variant="ghost" onClick={onDiscard} disabled={saving}>
              Discard
            </button>
            <button type="button" class="btn" onClick={onSave} disabled={saving}>
              {saving ? "Saving…" : "Save changes"}
            </button>
          </>
        ) : (
          <span role="status" class="flex items-center gap-2">
            <i class="size-2 rounded-full bg-ink-green" aria-hidden="true" />
            {notice}
          </span>
        )}
      </div>
    </div>
  );
}

// The page's own "Loading" line goes, so it does not sit above the app.
const root = document.getElementById("app") as HTMLElement;
root.replaceChildren();
render(<App />, root);
