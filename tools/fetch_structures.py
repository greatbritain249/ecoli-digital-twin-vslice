#!/usr/bin/env python3
"""Fetch curated experimental PDB mmCIF files into a local content-addressed cache.

Never commit large downloaded structures or claim the associated atomic
coordinates give their true positions in an intact E. coli cell.
"""
import argparse
import hashlib
import json
from datetime import datetime, timezone
from pathlib import Path
from urllib.request import Request, urlopen

ROOT = Path(__file__).resolve().parents[1]
CURATED = {'4YBB':'E. coli K-12 ribosome',
           '4YG2':'E. coli O157:H7 RNA polymerase (different strain)',
           '5T4O':'E. coli ATP synthase (check chains/strains)'}
MAX_BYTES = 85*1024*1024

def fetch(pdb_id, directory):
    pdb_id=pdb_id.upper()
    if pdb_id not in CURATED:
        raise ValueError(f'unsupported accession {pdb_id}; use curated: {list(CURATED)}')
    url=f'https://files.rcsb.org/download/{pdb_id}.cif'
    request=Request(url,headers={'User-Agent':'CellscopeEducationalTwin/0.1 (scientific data download)'})
    with urlopen(request,timeout=60) as response:
        content=response.read(MAX_BYTES+1)
    if len(content)>MAX_BYTES:
        raise ValueError('mmCIF exceeds 85 MiB maximum')
    if not content.startswith(b'data_'):
        raise ValueError('unexpected content; did not receive a CIF structure')
    checksum=hashlib.sha256(content).hexdigest()
    target=directory/f'{pdb_id}-{checksum[:12]}.cif'
    target.write_bytes(content)
    return {'pdb_id':pdb_id,'description':CURATED[pdb_id],'download_url':url,
            'sha256':checksum,'bytes':len(content),'retrieved_utc':datetime.now(timezone.utc).isoformat(),
            'file':target.name}

def main():
    parser=argparse.ArgumentParser(description=__doc__)
    parser.add_argument('pdb_ids',nargs='*',choices=sorted(CURATED),default=['4YBB'],
                        help='Curated entry IDs (default: 4YBB)')
    args=parser.parse_args()
    dest=ROOT/'data'/'structures';dest.mkdir(exist_ok=True)
    records=[]
    for pid in args.pdb_ids:
        record=fetch(pid,dest)
        records.append(record)
        print(f'{pid}: {record["bytes"]:,} bytes SHA256 {record["sha256"][:16]}…')
    (dest/'manifest.json').write_text(json.dumps({'structures':records},indent=2)+'\n')
if __name__=='__main__':main()
