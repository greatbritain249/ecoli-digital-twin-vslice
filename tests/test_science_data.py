import json
import sys
import unittest
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
sys.path.insert(0,str(ROOT/'tools'))
from generate_scene import build, inside
from normalize_vecoli_trace import normalize

class TestScene(unittest.TestCase):
    def test_determinism(self):
        self.assertEqual(build(),build())
        self.assertEqual(build(),json.loads((ROOT/'data'/'scene.json').read_text()))

    def test_all_positions_are_inside_cell(self):
        for instance in build()['instances']:
            self.assertTrue(inside(*instance['position']),instance)

    def test_manifest_explicitly_marks_illustration(self):
        scene=build()
        self.assertIn('NOT measured',scene['placement'])
        self.assertEqual(len(scene['instances']),360)
        self.assertEqual(scene['schema_version'],1)

    def test_catalog_accessions_have_explicit_provenance(self):
        catalog=json.loads((ROOT/'data'/'catalog.json').read_text())
        pdbs={x['pdb_id']:x for x in catalog['structures'] if x['pdb_id']}
        self.assertIn('4YBB',pdbs)
        self.assertIn('O157:H7',pdbs['4YG2']['pdb_strain'])

    def test_normalization_checks_units_and_time(self):
        row={'time_min':'0','glucose_mM':'2','atp_mM':'2.4','biomass_fg':'280','translation_activity':'.7'}
        two=[row,{**row,'time_min':'2'}]
        result=normalize(two,'Test export: correct units supplied')
        self.assertEqual(len(result['frames']),2)
        with self.assertRaises(ValueError):normalize([row,row],'bad ordering')
        with self.assertRaises(ValueError):normalize([{**row,'glucose_mM':'-1'},{**row,'time_min':'2'}],'bad units')

if __name__=='__main__': unittest.main()
