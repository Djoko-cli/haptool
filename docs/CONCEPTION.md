# Conception de haptool

`haptool` parle HAP directement à des caméras Logitech Circle View appairées à go2rtc, avec les clés d'appairage de go2rtc. Il sert à lire leurs services et caractéristiques, et à régler leur voyant d'état.

## Processus suivi

1. Conception validée par le propriétaire.
2. Prototype sur une branche.
3. Relecture sécurité par Opus **avant tout contact avec les caméras**.
4. Banc avec le propriétaire.
5. Report sur `main` en quelques commits clairs, puis relecture finale par Opus.

Le prototype s'est fait en deux phases :

- **Phase A, lecture seule** : `dump` et `get`.
- **Phase B, écriture** : `set`. Elle a été conçue une fois l'inventaire connu, et a eu sa propre relecture sécurité.

## Choix techniques

Go, un binaire, module `github.com/Djoko-cli/haptool`. Dépendances : `github.com/AlexxIT/go2rtc v1.9.14` et `gopkg.in/yaml.v3`, déjà présent dans le `go.mod` de go2rtc. De go2rtc, l'outil utilise :

- les primitives `curve25519`, `hkdf`, `chacha20poly1305`, `ed25519` et `tlv8` ;
- le canal chiffré `hap.NewConn` ;
- les aides `hap.ReadResponse` et `hap.Append` ;
- les constantes de `pkg/hap` ;
- les structures TLV8 de `pkg/hap/camera`, pour l'affichage.

**Pas de `hap.Client.Dial()`.**

- *Pourquoi.* `Dial()` lance toujours une découverte mDNS dans le processus, avec des sockets d'écoute UDP `:0` et `:5353` (`pkg/mdns/client.go:118, 193, 230`). Avec le pare-feu applicatif de macOS, on risque :
  - une fenêtre d'autorisation à chaque binaire recompilé ;
  - des réponses bloquées ;
  - 4 s d'attente quand le mDNS ne répond pas.
- *Ce que fait l'outil à la place.*
  - Il résout l'adresse par le démon système (`dns-sd`).
  - Il rejoue la séquence pair-verify de `Client.Dial()` (`pkg/hap/client.go:107-232`, licence MIT, voir `THIRD_PARTY_NOTICES.md`) sur sa propre connexion TCP, avec un délai sur chaque échange.
  - Il n'ouvre aucun socket d'écoute.

Défauts lus dans `pkg/hap` v1.9.14, et parades :

| Défaut (fichier:ligne) | Conséquence | Parade |
|---|---|---|
| `PutCharacters` lit la réponse puis la jette (`client.go:341`) | un rejet `207` / `-70410` passe inaperçu | `hapc.Session.Write`, qui lit toujours le statut `204` ou `207` |
| `Character` ignore `minValue` … `valid-values` (`character.go:25-30`) | bornes perdues par `GetAccessories()` | `/accessories` décodé par nos structures |
| Signature de la caméra non vérifiée si `DevicePublic` est `nil` (`client.go:168`) ; `DecodeKey` renvoie `nil` sans erreur (`client.go:366`) | identité envoyée à l'appareil qui occupe une adresse périmée | refus sans `device_public` de 32 octets ni `client_private` de 64 octets cohérent |
| `hap.Dial(rawURL)` appelle `url.Parse`, dont l'erreur recopie l'URL (`net/url/url.go:40`) | clés dans un message d'erreur | ni `hap.Dial` ni `hap.Client` : pair-verify maison |
| `ConnDeadline` déclaré mais inutilisé (`client.go:27`) ; socket non fermé sur erreur (`client.go:102`) | blocage indéfini | délai sur chaque échange, pair-verify compris ; fermeture systématique ; chien de garde global |
| `Dial()` découvre toujours en mDNS dans le processus (`client.go:94`) | sockets d'écoute : pare-feu, 4 s d'attente | résolution par `dns-sd`, pair-verify sur notre connexion |

## Frontière de sécurité, appliquée par le code

1. **Les clés restent en mémoire.**
   - L'outil les lit au lancement dans le fichier YAML de go2rtc, ouvert en lecture seule. Ce fichier est indiqué par `--config` ou `HAPTOOL_CONFIG`, sans chemin par défaut.
   - Les clés de toutes les caméras du fichier sont enregistrées dans le filtre avant toute autre opération.
   - La clé privée ne sort jamais de `creds`, qui signe (`Sign`) et vérifie la signature de la caméra (`VerifyDevice`) lui-même.
2. **Toute sortie est filtrée**, ligne à ligne : stdout, stderr, `log`.
   - Le filtre masque les valeurs exactes des clés : hex en toute casse, base64, liste décimale.
   - Il masque aussi les motifs `homekit://…?`, `client_private=`, `device_public=` et `client_id=`.
   - Il ne panique jamais : une chaîne en UTF-8 invalide est enregistrée sous sa forme hexadécimale.
   - Une panique s'affiche avec un message fixe, sauf s'il s'agit d'une erreur d'exécution de Go.
3. **Les caractères de contrôle et bidirectionnels sont neutralisés** dans toute sortie. En Markdown, `|`, l'accent grave et `<` sont échappés.
4. **Les erreurs ne recopient rien.** Celles du YAML et de l'URL sont remplacées par des messages sans contenu. Le texte de statut HTTP d'un pair est cité.
5. **Liste blanche des requêtes**, vérifiée à chaque envoi (`hapc.allowed`) :
   - `POST /pair-verify` ;
   - `GET /accessories` et `GET /characteristics` ;
   - `PUT /characteristics` exactement, sans paramètre.

   Le test de clôture échoue si le code de production cite `/pair-setup`, `/pairings`, `/resource`, `/prepare`, les fonctions d'appairage de go2rtc ou `hap.Client`.
