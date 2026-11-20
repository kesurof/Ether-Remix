import subprocess
import sys
import tempfile
import unittest
from pathlib import Path


CHECKER = Path(__file__).with_name("check_format_aerya.py").resolve()
FORK_DOC = "docs/pse/FORK.fr.md"
ETHER_FILE = "internal/app/app.go"
ETHER_KIT_FILE = "internal/frontend/kit/authhdr/authhdr.go"
LOCALE = "web/src/i18n/locales/fr.json"
UPDATES = "web/src/pse/RemixUpdates.tsx"
UPDATE_LIST = "export const REMIX_UPDATES = [\n"
HOOK_TABLE = (
    "Points d’accroche actuels :\n\n"
    "| Fichier d'Ether | Ajout |\n"
    "| --- | --- |\n"
    f"| `{ETHER_FILE}` | Ancien branchement |\n"
)


class FormatCheckTest(unittest.TestCase):
    def setUp(self):
        directory = tempfile.TemporaryDirectory()
        self.addCleanup(directory.cleanup)
        self.repo = Path(directory.name)
        self.git("init", "-q")
        self.write(FORK_DOC, HOOK_TABLE)
        self.write(ETHER_FILE, "package app\n")
        self.write(ETHER_KIT_FILE, "package authhdr\n")
        self.write(LOCALE, "{}\n")
        self.write(".github/workflows/release.yml", "name: Release\n")
        self.write(UPDATES, UPDATE_LIST + '  {\n    id: "2026-11-19-example",\n  },\n];\n')
        for readme in ("README.md", "README.en.md"):
            self.write(readme, "# Ether-Remix\n")
        self.commit()
        self.base = self.git("rev-parse", "HEAD").strip()

    def git(self, *args):
        return subprocess.run(
            ["git", *args], cwd=self.repo, capture_output=True, text=True, check=True
        ).stdout

    def write(self, name, content):
        path = self.repo / name
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_text(content, encoding="utf-8")

    def commit(self):
        self.git("add", "-A")
        self.git("-c", "user.name=Format test", "-c", "user.email=test@example.invalid",
                 "-c", "commit.gpgsign=false", "commit", "-qm", "fixture")

    def check_format(self, title="ci: test du format"):
        self.commit()
        return subprocess.run(
            [sys.executable, str(CHECKER), self.base, title], cwd=self.repo,
            capture_output=True, text=True,
        )

    def test_rejects_file_outside_remix_locations(self):
        self.write("scripts/foreign.py", "pass\n")
        result = self.check_format()
        self.assertEqual(result.returncode, 1, result.stdout + result.stderr)
        self.assertIn("scripts/foreign.py", result.stderr)

    def test_rejects_ether_workflow_without_hook(self):
        self.write(".github/workflows/release.yml", "name: New release workflow\n")
        result = self.check_format()
        self.assertEqual(result.returncode, 1, result.stdout + result.stderr)
        self.assertIn("PSE-HOOK", result.stderr)

    def test_rejects_ether_kit_change_without_hook(self):
        self.write(ETHER_KIT_FILE, "package authhdr\n\nconst user = \"example\"\n")
        result = self.check_format()
        self.assertEqual(result.returncode, 1, result.stdout + result.stderr)
        self.assertIn(ETHER_KIT_FILE, result.stderr)

    def test_accepts_new_remix_kit_package(self):
        self.write("internal/frontend/kit/remixview/view.go", "package remixview\n")
        result = self.check_format()
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)

    def test_requires_hook_in_updated_table(self):
        self.write(ETHER_FILE, "package app\n// PSE-HOOK: nouveau filtre\n")
        self.write(FORK_DOC, HOOK_TABLE + f"\nLe nouveau filtre touche `{ETHER_FILE}`.\n")
        result = self.check_format()
        self.assertEqual(result.returncode, 1, result.stdout + result.stderr)
        self.assertIn("tableau", result.stderr)

    def test_accepts_new_hook_documented_in_table(self):
        self.write(ETHER_FILE, "package app\n// PSE-HOOK: nouveau filtre\n")
        self.write(FORK_DOC, HOOK_TABLE + f"| `{ETHER_FILE}` | nouveau filtre |\n")
        result = self.check_format()
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)

    def test_rejects_deleted_ether_translation(self):
        (self.repo / LOCALE).unlink()
        result = self.check_format()
        self.assertEqual(result.returncode, 1, result.stdout + result.stderr)
        self.assertIn(LOCALE, result.stderr)

    def test_requires_new_update_and_preserved_readmes_for_feature(self):
        self.write("internal/pse/feature/feature.go", "package feature\n")
        self.write(UPDATES, (self.repo / UPDATES).read_text(encoding="utf-8") + "\n")
        (self.repo / "README.md").unlink()
        (self.repo / "README.en.md").unlink()
        result = self.check_format("feat!: ajout Remix")
        self.assertEqual(result.returncode, 1, result.stdout + result.stderr)
        for name in (UPDATES, "README.md", "README.en.md"):
            self.assertIn(name, result.stderr)

    def test_accepts_feature_with_new_update_and_readmes(self):
        self.write("internal/pse/feature/feature.go", "package feature\n")
        updates = (self.repo / UPDATES).read_text(encoding="utf-8")
        self.write(UPDATES, updates.replace(UPDATE_LIST, UPDATE_LIST + '  {\n    id: "2026-11-20-new",\n  },\n'))
        for readme in ("README.md", "README.en.md"):
            self.write(readme, "# Ether-Remix\n\nNouveau réglage.\n")
        result = self.check_format("feat: ajout Remix")
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
