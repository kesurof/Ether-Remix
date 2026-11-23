#!/usr/bin/env python3
"""Contrôle du format d'Aerya (docs/pse/FORK.fr.md, AGENTS.md) sur une PR du fork.

Bloque les traductions Ether, les nouveaux fichiers hors des emplacements
Remix et les modifications d'Ether sans point d'accroche marqué et documenté.
Contrôle aussi la note RemixUpdates et les README selon le type de PR.

Usage : python3 scripts/check_format_aerya.py <base> [titre de la PR]
Compare HEAD à la base de fusion avec <base> (ex. origin/main).
"""
import re
import subprocess
import sys

# Le kit est partagé avec Ether. scrobview est un paquet Remix historique.
REMIX_DIRS = (
    "internal/pse/", "web/src/pse/", "internal/frontend/kit/scrobview/",
    ".github/pse/", "docs/pse/", "internal/admin/dist/",
)
REMIX_FILES = {"README.md", "README.en.md", "README.fr.md"}
FORK_FILES = {
    ".github/workflows/fork-checks.yml",
    ".github/workflows/fork-image.yml",
    "docs/discord-streamfusion.md",
    "docs/streamfusion-ssdv2.md",
    "scripts/check_discord.py",
    "scripts/check_format_aerya.py",
    "scripts/test_check_format_aerya.py",
}
# Fichiers partagés qui portent un ajout du fork (étape check_discord, lien SSDV2).
FORK_TOUCHED = FORK_FILES | {".github/workflows/ci.yml", "docs/pse/STREAMFUSION.fr.md"}
FORK_DOC = "docs/pse/FORK.fr.md"
UPDATES = "web/src/pse/RemixUpdates.tsx"
LOCALES = re.compile(r"^web/src/i18n/locales/[^/]+\.json$")
TEST = re.compile(r"(_test\.go|\.test\.tsx?)$")
REMIX_WORKFLOW = re.compile(r"^\.github/workflows/pse-[^/]+\.ya?ml$")
FRONTEND_KIT_PACKAGE = re.compile(r"^(internal/frontend/kit/[^/]+)/")
HOOK_TABLE_HEADER = re.compile(r"^\| *Fichier d'Ether *\| *Ajout *\| *$", re.MULTILINE)
NOTE_ITEM = re.compile(r"^      \[", re.MULTILINE)
NOTE_CURRENT = re.compile(r'^    version: "current",$', re.MULTILINE)
NOTE_ID = re.compile(r'^  \{\n    id: "(\d{4}-\d{2}-\d{2}-[a-z0-9-]+)",$', re.MULTILINE)


def git(*args):
    return subprocess.run(
        ["git", "--literal-pathspecs", *args], check=True, capture_output=True, text=True
    ).stdout


def remix(path):
    name = path.rsplit("/", 1)[-1]
    return (
        path.startswith(REMIX_DIRS)
        or path in REMIX_FILES
        or path in FORK_FILES
        or REMIX_WORKFLOW.fullmatch(path)
        or path.startswith("internal/frontend/kit/pse_")
        or (path.startswith(("internal/", "web/src/")) and name.startswith("pse_"))
    )


def hook_rows(fork_doc):
    header = HOOK_TABLE_HEADER.search(fork_doc)
    if not header:
        return set()
    rows = set()
    for row in fork_doc[header.end() + 1:].splitlines():
        if not row.startswith("|"):
            break
        rows.add(row)
    return rows


def note_ids(contents):
    _, _, entries = contents.partition("export const REMIX_UPDATES = [\n")
    return set(NOTE_ID.findall(entries.split("\n]", 1)[0]))


def current_note(contents):
    """Identifiant et texte de la note « current », ou (None, "")."""
    _, _, entries = contents.partition("export const REMIX_UPDATES = [\n")
    entries = entries.split("\n]", 1)[0]
    starts = [m.start() for m in NOTE_ID.finditer(entries)] + [len(entries)]
    current = [entries[b:e] for b, e in zip(starts, starts[1:]) if NOTE_CURRENT.search(entries[b:e])]
    if len(current) != 1:  # AGENTS.md : une seule note porte « current ».
        return None, ""
    return NOTE_ID.search(current[0]).group(1), current[0]


def grows_current_note(before, after):
    """Le fork ne publie pas de version : ses PR complètent la note « current »
    encore non publiée au lieu d'en figer une à une version inexistante."""
    note, old = current_note(before)
    same, new = current_note(after)
    return note is not None and note == same and len(NOTE_ITEM.findall(new)) > len(NOTE_ITEM.findall(old))


