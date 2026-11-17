# Cellscope — E. coli Digital Twin (vertical slice)

A functioning *educational prototype* that connects a scientific data catalog,
a deterministic 3D bacterial cell scene, experimental protein coordinates
(RCSB PDB / Mol*), an independent Go simulation API, and streaming live
physiology metrics. The project is intended as the **first integrated vertical slice**
of a larger molecular digital-twin platform.

> **Scientific caveat:** This is **not** a high-fidelity experimental reconstruction
> or a predictive, validated biological twin. Cell geometry, molecule counts,
> cellular locations, DNA strands, and visible motion are illustrative. The
> embedded molecule inspector displays **real experimental PDB structures**.
> The default live model is a transparent, coarse-grained educational model,
> **not vEcoli**. The optional recorded trace illustrates how separately
> generated vEcoli output can be connected after units and observables are mapped.

## Start in two commands

Requires **Go 1.23+**, a modern WebGL-capable browser, and internet access to
load pinned Three.js and Mol* scripts (plus PDB structures when inspecting).
**Python 3.9+** is only needed for regenerating data and optional conversions.
Node/npm, Docker, Kubernetes, and a database are **not required** to run.

```bash
# From the extracted ecoli-digital-twin-vslice directory:
go run ./cmd/server
# Open http://localhost:8080
```

Or use Docker Compose:

```bash
docker compose up --build
# Open http://localhost:8080
```

In the browser, click **Start simulation**, change **Nutrient conditions** from
rich to starved, and watch the telemetry and scene react. Click a ribosome,
RNA polymerase, or ATP synthase instance (or choose it in the right panel), then
**Inspect atomic structure** to load that PDB in Mol*.

The **Load sample trajectory** button switches to a bundled 51-frame synthetic
trace for demonstrating the replay connection; it is **not vEcoli data**.
Click the button again to return to the live model.

### Verify the API

```bash
curl http://localhost:8080/api/healthz
curl http://localhost:8080/api/catalog
curl http://localhost:8080/api/scene
curl http://localhost:8080/api/replay/info
```

To use the APIs directly:

```bash
SESSION=$(curl -s -X POST http://localhost:8080/api/simulations \
  -H 'Content-Type: application/json' -d '{"medium":"rich"}' \
  | python3 -c 'import json,sys;print(json.load(sys.stdin)["id"])')
curl -N "http://localhost:8080/api/simulations/$SESSION/events"  # separate terminal
curl -X POST "http://localhost:8080/api/simulations/$SESSION/actions" \
  -H 'Content-Type: application/json' -d '{"action":"start"}'
```

## What's implemented

| Slice | Actual implementation | Important limitation |
|---|---|---|
| 3D cell | Three.js WebGL capsule layers, schematic DNA, flagella, pili and 360 deterministically placed molecular markers | Procedural, not cryo-ET reconstruction |
| Molecule inspector | In-app Mol* for curated experimental structures **4YBB**, **4YG2**, **5T4O** | 4YG2 is E. coli O157:H7, not K-12; remote download required |
| Scene ingestion | RCSB mmCIF download CLI, curated catalog, checksummed optional local cache | Not yet an automated EcoCyc/AlphaFold enrichment pipeline |
| Simulation | Go coarse-grained glucose, ATP, biomass, translation and division state, rich/limited/starved media | Illustrative rate equations, not fitted or validated |
| Streaming | Independent session IDs and Server-Sent Events with latest-value backpressure | Sessions live in API RAM; no persistence or horizontal replication |
| Recorded trace | Normalized CSV/Parquet → JSON adapter and replay of externally generated observations | vEcoli is not installed, run, or magically mapped from raw output |
| Deployment | Single Go binary serves JSON and web assets; Docker Compose and CI | Single process, no S3/CDN/Kubernetes/Argo yet |

## Repository layout (repo-ready boundaries)

```text
.
├── apps/web/                    # Target: ecoli-web
│   ├── index.html               # Browser shell with HUD and Mol* modal
│   ├── styles.css               # Responsive research dashboard
│   └── app.js                   # Three.js viewer + SSE API client
├── cmd/server/                  # Target: ecoli-api entry point
├── internal/api/                # Go REST and SSE handlers
├── internal/sim/                # Target: ecoli-simulation
├── data/
│   ├── catalog.json             # Scientific catalog, PDB provenances
│   ├── scene.json               # Generated scene with placements
│   └── replay-example.json      # Synthetic trajectory / clearly labeled
├── tools/
│   ├── generate_scene.py        # Target: ecoli-structure, deterministic packing
│   ├── fetch_structures.py      # Target: ecoli-ingestion, optional RCSB downloads
│   ├── normalize_vecoli_trace.py # Adapter for prepared vEcoli exports
│   └── generate_demo_trace.py
├── docs/                        # Target: ecoli-docs
├── Dockerfile                   # Target: ecoli-infrastructure
├── compose.yaml
├── Makefile
└── .github/workflows/          # CI scaffolding
```

