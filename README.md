# haptool

[English](#english) · [Français](#français)

## English

A small command-line tool. It talks HomeKit (HAP) directly to Logitech Circle View cameras paired with go2rtc, using go2rtc's own pairing keys. It is used to:

- read the inventory of their services and characteristics (`dump`, `get`);
- turn their status light and night vision on or off (`set`).

Documents (in French):

- Design: [`docs/CONCEPTION.md`](docs/CONCEPTION.md)
- Log: [`docs/JOURNAL.md`](docs/JOURNAL.md)
- Example inventory, with identifiers replaced: [`docs/inventaire.md`](docs/inventaire.md)

### Security rules

- **Keys never leave the tool's memory.** It reads them from go2rtc's YAML file at every run, read-only. The private key never leaves the `creds` package, which signs by itself.
- **All output is redacted.** A URL is shown as `homekit://IP:port?<REDACTED>`. Control characters coming from the network are escaped.
- **The HAP surface is minimal**, checked on every request:
  - pair-verify;
  - `GET /accessories` and `GET /characteristics`;
  - `PUT /characteristics`, with no query string.
- **Never touched:** `/pair-setup`, `/pairings` and `/resource`.
- **Writes are restricted** to the status light (`21D`, Camera Operating Mode Indicator) and night vision (`11B`), both in the Camera Operating Mode service (`21A`). Writes are always refused for:
  - the streaming service of go2rtc (`110`, `117`, `118`);
  - vendor types and opaque values;
  - timed writes.

### Requirements

- Go 1.24 or later.
- macOS: mDNS discovery uses `/usr/bin/dns-sd`, and the tool opens no listening socket. Elsewhere, it falls back to the address in the go2rtc URL, then to a scan of ports 49150 to 49160.

### Getting started

First enable the anti-leak hooks, once per clone. The pre-commit hook refuses keys, private IP addresses and real MAC addresses; the pre-push guard also checks the noreply author and refuses any identifier listed in `.githooks/identifiants-reels.local` (local file, ignored by git, one value per line; the push is refused while it is missing):

```sh
git config core.hooksPath .githooks
```

Then run the tests. They run on loopback, against a fake camera:

```sh
go test ./...
```

Build the tool, then point it to go2rtc's file. The binary is ignored by git:

```sh
go build -o haptool .
export HAPTOOL_CONFIG=/path/to/go2rtc.yaml
```

### Usage

`<camera>` is the name of the go2rtc stream without its `circleview_` prefix.

Readable inventory of one camera:

```sh
./haptool dump cam1
```

Markdown inventory of two cameras, with the table of differences. Write it under `dumps/`, which is ignored by git: it contains real identifiers.

```sh
./haptool dump --md cam1 cam2 > dumps/inventaire.md
```

Targeted read, with metadata from `/accessories`:

```sh
./haptool get cam1 1.948
```

Status light: `set` is a dry run by default. It shows the current value, the new value, the exact command to write, and the rollback:

```sh
./haptool set cam1 1.948 1
```

The write requires `--confirm` and `--expect TYPE:CURRENT`, as announced by the dry run. The camera status is read, then the value is read back:

```sh
./haptool set --confirm --expect 21D:0 cam1 1.948 1
```

Options:

- `--config`: go2rtc file (default: `$HAPTOOL_CONFIG`). It is read, never written.
- `--addr host:port`: forces a camera's address. Otherwise the tool tries, in order:
  1. mDNS, matched by `device_id`;
  2. the go2rtc URL;
  3. a port scan.
- `--json` or `--md` (`dump`): output format.
- `--confirm` and `--expect` (`set`): perform the announced write.

Exit codes:

- `0`: success or dry run;
- `1`: failure of the camera or the network;
- `2`: refusal or incorrect usage;
- `3`: global timeout (2 min).

### License

MIT, see [`LICENSE`](LICENSE). Third-party code and data:

- pair-verify comes from go2rtc (MIT);
- type names come from HAP-NodeJS (Apache-2.0).

Details are in [`THIRD_PARTY_NOTICES.md`](THIRD_PARTY_NOTICES.md).

## Français

Petit outil en ligne de commande. Il parle HomeKit (HAP) directement à des caméras Logitech Circle View appairées à go2rtc, avec les clés d'appairage de go2rtc. Il sert à :

- lire l'inventaire de leurs services et caractéristiques (`dump`, `get`) ;
- allumer ou éteindre leur voyant d'état et leur vision nocturne (`set`).

Documents :

- Conception : [`docs/CONCEPTION.md`](docs/CONCEPTION.md)
- Journal : [`docs/JOURNAL.md`](docs/JOURNAL.md)
- Exemple d'inventaire, identifiants remplacés : [`docs/inventaire.md`](docs/inventaire.md)

### Règles de sécurité

- **Les clés ne quittent jamais la mémoire de l'outil.** Il les relit à chaque lancement dans le fichier YAML de go2rtc, en lecture seule. La clé privée ne sort jamais du paquet `creds`, qui signe lui-même.
- **Toute sortie est expurgée.** Une URL s'affiche `homekit://IP:port?<REDACTED>`. Les caractères de contrôle venus du réseau sont échappés.
- **La surface HAP est réduite**, et vérifiée à chaque requête :
  - pair-verify ;
  - `GET /accessories` et `GET /characteristics` ;
  - `PUT /characteristics`, sans paramètre.
- **Jamais touchés :** `/pair-setup`, `/pairings` et `/resource`.
- **Les écritures sont limitées** au voyant (`21D`, Camera Operating Mode Indicator) et à la vision nocturne (`11B`), tous deux dans le service Camera Operating Mode (`21A`). Elles sont toujours refusées pour :
  - le service de streaming de go2rtc (`110`, `117`, `118`) ;
  - les types de fabricant et les valeurs opaques ;
  - les écritures temporisées.

### Prérequis

- Go 1.24 ou plus récent.
- macOS : la découverte mDNS passe par `/usr/bin/dns-sd`, et l'outil n'ouvre aucun socket d'écoute. Ailleurs, il se rabat sur l'adresse de l'URL go2rtc, puis sur un balayage des ports 49150 à 49160.

### Mise en route

Active d'abord les gardes anti-fuite, une fois par clone. Le hook pre-commit refuse les clés, les IP privées et les adresses MAC réelles ; le garde pre-push vérifie aussi l'auteur noreply et refuse tout identifiant listé dans `.githooks/identifiants-reels.local` (fichier local, ignoré par git, une valeur par ligne ; tant qu'il manque, le push est refusé) :

```sh
git config core.hooksPath .githooks
```

Lance ensuite les tests. Ils tournent sur la boucle locale, contre une fausse caméra :

```sh
go test ./...
```

Compile l'outil, puis indique-lui le fichier de go2rtc. Le binaire est ignoré par git :

```sh
go build -o haptool .
export HAPTOOL_CONFIG=/chemin/vers/go2rtc.yaml
```

### Usage

`<caméra>` est le nom du flux go2rtc, sans son préfixe `circleview_`.

Inventaire lisible d'une caméra :

```sh
./haptool dump cam1
```

Inventaire de deux caméras en Markdown, tableau des différences compris. Écris-le sous `dumps/`, ignoré par git : il contient les vrais identifiants.

```sh
./haptool dump --md cam1 cam2 > dumps/inventaire.md
```

Lecture ciblée, avec les métadonnées de `/accessories` :

```sh
./haptool get cam1 1.948
```

Voyant : `set` est à blanc par défaut. Il affiche la valeur actuelle, la nouvelle, la commande exacte pour écrire et le retour arrière :

```sh
./haptool set cam1 1.948 1
```

L'écriture exige `--confirm` et `--expect TYPE:ACTUELLE`, tels qu'annoncés à blanc. Le statut de la caméra est lu, puis la valeur est relue :

```sh
./haptool set --confirm --expect 21D:0 cam1 1.948 1
```

Options :

- `--config` : fichier de go2rtc (défaut : `$HAPTOOL_CONFIG`). Il est lu, jamais écrit.
- `--addr hôte:port` : impose l'adresse d'une caméra. Sinon, l'outil essaie dans l'ordre :
  1. le mDNS, reconnu par le `device_id` ;
  2. l'URL go2rtc ;
  3. un balayage des ports.
- `--json` ou `--md` (`dump`) : format de sortie.
- `--confirm` et `--expect` (`set`) : effectuent l'écriture annoncée.

Codes de sortie :

- `0` : succès ou exécution à blanc ;
- `1` : échec de la caméra ou du réseau ;
- `2` : refus ou usage incorrect ;
- `3` : délai global dépassé (2 min).

### Licence

MIT, voir [`LICENSE`](LICENSE). Code et données de tiers :

- pair-verify vient de go2rtc (MIT) ;
- les noms des types viennent de HAP-NodeJS (Apache-2.0).

Le détail est dans [`THIRD_PARTY_NOTICES.md`](THIRD_PARTY_NOTICES.md).
