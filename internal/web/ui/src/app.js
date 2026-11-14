// E-maki's settings page: Preact + htm, styled with Tailwind and Basecoat.
// `npm run build` bundles it into ../static/app.js, which the Go binary embeds.

import htm from "htm";
import justifiedLayout from "justified-layout";
import PhotoSwipe from "photoswipe";
import { h, render } from "preact";
import { useCallback, useEffect, useRef, useState } from "preact/hooks";

const html = htm.bind(h);

// Basecoat's dark theme hangs on a class; follow the system setting.
const dark = matchMedia("(prefers-color-scheme: dark)");
const theme = () => document.documentElement.classList.toggle("dark", dark.matches);
dark.addEventListener("change", theme);
theme();

const CATEGORIES = {
  health: ["Health", "Illness, doctors, bodies"],
  money: ["Money", "Salaries, debt, what things cost"],
  relationship: ["Relationships", "Dating, break-ups, family rows"],
  confidential_work: ["Work secrets", "Clients, deals, anything under NDA"],
};

const HALF_LIVES = [
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

async function api(path, method = "GET", body) {
  const r = await fetch(path, {
    method,
    cache: "no-store",
    headers: body ? { "Content-Type": "application/json" } : {},
    body: body ? JSON.stringify(body) : undefined,
  });
  if (r.status === 401 && path !== "api/login") {
    throw Object.assign(new Error("Sign in first."), { signin: true });
  }
  if (!r.ok) throw new Error((await r.text()).trim() || r.statusText);
  return r.status === 204 ? null : r.json();
}

const same = (a, b) => JSON.stringify(a) === JSON.stringify(b);
// focus moves to an element once, when it appears (a stable ref callback).
const focus = (e) => e?.focus();

function when(iso) {
  const d = new Date(iso);
  const today = new Date().toDateString() === d.toDateString();
  const time = d.toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" });
  return today ? `today at ${time}` : `${d.toLocaleDateString([], { weekday: "long" })} at ${time}`;
}

function App() {
  const [state, setState] = useState(null);
  const [draft, setDraft] = useState(null);
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");
  const [saving, setSaving] = useState(false);
  const [tab, setTab] = useState(TABS.find(([k]) => location.hash.startsWith("#" + k))?.[0] ?? "frame");
  const [signin, setSignin] = useState(false);
  const [hot, setHot] = useState(""); // the tile pointed at, in the list or in the picture
  const [redrawing, setRedrawing] = useState(""); // the picture being redrawn

  const load = useCallback(async () => {
    try {
      const s = await api("api/state");
      setState(s);
      setDraft((d) => d ?? s.settings);
      setSignin(false);
      setError("");
    } catch (e) {
      if (e.signin) setSignin(true);
      else setError("Can't reach E-maki. Is it still running? " + e.message);
    }
  }, []);

  useEffect(() => {
    load();
    const t = setInterval(load, 15000);
    return () => clearInterval(t);
  }, [load]);

  if (signin) return html`<${SignIn} onDone=${load} />`;
  if (!state || !draft) return html`<${Loading} error=${error} />`;

  // Pausing and the timeline switch apply at once, so they are never part of the draft.
  const pending = { ...draft, paused: state.settings.paused, timeline: state.settings.timeline };
  const changed = Object.keys(pending).filter((k) => !same(pending[k], state.settings[k]));
  const dirty = changed.length > 0;
  const where = {
    frame: changed.some((k) => !EMAKI_KEYS.includes(k)),
    emaki: changed.some((k) => EMAKI_KEYS.includes(k)),
  };
  const set = (patch) => setDraft({ ...draft, ...patch });
  const show = (t) => {
    setTab(t);
    history.replaceState(null, "", "#" + t);
  };

  const call = async (path, body, done) => {
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
  const redraw = async (label) => {
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
      setNotice(s.warning || (s.status.saved ? "Saved." : "Applied. These last until E-maki restarts."));
      setError("");
    } catch (e) {
      if (e.signin) setSignin(true);
      else setError("Not saved: " + e.message);
    }
    setSaving(false);
  };

  const paused = state.settings.paused;
  const waiting = notReady(state.health);
  const togglePause = () => call("api/pause", { paused: !paused }, paused ? "Listening again." : "Listening paused.");

  return html`
    <header class="sticky top-0 z-10 border-b bg-background">
      <div class="mx-auto flex max-w-6xl flex-wrap items-center gap-x-6 gap-y-3 px-4 py-3">
        <${Brand} />
        <div class="tabs fixed inset-x-0 bottom-0 z-20 border-t bg-background px-2 pt-2 pb-[max(0.5rem,env(safe-area-inset-bottom))] sm:static sm:z-auto sm:w-auto sm:border-0 sm:bg-transparent sm:p-0">
          <nav role="tablist" aria-orientation="horizontal" aria-label="Sections" class="w-full sm:w-auto"
            onKeyDown=${(e) => {
              if (e.key !== "ArrowLeft" && e.key !== "ArrowRight") return;
              const i = TABS.findIndex(([k]) => k === tab);
              const other = TABS[(i + (e.key === "ArrowRight" ? 1 : TABS.length - 1)) % TABS.length][0];
              show(other);
              document.getElementById("tab-" + other).focus();
            }}>
            ${TABS.map(
              ([k, name]) => html`
              <button type="button" role="tab" id=${"tab-" + k} aria-selected=${tab === k} aria-controls=${"panel-" + k}
                tabindex=${tab === k ? 0 : -1} class="relative"
                onClick=${() => show(k)}>${name}
                <span class=${"absolute top-1 right-0.5 size-1.5 rounded-full bg-ink-yellow " + (where[k] ? "" : "invisible")} role="img"
                  aria-label=${where[k] ? "has unsaved changes" : undefined} aria-hidden=${!where[k]}></span></button>`,
            )}
          </nav>
        </div>
        <div class="ms-auto flex items-center gap-3">
          <span class="flex items-center gap-2 text-sm font-medium" role="status">
            <span aria-hidden="true" class=${"size-2.5 rounded-full " + (paused || waiting ? "bg-ink-yellow" : "bg-ink-green ring-4 ring-ink-green/20")}></span>
            ${paused ? "Paused" : waiting || "Listening"}</span>
          <button type="button" class="btn" data-variant="outline" onClick=${togglePause}>${paused ? "Resume" : "Pause"}</button>
        </div>
      </div>
    </header>
    ${error && html`<div class="mx-auto max-w-6xl px-4 pt-4"><div class="alert" data-variant="destructive" role="alert"><section>${error}</section></div></div>`}
    ${
      tab === "frame"
        ? html`<main id="panel-frame" role="tabpanel" aria-labelledby="tab-frame"
          class="mx-auto grid max-w-6xl gap-6 px-4 py-6 lg:grid-cols-[minmax(0,1.1fr)_minmax(0,1fr)] lg:items-start">
          <section class="grid gap-6 lg:sticky lg:top-20" aria-label="The frame">
            <p class="text-sm text-muted-foreground">${summary(state)}</p>
            <${Frame} page=${state.page} wall=${state.wall} draft=${pending}
              changed=${PICTURE_KEYS.some((k) => !same(pending[k], state.settings[k]))} />
          </section>
          <div class="grid gap-6">
            <${Layout} draft=${draft} set=${set} />
            <${Timing} draft=${draft} set=${set} />
            <${Look} draft=${draft} set=${set} styles=${state.styles} />
          </div>
        </main>`
        : tab === "pictures"
          ? html`<main id="panel-pictures" role="tabpanel" aria-labelledby="tab-pictures"
          class="mx-auto grid max-w-6xl gap-6 px-4 py-6 lg:grid-cols-2 lg:items-start">
          <${Wall} state=${state} hot=${hot} onHot=${setHot} onTile=${(a, label, to) => call("api/tiles/" + a, { label, to })}
            onClear=${() => call("api/clear", {}, "Frame cleared.")} redrawing=${redrawing} onRedraw=${redraw} />
          <div class="grid gap-6">
            <${Queue} state=${state} onLine=${(a, label, done) => call("api/queue/" + a, { label }, done)}
              redrawing=${redrawing} onRedraw=${redraw} />
            <${Removed} removed=${state.removed} onAllow=${(label) => call("api/removed/allow", { label }, "It can come back now.")} />
          </div>
        </main>`
          : tab === "timeline"
            ? html`<${Timeline} onState=${setState} onSignedOut=${() => setSignin(true)} />`
            : tab === "emaki"
              ? html`<${EmakiTab} state=${state} draft=${draft} set=${set} paused=${paused}
          onState=${setState} onSignedOut=${() => setSignin(true)} />`
              : html`<${Logs} />`
    }
    <${SaveBar} dirty=${dirty} saving=${saving} notice=${notice}
      what=${`${changed.length === 1 ? "1 unsaved change" : changed.length + " unsaved changes"} on ${[where.frame && "Frame", where.emaki && "E-maki"].filter(Boolean).join(" and ")}.`}
      onSave=${save} onDiscard=${() => setDraft(state.settings)} onDone=${() => setNotice("")} />
  `;
}

const TABS = [
  ["frame", "Frame"],
  ["pictures", "Pictures"],
  ["timeline", "Timeline"],
  ["emaki", "E-maki"],
  ["logs", "Logs"],
];

// Settings that change the picture itself, which the frame can show before saving.
const PICTURE_KEYS = [
  "layout",
  "tiles",
  "newest_big",
  "hide_captions",
  "half_life_hours",
  "panel_width",
  "panel_height",
  "panel_rotation",
];
// Settings edited on the E-maki tab; the rest are on Frame.
const EMAKI_KEYS = [
  "mode",
  "mic_device",
  "min_speech_dbfs",
  "languages",
  "window_segments",
  "extract_every",
  "topic_prompt",
  "llm_temperature",
  "min_confidence",
  "daily_image_cap",
  "block_sensitive",
  "block_names",
  "never_paint",
  "keep_pictures",
  "panel_buttons",
  "panel_width",
  "panel_height",
  "panel_rotation",
  "panel_poll_minutes",
];

function summary(s) {
  const layout = { collage: "Collage", single: "One picture", rolling: "Rolling" }[s.settings.layout];
  const n = s.wall.length;
  const tiles =
    n === 0 ? "nothing on the frame yet" : n === 1 ? "1 picture on the frame" : `${n} pictures on the frame`;
  return `${layout}, ${tiles}.`;
}

// The six inks of the panel, the brand mark.
const INKS = ["#1d1c21", "#f4f4f0", "#e2bd2f", "#b23a3a", "#2f3c7e", "#2e6a4b"];

function Brand() {
  return html`<div class="flex items-center gap-3">
    <span class="rounded-md bg-neutral-900 p-[3px] shadow-sm ring-1 ring-black/20 dark:ring-white/25" aria-hidden="true">
      <span class="grid grid-cols-3 gap-0.5 rounded-[2px] bg-[#fbfbf8] p-[3px]">
        ${INKS.map((c) => html`<i class="size-2 rounded-[1px] ring-1 ring-black/15" style=${{ background: c }}></i>`)}
      </span>
    </span>
    <h1 class="text-xl font-semibold tracking-tight">E-maki</h1>
  </div>`;
}

// Frame shows the layout the settings give: the pictures on the frame, with
// sample pictures in the places they do not fill yet, and unsaved layout
// changes applied. Before the first topic it is all sample pictures.
function Frame({ page, draft, changed, wall }) {
  const empty = wall.length === 0;
  const mark = changed || empty;
  const [away, setAway] = useState(false);
  const fig = useRef(null);
  // On a narrow screen the picture scrolls away while you edit: keep a small one in the corner.
  useEffect(() => {
    if (!fig.current || !window.IntersectionObserver) return;
    const io = new IntersectionObserver(([e]) => setAway(!e.isIntersecting));
    io.observe(fig.current);
    return () => io.disconnect();
  }, []);
  const src = "api/preview.png?" + new URLSearchParams([...PICTURE_KEYS.map((k) => [k, draft[k]]), ["v", page]]);
  const pic = usePicture(src);
  const alt = empty
    ? "Example pictures in the chosen layout"
    : changed
      ? "The frame as it would look with your unsaved changes"
      : "The layout with sample pictures in the empty places";
  const badge = empty ? "Example, not on the frame" : changed ? "Not saved yet" : "";
  return html`
    <figure class="grid gap-2" ref=${fig} aria-busy=${pic.loading}>
      <div class="flex min-h-10 flex-wrap items-center justify-between gap-2 pointer-coarse:min-h-12">
        <span class="text-sm font-medium">Layout preview</span>
        <span class="flex items-center gap-2">
          <span class=${"loader size-4 " + (pic.slow ? "" : "invisible")} role="status" aria-label=${pic.slow ? "Drawing the preview" : undefined}></span>
          ${badge && html`<span class="rounded-full bg-ink-yellow px-2.5 py-0.5 text-xs font-medium text-neutral-900">${badge}</span>`}
        </span>
      </div>
      <div class=${"bezel " + (mark ? "outline-2 outline-offset-2 outline-ink-yellow" : "")}>
        <div class="relative" style=${{ aspectRatio: aspect(draft) }}>
          ${pic.prev && html`<img class="absolute inset-0 size-full object-contain" src=${pic.prev} alt="" />`}
          ${pic.cur && html`<img key=${pic.cur} class="fade-in relative size-full object-contain" src=${pic.cur} alt=${alt} />`}
        </div>
      </div>
      <figcaption class="flex justify-between text-sm text-muted-foreground">
        <span>${
          empty
            ? "Your layout once pictures come in."
            : changed
              ? "What the frame will show once you save. Sample pictures fill the empty places."
              : "Sample pictures fill the places new topics will take. The Pictures tab lists what is on the frame."
        }</span>
        <a class="underline underline-offset-4 hover:text-foreground" href="view" target="_blank" rel="noopener">Open full screen</a>
      </figcaption>
      <button type="button" aria-label="Back to the frame picture" inert=${!away}
        onClick=${() => window.scrollTo({ top: 0, behavior: matchMedia("(prefers-reduced-motion: reduce)").matches ? "auto" : "smooth" })}
        class=${
          "mini fixed right-3 top-20 z-10 w-32 rounded-lg bg-neutral-900 p-1.5 shadow-xl ring-1 ring-white/25 lg:hidden " +
          (mark ? "outline-2 outline-ink-yellow " : "") +
          (away ? "" : "pointer-events-none opacity-0")
        }>
        <span class="block w-full overflow-hidden rounded-[2px] bg-[#fbfbf8]" style=${{ aspectRatio: aspect(draft) }}>
          ${pic.cur && html`<img class="size-full object-contain" src=${pic.cur} alt="" />`}</span>
      </button>
    </figure>`;
}

// aspect is the page's shape as the viewer sees it, for CSS aspect-ratio.
const aspect = (d) =>
  d.panel_rotation % 180 ? `${d.panel_height} / ${d.panel_width}` : `${d.panel_width} / ${d.panel_height}`;

// usePicture loads a new picture before showing it, so the old one stays up
// until the new one is ready and then fades over it. A spinner shows only
// if drawing takes longer than a tenth of a second.
function usePicture(src) {
  const [pic, setPic] = useState({ cur: "", prev: "" });
  const [slow, setSlow] = useState(false);
  useEffect(() => {
    if (src === pic.cur) return;
    let on = true;
    const t = setTimeout(() => on && setSlow(true), 100);
    const im = new Image();
    const done = (ok) => {
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
function Loading({ error }) {
  const [show, setShow] = useState(false);
  useEffect(() => {
    const t = setTimeout(() => setShow(true), 100);
    return () => clearTimeout(t);
  }, []);
  return html`
    <header class="border-b">
      <div class="mx-auto flex max-w-6xl flex-wrap items-center gap-x-6 gap-y-3 px-4 py-3">
        <${Brand} />
        <span class=${"skeleton order-3 h-9 w-full sm:order-none sm:w-56 " + (show ? "" : "invisible")}></span>
        <span class=${"skeleton ms-auto h-9 w-36 " + (show ? "" : "invisible")}></span>
      </div>
    </header>
    ${
      error
        ? html`<p class="p-8 text-center text-muted-foreground" role="alert">${error}</p>`
        : html`<div aria-busy="true" aria-label="Loading"
          class=${"mx-auto grid max-w-6xl gap-6 px-4 py-6 lg:grid-cols-[minmax(0,1.1fr)_minmax(0,1fr)] " + (show ? "fade-in" : "invisible")}>
          <div class="grid content-start gap-6"><span class="skeleton h-5 w-56"></span><span class="skeleton h-10 w-72"></span>
            <div class="bezel"><div></div></div></div>
          <div class="grid content-start gap-6"><span class="skeleton h-72"></span><span class="skeleton h-48"></span></div>
        </div>`
    }`;
}

const WALL_LEDE = {
  collage: "The most talked-about topic gets the big tile. Tiles fade as a topic stops coming up; pin one to keep it.",
  single: "Only the newest topic is on the frame. Pin it to keep it there for good.",
  rolling: "The newest topic comes first, and the oldest drops off when the frame is full. Pin one to keep it.",
};

function Wall({ state, hot, onHot, onTile, onClear, redrawing, onRedraw }) {
  const { wall, max_tiles: max } = state;
  const [asking, setAsking] = useState("");
  const [renaming, setRenaming] = useState("");
  return html`
    <${Card} title="On the frame now" intro=${WALL_LEDE[state.settings.layout]}>
      ${
        wall.length === 0
          ? html`<p class="rounded-lg border border-dashed p-4 text-center text-muted-foreground">No tiles yet. Topics show up here as soon as they are on the frame.</p>`
          : html`<ol class="grid gap-2">
            ${wall.map(
              (t, i) => html`
              <li key=${t.label} onMouseEnter=${() => onHot(t.label)} onMouseLeave=${() => onHot("")}
                class=${"flex flex-wrap items-center gap-3 rounded-lg border p-3 transition-shadow " + (t.pinned ? "border-ink-yellow bg-ink-yellow/10 " : "") + (hot === t.label ? "ring-2 ring-primary" : "")}>
                <span class=${"grid size-7 shrink-0 place-content-center rounded-md text-xs font-semibold " + (i === 0 ? "bg-primary text-primary-foreground" : "bg-muted text-muted-foreground")}
                  aria-label=${i === 0 ? "Big tile" : "Tile " + (i + 1)}>${i + 1}</span>
                <i class="h-12 w-16 shrink-0 rounded-sm bg-[#e9e1ca] ring-1 ring-black/15" aria-hidden="true" style=${thumb(t.rect, state.page)}></i>
                <div class="min-w-0 flex-1 basis-40">
                  ${
                    renaming === t.label
                      ? html`<form class="flex flex-wrap items-center gap-2" onSubmit=${(e) => {
                          e.preventDefault();
                          setRenaming("");
                          onTile("rename", t.label, e.target.elements.to.value);
                        }}
                        onKeyDown=${(e) => e.key === "Escape" && setRenaming("")}>
                        <input name="to" class="input min-w-0 flex-1" aria-label="New caption" value=${t.label} maxlength="120" required ref=${focus} />
                        <button type="submit" class="btn" data-size="sm">Save</button>
                        <button type="button" class="btn" data-size="sm" data-variant="ghost" onClick=${() => setRenaming("")}>Cancel</button>
                      </form>`
                      : html`<p class="text-lg font-semibold leading-snug tracking-tight break-words">${t.label}</p>`
                  }
                  <p class="text-muted-foreground">${t.pinned ? "Pinned. " : ""}Came up ${t.mentions === 1 ? "once" : t.mentions + " times"}, last ${when(t.last_seen)}</p>
                  <${Origin} t=${t} />
                </div>
                ${
                  asking === t.label
                    ? html`<div class="flex flex-wrap items-center gap-2" role="group" aria-label="Confirm remove" onKeyDown=${(e) => e.key === "Escape" && setAsking("")}>
                      <span>Remove “${t.label}” and keep it off the frame for a week?</span>
                      <button type="button" class="btn" data-size="sm" data-variant="destructive" onClick=${() => {
                        setAsking("");
                        onTile("remove", t.label);
                      }}>Remove</button>
                      <button type="button" class="btn" data-size="sm" data-variant="ghost" ref=${focus} onClick=${() => setAsking("")}>Keep</button>
                    </div>`
                    : html`<div class="flex gap-2">
                      <button type="button" class=${"btn " + (t.pinned ? "bg-ink-yellow text-neutral-900 hover:bg-ink-yellow/80" : "")} data-size="sm"
                        data-variant=${t.pinned ? undefined : "outline"} aria-pressed=${t.pinned}
                        onClick=${() => onTile(t.pinned ? "unpin" : "pin", t.label)}>
                        <${PinIcon} /> ${t.pinned ? "Pinned" : "Pin"}</button>
                      <${Redraw} label=${t.label} redrawing=${redrawing} onRedraw=${onRedraw} />
                      <button type="button" class="btn" data-size="sm" data-variant="ghost" onClick=${() => setRenaming(t.label)}>Rename</button>
                      <button type="button" class="btn" data-size="sm" data-variant="ghost" onClick=${() => setAsking(t.label)}>Remove</button>
                    </div>`
                }
              </li>`,
            )}
          </ol>`
      }
      <div class="flex flex-wrap items-center justify-between gap-2">
        <p class="text-muted-foreground">${wall.length > 0 && wall.length < max ? `Room for ${max - wall.length} more.` : ""}</p>
        ${
          wall.some((t) => !t.pinned) &&
          (asking === "*"
            ? html`<div class="flex flex-wrap items-center gap-2" role="group" aria-label="Confirm clear" onKeyDown=${(e) => e.key === "Escape" && setAsking("")}>
              <span>Take ${wall.filter((t) => !t.pinned).length} unpinned pictures off the frame?</span>
              <button type="button" class="btn" data-size="sm" data-variant="destructive" onClick=${() => {
                setAsking("");
                onClear();
              }}>Clear the frame</button>
              <button type="button" class="btn" data-size="sm" data-variant="ghost" ref=${focus} onClick=${() => setAsking("")}>Keep</button></div>`
            : html`<button type="button" class="btn" data-size="sm" data-variant="outline" onClick=${() => setAsking("*")}>Clear the frame</button>`)
        }
      </div>
    <//>`;
}

// Redraw makes a new picture from the same description, in the current
// style. One at a time: the others wait while one is drawn.
function Redraw({ label, redrawing, onRedraw }) {
  const busy = redrawing === label;
  return html`<button type="button" class="btn" data-size="sm" data-variant="outline" disabled=${redrawing !== ""}
    aria-label=${"Redraw " + label} onClick=${() => onRedraw(label)}>
    ${busy && html`<span class="loader size-3.5" aria-hidden="true"></span>`}${busy ? "Redrawing…" : "Redraw"}</button>`;
}

// Origin is the description the picture was made from (what the image API
// got, never what was said), with its style and when it was made.
function Origin({ t }) {
  if (!t.prompt) return null;
  return html`<p class="text-sm text-muted-foreground"><span class="block italic">${t.prompt}</span>
    <span class="block">${t.style ? `${t.style}, ` : ""}${when(t.made)}</span></p>`;
}

// Queue lists the new pictures waiting for their turn on the frame, the next
// one first, so they can be moved up, put up now or dropped.
function Queue({ state, onLine, redrawing, onRedraw }) {
  const { queue, next_at: nextAt, settings } = state;
  const [asking, setAsking] = useState("");
  if (queue.length === 0) return null;
  const why =
    settings.quiet_from && settings.new_every_minutes === 0
      ? "Quiet hours hold them back."
      : `One goes up at most every ${settings.new_every_minutes >= 60 ? settings.new_every_minutes / 60 + " h" : settings.new_every_minutes + " min"}${settings.quiet_from ? ", never in quiet hours" : ""}.`;
  return html`
    <${Card} title="Queue" intro=${`${why} Next one ${when(nextAt)}.`}>
      <ol class="grid gap-2">
        ${queue.map(
          (t, i) => html`
          <li key=${t.label} class="flex flex-wrap items-center gap-3 rounded-lg border border-dashed p-3">
            <span class="grid size-7 shrink-0 place-content-center rounded-md bg-muted text-xs font-semibold text-muted-foreground" aria-label=${"Place " + (i + 1) + " in line"}>${i + 1}</span>
            <img class="h-12 w-16 shrink-0 rounded-sm object-cover ring-1 ring-black/15" alt="" loading="lazy"
              src=${"api/picture.png?label=" + encodeURIComponent(t.label) + "&made=" + encodeURIComponent(t.made)} />
            <div class="min-w-0 flex-1 basis-40">
              <p class="text-lg font-semibold leading-snug tracking-tight break-words">${t.label}</p>
              <${Origin} t=${t} />
            </div>
            ${
              asking === t.label
                ? html`<div class="flex flex-wrap items-center gap-2" role="group" aria-label="Confirm drop" onKeyDown=${(e) => e.key === "Escape" && setAsking("")}>
                  <span>Drop “${t.label}” and keep the topic off the frame for a week?</span>
                  <button type="button" class="btn" data-size="sm" data-variant="destructive" onClick=${() => {
                    setAsking("");
                    onLine("drop", t.label, "Picture dropped.");
                  }}>Drop</button>
                  <button type="button" class="btn" data-size="sm" data-variant="ghost" ref=${focus} onClick=${() => setAsking("")}>Keep</button>
                </div>`
                : html`<div class="flex flex-wrap gap-2">
                  <button type="button" class="btn" data-size="sm" onClick=${() => onLine("now", t.label, "Put up on the frame.")}>Put up now</button>
                  ${i > 0 && html`<button type="button" class="btn" data-size="sm" data-variant="outline" aria-label=${"Move " + t.label + " up"} onClick=${() => onLine("up", t.label)}>Move up</button>`}
                  <${Redraw} label=${t.label} redrawing=${redrawing} onRedraw=${onRedraw} />
                  <button type="button" class="btn" data-size="sm" data-variant="ghost" onClick=${() => setAsking(t.label)}>Drop</button>
                </div>`
            }
          </li>`,
        )}
      </ol>
    <//>`;
}

// Timeline shows every saved picture, newest first, grouped by day in
// justified rows (Flickr's justified-layout), with a date scrubber at the
// side and a full-screen viewer (PhotoSwipe) that shows each picture's
// topic, description, style and time.
function Timeline({ onState, onSignedOut }) {
  const [data, setData] = useState(null);
  const [error, setError] = useState("");
  const [asking, setAsking] = useState(false);
  const [width, setWidth] = useState(0);
  const box = useRef(null);

  const fail = (e) => (e.signin ? onSignedOut() : setError(e.message));
  const load = useCallback(
    () =>
      api("api/timeline").then((d) => {
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
    const ho = new ResizeObserver(() => document.documentElement.style.setProperty("--head", head.offsetHeight + "px"));
    if (head) ho.observe(head);
    return () => {
      ro.disconnect();
      ho.disconnect();
    };
  }, [data !== null]);

  const keep = async (on) => {
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
      setData(await api("api/timeline/delete", "POST", {}));
    } catch (e) {
      fail(e);
    }
  };

  const pics = data?.pictures ?? [];
  const days = [];
  for (const p of pics) {
    const k = new Date(p.made).toDateString();
    if (days.at(-1)?.key !== k) days.push({ key: k, date: new Date(p.made), pics: [] });
    days.at(-1).pics.push(p);
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

  const open = (index) => {
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
      pswp.ui.registerElement({
        name: "info",
        appendTo: "root",
        order: 9,
        onInit: (el) => {
          el.className = "pswp-info";
          const show = () => {
            const p = pswp.currSlide?.data.pic;
            if (!p) return;
            const line = (tag, cls, text) =>
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

  return html`
    <main id="panel-timeline" role="tabpanel" aria-labelledby="tab-timeline" class="mx-auto grid max-w-6xl gap-4 py-6 ps-4 pe-10 sm:pe-20">
      <div class="flex flex-wrap items-end justify-between gap-3">
        <div class="grid gap-1">
          <h2 class="text-lg font-semibold tracking-tight">Timeline</h2>
          <p class="text-muted-foreground">${pics.length === 1 ? "1 picture" : pics.length + " pictures"}${days.length > 1 ? ` over ${days.length} days` : ""}</p>
        </div>
        ${
          data?.saved &&
          html`<div class="flex flex-wrap items-center gap-3">
          <label class="flex items-center gap-2 text-sm font-medium"><input type="checkbox" role="switch" class="input"
            checked=${data.on} onChange=${(e) => keep(e.target.checked)} />Save new pictures</label>
          ${
            pics.length > 0 &&
            (asking
              ? html`<div class="flex flex-wrap items-center gap-2" role="group" aria-label="Confirm delete" onKeyDown=${(e) => e.key === "Escape" && setAsking(false)}>
                <span>Delete all ${pics.length} saved pictures?</span>
                <button type="button" class="btn" data-size="sm" data-variant="destructive" onClick=${remove}>Delete all</button>
                <button type="button" class="btn" data-size="sm" data-variant="ghost" ref=${focus} onClick=${() => setAsking(false)}>Keep</button></div>`
              : html`<button type="button" class="btn" data-size="sm" data-variant="outline" onClick=${() => setAsking(true)}>Delete all</button>`)
          }
        </div>`
        }
      </div>
      ${error && html`<div class="alert" data-variant="destructive" role="alert"><section>${error}</section></div>`}
      ${data && !data.saved && html`<${Note}>The timeline needs a settings file: start E-maki with -settings.<//>`}
      ${
        data?.saved &&
        pics.length === 0 &&
        html`<p class="rounded-lg border border-dashed p-8 text-center text-muted-foreground">
        ${data.on ? "No pictures yet. Each new picture lands here, day by day." : "Saving is off."}</p>`
      }
      <div ref=${box} class="grid gap-8">
        ${days.map(
          (d) => html`
          <section key=${d.key} data-day=${d.key} aria-label=${longDate(d.date)}>
            <h3 class="sticky top-(--head) z-[5] -mx-1 mb-2 bg-background/90 px-1 py-1 font-semibold backdrop-blur-sm">${longDate(d.date)}
              <span class="ms-2 text-sm font-normal text-muted-foreground">${d.pics.length}</span></h3>
            <div class="relative" style=${{ height: d.layout.containerHeight + "px" }}>
              ${d.layout.boxes.map((b, i) => {
                const p = d.pics[i];
                return html`<button type="button" key=${p.id} data-made=${p.made} onClick=${() => open(d.first + i)}
                  class="absolute overflow-hidden rounded-sm bg-muted focus-visible:ring-2 focus-visible:ring-ring focus-visible:outline-none"
                  style=${{ left: b.left + "px", top: b.top + "px", width: b.width + "px", height: b.height + "px" }}
                  aria-label=${`${p.label}, ${new Date(p.made).toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" })}`}>
                  <img class="size-full object-cover transition-transform duration-200 hover:scale-[1.03]" src=${"api/timeline/" + p.id + "?size=thumb"}
                    alt="" loading="lazy" decoding="async" /></button>`;
              })}
            </div>
          </section>`,
        )}
      </div>
      ${pics.length > 0 && html`<${Scrubber} count=${pics.length} box=${box} width=${width} />`}
    </main>`;
}

const headHeight = () => document.querySelector("header")?.offsetHeight ?? 0;

function longDate(d, time) {
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
function Scrubber({ count, box, width }) {
  const bar = useRef(null);
  const [marks, setMarks] = useState([]);
  const [times, setTimes] = useState([]); // where each picture starts, with when it was made
  const [pos, setPos] = useState(0);
  const [hover, setHover] = useState(null); // [fraction, day label] while pointing
  const span = () => {
    const r = box.current.getBoundingClientRect();
    return { top: r.top + scrollY - headHeight(), height: Math.max(1, r.height) };
  };
  useEffect(() => {
    const measure = () => {
      if (!box.current) return;
      const { top, height } = span();
      const at = (el) => (el.getBoundingClientRect().top + scrollY - headHeight() - top) / height;
      setTimes(
        [...box.current.querySelectorAll("[data-made]")]
          .map((el) => ({ at: at(el), long: longDate(new Date(el.dataset.made), true) }))
          .sort((a, b) => a.at - b.at),
      );
      setMarks(
        [...box.current.querySelectorAll("[data-day]")].map((el) => ({
          key: el.dataset.day,
          at: at(el),
          short: new Date(el.dataset.day).toLocaleDateString([], { day: "numeric", month: "short" }),
          long: longDate(new Date(el.dataset.day)),
        })),
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
  const frac = (e) => {
    const r = bar.current.getBoundingClientRect();
    return Math.min(1, Math.max(0, (e.clientY - r.top) / r.height));
  };
  const timeAt = (f) => times.filter((m) => m.at <= f + 0.001).at(-1) ?? times[0];
  const go = (f) => {
    const { top, height } = span();
    scrollTo({ top: top + f * height });
  };
  const move = (e) => {
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
  return html`
    <nav aria-label="Jump to a day" ref=${bar}
      class="fixed top-[calc(var(--head)+1rem)] right-0 bottom-24 z-[6] w-9 cursor-ns-resize sm:right-1 sm:w-16 touch-none select-none sm:bottom-6"
      onPointerDown=${(e) => {
        e.currentTarget.setPointerCapture(e.pointerId);
        go(frac(e));
      }}
      onPointerMove=${move} onPointerLeave=${() => setHover(null)}>
      ${shown.map(
        (m) => html`<button type="button" key=${m.key} tabindex="-1" aria-label=${m.long}
        class="absolute right-1 hidden text-xs tabular-nums sm:block text-muted-foreground hover:text-foreground" style=${{ top: m.at * 100 + "%" }}
        onClick=${() => go(m.at)}>${m.short}</button>`,
      )}
      <span aria-hidden="true" class="absolute inset-y-0 right-1 w-px bg-border"></span>
      <span aria-hidden="true" class="absolute right-0 h-0.5 w-8 rounded bg-primary" style=${{ top: pos * 100 + "%" }}></span>
      ${
        hover &&
        html`<span aria-hidden="true" class="absolute right-full me-1 -translate-y-1/2 whitespace-nowrap rounded-md bg-popover px-2 py-1 text-sm shadow ring-1 ring-border"
        style=${{ top: hover[0] * 100 + "%" }}>${hover[1]}</span>`
      }
    </nav>`;
}

// Removed lists the topics taken off the frame that are kept away for a
// while, so one removed by mistake can come back.
function Removed({ removed, onAllow }) {
  if (!removed?.length) return null;
  return html`
    <${Card} title="Removed topics" intro="These stay off the frame until the date shown, even when they come up again.">
      <ul class="grid gap-2">
        ${removed.map(
          (r) => html`
          <li key=${r.label} class="flex flex-wrap items-center gap-3 rounded-lg border p-3">
            <div class="min-w-0 flex-1 basis-40">
              <p class="font-medium break-words">${r.label}</p>
              <p class="text-sm text-muted-foreground">Kept away until ${new Date(r.until).toLocaleDateString(undefined, { weekday: "short", day: "numeric", month: "short" })}</p>
            </div>
            <button type="button" class="btn" data-size="sm" data-variant="outline" onClick=${() => onAllow(r.label)}>Allow again</button>
          </li>`,
        )}
      </ul>
    <//>`;
}

// Logs shows E-maki's own recent log, newest at the bottom, from memory.
function Logs() {
  const [lines, setLines] = useState([]);
  const [filter, setFilter] = useState("");
  const [follow, setFollow] = useState(true);
  const [error, setError] = useState("");
  const [copied, setCopied] = useState(false);
  const box = useRef(null);
  useEffect(() => {
    let on = true,
      last = 0;
    const get = async () => {
      try {
        const add = await api("api/logs?after=" + last);
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
  const tone = (t) =>
    /error|fail|can't|cannot|refused|warning/i.test(t)
      ? "text-destructive"
      : /rejected|held back|dropped|skipped/i.test(t)
        ? "text-muted-foreground"
        : "";
  return html`
    <main id="panel-logs" role="tabpanel" aria-labelledby="tab-logs" class="mx-auto grid max-w-6xl gap-4 px-4 py-6">
      <div class="flex flex-wrap items-end justify-between gap-3">
        <div class="grid gap-1">
          <h2 class="text-lg font-semibold tracking-tight">Logs</h2>
          <p class="max-w-prose text-muted-foreground">The last ${lines.length} lines E-maki wrote, kept in memory only. Loudness, timings, topics and picture descriptions, never what was said.</p>
        </div>
        <div class="flex flex-wrap items-center gap-3">
          <label class="flex items-center gap-2 text-sm font-medium">Filter<input class="input w-56" type="search" placeholder="topic, microphone" value=${filter} onInput=${(e) => setFilter(e.target.value)} /></label>
          <label class="flex items-center gap-2 text-sm"><input type="checkbox" role="switch" class="input" checked=${follow} onChange=${(e) => setFollow(e.target.checked)} />Follow new lines</label>
          ${
            navigator.clipboard &&
            html`<button type="button" class="btn" data-size="sm" data-variant="outline" disabled=${shown.length === 0}
            onClick=${async () => {
              try {
                await navigator.clipboard.writeText(shown.map((l) => l.text).join("\n") + "\n");
                setCopied(true);
                setTimeout(() => setCopied(false), 2000);
              } catch (e) {
                setError("Can't copy: " + e.message);
              }
            }}>${copied ? "Copied" : "Copy"}</button>`
          }
          <a class="btn" data-size="sm" data-variant="outline" href="api/logs.txt" download>Download</a>
        </div>
      </div>
      ${error && html`<div class="alert" data-variant="destructive" role="alert"><section>${error}</section></div>`}
      <div ref=${box} class="h-[65dvh] overflow-auto rounded-xl border bg-muted/40 p-3 font-mono text-sm leading-relaxed" role="log" aria-live="polite">
        ${
          shown.length === 0
            ? html`<p class="p-4 text-center text-muted-foreground">${lines.length === 0 ? "Nothing logged yet." : "No line matches the filter."}</p>`
            : shown.map(
                (l) => html`<div key=${l.n} class=${"whitespace-pre-wrap break-words " + tone(l.text)}>${l.text}</div>`,
              )
        }
      </div>
    </main>`;
}

// thumb crops a tile out of the frame's picture, as a CSS background.
function thumb([x, y, w, h], page) {
  const at = (pos, size) => (size >= 1 ? 0 : (pos / (1 - size)) * 100) + "%";
  return {
    backgroundImage: `url(preview.png?v=${encodeURIComponent(page)})`,
    backgroundSize: `${100 / w}% ${100 / h}%`,
    backgroundPosition: `${at(x, w)} ${at(y, h)}`,
  };
}

function PinIcon() {
  return html`<svg viewBox="0 0 16 16" width="14" height="14" aria-hidden="true"><path d="M5 1.5h6l-1 4.5 2.5 2.5V10h-9V8.5L6 6zM7.4 10h1.2v4l-.6 1.2-.6-1.2z" fill="currentColor"/></svg>`;
}

function Card({ title, intro, children }) {
  return html`
    <section class="card">
      <header>
        <h2 class="text-lg font-semibold tracking-tight">${title}</h2>
        ${intro && html`<p>${intro}</p>`}
      </header>
      <section class="grid gap-5">${children}</section>
    </section>`;
}

// Field is a settings row: the label and its help on the left, the control
// (and its value) on the right. On a phone, or with stack, they stack.
function Field({ label, value, help, stack, children }) {
  return html`
    <label class=${"grid gap-2 " + (stack ? "" : "sm:grid-cols-[minmax(0,2fr)_minmax(0,3fr)] sm:items-center sm:gap-x-8")}>
      <span class="grid gap-0.5"><span class="font-medium">${label}</span>
        ${help && html`<small class="text-muted-foreground">${help}</small>`}</span>
      <span class="flex items-center gap-3"><span class="min-w-0 flex-1">${children}</span>
        ${value !== undefined && html`<span class="shrink-0 text-sm tabular-nums text-muted-foreground">${value}</span>`}</span>
    </label>`;
}

// Range is a slider whose filled part follows its value (Basecoat reads --slider-value).
function Range(props) {
  const { min, max, value } = props;
  return html`<input class="input w-full" type="range" ...${props} style=${{ "--slider-value": ((value - min) / (max - min)) * 100 + "%" }} />`;
}

// Toggle is an on/off switch with a title and an explanation.
function Toggle({ checked, onChange, title, text, disabled }) {
  return html`
    <label class="flex items-start justify-between gap-4">
      <span class="grid gap-0.5"><span class="font-medium">${title}</span>
        ${text && html`<small class="text-muted-foreground">${text}</small>`}</span>
      <input type="checkbox" role="switch" class="input mt-0.5" checked=${checked} disabled=${disabled} onChange=${(e) => onChange(e.target.checked)} />
    </label>`;
}

// Choice is one option in a group of big radio cards.
function Choice({ name, checked, onChange, children }) {
  return html`
    <label class="flex cursor-pointer gap-3 rounded-lg border p-3 transition-colors hover:bg-accent has-checked:border-primary has-checked:bg-primary/5 has-checked:ring-1 has-checked:ring-primary has-focus-visible:ring-2 has-focus-visible:ring-ring">
      <input type="radio" name=${name} class="sr-only" checked=${checked} onChange=${onChange} />
      ${children}
    </label>`;
}

function Segments({ name, options, value, onChange, label, inline }) {
  return html`
    <div class="grid gap-2" role="radiogroup" aria-label=${label}>
      ${!inline && html`<span class="font-medium">${label}</span>`}
      <div class="inline-flex w-fit rounded-lg bg-muted p-1">
        ${options.map(
          ([v, text]) => html`
          <label class="tap cursor-pointer rounded-md px-3 py-1.5 text-sm sm:px-4 font-medium text-muted-foreground has-checked:bg-background has-checked:text-foreground has-checked:shadow-sm has-focus-visible:ring-2 has-focus-visible:ring-ring">
            <input type="radio" name=${name} class="sr-only" checked=${value === v} onChange=${() => onChange(v)} />${text}</label>`,
        )}
      </div>
    </div>`;
}

function Note({ tone, children }) {
  const color =
    { ok: "border-ink-green/40 bg-ink-green/10", warn: "border-ink-yellow/60 bg-ink-yellow/15" }[tone] || "bg-muted/50";
  return html`<p class=${"rounded-lg border px-3 py-2 " + color} role=${tone ? "status" : undefined}>${children}</p>`;
}

const SECTIONS = [
  ["listening", "Microphone"],
  ["topics", "Topic detection"],
  ["privacy", "Privacy"],
  ["pictures", "AI image provider"],
  ["frame", "E-ink frame"],
  ["security", "Security"],
  ["system", "System status"],
];

function EmakiTab({ state, draft, set, paused, onState, onSignedOut }) {
  const initial = SECTIONS.find(([k]) => location.hash === "#emaki/" + k)?.[0] ?? "listening";
  const [section, setSection] = useState(initial);
  const go = (k) => {
    setSection(k);
    history.replaceState(null, "", "#emaki/" + k);
  };
  return html`
    <main id="panel-emaki" role="tabpanel" aria-labelledby="tab-emaki"
      class="mx-auto grid max-w-6xl gap-6 px-4 py-6 md:grid-cols-[15rem_minmax(0,1fr)] md:items-start">
      <nav aria-label="E-maki settings" class="flex flex-wrap gap-1 md:sticky md:top-20 md:flex-col md:flex-nowrap">
        ${SECTIONS.map(
          ([k, name]) => html`
          <button type="button" aria-current=${section === k ? "page" : undefined} onClick=${() => go(k)}
            class=${
              "flex shrink-0 items-center gap-2 rounded-lg px-3 py-2 text-left text-sm font-medium transition-colors hover:bg-accent " +
              (section === k ? "bg-accent" : "")
            }>
            ${section === k && html`<i class="size-[7px] shrink-0 rounded-full bg-primary" aria-hidden="true"></i>`}${name}
          </button>`,
        )}
      </nav>
      <div class="grid max-w-3xl gap-6">
        ${section === "listening" && html`<${Listening} state=${state} draft=${draft} set=${set} paused=${paused} />`}
        ${section === "topics" && html`<${Topics} state=${state} draft=${draft} set=${set} />`}
        ${section === "privacy" && html`<${Privacy} draft=${draft} set=${set} saved=${state.status.saved} topicAPI=${state.status.topic_api} />`}
        ${section === "pictures" && html`<${Pictures} state=${state} draft=${draft} set=${set} />`}
        ${section === "frame" && html`<${Hanging} draft=${draft} set=${set} />`}
        ${section === "security" && html`<${Security} state=${state} onState=${onState} onSignedOut=${onSignedOut} />`}
        ${section === "system" && html`<${System} state=${state} />`}
      </div>
    </main>`;
}

function SignIn({ onDone }) {
  const [password, setPassword] = useState("");
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  const submit = async (e) => {
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
  return html`
    <main class="grid min-h-dvh place-items-center p-4">
      <form class="card w-full max-w-sm" onSubmit=${submit}>
        <header><${Brand} /><p>This page is locked with a password. The frame keeps showing its picture either way.</p></header>
        <section class="grid gap-4">
          <${Field} label="Password">
            <input class="input" type="password" autocomplete="current-password" autofocus value=${password}
              onInput=${(e) => setPassword(e.target.value)} /><//>
          ${error && html`<div class="alert" data-variant="destructive" role="alert"><section>${error}</section></div>`}
          <button class="btn w-full" type="submit" disabled=${busy || !password}>${busy ? "Signing in…" : "Sign in"}</button>
          <p class="text-muted-foreground">Forgot it? Delete <code>password.json</code> next to the settings file (in docker: the <code>emaki-data</code> volume) and restart E-maki.</p>
        </section>
      </form>
    </main>`;
}

function Listening({ state, draft, set, paused }) {
  const a = state.activity;
  // Go sends the zero time (year 1) until a first segment is heard.
  const level = a.last_at && !a.last_at.startsWith("0001-") ? a.last_level : null;
  return html`
    <${Microphone} draft=${draft} set=${set} paused=${paused} />
    <${Card} title="Speech threshold"
      intro="Sound quieter than the threshold is skipped without being transcribed, because Whisper tends to invent words in silence.">
      <${LevelMeter} level=${level} threshold=${draft.min_speech_dbfs} />
      <${Field} label="Threshold" value=${draft.min_speech_dbfs + " dB"}
        help=${
          (level === null
            ? "Nothing heard yet."
            : `Last heard: ${Math.round(level)} dB, ${when(a.last_at)}${a.last_lang ? `, in ${langName(a.last_lang)}` : ""}.`) +
          " 0 dB is the loudest the microphone can record, -80 dB is silence. Set the threshold a little above a quiet room."
        }>
        <${Range} min="-80" max="-10" step="1" value=${draft.min_speech_dbfs}
          onInput=${(e) => set({ min_speech_dbfs: +e.target.value })} /><//>
      <p class="text-muted-foreground">Each segment is ${state.status.segment} seconds of audio. That is set in the profile (<code>segment_seconds</code>) and needs a restart.</p>
    <//>
    <${Languages} state=${state} draft=${draft} set=${set} />`;
}

// Languages lists the ones ticked and those with their own speech server;
// any other language Whisper knows can be added from the list.
function Languages({ state, draft, set }) {
  const on = draft.languages;
  const toggle = (l) => set({ languages: on.includes(l) ? on.filter((x) => x !== l) : [...on, l] });
  const own = state.status.by_lang || {};
  const name = (code) => {
    const n = langName(code);
    return n === code ? (state.languages.find(([c]) => c === code)?.[1] ?? code) : n;
  };
  const rows = [...on, ...Object.keys(own).filter((l) => !on.includes(l))];
  const rest = state.languages
    .map(([c]) => c)
    .filter((c) => !rows.includes(c))
    .sort((x, y) => name(x).localeCompare(name(y)));
  return html`
    <${Card} title="Languages" intro="The languages spoken here. With none, Whisper guesses from all it knows.">
      ${rows.map(
        (l) => html`
        <${Toggle} checked=${on.includes(l)} onChange=${() => toggle(l)} title=${name(l)} text=${own[l] ? "Own speech model" : ""} />`,
      )}
      <select class="select" value="" aria-label="Add a language" onChange=${(e) => e.target.value && toggle(e.target.value)}>
        <option value="">Add a language</option>
        ${rest.map((c) => html`<option value=${c}>${name(c)}</option>`)}
      </select>
      ${on.length > 1 && html`<p class="text-muted-foreground">A guess outside these is redone in ${name(on[0])}, the first one.</p>`}
    <//>`;
}

// Microphone lists the inputs the machine offers, marks the one E-maki
// listens on and shows its loudness live. Only names and a level come from
// the server, never audio.
function Microphone({ draft, set, paused }) {
  const [mic, setMic] = useState(null);
  useEffect(() => {
    let on = true;
    const get = () =>
      api("api/mic")
        .then((m) => on && setMic(m))
        .catch(() => {});
    get();
    const t = setInterval(get, 1000);
    return () => {
      on = false;
      clearInterval(t);
    };
  }, []);
  const intro = "Plug in another mic and it shows up here within a few seconds.";
  if (!mic) {
    return html`<${Card} title="Microphone" intro=${intro}>
      <div class="grid gap-2" aria-busy="true" aria-label="Looking for inputs"><span class="skeleton h-16"></span><span class="skeleton h-16"></span></div><//>`;
  }
  const pick = draft.mic_device;
  const fallback = mic.devices.find((d) => d.default);
  const rows = [
    ["", "System default", fallback ? fallback.name : "Whatever the system picks"],
    ...mic.devices.map((d) => [d.name, d.name, d.default ? "The system default right now" : ""]),
  ];
  const missing = pick !== "" && !mic.devices.some((d) => d.name === pick);
  if (missing) rows.push([pick, pick, "Not connected. E-maki uses the default until it is back."]);
  // The row that is listening: the picked one if it is open, else the default's own row.
  const inUse = (v) =>
    mic.live &&
    mic.active !== "" &&
    (v === mic.active || (v === "" && pick === "" && !mic.devices.some((d) => d.name === mic.active)));
  const pos = (db) => Math.max(0, Math.min(100, ((db + 80) / 80) * 100));
  return html`
    <${Card} title="Microphone" intro=${intro}>
      ${!mic.live && html`<${Note}>This run is not listening on a microphone (it plays a recording or a script). Your choice is used the next time E-maki starts with one.<//>`}
      ${mic.error && html`<${Note} tone="warn">${mic.error}<//>`}
      ${mic.devices.length === 0 && !mic.error && html`<${Note} tone="warn">No inputs found. In docker, check that the sound server is passed into the container.<//>`}
      <div class="grid gap-2" role="radiogroup" aria-label="Microphone">
        ${rows.map(
          ([v, name, text]) => html`
          <${Choice} name="mic" checked=${pick === v} onChange=${() => set({ mic_device: v })}>
            <span class="grid min-w-0 flex-1 gap-1">
              <span class="flex flex-wrap items-center gap-2 font-medium"><span class="break-words">${name}</span>
                ${inUse(v) && html`<span class="rounded-full bg-ink-green px-2 py-0.5 text-xs text-white dark:text-neutral-900">${paused ? "In use, paused" : "Listening"}</span>`}</span>
              ${text && html`<small class=${missing && v === pick ? "text-destructive" : "text-muted-foreground"}>${text}</small>`}
              ${
                inUse(v) &&
                mic.level != null &&
                html`<span class="relative mt-1 block h-2 rounded-full bg-muted" role="img" aria-label=${`Level now ${Math.round(mic.level)} dB`}>
                <i class="absolute inset-0 origin-left rounded-full bg-ink-green transition-transform duration-200" style=${{ transform: `scaleX(${pos(mic.level) / 100})` }}></i>
                <b class="absolute -inset-y-1 w-0.5 rounded bg-primary" style=${{ left: pos(draft.min_speech_dbfs) + "%" }}></b></span>`
              }
            </span>
          <//>`,
        )}
      </div>
      <p class="text-muted-foreground">The bar is the level right now. The line is where talk starts.</p>
    <//>`;
}

function langName(code) {
  try {
    return new Intl.DisplayNames(["en"], { type: "language" }).of(code) ?? code;
  } catch {
    return code;
  }
}

// LevelMeter shows the last segment's loudness against the speech line, on a -80 to 0 dBFS scale.
function LevelMeter({ level, threshold }) {
  const pos = (db) => Math.max(0, Math.min(100, ((db + 80) / 80) * 100));
  return html`
    <div class="grid gap-1">
      <div class="relative h-3 rounded-full bg-muted" role="img"
        aria-label=${level === null ? "No level yet" : `Last heard ${Math.round(level)} dB, threshold ${threshold} dB`}>
        <i class=${"absolute inset-0 origin-left rounded-full transition-transform duration-200 " + (level !== null && level >= threshold ? "bg-ink-green" : "bg-muted-foreground/40")}
          style=${{ transform: `scaleX(${level === null ? 0 : pos(level) / 100})` }}></i>
        <b class="absolute -inset-y-1 w-0.5 rounded bg-primary" style=${{ left: pos(threshold) + "%" }}></b>
      </div>
      <div class="flex justify-between text-xs text-muted-foreground" aria-hidden="true"><span>Silent</span><span>Loud</span></div>
    </div>`;
}

// diaryNote says what diary mode has collected so far, and when it paints.
function diaryNote(s) {
  const base =
    "Topics talked about at least twice wait for this time. The heaviest get pictures, within the daily limit.";
  if (!s.diary_next) return base;
  const n = s.collected === 1 ? "1 topic" : `${s.collected} topics`;
  return `${base} ${n} so far, painted ${when(s.diary_next)}.`;
}

function Topics({ state, draft, set }) {
  const modes = [
    ["diary", "Diary", "Collects the day's topics and paints the top ones once a day. For everyday life."],
    ["party", "Continuous", "Turns each new topic into a picture straight away."],
  ];
  const seconds = state.status.segment;
  const minutes = (n) => {
    const m = (n * seconds) / 60;
    return m >= 1 ? `${Math.round(m * 10) / 10} min` : `${n * seconds} s`;
  };
  const temp = draft.llm_temperature ?? 0.2;
  const st = state.status;
  const external = draft.topic_provider === "external";
  return html`
    <${Card} title="Topic model">
      <${Field} label="Provider"
        help=${
          external
            ? ""
            : "Runs on this machine. What was said stays here." +
              (
                st.topic_api
                  ? ""
                  : " To use an external service instead, set TOPIC_API_KEY and TOPIC_API_MODEL in .env and restart E-maki."
              )
        }>
        <select class="select" value=${external ? "external" : "local"} onChange=${(e) => set({ topic_provider: e.target.value })}>
          <option value="local">On this machine: ${st.topics}</option>
          ${st.topic_api && html`<option value="external">External: ${st.topic_api}</option>`}
        </select><//>
      ${
        external &&
        html`<${Note} tone="warn"><strong class="font-semibold">What is said in the room leaves your network.</strong>${" "}
        Every ${minutes(draft.extract_every)}, the last ${minutes(draft.window_segments)} of transcript is sent to ${st.topic_api}.
        The privacy filters still decide which pictures get made, but the service has already read the text. Tell the people in the room.<//>`
      }
    <//>
    <${Card} title="Mode">
      <div class="grid gap-3 sm:grid-cols-2" role="radiogroup" aria-label="Mode">
        ${modes.map(
          ([v, name, text]) => html`
          <${Choice} name="mode" checked=${draft.mode === v} onChange=${() => set({ mode: v })}>
            <span class="grid gap-1"><span class="font-medium">${name}</span><small class="text-muted-foreground">${text}</small></span>
          <//>`,
        )}
      </div>
      ${
        draft.mode === "diary" &&
        html`
        <${Field} label="Paint at" help=${diaryNote(state)}>
          <input class="input w-32" type="time" value=${draft.diary_at} onChange=${(e) => set({ diary_at: e.target.value || "07:00" })} /><//>`
      }
      <${Field} label="Minimum confidence" value=${Math.round(draft.min_confidence * 100) + "%"}
        help="Higher means fewer, clearer topics. Lower catches more of the conversation.">
        <${Range} min="0" max="1" step="0.05" value=${draft.min_confidence}
          onInput=${(e) => set({ min_confidence: +e.target.value })} /><//>
    <//>
    <${Card} title="Transcript window">
      <${Field} label="Window length" value=${`${draft.window_segments} segments, ${minutes(draft.window_segments)}`}
        help="Longer gives the model more context; shorter follows the conversation faster.">
        <${Range} min="1" max="40" step="1" value=${draft.window_segments}
          onInput=${(e) => set({ window_segments: +e.target.value })} /><//>
      <${Field} label="Topic check interval" value=${`${draft.extract_every} segments, ${minutes(draft.extract_every)}`}
        help="More often reacts faster but keeps the topic model busier.">
        <${Range} min="1" max="20" step="1" value=${draft.extract_every}
          onInput=${(e) => set({ extract_every: +e.target.value })} /><//>
    <//>
    <${Card} title="Topic prompt"
      intro="What the topic model is told a good topic and picture description look like. The privacy rules and answer format are always added.">
      <details open=${!!draft.topic_prompt}>
        <summary class="cursor-pointer font-medium">${draft.topic_prompt ? "Custom prompt in use" : "Built-in prompt in use"}
          <span class="ms-1 font-normal text-muted-foreground">(show and edit)</span></summary>
        <textarea class="textarea mt-3 font-mono text-xs" rows="9" maxlength="8000" aria-label="Topic prompt" value=${draft.topic_prompt || state.default_prompt}
          onInput=${(e) => set({ topic_prompt: e.target.value === state.default_prompt ? "" : e.target.value })}></textarea>
      </details>
      ${draft.topic_prompt && html`<div><button type="button" class="btn" data-size="sm" data-variant="outline" onClick=${() => set({ topic_prompt: "" })}>Reset to built-in prompt</button></div>`}
      <${Field} label="Temperature" value=${temp.toFixed(1)} help="0 gives the most predictable topics; higher gives more surprising ones.">
        <${Range} min="0" max="1.5" step="0.1" value=${temp} onInput=${(e) => set({ llm_temperature: +e.target.value })} /><//>
    <//>`;
}

// provider is the image provider image_provider picks: "" is the first one
// that is set up, as in Go's imagegen.Pick.
function provider(providers = [], id) {
  return providers.find((p) => p.ready && (id === "" || p.id === id));
}

function Pictures({ state, draft, set }) {
  const cap = draft.daily_image_cap;
  const today = state.images_today;
  const providers = state.providers || [];
  const ready = providers.filter((p) => p.ready);
  const auto = ready[0];
  const chosen = provider(providers, draft.image_provider);
  const missing = providers.filter((p) => !p.ready);
  return html`
    <${Card} title="AI image provider">
      <${Field} label="Provider"
        help=${[
          chosen?.id === "placeholder"
            ? "Coloured tiles, no pictures. Nothing is sent."
            : chosen?.local
              ? "Drawn on this machine. Nothing is sent."
              : "Only the English picture description is sent.",
          missing.length > 0 &&
            `Also possible: ${missing.map((p) => `${p.id === "local" ? "on this machine" : p.name} (${p.setup})`).join("; ")}.`,
        ]
          .filter(Boolean)
          .join(" ")}>
        <select class="select" value=${draft.image_provider} onChange=${(e) => set({ image_provider: e.target.value })}>
          <option value="">Automatic: ${auto?.name}</option>
          ${ready.map((p) => html`<option value=${p.id}>${p.name}</option>`)}
        </select><//>
      <${Field} label="New pictures per day, at most" help=${`${today} made today.${chosen?.local ? "" : " Each one is a paid call to your image API."}`}>
        <input class="input w-32" type="number" min="0" max="1000" value=${cap}
          onInput=${(e) => set({ daily_image_cap: Math.max(0, parseInt(e.target.value, 10) || 0) })} /><//>
      ${
        cap > 0 &&
        html`<div class="progress" role="img" aria-label=${`${today} of ${cap} pictures used today`}>
        <span style=${{ width: Math.min(100, (today / cap) * 100) + "%" }}></span></div>`
      }
      <p class="text-muted-foreground">The picture style is on the Frame tab.</p>
    <//>
    <${TestPicture} provider=${draft.image_provider} style=${draft.image_style} name=${chosen?.name} />`;
}

// TestPicture makes one picture from a fixed description with the provider
// picked above, saved or not. It goes nowhere near the frame and does not
// count towards the daily limit.
function TestPicture({ provider, style, name }) {
  const [test, setTest] = useState(null); // { busy } | { image, took } | { error }
  const run = async () => {
    setTest({ busy: true });
    try {
      const r = await api("api/test-picture", "POST", { provider, style });
      setTest({ image: r.image, took: r.took, prompt: r.prompt });
    } catch (e) {
      setTest({ error: e.message });
    }
  };
  return html`
    <${Card} title="Test picture" intro="Draws one picture with the provider above and the current style. Not shown on the frame, not counted.">
      <div><button type="button" class="btn" data-variant="outline" disabled=${test?.busy} onClick=${run}>
        ${test?.busy && html`<span class="loader size-4" aria-hidden="true"></span>`}${test?.busy ? "Drawing…" : "Make a test picture"}</button></div>
      <div aria-live="polite">
        ${test?.busy && html`<span class="skeleton block aspect-4/3 w-full max-w-sm" aria-label=${"Waiting for " + name}></span>`}
        ${
          test?.image &&
          html`<figure class="grid gap-2">
          <img class="fade-in w-full max-w-sm rounded-md ring-1 ring-black/15" src=${test.image} alt=${"Test picture: " + test.prompt} />
          <figcaption class="text-sm text-muted-foreground"><span class="block italic">${test.prompt}</span>${name} answered in ${secs(test.took)}. Dithered as the frame shows it.</figcaption></figure>`
        }
        ${test?.error && html`<div class="alert" data-variant="destructive" role="alert"><section>${test.error}</section></div>`}
      </div>
    <//>`;
}

function Security({ state, onState, onSignedOut }) {
  const has = state.status.password;
  const [current, setCurrent] = useState("");
  const [next, setNext] = useState("");
  const [msg, setMsg] = useState(["", ""]);
  const submit = async (password) => {
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
  return html`
    <${Card} title="Page password"
      intro="A password on top of the page's address. The frame's picture and buttons need no password, because e-ink frames can't sign in. The secret address guards those.">
      <${Note} tone=${has || state.status.locked ? "ok" : "warn"}>${has ? "This page is locked with a password." : state.status.locked ? "No password: anyone who has this page's address can change the frame and pause listening." : "No password and no secret address: anyone on your network can change the frame and pause listening."}<//>
      <form class="grid gap-4" onSubmit=${(e) => {
        e.preventDefault();
        submit(next);
      }}>
        ${
          has &&
          html`<${Field} label="Current password">
          <input class="input" type="password" autocomplete="current-password" value=${current} onInput=${(e) => setCurrent(e.target.value)} /><//>`
        }
        <${Field} label=${has ? "New password" : "Password"} help="At least 8 characters. A few random words work well.">
          <input class="input" type="password" autocomplete="new-password" minlength="8" value=${next} onInput=${(e) => setNext(e.target.value)} /><//>
        ${msg[1] && html`<${Note} tone=${msg[0]}>${msg[1]}<//>`}
        <div class="flex flex-wrap gap-2">
          <button class="btn" type="submit">${has ? "Change password" : "Set password"}</button>
          ${has && html`<button type="button" class="btn" data-variant="outline" onClick=${() => submit("")}>Remove password</button>`}
          ${has && html`<button type="button" class="btn" data-variant="ghost" onClick=${signOut}>Sign out</button>`}
        </div>
      </form>
    <//>
    <${Card} title="Panel token">
      ${
        state.status.locked
          ? html`<${Note} tone="ok">On. This page and the frame live under a secret address.<//>`
          : html`<${Note} tone="warn">Off (PANEL_OPEN is set in .env, or E-maki runs without a settings file). Anything on your network can open this page and the frame.<//>`
      }
    <//>`;
}

const TILE_COUNTS = [2, 3, 4, 5, 6, 7, 8, 9];

function Layout({ draft, set }) {
  const layouts = [
    ["collage", "Collage", "Up to 9 pictures. The most talked-about is biggest; tiles fade over time."],
    ["single", "One picture", "The newest topic fills the frame. Pin it to keep it."],
    ["rolling", "Rolling", "A set number of pictures. Each new one is added and the oldest drops off."],
  ];
  return html`
    <${Card} title="Layout">
      <div class="grid gap-3 sm:grid-cols-3" role="radiogroup" aria-label="Layout">
        ${layouts.map(
          ([v, name, text]) => html`
          <${Choice} name="layout" checked=${draft.layout === v} onChange=${() => set({ layout: v })}>
            <span class="grid gap-2">
              <${LayoutIcon} kind=${v} on=${draft.layout === v} />
              <span class="font-medium">${name}</span>
              <small class="text-muted-foreground">${text}</small>
            </span>
          <//>`,
        )}
      </div>
      ${
        draft.layout !== "single" &&
        html`<${Segments} name="tiles" value=${draft.tiles} onChange=${(n) => set({ tiles: n })}
        label=${draft.layout === "rolling" ? "Number of pictures" : "Maximum pictures"}
        options=${TILE_COUNTS.map((n) => [n, String(n)])} />`
      }
      ${
        draft.layout === "collage" &&
        html`<${Toggle} checked=${draft.newest_big} onChange=${(v) => set({ newest_big: v })}
        title="Newest picture largest" text="Instead of the most talked-about one." />`
      }
      <${Toggle} checked=${!draft.hide_captions} onChange=${(v) => set({ hide_captions: !v })}
        title="Captions" text="Topic names under the pictures." />
    <//>`;
}

// Small drawings of the three layouts, in the frame's proportions.
function LayoutIcon({ kind, on }) {
  const cells = {
    collage: [
      [0, 0, 2, 2],
      [2, 0, 1, 1],
      [3, 0, 1, 1],
      [2, 1, 1, 1],
      [3, 1, 1, 1],
      [0, 2, 1, 1],
      [1, 2, 1, 1],
      [2, 2, 1, 1],
      [3, 2, 1, 1],
    ],
    single: [[0, 0, 4, 3]],
    rolling: [
      [0, 0, 2, 3],
      [2, 0, 2, 3],
    ],
  }[kind];
  return html`<svg class="h-9 w-12" viewBox="0 0 44 33" aria-hidden="true">
    ${cells.map(
      (
        [c, r, cs, rs],
        i,
      ) => html`<rect x=${c * 11 + 1} y=${r * 11 + 1} width=${cs * 11 - 2} height=${rs * 11 - 2} rx="1.5"
      class=${i === 0 && on ? "fill-primary" : "fill-muted-foreground/30"} />`,
    )}
  </svg>`;
}

const EVERY = [
  [0, "Straight away"],
  [5, "5 minutes"],
  [15, "15 minutes"],
  [30, "30 minutes"],
  [60, "1 hour"],
  [120, "2 hours"],
  [240, "4 hours"],
];

function Timing({ draft, set }) {
  const quiet = draft.quiet_from !== "";
  return html`
    <${Card} title="Timing">
      ${
        draft.mode === "diary"
          ? html`<p class="text-sm text-muted-foreground">Diary mode paints once a day, at ${draft.diary_at}. Change it under E-maki, Topic detection.</p>`
          : html`<${Field} label="Minimum time between pictures" help="New topics wait in line and go up one at a time.">
        <select class="select w-full" value=${draft.new_every_minutes} onChange=${(e) => set({ new_every_minutes: +e.target.value })}>
          ${withCurrent(EVERY, draft.new_every_minutes, (v) => v + " minutes").map(([v, t]) => html`<option value=${v}>${t}</option>`)}
        </select><//>`
      }
      <div class="grid gap-3">
        <${Toggle} checked=${quiet} title="Quiet hours"
          text="No new pictures go up in between. What was talked about appears when they end."
          onChange=${(v) => set(v ? { quiet_from: "22:00", quiet_to: "07:00" } : { quiet_from: "", quiet_to: "" })} />
        ${
          quiet &&
          html`<div class="flex flex-wrap items-center gap-3">
          <label class="flex items-center gap-2">From <input class="input w-32" type="time" value=${draft.quiet_from} onChange=${(e) => set({ quiet_from: e.target.value })} /></label>
          <label class="flex items-center gap-2">until <input class="input w-32" type="time" value=${draft.quiet_to} onChange=${(e) => set({ quiet_to: e.target.value })} /></label>
        </div>`
        }
      </div>
      ${
        draft.layout === "collage" &&
        html`
        <${Field} label="Fade time" help="Time for a tile to shrink to half size. Talking about it again makes it grow back.">
          <select class="select w-full" value=${draft.half_life_hours} onChange=${(e) => set({ half_life_hours: +e.target.value })}>
            ${withCurrent(HALF_LIVES, draft.half_life_hours, (v) => v + " hours").map(([v, t]) => html`<option value=${v}>${t}</option>`)}
          </select><//>`
      }
    <//>`;
}

function Privacy({ draft, set, saved, topicAPI }) {
  const external = draft.topic_provider === "external";
  const blocked = draft.block_sensitive;
  const toggle = (c) =>
    set({ block_sensitive: blocked.includes(c) ? blocked.filter((x) => x !== c) : [...blocked, c] });
  return html`
    <${Card} title="Blocked topics"
      intro=${external ? "Topics like these never get a picture." : "Topics like these never get a picture, and nothing about them leaves the house."}>
      ${Object.entries(CATEGORIES).map(
        ([c, [name, text]]) => html`
        <${Toggle} checked=${blocked.includes(c)} onChange=${() => toggle(c)} title=${name} text=${text} />`,
      )}
      <${Toggle} checked=${draft.block_names} onChange=${(v) => set({ block_names: v })} title="Spoken names"
        text='Any picture whose description repeats a name that was said out loud. It cannot tell people from places, so "Lofoten" goes too.' />
      <${NeverPaint} value=${draft.never_paint} onChange=${(v) => set({ never_paint: v })} />
      ${blocked.length === 0 && !draft.block_names && !(draft.never_paint || []).length && html`<${Note} tone="warn">Nothing is held back: every topic can become a picture.<//>`}
    <//>
    <${Card} title="Stored data">
      <p><strong class="font-semibold">Nothing is recorded.</strong> Audio is never written to disk. What was said lives only in a short rolling window in memory and is gone when E-maki stops or you pause listening.</p>
      ${external && html`<${Note} tone="warn">The external topic model is on: that window is also sent to ${topicAPI}. Switch back under Topic detection.<//>`}
      <p class="text-muted-foreground">The settings and password files hold no audio and no words. See PRIVACY.md in the repository for the full list.</p>
      <${Toggle} checked=${draft.keep_pictures !== false} onChange=${(v) => set({ keep_pictures: v })} disabled=${!saved} title="Keep pictures over a restart"
        text=${
          saved
            ? "Saves the pictures and their descriptions next to the settings. Turning it off deletes them."
            : "Needs a settings file: start E-maki with -settings."
        } />
    <//>`;
}

// NeverPaint edits the never-paint list, one word or phrase a line. The text
// is kept as typed so a new line survives until something is written on it.
function NeverPaint({ value, onChange }) {
  const list = (t) =>
    t
      .split("\n")
      .map((w) => w.trim())
      .filter(Boolean);
  const [text, setText] = useState((value || []).join("\n"));
  useEffect(() => {
    if (!same(list(text), value || [])) setText((value || []).join("\n"));
  }, [value]);
  return html`
    <${Field} stack label="Never paint" help="One word or phrase a line. A topic or picture description that contains one never becomes a picture. Not case sensitive.">
      <textarea class="textarea" rows="4" maxlength="20000" placeholder=${"For example:\nwork\nAnna"} value=${text}
        onInput=${(e) => {
          setText(e.target.value);
          const l = list(e.target.value);
          onChange(l.length ? l : null);
        }} />
    <//>`;
}

function Look({ draft, set, styles }) {
  // image_style holds a preset's name; older settings hold its prompt, and empty is the first preset.
  const style = draft.image_style || styles[0].name;
  const preset = styles.find((s) => s.name.toLowerCase() === style.toLowerCase() || s.prompt === style);
  const [custom, setCustom] = useState(!preset);
  return html`
    <${Card} title="Picture style" intro="Used for new pictures. Tiles already on the frame keep their look.">
      <div class="flex flex-wrap gap-2" role="radiogroup" aria-label="Picture style">
        ${[
          ...styles.map((s) => [
            s.name,
            !custom && preset === s,
            () => {
              setCustom(false);
              set({ image_style: s.name });
            },
          ]),
          [
            "Your own",
            custom,
            () => {
              setCustom(true);
              if (preset) set({ image_style: preset.prompt });
            },
          ],
        ].map(
          ([name, on, pick]) => html`
          <label class="tap cursor-pointer rounded-full border px-3 py-1 text-sm font-medium transition-colors hover:bg-accent has-checked:border-primary has-checked:bg-primary has-checked:text-primary-foreground has-focus-visible:ring-2 has-focus-visible:ring-ring">
            <input type="radio" name="style" class="sr-only" checked=${on} onChange=${pick} />${name}</label>`,
        )}
      </div>
      <div class="grid">
        ${[
          ...styles.map((s) => [
            s.name,
            !custom && preset === s,
            html`<p class="rounded-lg bg-muted p-3 italic text-muted-foreground">${s.prompt}</p>`,
          ]),
          [
            "",
            custom,
            html`<${Field} stack label="Custom style" help="Flat colours in black, white, yellow, red, blue and green look best on the e-ink panel.">
             <textarea class="textarea" rows="4" maxlength="1000" placeholder="For example: woodcut print with thick black lines, flat yellow and blue"
               value=${draft.image_style} onInput=${(e) => set({ image_style: e.target.value })}></textarea><//>`,
          ],
        ].map(
          ([k, on, el]) => html`
          <div key=${k} class=${"[grid-area:1/1] " + (on ? "" : "invisible")} inert=${!on}>${el}</div>`,
        )}
      </div>
    <//>`;
}

// The Inky Impression panels Fugleramme knows, by their resolution (landscape).
const PANELS = [
  ["1600x1200", '13.3" (1600 × 1200)'],
  ["800x480", '7.3" (800 × 480)'],
  ["600x448", '5.7" (600 × 448)'],
  ["640x400", '4" (640 × 400)'],
];

// withSize keeps a panel size set in the profile selectable.
function withSize(d) {
  const v = d.panel_width + "x" + d.panel_height;
  return PANELS.some(([p]) => p === v) ? PANELS : [...PANELS, [v, `${d.panel_width} × ${d.panel_height}`]];
}

// withCurrent keeps a value set in the profile selectable when it is not one of the options.
function withCurrent(options, value, label) {
  return options.some(([v]) => v === value) ? options : [...options, [value, label(value)]].sort((a, b) => a[0] - b[0]);
}

function Hanging({ draft, set }) {
  return html`
    <${Card} title="E-ink frame" intro="Set these once, when you hang the frame.">
      <${Field} label="Panel" help="The size of your e-ink screen. The picture is drawn at its resolution.">
        <select class="select w-full" value=${draft.panel_width + "x" + draft.panel_height}
          onChange=${(e) => {
            const [w, h] = e.target.value.split("x").map(Number);
            set({ panel_width: w, panel_height: h });
          }}>
          ${withSize(draft).map(([v, t]) => html`<option value=${v}>${t}</option>`)}
        </select><//>
      <div class="grid gap-2" role="radiogroup" aria-label="Rotation">
        <span class="font-medium">Rotation</span>
        <div class="flex flex-wrap gap-2">
          ${ROTATIONS.map(
            (v) => html`
            <label class="grid w-16 cursor-pointer justify-items-center gap-1 rounded-lg border p-2 hover:bg-accent has-checked:border-primary has-checked:ring-1 has-checked:ring-primary has-focus-visible:ring-2 has-focus-visible:ring-ring">
              <input type="radio" name="rotation" class="sr-only" checked=${draft.panel_rotation === v} onChange=${() => set({ panel_rotation: v })} />
              <span class="grid h-6 w-8 place-content-center rounded-sm border-2 border-foreground/70 text-xs" style=${{ transform: `rotate(${v}deg)` }} aria-hidden="true">A</span>
              <small>${v}°</small>
            </label>`,
          )}
        </div>
        <small class="text-muted-foreground">Pick another if the picture shows sideways or upside down on your frame.</small>
      </div>
      ${
        hasE6(draft) &&
        html`<${Field} label="Refresh interval" help="How long a battery frame (like the EE02) sleeps between checks for a new picture. Less often saves battery. Frames on mains power check on their own.">
        <select class="select w-full" value=${draft.panel_poll_minutes} onChange=${(e) => set({ panel_poll_minutes: +e.target.value })}>
          ${withCurrent(
            POLLS.map((m) => [m, m === 1 ? "minute" : m + " minutes"]),
            draft.panel_poll_minutes,
            (v) => v + " minutes",
          ).map(([v, t]) => html`<option value=${v}>${t}</option>`)}
        </select><//>`
      }
      <div class="grid gap-3">
        <${Field} label="Buttons" help="How many buttons your frame has. The frame sends A, B, C… when one is pressed; an Inky Impression has four, top to bottom.">
          <select class="select w-full" value=${Object.keys(draft.panel_buttons || {}).length}
            onChange=${(e) => set({ panel_buttons: buttons(draft.panel_buttons, +e.target.value) })}>
            ${["", ...BUTTON_LETTERS].map((_, n) => html`<option value=${n}>${n === 0 ? "None" : n}</option>`)}
          </select><//>
        ${Object.keys(draft.panel_buttons || {})
          .sort()
          .map(
            (b) => html`
          <${Field} label=${"Button " + b}>
            <select class="select w-full" value=${draft.panel_buttons[b]}
              onChange=${(e) => set({ panel_buttons: { ...draft.panel_buttons, [b]: e.target.value } })}>
              ${BUTTON_ACTIONS.map(([v, t]) => html`<option value=${v}>${t}</option>`)}
            </select><//>`,
          )}
      </div>
    <//>`;
}

// hasE6 is whether the panel gets /frame.e6, the only place the refresh interval goes.
const hasE6 = (d) => ["800x480", "1600x1200"].includes(d.panel_width + "x" + d.panel_height);

// The letters a frame's buttons can send (config.Buttons), and the first n of
// them with the actions they had, new ones as on an Inky.
const BUTTON_LETTERS = ["A", "B", "C", "D", "E", "F", "G", "H"];
const INKY_BUTTONS = { A: "pause", B: "remove", C: "pin", D: "next" };
function buttons(old, n) {
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

function Facts({ rows }) {
  return html`<dl class="grid gap-x-6 gap-y-2 sm:grid-cols-[max-content_1fr]">
    ${rows.map(([k, v]) => html`<dt class="text-muted-foreground">${k}</dt><dd class="break-words">${v}</dd>`)}
  </dl>`;
}

function System({ state }) {
  const st = state.status;
  const a = state.activity;
  const langs = Object.entries(st.by_lang || {});
  const h = state.health;
  return html`
    <${Card} title="Activity since start">
      <div class="grid grid-cols-2 gap-3 sm:grid-cols-4">
        ${[
          [a.heard, "segments transcribed"],
          [a.quiet, "too quiet, skipped"],
          [a.topics, "topics found"],
          [a.held_back, "held back for privacy"],
          ...(a.dropped ? [[a.dropped, "dropped, topic model too slow"]] : []),
        ].map(
          ([n, text]) => html`
          <div class="rounded-lg border p-3"><b class="block text-2xl font-semibold tabular-nums">${n}</b><small class="text-muted-foreground">${text}</small></div>`,
        )}
      </div>
    <//>
    <${Card} title="Models and servers" intro="These are set in the profile and .env, and need a restart to change.">
      <div class="grid gap-3">
        <${Service} name="Speech to text" where=${st.speech} s=${h.speech} />
        <${Service} name="Topic model" s=${h.topics}
          where=${state.settings.topic_provider === "external" && st.topic_api ? `${st.topic_api} (external)` : `${st.topics} at ${st.llm_url}`} />
        <${Service} name="AI image provider" where=${st.images} s=${h.images} idle="No picture made since E-maki started. Make a test picture under AI image provider." />
      </div>
      <${Facts} rows=${[
        ["Profile", st.profile],
        ...langs.map(([l, u]) => ["Speech to text, " + langName(l), u]),
        ["Running since", when(st.started)],
        [
          "Settings",
          st.saved ? "Saved, kept after a restart" : "Last until restart (start with -settings to keep them)",
        ],
      ]} />
    <//>`;
}

// HEALTH is each service state's dot and words.
const HEALTH = {
  up: ["bg-ink-green", "Answering"],
  loading: ["bg-ink-yellow", "Loading its model"],
  down: ["bg-destructive", "Not answering"],
  failing: ["bg-destructive", "Last call failed"],
  unknown: ["bg-muted-foreground/40", "Not used yet"],
};

const secs = (s) => (s < 10 ? s.toFixed(1) : Math.round(s)) + " s";

// notReady names the models that do not answer yet, "" when both do.
function notReady(h) {
  const no = (s) => s && (s.state === "down" || s.state === "loading");
  if (no(h.speech) && no(h.topics)) return "Models not ready";
  if (no(h.speech)) return "Speech model not ready";
  if (no(h.topics)) return "Topic model not ready";
  return "";
}

// Service is one model or provider: whether it answers, and how its last call went.
function Service({ name, where, s, idle }) {
  const [dot, word] = HEALTH[s.state] || HEALTH.unknown;
  const last = s.last_at
    ? s.error
      ? `Failed ${when(s.last_at)}`
      : `Last answer in ${secs(s.took)}, ${when(s.last_at)}`
    : "";
  const detail =
    s.state === "down"
      ? "On a first start it may still be downloading its model."
      : s.state === "unknown"
        ? idle || ""
        : "";
  return html`
    <div class="grid gap-1 rounded-lg border p-3">
      <div class="flex flex-wrap items-center justify-between gap-x-4 gap-y-1">
        <span class="font-medium">${name}</span>
        <span class="flex items-center gap-2 text-sm font-medium"><i class=${"size-2 rounded-full " + dot} aria-hidden="true"></i>${word}</span>
      </div>
      ${where && html`<small class="break-words text-muted-foreground">${where}</small>`}
      ${(last || detail) && html`<small class="text-muted-foreground">${[last, detail].filter(Boolean).join(". ")}</small>`}
      ${s.error && html`<small class="break-words text-destructive">${s.error}</small>`}
    </div>`;
}

function SaveBar({ dirty, saving, notice, what, onSave, onDiscard, onDone }) {
  // Closing or reloading the page would drop the changes: let the browser ask first.
  useEffect(() => {
    if (!dirty) return;
    const ask = (e) => {
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
  return html`
    <div class="fixed inset-x-0 bottom-20 z-20 flex justify-center px-4 sm:bottom-4" role="region" aria-label="Unsaved changes">
      <div class="rise-in flex flex-wrap items-center gap-3 rounded-xl border bg-popover px-4 py-3 text-sm text-popover-foreground shadow-lg">
        ${
          dirty
            ? html`<span role="status">${what}</span>
              <button type="button" class="btn" data-variant="ghost" onClick=${onDiscard} disabled=${saving}>Discard</button>
              <button type="button" class="btn" onClick=${onSave} disabled=${saving}>${saving ? "Saving…" : "Save changes"}</button>`
            : html`<span role="status" class="flex items-center gap-2"><i class="size-2 rounded-full bg-ink-green" aria-hidden="true"></i>${notice}</span>`
        }
      </div>
    </div>`;
}

// The page's own "Loading" line goes, so it does not sit above the app.
const root = document.getElementById("app");
root.replaceChildren();
render(html`<${App} />`, root);
