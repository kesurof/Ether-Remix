# Ajouter StreamFusion Reborn comme source (SSDV2)

Ce guide s'adresse aux utilisateurs de SSDV2 (seedbox-compose) qui ont déjà StreamFusion Reborn (SFR) et Ether Remix. Il explique comment brancher la base de SFR sur Ether Remix, en lecture seule.

Le détail technique complet est dans [STREAMFUSION.fr.md](pse/STREAMFUSION.fr.md). La version à copier dans Discord est dans [discord-streamfusion.md](discord-streamfusion.md).

## Ce que cela apporte

- Remix lit la base de SFR pour enrichir son catalogue et sa détection VF.
- Une source de recherche **StreamFusion Reborn (base)** s'ajoute aux sources de Remix : recherche par film, saison et épisode.
- Vos autres sources restent actives. Avant lecture, la disponibilité est revérifiée par vos fournisseurs debrid.

## Prérequis

- Ether Remix installé avec SSDV2 (application `etherremix`). Sa console s'ouvre sur `https://etherremix.mondomaine.fr/ether/` (le sous-domaine peut différer selon votre configuration SSDV2). Par défaut, la console `/ether/` n'est accessible que depuis le réseau local ou le VPN. Depuis l'extérieur, passez par le réseau local, ou activez le réglage `allowRemoteConsole` (libellé anglais « Ouvrir cette console depuis l'extérieur de votre réseau », rubrique « Comment les applications se connectent » des réglages, dans les réglages, rubrique « Comment les applications se connectent »). Seul le compte Plex propriétaire du serveur peut alors se connecter.
- StreamFusion Reborn installé avec SSDV2 (application `streamfusion`), donc avec ses conteneurs `stream-fusion` et `stremio-postgres`.
- Un compte debrid configuré dans Ether Remix : la lecture en dépend.
- Un accès SSH au serveur pour lancer une commande `docker exec`.

> Les noms de conteneurs ci-dessus sont ceux des fichiers de l'installation SSDV2. Vérifiez les vôtres avec `docker ps`.

## Étape 1 : créer un compte en lecture seule dans la base de SFR

Ne donnez jamais à Remix le compte principal de SFR. Créez un compte qui ne peut que lire.

Dans la commande ci-dessous, `-U streamfusion -d streamfusion` désigne le compte administrateur de la base, différent du compte `streamfusion_runtime` utilisé par SFR au quotidien.

Choisissez d'abord un mot de passe long, avec uniquement des lettres et des chiffres (les symboles compliquent l'adresse de l'étape 3). Puis lancez, en remplaçant `CHOISISSEZ_UN_MOT_DE_PASSE` :

```bash
docker exec -it stremio-postgres psql -U streamfusion -d streamfusion -c "CREATE ROLE remix_lecture LOGIN PASSWORD 'CHOISISSEZ_UN_MOT_DE_PASSE'; GRANT CONNECT ON DATABASE streamfusion TO remix_lecture; GRANT USAGE ON SCHEMA public TO remix_lecture; GRANT SELECT ON public.torrent_items TO remix_lecture;"
```

Ne gardez pas le mot de passe d'exemple. Pour éviter qu'il reste dans l'historique du shell, vous pouvez créer le compte sans mot de passe (`CREATE ROLE remix_lecture LOGIN;`), puis le définir dans une session `psql` interactive avec `\password remix_lecture`. Si la commande échoue, vérifiez le nom du conteneur et de la base avec `docker ps`.

### Option : disponibilité debrid et fichiers Alkodiques

Ces droits sont facultatifs. Sans eux, l'import des torrents fonctionne quand même.

Ouvrez d'abord une session sur la base, puis collez-y les commandes SQL ci-dessous :

```bash
docker exec -it stremio-postgres psql -U streamfusion -d streamfusion
```

Pour que Remix tienne compte des indications de disponibilité debrid de SFR, **si la table `debrid_cache` existe** :

```sql
GRANT SELECT ON public.debrid_cache TO remix_lecture;
```

Les **Alkodiques** sont des fichiers AllDebrid déjà vérifiés par SFR. Ils ne sont disponibles que si l'intégration Alkodiques est installée dans SFR (tables `alko_integration.files` et `alko_integration.source_links`). Dans ce cas :

