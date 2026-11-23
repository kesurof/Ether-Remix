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

    def sync_check(self, extra):
        self.git("checkout", "-qb", "amont")
        self.write(LOCALE, '{"ether": "nouveau"}\n')
        self.write(ETHER_FILE, "package app\n\nvar amont = 1\n")
        self.commit()
        self.git("checkout", "-q", "-")
        self.write("scripts/check_discord.py", "# fork\n")
        self.commit()
        self.git("-c", "user.name=t", "-c", "user.email=t@example.invalid", "merge", "-q",
                 "--no-edit", "-X", "theirs", "amont")
        extra()
        return subprocess.run(
            [sys.executable, str(CHECKER), self.base, "chore: synchro", "amont"], cwd=self.repo,
            capture_output=True, text=True,
        )

    def test_accepts_sync_that_only_adds_fork_files(self):
        result = self.sync_check(lambda: None)
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertIn("Synchronisation", result.stdout)

    def test_checks_sync_that_changes_ether_beyond_upstream(self):
        def extra():
            self.write(ETHER_FILE, "package app\n\nvar fork = 2\n")
            self.commit()
        result = self.sync_check(extra)
        self.assertEqual(result.returncode, 1, result.stdout + result.stderr)
        self.assertIn(LOCALE, result.stderr)

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

    CURRENT = (UPDATE_LIST + '  {\n    id: "2026-11-21-current",\n    version: "current",\n'
               '    items: [\n      ["a", "a"],\n    ],\n  },\n'
               '  {\n    id: "2026-11-19-example",\n    version: "0.2.37-remix.114.1",\n  },\n];\n')

    def fix_with_updates(self, updates):
        self.write(UPDATES, self.CURRENT)
        self.commit()
        self.base = self.git("rev-parse", "HEAD").strip()
        self.write("internal/pse/feature/feature.go", "package feature\n")
        self.write(UPDATES, updates)
        return self.check_format("fix: correction Remix")

    def test_accepts_lines_added_to_current_note(self):
        grown = self.CURRENT.replace('      ["a", "a"],\n', '      ["a", "a"],\n      ["b", "b"],\n')
        result = self.fix_with_updates(grown)
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)

    def test_rejects_unchanged_current_note(self):
        result = self.fix_with_updates(self.CURRENT + "\n")
        self.assertEqual(result.returncode, 1, result.stdout + result.stderr)
        self.assertIn(UPDATES, result.stderr)

    def test_rejects_blank_line_or_comment_in_current_note(self):
        for filler in ("\n", "      // x\n"):
            with self.subTest(filler=filler):
                padded = self.CURRENT.replace('      ["a", "a"],\n', '      ["a", "a"],\n' + filler)
                result = self.fix_with_updates(padded)
                self.assertEqual(result.returncode, 1, result.stdout + result.stderr)

    def test_rejects_second_current_note(self):
        doubled = self.CURRENT.replace('"0.2.37-remix.114.1"', '"current"').replace('      ["a", "a"],\n', '      ["a", "a"],\n      ["b", "b"],\n')
        result = self.fix_with_updates(doubled)
        self.assertEqual(result.returncode, 1, result.stdout + result.stderr)

    def test_rejects_grown_note_when_an_id_is_removed(self):
        grown = self.CURRENT.replace('      ["a", "a"],\n', '      ["a", "a"],\n      ["b", "b"],\n')
        grown = grown.replace('  {\n    id: "2026-11-19-example",\n    version: "0.2.37-remix.114.1",\n  },\n', "")
        result = self.fix_with_updates(grown)
        self.assertEqual(result.returncode, 1, result.stdout + result.stderr)

    def test_rejects_lines_added_to_a_frozen_note(self):
        frozen = self.CURRENT.replace('    version: "0.2.37-remix.114.1",\n', '    version: "0.2.37-remix.114.1",\n    items: [["b", "b"]],\n')
        result = self.fix_with_updates(frozen)
        self.assertEqual(result.returncode, 1, result.stdout + result.stderr)

    def test_accepts_feature_with_new_update_and_readmes(self):
        self.write("internal/pse/feature/feature.go", "package feature\n")
        updates = (self.repo / UPDATES).read_text(encoding="utf-8")
        self.write(UPDATES, updates.replace(UPDATE_LIST, UPDATE_LIST + '  {\n    id: "2026-11-20-new",\n  },\n'))
        for readme in ("README.md", "README.en.md"):
            self.write(readme, "# Ether-Remix\n\nNouveau réglage.\n")
        result = self.check_format("feat: ajout Remix")
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
