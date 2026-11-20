#!/usr/bin/env python3
"""Contrôle du format d'Aerya (docs/pse/FORK.fr.md, AGENTS.md) sur une PR du fork.

Calibré sur les PR fusionnées d'Aerya (#139 à #148) : bloque ce qu'il fait
systématiquement (aucune traduction Ether, note RemixUpdates, README fr/en),
avertit pour les points d'accroche, que la revue doit arbitrer.

Usage : python3 scripts/check_format_aerya.py <base> [titre de la PR]
Compare HEAD à la base de fusion avec <base> (ex. origin/main).
"""
import re, subprocess, sys

# Emplacements propres à Ether-Remix : tout peut y être ajouté ou modifié.
REMIX = ("internal/pse/", "web/src/pse/", "internal/frontend/kit/", ".github/", "docs/", "scripts/",
         "internal/admin/dist/", "README.md", "README.en.md", "README.fr.md", "AGENTS.md")
LOCALES = re.compile(r"^web/src/i18n/locales/[^/]+\.json$")
TEST = re.compile(r"(_test\.go|\.test\.tsx?)$")


def git(*args):
    return subprocess.run(["git", *args], check=True, capture_output=True, text=True).stdout


def remix(path):
    name = path.rsplit("/", 1)[-1]
    return path.startswith(REMIX) or name.startswith("pse_") or name.startswith("pse-")


def main():
    base = sys.argv[1] if len(sys.argv) > 1 else "origin/main"
    title = sys.argv[2] if len(sys.argv) > 2 else ""
    mb = git("merge-base", base, "HEAD").strip()
    changes = [l.split("\t") for l in git("diff", "--name-status", "--no-renames", mb, "HEAD").splitlines() if l]
    files = {p: s for s, p in changes}
    fork_doc = git("show", f"HEAD:docs/pse/FORK.fr.md") if "docs/pse/FORK.fr.md" not in files or files["docs/pse/FORK.fr.md"] != "D" else ""
    errors, warnings = [], []
    for path, status in sorted(files.items()):
        if status == "D" or remix(path):
            continue
        if LOCALES.match(path):
            errors.append(f"{path} : traductions Ether modifiées ; les traductions Remix vont dans web/src/pse/i18n/")
            continue
        if status == "A":
            warnings.append(f"{path} : nouveau fichier hors des emplacements Remix (internal/pse/, web/src/pse/, ou préfixe pse_ à côté du point d'accroche)")
            continue
        added = [l[1:] for l in git("diff", "-U0", mb, "HEAD", "--", path).splitlines()
                 if l.startswith("+") and not l.startswith("+++")]
        if TEST.search(path):
            warnings.append(f"{path} : test d'Ether modifié ; à justifier dans la PR et dans FORK.fr.md")
            continue
        if not any("PSE-HOOK" in l for l in added):
            warnings.append(f"{path} : fichier d'Ether modifié sans marqueur « PSE-HOOK: <ajout> — voir docs/pse/FORK.fr.md » dans les lignes ajoutées")
        elif f"`{path}`" not in fork_doc:
            warnings.append(f"{path} : point d'accroche absent du tableau de docs/pse/FORK.fr.md")
    code = [p for p, s in files.items() if s != "D" and not p.startswith(("docs/", ".github/", "internal/admin/dist/")) and not p.endswith(".md")]
    kind = title.split(":", 1)[0].split("(", 1)[0].strip().lower()
    if code and "web/src/pse/RemixUpdates.tsx" not in files:
        msg = "pas de note dans web/src/pse/RemixUpdates.tsx (AGENTS.md)"
        (errors if kind in ("feat", "fix") else warnings).append(msg)
    missing = {"README.md", "README.en.md"} - files.keys()
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