def synced_on(upstream, base):
    """Vrai pour une synchronisation : HEAD contient l'amont et n'en diffère que par le fork."""
    try:
        git("merge-base", "--is-ancestor", upstream, "HEAD")
    except subprocess.CalledProcessError:
        return False
    diff = set(git("diff", "--name-only", "-z", upstream, "HEAD").split("\0")[:-1])
    if not diff <= FORK_TOUCHED:
        return False
    # La PR doit apporter du code d'Aerya : une PR limitée aux fichiers du fork reste contrôlée.
    mb = git("merge-base", base, "HEAD").strip()
    brought = set(git("diff", "--name-only", "-z", mb, "HEAD").split("\0")[:-1])
    return bool(brought - FORK_TOUCHED)


def main():
    base = sys.argv[1] if len(sys.argv) > 1 else "origin/main"
    title = sys.argv[2] if len(sys.argv) > 2 else ""
    upstream = sys.argv[3] if len(sys.argv) > 3 else ""
    if upstream and synced_on(upstream, base):
        print(f"Synchronisation sur {upstream} : seuls les fichiers propres au fork diffèrent.")
        sys.exit(0)
    mb = git("merge-base", base, "HEAD").strip()
    changes = iter(git("diff", "--name-status", "--no-renames", "-z", mb, "HEAD").split("\0")[:-1])
    files = {path: status for status, path in zip(changes, changes)}
    hook_paths = set()
    doc_status = files.get(FORK_DOC)
    if doc_status and doc_status != "D":
        previous_rows = set() if doc_status == "A" else hook_rows(git("show", f"{mb}:{FORK_DOC}"))
        for row in hook_rows(git("show", f"HEAD:{FORK_DOC}")) - previous_rows:
            file_column = row.split("|", 2)[1]
            hook_paths.update(re.findall(r"`([^`]+)`", file_column))
    errors, warnings = [], []
    for path, status in sorted(files.items()):
        if LOCALES.match(path):
            errors.append(f"{path} : traductions Ether modifiées ; les traductions Remix vont dans web/src/pse/i18n/")
            continue
        if status == "D" or remix(path):
            continue
        if status == "A":
            kit_package = FRONTEND_KIT_PACKAGE.match(path)
            # Un nouveau paquet du kit est autorisé. Pour le reconnaître lors des
            # PR suivantes, utiliser un préfixe pse_ ou le déclarer dans REMIX_DIRS.
            if kit_package and not git("ls-tree", "--name-only", mb, "--", kit_package[1]):
                continue
            errors.append(f"{path} : nouveau fichier hors des emplacements Remix (internal/pse/, web/src/pse/, ou préfixe pse_ à côté du point d'accroche)")
            continue
        added = [l[1:] for l in git("diff", "-U0", mb, "HEAD", "--", path).splitlines()
                 if l.startswith("+") and not l.startswith("+++")]
        if TEST.search(path):
            warnings.append(f"{path} : test d'Ether modifié ; à justifier dans la PR et dans FORK.fr.md")
        if not any("PSE-HOOK:" in l for l in added):
            errors.append(f"{path} : fichier d'Ether modifié sans marqueur « PSE-HOOK: <ajout> — voir docs/pse/FORK.fr.md » dans les lignes ajoutées")
        elif path not in hook_paths:
            errors.append(f"{path} : point d'accroche absent des lignes ajoutées ou mises à jour du tableau de {FORK_DOC}")
    code = any(
        not path.startswith(("docs/", ".github/", "internal/admin/dist/")) and not path.endswith(".md")
        for path in files
    )
    kind = title.split(":", 1)[0].split("(", 1)[0].strip().rstrip("!").lower()
    if code:
        note_status = files.get(UPDATES)
        if kind in ("feat", "fix"):
            added_note = False
            if note_status and note_status != "D":
                before = "" if note_status == "A" else git("show", f"{mb}:{UPDATES}")
                after = git("show", f"HEAD:{UPDATES}")
                previous_ids, current_ids = note_ids(before), note_ids(after)
                added_note = previous_ids < current_ids or (
                    previous_ids == current_ids and grows_current_note(before, after)
                )
            if not added_note:
                errors.append(f"pas de nouvelle note avec identifiant inédit ni de nouvel item dans la note current de {UPDATES} (AGENTS.md)")
        elif not note_status or note_status == "D":
            warnings.append(f"pas de note dans {UPDATES} (AGENTS.md)")
    missing = {path for path in ("README.md", "README.en.md") if files.get(path) in (None, "D")}
    if code and kind in ("feat", "fix") and missing:
        msg = ", ".join(sorted(missing)) + " non mis à jour (AGENTS.md)"
        (errors if kind == "feat" else warnings).append(msg)
    for w in warnings:
        print("ATTENTION : " + w)
    for e in errors:
        print("ÉCHEC : " + e, file=sys.stderr)
    print(f"{len(files)} fichier(s) examiné(s), {len(errors)} erreur(s), {len(warnings)} avertissement(s).")
    sys.exit(1 if errors else 0)


if __name__ == "__main__":
    main()
