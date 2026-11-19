# Guide StreamFusion Reborn pour Discord

Ce fichier contient le guide découpé en messages prêts à être publiés sur Discord (limite de 2000 caractères par message).

Pour chaque message ci-dessous : cliquez sur le bouton « copier » du bloc, puis collez-le dans un message séparé sur Discord, **dans l'ordre**.

Version complète : [streamfusion-ssdv2.md](streamfusion-ssdv2.md).

## Message 1 — Présentation

````markdown
# 🔗 StreamFusion Reborn dans Ether Remix
**Pour les utilisateurs SSDV2**

Ce guide ajoute la base de StreamFusion Reborn (SFR) comme source dans Ether Remix.

### Ce que cela apporte
- Un catalogue et une détection VF enrichis
- Une source de recherche **StreamFusion Reborn (base)** par film, saison et épisode
- Vos autres sources restent actives
- Accès à la base en **lecture seule**

> La disponibilité debrid est revérifiée avant chaque lecture.
````

## Message 2 — Prérequis

````markdown
## ✅ Prérequis
- Ether Remix installé avec SSDV2 (application `etherremix`)
- StreamFusion Reborn installé avec SSDV2 (application `streamfusion`)
- Un compte debrid configuré dans Ether Remix
- Un accès SSH au serveur