6. **Une connexion courte par commande**, sans abonnement aux événements, refermée même en cas de panique.
7. **Valeurs masquées** :
   - `118` (Setup Endpoints) : sa réponse porte les clés SRTP maîtresses, d'après `camera.SetupEndpointsResponse` de go2rtc ;
   - `131` (Setup Data Stream Transport) ;
   - toute valeur opaque sans type.

   Le masquage s'applique dans tous les rendus.
8. **Écritures** (`internal/guard`, commande `set`).
   - *Liste blanche*, chaque type lié au service qui doit le porter : le voyant `21D` et la vision nocturne `11B`, dans `21A`. Tout autre type, comme `21B` (HomeKit Camera Active, qui couperait le flux de go2rtc) ou `11A` (Mute), demande une décision du propriétaire.
   - *Toujours refusés* :
     - le service `110` et les types `117`, `118` et `131` ;
     - les types de fabricant et les formats opaques ;
     - l'absence de `pw`, les écritures temporisées (`tw`) et l'autorisation additionnelle (`aa`).
   - *Valeur contrôlée* : type, bornes, pas, valeurs admises, longueur.
   - *Déroulé de `set`* :
     - à blanc par défaut, il affiche la valeur actuelle, la nouvelle, la commande exacte pour écrire et le retour arrière ;
     - `--confirm` exige `--expect TYPE:VALEUR_ACTUELLE`, tel qu'annoncé à blanc ;
     - une seule écriture, dont les statuts HTTP et HAP sont lus, puis la valeur est relue ;
     - une erreur survenue après l'envoi donne « état inconnu », la relecture et le retour arrière.

## Unités

| Paquet | Rôle |
|---|---|
| `internal/redact` | Filtre d'expurgation et de neutralisation ; écrivain ligne à ligne. |
| `internal/creds` | Lit le YAML, extrait, valide et enregistre les clés ; signe et vérifie. |
| `internal/hapc` | Session HAP : résolution d'adresse, pair-verify, liste blanche, lectures, écriture. |
| `internal/guard` | Politique d'écriture et encodage des valeurs. |
| `internal/catalog` | Noms des types et des statuts HAP, d'après HAP-NodeJS v2.2.3. |
| `internal/report` | Rendus texte, JSON et Markdown, et différences entre caméras. |
| `internal/fakeacc` | Réservé aux tests. Fausse caméra sur la boucle locale, avec le `PairVerify` côté serveur de go2rtc. |
| `main.go` | Commandes, chien de garde, branchement des sorties filtrées. |

## Commandes

- `haptool dump <caméra>… [--json|--md]` : inventaire. Avec deux caméras, `--md` ajoute le tableau des différences.
- `haptool get <caméra> <aid>.<iid>[,…]` : valeur et statut, complétés par les métadonnées de `/accessories`.
- `haptool set [--confirm --expect TYPE:ACTUELLE] <caméra> <aid>.<iid> <valeur>` : réglage du voyant.
- Options communes :
  - `--config`, ou la variable `HAPTOOL_CONFIG` ;
  - `--addr hôte:port`.
- Délais : 5 s par échange réseau, et un chien de garde arrête l'outil au bout de 2 min (code 3).
- `<caméra>` désigne le flux go2rtc `circleview_<caméra>`.

## Résolution d'adresse

1. **Adresse imposée** par `--addr`, le cas échéant.
2. **mDNS, par le démon système.**
   - `dns-sd -Z _hap._tcp` (environ 3 s) liste les annonces.
   - Toutes celles dont le TXT `id` vaut le `device_id`, en toute casse, sont essayées.
   - Leur hôte SRV est résolu par `net.LookupIP` ; un hôte non résolu est sauté.
3. **URL go2rtc.** pair-verify y est tenté directement.
4. **Balayage** des ports 49150 à 49160 de l'hôte de l'URL. Seuls les ports dont un `GET /accessories` en clair répond `470` sont retenus.

Dans tous les cas, c'est pair-verify qui prouve l'identité de la caméra : face à un autre appareil, seul M1 part, avec une clé de session éphémère. Le fichier de go2rtc n'est jamais corrigé.

## Particularité du firmware 8.2.64

`GET /characteristics` renvoie un JSON invalide dès que `meta`, `perms` ou `ev` figure dans la requête. L'outil ne demande donc que `type=1`, et prend les métadonnées dans `/accessories`. La fausse caméra des tests reproduit ce défaut.

## Tests

Les clés factices sont générées à l'exécution et jamais écrites dans le dépôt. Les tests n'utilisent que la boucle locale. Chaque garde-fou a été vérifié par mutation : cassé exprès, son test doit échouer.

- **Expurgation et neutralisation** : URL, paramètres, erreur de `url.Parse`, secret coupé entre deux écritures, UTF-8 invalide, caractères de contrôle.
- **Lecture du YAML** : cas nominal, flux absent, clés invalides ou incohérentes. Aucun message d'erreur ne doit contenir de secret.
- **Fausse caméra** (`hap.Server.PairVerify` et `hap.NewConn`) :
  - `dump`, `get` et `set` de bout en bout ;
  - un imposteur ne reçoit jamais M3 ;
  - délais, statuts `207`, écriture ignorée, statut mensonger, réponse illisible ;
  - défaut du firmware reproduit.
- **Résolution** : sortie `dns-sd`, sonde `470`, ordre des sources, annonces multiples.
- **Sorties sans secret** : chaque commande, lancée sans le filtre, ne doit écrire aucune clé.
- **Clôture** : le code de production reste dans le périmètre HAP autorisé.
