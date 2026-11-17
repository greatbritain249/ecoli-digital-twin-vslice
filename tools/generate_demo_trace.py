#!/usr/bin/env python3
"""Write a synthetic, clearly-labeled trajectory for testing replay UI.
This file is NOT produced by vEcoli and is not a physiological prediction.
"""
import json
from pathlib import Path

root = Path(__file__).resolve().parents[1]
frames = []
biomass = 280.0
atp = 2.4
glucose = 3.4
for i in range(51):
    t = i * 2.0
    medium = 'rich' if i < 18 else 'limited' if i < 32 else 'starved'
    if i:
        glucose = max(0.02, glucose + {'rich':.04,'limited':-.035,'starved':-.14}[medium])
        atp = min(2.9,max(.18,atp + {'rich':.012,'limited':-.014,'starved':-.050}[medium]))
    substrate = glucose/(.65+glucose)
    translation = max(.03,min(1.0,(.08+.92*substrate)*(.18+.82*atp/3)))
    growth = .93*substrate*translation
    biomass += biomass * growth / 30.0
    frames.append({'time_min':t,'medium':medium,'glucose_mM':round(glucose,5),
                   'atp_mM':round(atp,5),'biomass_fg':round(biomass,5),
                   'growth_per_hour':round(growth,6),
                   'translation_activity':round(translation,5),
                   'division_progress':round(min(1,max(0,biomass/280-1)),5),
                   'divisions':0})
(root/'data'/'replay-example.json').write_text(json.dumps({
    'schema_version':1,
    'source':'SYNTHETIC DEMO — not vEcoli; not measured data',
    'frames':frames},indent=2)+'\n')
print('generated illustrative trajectory (51 frames)')
