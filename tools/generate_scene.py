#!/usr/bin/env python3
"""Deterministically produce a *schematic*, spatially plausible scene manifest.

The coordinates ARE NOT cryo-ET measurements. They are dimensionless scene units,
not nanometers. The visible particles are deliberately subsampled and have
schematic radii; no cell-wide atomistic accuracy is claimed.
"""
import json
import math
import random
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
SEED = 728104
RADIUS = 1.50
CYL_HALF = 2.45

def envelope_radius(x):
    ax = abs(x)
    if ax <= CYL_HALF:
        return RADIUS
    extra = ax - CYL_HALF
    return math.sqrt(max(0, RADIUS * RADIUS - extra * extra))

def inside(x, y, z, margin=0.08):
    return math.hypot(y, z) <= envelope_radius(x) - margin

def random_point(rng, kind):
    for _ in range(30000):
        x = rng.uniform(-CYL_HALF - RADIUS + 0.12, CYL_HALF + RADIUS - 0.12)
        r = envelope_radius(x)
        theta = rng.uniform(0, math.tau)
        if kind == 'atp_synthase':
            rad = r - 0.23
        elif kind == 'rna_polymerase':
            x *= 0.75
            r = envelope_radius(x)
            rad = rng.uniform(0.12, min(0.81, r - 0.12))
        else:
            rad = math.sqrt(rng.random()) * max(0, r - 0.28)
            # Nucleoid exclusion: bias ribosomes to peripheral cytoplasm.
            if abs(x) < 1.8 and rad < 0.78:
                continue
        y, z = rad * math.cos(theta), rad * math.sin(theta)
        if inside(x, y, z):
            return [round(x, 4), round(y, 4), round(z, 4)]
    raise RuntimeError('No space in cell for ' + kind)

def build():
    rng = random.Random(SEED)
    records = []
    for kind, num, radius in [('ribosome', 230, .081), ('rna_polymerase', 45, .099), ('atp_synthase', 85, .075)]:
        for i in range(num):
            records.append({'id': f'{kind}-{i+1:04d}', 'kind': kind,
                            'position': random_point(rng, kind), 'radius': radius,
                            'phase': round(rng.random()*math.tau, 4)})
    return {
        'schema_version': 1, 'seed': SEED,
        'species': 'Escherichia coli', 'reference_strain': 'K-12 MG1655',
        'coordinates': 'arbitrary renderer units (NOT molecular coordinates)',
        'cell': {'length_um': 2.2, 'diameter_um': 0.85, 'body_half_length_units': CYL_HALF,
                 'radius_units': RADIUS, 'note': 'cell proportions are schematic'},
        'placement': 'procedural deterministic packing; nucleoid-exclusion heuristic; NOT measured positions',
        'counts': {'visible_ribosomes': 230, 'visible_rna_polymerases': 45, 'visible_atp_synthases': 85,
                   'note': 'subsampled display objects; NOT biological copy numbers'},
        'instances': records
    }

if __name__ == '__main__':
    target = ROOT / 'data' / 'scene.json'
    target.write_text(json.dumps(build(), separators=(',', ':')) + '\n', encoding='utf-8')
    print(f'generated {target} (seed {SEED})')
