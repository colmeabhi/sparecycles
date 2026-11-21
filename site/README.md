# site

The product page for SpareCycles, published with GitHub Pages.

**Owner:** all · **State:** complete

## Files

| File | Content |
| --- | --- |
| `index.html` | The whole page: HTML, CSS and one small script, in one file |
| `assets/fractal.png` | Real output from a test run (16 units, 2 volunteers). The hero simulation shows slices of it. |

There is no build step. Edit `index.html` and push.

## Design

The page uses a pixel tile style. Keep it consistent:

| Item | Value |
| --- | --- |
| Fonts | `Silkscreen` for headings, `Space Grotesk` for text, `JetBrains Mono` for labels and code |
| Paper | `--paper` `#f3f2ed` |
| Ink | `--ink` `#161616` (also "accepted") |
| Running | `--run` `#2f5bff` |
| Outvoted | `--bad` `#ff5a1f` |
| Waiting | `--pending` `#dcdad2` |
| Shapes | Square tiles, 2px black borders, hard offset shadows. No rounded corners, no gradients, no emoji. |

Use the 4 tile colors only with their meaning (waiting, running, outvoted, accepted).

## Page sections

Hero with the simulation → algorithm ticker → 01 how it works → 02 when things go wrong → 03 use cases → 04 under the hood → 05 run it → 06 roadmap → 07 team.

## Hero simulation

The script at the end of `index.html` animates 64 units: they run, some are outvoted and retried, and accepted units show their slice of `fractal.png`. It is an illustration and is labeled "simulation". With `prefers-reduced-motion`, it shows the finished image with no motion.

## Preview

Serve the folder and open it in a browser. A plain file open does not load the fonts in some browsers.

```bash
python3 -m http.server 8088 --directory site
```

Then open `http://localhost:8088`.

## Publish

`.github/workflows/pages.yml` deploys `site/` on each push to `main` that changes it.

The repo owner must turn Pages on once:

1. **Settings → Pages → Source:** GitHub Actions.
2. **Settings → Secrets and variables → Actions → Variables:** add `PAGES_ENABLED` with the value `true`.

A private repo on the free plan can not use Pages. The owner needs GitHub Pro (free with the GitHub Student Developer Pack), or the repo must be public. The published page is public in both cases.

## Rules

- Keep the page true. Mark unfinished features as in progress with their issue number, and keep the roadmap section up to date.
- When a module changes in a way that users see (a new workload, a new setting, a finished algorithm), update this page in the same PR.
- Use simple language.
- Check the page at phone width (375 px) before you push. It must not scroll sideways.
- Keep images small (under 200 KB each).