```sql
GRANT USAGE ON SCHEMA alko_integration TO remix_lecture;
GRANT SELECT ON alko_integration.files, alko_integration.source_links TO remix_lecture;
```

Pour lire un fichier Alkodique, un compte **AllDebrid activé dans Remix** est nécessaire : Remix débride directement le lien du fichier, sans ajouter de magnet.

## Étape 2 : vérifier que Remix peut joindre la base

Dans une installation SSDV2 standard, `stremio-postgres` et Ether Remix partagent le réseau Docker `traefik_proxy`. Rien à ajouter. Pour le vérifier :

```bash
docker inspect -f '{{json .NetworkSettings.Networks}}' stremio-postgres
docker inspect -f '{{json .NetworkSettings.Networks}}' etherremix
```

Les deux doivent citer `traefik_proxy`. Si ce n'est pas le cas, ajoutez à Remix le réseau de la base (nom exact avec `docker network ls`), puis recréez le conteneur. Gardez les réseaux déjà présents. N'exposez jamais PostgreSQL sur Internet.

## Étape 3 : saisir l'adresse dans la console Ether Remix

1. Ouvrez la console, page **Ether-Remix**, onglet **Sources**, catégorie **Stremio**.
2. Dans le bloc **Base StreamFusion Reborn auto-hébergée**, remplissez **Adresse PostgreSQL** :

   ```text
   postgres://remix_lecture:CHOISISSEZ_UN_MOT_DE_PASSE@stremio-postgres:5432/streamfusion
   ```

   Le mot de passe est celui de l'étape 1. Une fois enregistré, il n'est plus réaffiché.
3. Laissez **Inclure les trackers privés** désactivé, sauf besoin précis. Changer cette option relance un import complet.
4. Activez **Activer** (désactivé par défaut), puis cliquez sur le bouton d'enregistrement (« Enregistrer les modifications »).

L'option **Intervalle** se trouve dans **Options avancées** : 6 h par défaut, 30 minutes au minimum. Vous n'avez normalement pas à la changer.

## Étape 4 : vérifier que la source fonctionne

- Dans le bloc, le badge passe à **Actif** et une ligne indique « Import en cours : N releases », puis « N releases importées pour N titres ».
- Le premier import lit toute la base, par lots. Il peut être long ; il reprend après un redémarrage.
- Dans **Sources et activité**, la ligne **StreamFusion Reborn (base)** affiche « Import actif ». Cette source est aussi proposée parmi les sources de recherche.
- Lancez ensuite la lecture d'un film ou d'un épisode français : si un résultat SFR existe, il est proposé comme candidat.

## Dépannage

- **« adresse postgres://… attendue »** : l'adresse doit commencer par `postgres://` ou `postgresql://`.
- **« ressaisissez le mot de passe de l'adresse »** : le mot de passe est masqué après enregistrement. Ressaisissez l'adresse complète.
- **Badge « Inactif »** : l'option **Activer** est désactivée, ou l'adresse est vide. Les deux sont nécessaires.
- **Échec : …** dans le bloc : vérifiez le mot de passe, le nom du conteneur, le nom de la base, et que les deux conteneurs partagent un réseau (étape 2).
- **Import terminé mais peu de résultats** : seules les releases françaises, MULTi ou VOSTFR de titres déjà présents au catalogue de Remix sont gardées. Un titre ajouté plus tard est rattrapé automatiquement.
- **Pas de fichiers Alkodiques** : l'intégration Alkodiques n'est pas installée dans SFR, les droits de l'étape 1 manquent, ou aucun compte AllDebrid n'est activé dans Remix.
- **Un résultat échoue à la lecture** : un lien supprimé échoue normalement, et Remix essaie le candidat suivant.

## À retenir

- Un compte PostgreSQL en **lecture seule**, jamais le compte principal de SFR.
- Changez le mot de passe d'exemple. N'exposez jamais PostgreSQL sur Internet.
- **Activer** et **Adresse PostgreSQL** sont tous deux nécessaires.
- Trackers privés : désactivés par défaut.
- Alkodiques et `debrid_cache` sont facultatifs et dépendent de votre installation de SFR.
- La disponibilité debrid est revérifiée avant lecture, vos autres sources restent actives.

## Besoin d'aide ?

Posez vos questions directement dans le sujet Discord du projet.
