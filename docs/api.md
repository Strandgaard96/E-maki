# HTTP API

The settings page talks to E-maki over a small JSON API. Other tools on your network can use it too, for example Home Assistant to pause listening when you leave home.

All paths are under `http://<your-brain>:8090/<token>/`: your `PANEL_TOKEN`, or the token E-maki made on the first start (`docker compose exec emaki cat /data/token`). With `PANEL_OPEN=1` there is no token and they are under `http://<your-brain>:8090/`.

## Rules

- Requests that change something are `POST` (or `PUT`) with `Content-Type: application/json`. Anything else is refused, and so are requests that a browser marks as coming from another site.
- With a password set on the settings page, every `api/` call needs the session cookie that `POST api/login` sets. Without a password, no sign-in is needed.
- Every change answers with the same JSON as `GET api/state`.

## Frame

These are Fugleramme's panel protocol and need no password.

| Call | What it does |
| --- | --- |
| `GET frame.png` | The page as a 6-colour PNG. Send `If-None-Match` with the last `ETag` to get `304` when nothing changed |
| `GET frame.e6` | The same page packed for the panel |
| `POST press` | A frame button, as a form field: `button=A` to `H`. It does what `panel_buttons` sets for that letter, and nothing for a button the frame doesn't have |
| `GET preview.png` | The page for a browser, also without a password |

## Settings page

| Call | Body | What it does |
| --- | --- | --- |
| `GET api/state` | | Settings, tiles on the frame, the queue, removed topics, counts and status |
| `POST api/pause` | `{"paused": true}` | Pause or resume listening. The microphone is closed while paused |
| `POST api/clear` | `{}` | Take every unpinned tile off the frame |
| `POST api/tiles/pin`, `api/tiles/unpin` | `{"label": "..."}` | Keep a tile on the frame, or let it fade again |
| `POST api/tiles/remove` | `{"label": "..."}` | Take a tile off and keep the topic away for a week |
| `POST api/tiles/rename` | `{"label": "...", "to": "..."}` | Change a caption |
| `POST api/removed/allow` | `{"label": "..."}` | Let a removed topic come back |
| `POST api/queue/up`, `api/queue/now`, `api/queue/drop` | `{"label": "..."}` | Move a waiting picture up, put it up now, or drop it |
| `PUT api/settings` | the `settings` object from `api/state` | Change settings (see [configs/README.md](../configs/README.md)) |
| `GET api/mic` | | Microphones and the current level |
| `GET api/logs?after=N` | | Log lines after line `N` as JSON |
| `GET api/logs.txt` | | The log as a text file |
| `GET api/picture.png?label=...` | | One topic's picture, small |
| `POST api/login`, `api/logout` | `{"password": "..."}` | Sign in or out |

## Home Assistant

A pause switch, with no password set on the settings page. In `configuration.yaml`:

```yaml
rest_command:
  emaki_pause:
    url: http://<your-brain>:8090/<token>/api/pause
    method: post
    content_type: application/json
    payload: '{"paused": true}'
  emaki_resume:
    url: http://<your-brain>:8090/<token>/api/pause
    method: post
    content_type: application/json
    payload: '{"paused": false}'
```

Call `rest_command.emaki_pause` from an automation, for example when everyone has left. Put the token in the URL.
