# report

The final report in LaTeX (IEEE conference template), with its diagrams. The plan, owners and states are in [outline.md](outline.md).

## Files

| Path | Content |
| --- | --- |
| `main.tex` | Title, authors, abstract, and the list of sections |
| `sections/NN-*.tex` | One file per report section. The file header names the owner. |
| `references.bib` | References. [1] to [6] match the proposal. |
| `diagrams/*.puml` | PlantUML sources. The file header names the diagram ID (D1 to D6, I1 to I3) and the owner. |
| `diagrams/*.png` | Rendered diagrams. They are committed, so Overleaf can build the report. |
| `screenshots/*.png` | Screenshots for section 8 (`s1` success, `s2` to `s4` the 3 failure scenarios). Use a dark terminal, about 110 columns wide, at 2× scale. |
| `outline.md` | Sections, owners, page budget, UML plan and screenshots |

## Build

```bash
make report
```

It renders every diagram and writes `docs/report/build/main.pdf` (ignored by git). It needs `plantuml` and `tectonic` (`brew install plantuml tectonic`).

**Overleaf:** upload the `docs/report/` folder. Set the main file to `main.tex`. Overleaf does not run PlantUML, so commit the PNG after each diagram change.

## Rules

- Write in clear academic English, as in a conference paper.
- Mark anything still to write with `\todo{Owner: what to write}`. It shows in red. Remove every `\todo` before submission.
- Every number must come from the repo: `results/load.csv`, `docs/test-results.md`, or a script run. Write the date of the run.
- Describe only what the code does today. For features that are not merged yet, write what is planned, or leave a `\todo`.
- State only what a paper says. Cite it with `\cite{key}`.
- One PR per section or diagram is fine. Update `outline.md` (state) and `docs/status.md` in the same PR.
