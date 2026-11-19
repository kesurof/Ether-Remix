#!/usr/bin/env python3
"""Contrôle du guide Discord : python3 scripts/check_discord.py [docs/discord-streamfusion.md]"""
import re, sys

LIMITE = 1900
path = sys.argv[1] if len(sys.argv) > 1 else "docs/discord-streamfusion.md"
source = open(path, encoding="utf-8").read().replace("\r\n", "\n")
messages = re.findall(r"^## (Message [^\n]*)\n+````markdown\n([\s\S]*?)\n````[ \t]*$", source, re.M)
ouverts = len(re.findall(r"^````markdown", source, re.M))
erreurs = []
if len(messages) != ouverts:
    erreurs.append(f"{ouverts} bloc(s) ````markdown mais {len(messages)} message(s) extrait(s) : bloc mal formé")
if len(messages) < 5:
    erreurs.append(f"{len(messages)} message(s), 5 attendus au minimum")
for titre, contenu in messages:
    # len() Python compte les points de code ; Discord compte de façon proche : garder la marge de 1900.
    if not contenu.strip():
        erreurs.append(f"{titre} est vide")
    if len(contenu) > LIMITE:
        erreurs.append(f"{titre} : {len(contenu)} caractères, limite {LIMITE} dépassée")
for titre, contenu in messages:
    print(f"{len(contenu):5d}  {titre}")
if erreurs:
    print("\n".join("ÉCHEC : " + e for e in erreurs), file=sys.stderr)
    sys.exit(1)
