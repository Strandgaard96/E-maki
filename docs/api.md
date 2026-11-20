# HTTP API

The JSON API behind the settings page. Paths are under `http://<your-brain>:8090/`, or `/<token>/` with `PANEL_TOKEN` set.

## Rules

- Changes are `POST` or `PUT` with `Content-Type: application/json`. Cross-site requests are refused.
- With a password set, `api/` calls need the cookie from `POST api/login`.
- Every change returns the same JSON as `GET api/state`.

## Frame

No password needed.

| Call | What it does |
| --- | --- |
| `GET frame.png` | The page as a 6-colour PNG. Send `If-None-Match` with the last `ETag` to get `304` when nothing changed |
| `GET frame.e6` | The same page packed for the panel |
| `POST press` | A button press, form field `button=A` to `H` |
| `GET preview.png` | The page as shown on the settings page |

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

Pause and resume, without a password set. In `configuration.yaml`:

```yaml
rest_command:
  emaki_pause:
    url: http://<your-brain>:8090/api/pause
    method: post
    content_type: application/json
    payload: '{"paused": true}'
  emaki_resume:
    url: http://<your-brain>:8090/api/pause
    method: post
    content_type: application/json
    payload: '{"paused": false}'
```

Add `PANEL_TOKEN` to the URL if you use one.
