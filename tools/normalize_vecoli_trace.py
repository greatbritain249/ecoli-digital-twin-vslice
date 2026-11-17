#!/usr/bin/env python3
"""Normalize ALREADY-EXTRACTED vEcoli observables to the viewer replay schema.

Input is a curated CSV or Parquet table with time_min, glucose_mM, atp_mM,
biomass_fg, translation_activity; these must be converted with physically
correct units and supported by source metadata. This script does not infer
concentrations from molecule counts or run vEcoli itself.

Example:
  python tools/normalize_vecoli_trace.py --input exported.csv --out data/vecoli-replay.json --source 'vEcoli experiment X, run Y'
  ECOLI_REPLAY_FILE=data/vecoli-replay.json go run ./cmd/server
"""
import argparse
import csv
import json
import math
from pathlib import Path

FIELDS = ('time_min','glucose_mM','atp_mM','biomass_fg','translation_activity')

def read_rows(path):
    if path.suffix.lower() in ('.parquet','.pq'):
        try:
            import pyarrow.parquet as pq
        except ImportError as e:
            raise SystemExit('For Parquet input: pip install pyarrow') from e
        return pq.read_table(path).to_pylist()
    with path.open(newline='') as f:
        return list(csv.DictReader(f))

def normalize(rows, source):
    frames = []
    last = -1.0
    for i,row in enumerate(rows):
        try:
            vals = {k:float(row[k]) for k in FIELDS}
        except (KeyError,ValueError,TypeError) as e:
            raise ValueError(f'row {i}: missing or invalid numeric field {e}') from e
        if any(not math.isfinite(v) for v in vals.values()):
            raise ValueError(f'row {i}: non-finite value')
        if vals['time_min'] <= last or vals['time_min'] < 0:
            raise ValueError(f'row {i}: time_min must increase')
        if any(vals[k]<0 for k in FIELDS if k!='time_min') or not 0<=vals['translation_activity']<=1 or vals['biomass_fg']<=0:
            raise ValueError(f'row {i}: invalid physical range')
        last = vals['time_min']
        vals['medium'] = row.get('medium','rich') or 'rich'
        if vals['medium'] not in ('rich','limited','starved'):
            raise ValueError(f'row {i}: invalid medium')
        vals['growth_per_hour'] = float(row.get('growth_per_hour') or 0)
        vals['division_progress'] = float(row.get('division_progress') or 0)
        vals['divisions'] = int(row.get('divisions') or 0)
        frames.append(vals)
    if len(frames)<2:
        raise ValueError('trajectory must have at least 2 frames')
    if not source.strip():
        raise ValueError('must specify a truthful, non-empty provenance --source')
    return {'schema_version':1,'source':source,'frames':frames}

def main():
    parser=argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--input',type=Path,required=True)
    parser.add_argument('--out',type=Path,required=True)
    parser.add_argument('--source',required=True,help='Model/run/experiment provenance')
    args=parser.parse_args()
    normalized=normalize(read_rows(args.input),args.source)
    args.out.parent.mkdir(parents=True,exist_ok=True)
    args.out.write_text(json.dumps(normalized,indent=2)+'\n')
    print(f'exported {len(normalized["frames"])} snapshots to {args.out}')
if __name__=='__main__':main()
