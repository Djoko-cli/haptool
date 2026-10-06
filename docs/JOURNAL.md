# Journal de haptool

Version publique et condensée. Le journal détaillé reste chez le propriétaire. Les identifiants (IP, device_id, adresses MAC, numéros de série) sont remplacés par des valeurs de documentation.

## Étape 1 : mise en place

- **Environnement vérifié** : Go installé, go2rtc v1.9.14 lu (`pkg/hap`, `pkg/mdns`).
- **Conception validée** : voie courte, en deux phases, lecture puis écriture. Voir [`CONCEPTION.md`](CONCEPTION.md).
- **Garde-fous du dépôt** : hook pre-commit anti-fuite (clés, URL homekit://, IP privées, MAC réelles) et `.gitignore`.
- **Décision : ni `hap.Client.Dial()` ni mDNS dans le processus**, parce que le pare-feu applicatif de macOS est actif. L'adresse est résolue par `dns-sd`, et la séquence pair-verify est reprise de go2rtc.

## Étape 2 : inventaire

- **Deux `dump` en lecture seule**, l'un après l'autre. go2rtc n'a pas été perturbé : son journal ne montre rien de nouveau, et le flux livre ses paquets avant comme après.
- **Inventaire** : [`inventaire.md`](inventaire.md).
  - Firmware 8.2.64 sur les deux caméras.
  - Réglages HomeKit identiques. Seuls diffèrent les noms, les numéros de série, l'état du Wi-Fi et les adresses MAC.
- **Défaut du firmware** : `GET /characteristics` sert un JSON invalide avec `meta`, `perms` ou `ev`. L'outil ne demande plus que `type=1`.

## Étape 3 : diagnostic du micro

- **Symptôme mesuré** : une caméra n'émet que du silence, des trames Opus de 6 octets, alors que l'autre capte le son.
- **Aucun réglage HomeKit ne distingue les deux caméras** : Mute (`11A`), Volume (`119`), formats audio (`115`), enregistrement (`204`, `226`), firmware.
- **Conclusion** : les causes logicielles visibles en HAP sont exclues, ce qui oriente vers le matériel. L'étape 5, la correction du micro, est donc sans objet.

## Étape 4 : voyant

- **Phase B** : la commande `set` a été relue par Opus, et ses remarques corrigées avant la première écriture. Il y a désormais une liste blanche limitée à `21D`, `--expect`, et une relecture après toute erreur.
- **Écriture `21D = 1`** (Camera Operating Mode Indicator), avec l'accord explicite du propriétaire.
  - HTTP 204, valeur relue 1, flux de go2rtc intact.
  - Le voyant s'allume en **rouge pulsé**.
- **Pourquoi pulsé.** D'après la documentation de Logitech, le rouge respirant signale une vue en direct. Le rouge fixe signale « diffusion et enregistrement autorisés ». C'est une hypothèse : go2rtc gardant un flux ouvert en permanence, la caméra se croit toujours regardée.
- **Retour à `21D = 0`**, à la demande du propriétaire. HTTP 204, valeur relue 0.

## Étape 6 : compte rendu

- Toutes les étapes sont faites ; la 5 est sans objet.
- Deux relectures sécurité par Opus, une par phase, puis une relecture finale avant publication. Tous les points relevés sont corrigés et testés.
- Les constats qui concernent le pipeline (adresses, coupures de flux) ont été transmis à son responsable.
