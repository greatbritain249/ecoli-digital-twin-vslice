"""Lightweight guards for the browser's regression-prone rendering UI."""
from pathlib import Path
import re
import unittest

ROOT = Path(__file__).resolve().parents[1]

class TestPerformanceUi(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.html = (ROOT / 'apps/web/index.html').read_text()
        cls.css = (ROOT / 'apps/web/styles.css').read_text()
        cls.js = (ROOT / 'apps/web/app.js').read_text()

    def test_hidden_attribute_is_not_overridden_by_modal_display(self):
        self.assertRegex(self.css, r'\[hidden\]\s*\{\s*display:\s*none\s*!important;')
        self.assertRegex(self.html, r'id="inspector"[^>]*\bhidden\b')
        self.assertRegex(self.html, r'id="molstar-loading"')

    def test_quality_profiles_and_render_control_exist(self):
        self.assertIn('id="render-quality"', self.html)
        for profile in ['performance', 'balanced', 'quality']:
            self.assertIn(f'{profile}: {{pixelRatio:', self.js)
        self.assertIn('renderer.setPixelRatio(', self.js)

    def test_render_pause_and_idle_gate_exist(self):
        self.assertIn('document.hidden || suspended || !inViewport', self.js)
        self.assertIn('if (!moving && !orbitSettling && !dirty)', self.js)
        self.assertIn('three?.setSuspended(true)', self.js)
        self.assertIn('three?.setSuspended(false)', self.js)

if __name__ == '__main__':
    unittest.main()