A monorepo is intentional for this slice: a single reproducible checkout and
one process for interactive testing. Split into the seven previously discussed
repositories only when teams, versioning or deployment lifecycles justify it.
`ecoli-contracts` is currently represented by documented versioned JSON
schemas; introduce Protobuf once separately deployed services require it.

## Scientific input sources

- [4YBB — E. coli ribosome](https://www.rcsb.org/structure/4YBB), K-12 structures.
- [4YG2 — E. coli RNA polymerase](https://www.rcsb.org/structure/4YG2), **O157:H7 strain**.
- [5T4O — E. coli ATP synthase](https://www.rcsb.org/structure/5T4O), E. coli structure.
- [CovertLab/vEcoli](https://github.com/CovertLab/vEcoli), authentic whole-cell model,
  optional external simulation source (requires its own substantial setup).

All molecular instances placed in the scene are *symbolic markers*, not miniature
atomistic copies of the PDB objects. Clicking **Inspect** loads the full PDB
assembly in Mol*, maintaining an honest distinction between cell-space and
atomic-space datasets. Structure and visualization terms may need separate
attribution on commercial redistribution.

Fetch the mmCIF data into a checksummed local cache if needed:

```bash
python3 tools/fetch_structures.py 4YBB 4YG2 5T4O
```

The app itself loads remote PDB coordinates through Mol*. Cached CIFs are not
yet used by the scene renderer. Large structures are excluded from git.

## Work with vEcoli

1. Follow [vEcoli's documented Nextflow workflow](https://covertlab.github.io/vEcoli/workflows.html)
   **separately** to run an actual model and generate Parquet results.
2. Select and aggregate meaningful time-indexed observables from those outputs
   into a tabular CSV or Parquet file. **Convert molecules → mM based on
   compartment volume; obtain biomass in fg; normalize translation activity to
   0–1 with an explicit definition**. Do not assume all observables exist
   directly in raw vEcoli tables.
3. Normalize and serve the trajectory:

```bash
python3 tools/normalize_vecoli_trace.py \
  --input exported-observables.csv --out data/vecoli-replay.json \
  --source 'vEcoli 1.x / experiment XYZ / export of time-indexed observables'
ECOLI_REPLAY_FILE=data/vecoli-replay.json go run ./cmd/server
```

4. Click **Load sample trajectory** (the button now plays the configured file).
   The API returns that file's provenance string so viewers can see what source
   is currently loaded.

For now, this is **recorded playback**, not live vEcoli simulation control.
See [`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md) for the proposed vEcoli worker
and Kubernetes integration and [`docs/MODEL.md`](docs/MODEL.md) for rate equations.

## Regenerate and test

```bash
make generate   # regenerates the schema v1 scene and synthetic example trace
make test       # Go unit/integration tests + Python deterministic scene tests
make run
```

## Known limitations / scale-up plan

- CDN-hosted client libraries need internet connectivity. Vendor or self-host
  pinned library builds before deploying to isolated networks.
- No identity/access control, rate limiting, persistent sessions, per-run
  scientific provenance database or public multi-tenant security yet.
- Background sessions are capped at 200 and automatically pruned when inactive.
  For multi-replica deployment, move sessions to a durable job store and queue,
  while keeping SSE gateways stateless or sticky until fan-out is introduced.
- The scene manifest is a ~30–40 KiB JSON object for this demonstration; replace
  it with compressed, versioned LOD assets in S3-compatible storage and CDN.
- GPU instancing is implemented for molecular markers. Biological scale factors
  and protein abundance should ultimately come from measurements, not arbitrary
  visualization values.
- The current model does not have cell division topology (two daughter-cell
  meshes), molecular collisions, gene regulation, uptake kinetics calibration,
  noise, or a Cryo-ET-constrained packing engine.

This first version is intentionally modest scientifically and actually usable
as an integration platform. Do not deploy as a research-grade or safety-critical
predictive bacterial model without validation.