Console Ether Remix :
```text
https://etherremix.mondomaine.fr/ether/
```
> Le sous-domaine peut différer selon votre configuration SSDV2.
> Par défaut, la console n'est ouverte qu'au réseau local/VPN. Sinon, activez `allowRemoteConsole` (« Ouvrir cette console depuis l'extérieur de votre réseau », rubrique « Comment les applications se connectent » des réglages).

Les conteneurs utilisés ici sont `stremio-postgres` (la base de SFR) et `etherremix`. Vérifiez les vôtres avec `docker ps`.
````

## Message 3 — Étape 1 — Compte lecture seule

````markdown
## 🔐 Étape 1 : un compte en lecture seule
Ne donnez jamais à Remix le compte principal de SFR.
`-U streamfusion` est l'administrateur de la base, pas `streamfusion_runtime`.

Choisissez un mot de passe long, **lettres et chiffres uniquement**. Remplacez `CHOISISSEZ_UN_MOT_DE_PASSE` puis lancez :
```bash
docker exec -it stremio-postgres psql -U streamfusion -d streamfusion -c "CREATE ROLE remix_lecture LOGIN PASSWORD 'CHOISISSEZ_UN_MOT_DE_PASSE'; GRANT CONNECT ON DATABASE streamfusion TO remix_lecture; GRANT USAGE ON SCHEMA public TO remix_lecture; GRANT SELECT ON public.torrent_items TO remix_lecture;"
```
⚠️ Ne gardez pas le mot de passe d'exemple. Pour l'éviter dans l'historique : `CREATE ROLE remix_lecture LOGIN;` puis `\password remix_lecture` dans psql.

Si la commande échoue, vérifiez les noms avec `docker ps`.
````

## Message 4 — Étape 1 bis — Droits facultatifs

````markdown
## 🧩 Étape 1 bis : droits facultatifs
Sans ces droits, l'import des torrents fonctionne quand même.
Ouvrez d'abord une session, puis collez le SQL :
```bash
docker exec -it stremio-postgres psql -U streamfusion -d streamfusion
```

**Indications debrid** : seulement si la table `debrid_cache` existe.
```sql
GRANT SELECT ON public.debrid_cache TO remix_lecture;
```

**Alkodiques** : fichiers AllDebrid déjà vérifiés par SFR. Disponibles uniquement si l'intégration Alkodiques est installée dans SFR.
```sql
GRANT USAGE ON SCHEMA alko_integration TO remix_lecture;
GRANT SELECT ON alko_integration.files, alko_integration.source_links TO remix_lecture;
```

````

## Message 5 — Étape 2 — Réseau Docker

````markdown
## 🌐 Étape 2 : même réseau Docker
Dans une installation SSDV2 standard, `stremio-postgres` et `etherremix` partagent le réseau `traefik_proxy`. Rien à ajouter.

Pour vérifier :
```bash
docker inspect -f '{{json .NetworkSettings.Networks}}' stremio-postgres
docker inspect -f '{{json .NetworkSettings.Networks}}' etherremix
```
Les deux doivent citer `traefik_proxy`.

Sinon, ajoutez à Remix le réseau de la base (`docker network ls`), gardez ses réseaux actuels, puis recréez le conteneur.

⚠️ N'exposez **jamais** PostgreSQL sur Internet.
````

## Message 6 — Étape 3 — Saisir dans la console

````markdown
## 🖥️ Étape 3 : la console Ether Remix
1. Ouvrez la page **Ether-Remix**, onglet **Sources**, catégorie **Stremio**
2. Repérez le bloc **Base StreamFusion Reborn auto-hébergée**
3. Dans **Adresse PostgreSQL**, saisissez :
```text
postgres://remix_lecture:CHOISISSEZ_UN_MOT_DE_PASSE@stremio-postgres:5432/streamfusion
```
Le mot de passe est celui de l'étape 1. Après enregistrement, il n'est plus réaffiché.
````

## Message 7 — Étape 3 bis — Activer

````markdown
## ▶️ Étape 3 bis : activer et enregistrer
1. Activez **Activer** (désactivé par défaut)
2. Laissez **Inclure les trackers privés** désactivé, sauf besoin précis. Le modifier relance un import complet.
3. Cliquez sur le bouton d'enregistrement (« Enregistrer les modifications »)

**Intervalle** est dans **Options avancées** : 6 h par défaut, 30 minutes au minimum. Inutile d'y toucher en général.

> **Activer** et **Adresse PostgreSQL** sont tous deux nécessaires.
````

## Message 8 — Étape 4 — Vérifier

````markdown
## 🔎 Étape 4 : vérifier
- Le badge passe à **Actif**
- Une ligne affiche « Import en cours : N releases », puis « N releases importées pour N titres »
- Dans **Sources et activité**, **StreamFusion Reborn (base)** indique « Import actif »

Le premier import lit toute la base par lots : il peut être long, et reprend après un redémarrage.

Lancez ensuite un film ou un épisode en français : s'il existe un résultat SFR, il est proposé comme candidat.

Seules les releases françaises, MULTi ou VOSTFR de titres déjà au catalogue sont gardées.
````

## Message 9 — Alkodiques

````markdown
## 📁 Alkodiques (facultatif)
Ce sont des fichiers AllDebrid déjà vérifiés par SFR. Ils complètent les torrents.

Conditions :
- l'intégration Alkodiques est installée dans SFR
- les droits de l'étape 1 bis sont accordés
- un compte **AllDebrid** est activé dans Remix

Remix débride alors directement le lien du fichier, sans ajouter de magnet. Un lien supprimé échoue normalement, puis le candidat suivant est essayé.

Sans ces conditions, l'intégration torrent continue de fonctionner.
````

## Message 10 — Dépannage

````markdown
## 🛠️ Dépannage
- **« adresse postgres://… attendue »** : l'adresse doit commencer par `postgres://`
- **« ressaisissez le mot de passe de l'adresse »** : ressaisissez l'adresse complète
- **Badge « Inactif »** : **Activer** est désactivé ou l'adresse est vide
- **« Échec : … »** : vérifiez le mot de passe, les noms du conteneur et de la base, et le réseau (étape 2)
- **Peu de résultats** : seuls les titres déjà au catalogue sont gardés. Un titre ajouté plus tard est rattrapé automatiquement.
- **Pas de fichiers Alkodiques** : voir le message précédent
````

## Message 11 — À retenir

````markdown
## 📌 À retenir
- Compte PostgreSQL en **lecture seule**, jamais le compte principal de SFR
- Changez le mot de passe d'exemple
- N'exposez jamais PostgreSQL sur Internet
- **Activer** et **Adresse PostgreSQL** sont tous deux nécessaires
- Trackers privés : désactivés par défaut
- Alkodiques et `debrid_cache` sont facultatifs
- La disponibilité debrid est revérifiée avant lecture, vos autres sources restent actives

Besoin d'aide ? Posez vos questions directement dans ce sujet !
````
