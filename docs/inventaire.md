# Inventaire HomeKit des Circle View

Généré par `haptool dump --md cam1 cam2` le 2026-10-06, puis **identifiants remplacés avant publication** : IP (192.0.2.0/24), device_id et adresses MAC (02:00:00:…), numéros de série, noms d'instance, Product Data, heures de vérification du firmware et pays Wi-Fi.

- Noms des types : HAP-NodeJS v2.2.3 (Apache-2.0) : src/lib/definitions/ServiceDefinitions.ts, src/lib/definitions/CharacteristicDefinitions.ts, src/lib/HAPServer.ts.
- Valeurs masquées : `118` (Setup Endpoints) et `131` (Setup Data Stream Transport), qui peuvent porter des clés de session.
- Valeurs TLV8 décodées avec les structures de go2rtc v1.9.14 (`pkg/hap/camera`) quand elles existent.

## Différences entre cam1 et cam2

| Service | Caractéristique | Champ | cam1 | cam2 |
|---|---|---|---|---|
| Accessory Information (3E) | Name (23) | valeur | "Circle View AAAA" | "Circle View BBBB" |
| Accessory Information (3E) | Serial Number (30) | valeur | "SERIE-DEG" | "SERIE-COURS" |
| Camera Operating Mode (21A) | Name (23) | valeur | "Operating Mode (AAAA)" | "Operating Mode (BBBB)" |
| Light Sensor (84) | Current Ambient Light Level (6B) | valeur | 0.3856595 | 0.0001 |
| inconnu, fabricant (68163DDE-599E-C8BC-7B45-77DA153E1AE7) | inconnu, fabricant (84E14E8A-CF28-3DA8-A949-C505248B9CD9) | valeur | 58 | 45 |
| inconnu, fabricant (68163DDE-599E-C8BC-7B45-77DA153E1AE7) | inconnu, fabricant (E9DB48C2-954C-4BBC-8A4B-43BD90131F81) | valeur | 11 | 6 |
| inconnu, fabricant (68163DDE-599E-C8BC-7B45-77DA153E1AE7) | inconnu, fabricant (3BF3E754-1DE6-BA9E-6243-D25C272A72F5) | valeur | "02:00:00:00:00:0D" | "02:00:00:00:00:0C" |

## cam1

device_id `02:00:00:00:0D:01` · firmware `"8.2.64"` · adresse jointe `192.0.2.98:49152` (mDNS) · URL go2rtc `192.0.2.46:49154`

### [1.1] Accessory Information (3E)

| iid | type | nom | valeur | droits | format | unité | min | max | pas | valides | plage | maxLen | autres |
|---|---|---|---|---|---|---|---|---|---|---|---|---|---|
| 2 | 14 | Identify | — | pw | bool |  |  |  |  |  |  |  |  |
| 3 | 20 | Manufacturer | "Logitech" | pr | string |  |  |  |  |  |  |  |  |
| 4 | 21 | Model | "Circle View Camera" | pr | string |  |  |  |  |  |  |  |  |
| 5 | 23 | Name | "Circle View AAAA" | pr | string |  |  |  |  |  |  |  |  |
| 6 | 30 | Serial Number | "SERIE-DEG" | pr | string |  |  |  |  |  |  |  |  |
| 7 | 52 | Firmware Revision | "8.2.64" | pr | string |  |  |  |  |  |  |  |  |
| 8 | 53 | Hardware Revision | "8" | pr | string |  |  |  |  |  |  |  |  |
| 289 | 34AB8811-AC7F-4340-BAC3-FD6A85F9943B | inconnu (fabricant) | "3.0;17A126" | pr,hd | string |  |  |  |  |  |  |  |  |
| 290 | 220 | Product Data | 8 o : (masqué) | pr,hd | data |  |  |  |  |  |  |  | maxDataLen=8 |

### [1.305] Protocol Information (A2)

| iid | type | nom | valeur | droits | format | unité | min | max | pas | valides | plage | maxLen | autres |
|---|---|---|---|---|---|---|---|---|---|---|---|---|---|
| 307 | 37 | Version | "1.1.0" | pr | string |  |  |  |  |  |  |  |  |

### [1.9] Camera RTP Stream Management (110) · principal

| iid | type | nom | valeur | droits | format | unité | min | max | pas | valides | plage | maxLen | autres |
|---|---|---|---|---|---|---|---|---|---|---|---|---|---|
| 322 | B0 | Active | 1 | pr,pw,ev,tw | uint8 |  | 0 | 1 | 1 |  |  |  |  |
| 10 | 120 | Streaming Status | {Status:1} | pr,ev | tlv8 |  |  |  |  |  |  |  |  |
| 11 | 114 | Supported Video Stream Configuration | {Codecs:[{CodecType:0 CodecParams:[{ProfileID:[1] Level:[0 2] PacketizationMode:0 CVOEnabled:[0]}] VideoAttrs:[{Width:1280 Height:720 Framerate:30} {Width:640 Height:480 Framerate:30} {Width:640 Height:360 Framerate:30} {Width:480 Height:360 Framerate:30} {Width:480 Height:270 Framerate:30} {Width:320 Height:240 Framerate:30} {Width:320 Height:180 Framerate:30}]}]} | pr | tlv8 |  |  |  |  |  |  |  |  |
| 12 | 115 | Supported Audio Stream Configuration | {Codecs:[{CodecType:3 CodecParams:[{Channels:1 BitrateMode:0 SampleRate:[0 1]}]}] ComfortNoiseSupport:0} | pr | tlv8 |  |  |  |  |  |  |  |  |
| 13 | 116 | Supported RTP Configuration | {SRTPCryptoType:[0 1 2]} | pr | tlv8 |  |  |  |  |  |  |  |  |
| 14 | 118 | Setup Endpoints | (vide) | pr,pw | tlv8 |  |  |  |  |  |  |  |  |
| 15 | 117 | Selected RTP Stream Configuration | (vide) | pr,pw | tlv8 |  |  |  |  |  |  |  |  |

### [1.368] Camera Recording Management (204) · liés : [352 40]

| iid | type | nom | valeur | droits | format | unité | min | max | pas | valides | plage | maxLen | autres |
|---|---|---|---|---|---|---|---|---|---|---|---|---|---|
| 369 | B0 | Active | 0 | pr,pw,ev,tw | uint8 |  | 0 | 1 | 1 |  |  |  |  |
| 370 | 205 | Supported Camera Recording Configuration | {PrebufferLength:8000 EventTriggerOptions:1 MediaContainerConfigurations:{MediaContainerType:0 MediaContainerParameters:{FragmentLength:4000}}} | pr,ev | tlv8 |  |  |  |  |  |  |  |  |
| 371 | 206 | Supported Video Recording Configuration | {CodecConfigs:[{CodecType:0 CodecParams:{ProfileID:1 Level:2 Bitrate:0 IFrameInterval:0} CodecAttrs:{Width:640 Height:360 Framerate:15}}]} | pr,ev | tlv8 |  |  |  |  |  |  |  |  |
| 372 | 207 | Supported Audio Recording Configuration | {CodecConfigs:[{CodecType:0 CodecParams:[{Channels:1 BitrateMode:[0] SampleRate:[3]}]}]} | pr,ev | tlv8 |  |  |  |  |  |  |  |  |
| 373 | 209 | Selected Camera Recording Configuration | (vide) | pr,pw,ev | tlv8 |  |  |  |  |  |  |  |  |
| 374 | 226 | Recording Audio Active | 0 | pr,pw,ev,tw | uint8 |  | 0 | 1 | 1 |  |  |  |  |

### [1.34] Microphone (112)

| iid | type | nom | valeur | droits | format | unité | min | max | pas | valides | plage | maxLen | autres |
|---|---|---|---|---|---|---|---|---|---|---|---|---|---|
| 257 | 23 | Name | "Microphone" | pr | string |  |  |  |  |  |  |  |  |
| 35 | 11A | Mute | 0 | pr,pw,ev | bool |  |  |  |  |  |  |  |  |
| 36 | 119 | Volume | 79 | pr,pw,ev | uint8 | percentage | 0 | 100 | 1 |  |  |  |  |

### [1.37] Speaker (113)

| iid | type | nom | valeur | droits | format | unité | min | max | pas | valides | plage | maxLen | autres |
|---|---|---|---|---|---|---|---|---|---|---|---|---|---|
| 273 | 23 | Name | "Speaker" | pr | string |  |  |  |  |  |  |  |  |
| 38 | 11A | Mute | 0 | pr,pw,ev | bool |  |  |  |  |  |  |  |  |
| 39 | 119 | Volume | 88 | pr,pw,ev | uint8 | percentage | 0 | 100 | 1 |  |  |  |  |

### [1.40] Motion Sensor (85)

| iid | type | nom | valeur | droits | format | unité | min | max | pas | valides | plage | maxLen | autres |
|---|---|---|---|---|---|---|---|---|---|---|---|---|---|
| 41 | 22 | Motion Detected | 0 | pr,ev | bool |  |  |  |  |  |  |  |  |
| 42 | 75 | Status Active | 1 | pr,ev | bool |  |  |  |  |  |  |  |  |

### [1.352] Data Stream Transport Management (129)

| iid | type | nom | valeur | droits | format | unité | min | max | pas | valides | plage | maxLen | autres |
|---|---|---|---|---|---|---|---|---|---|---|---|---|---|
| 353 | 130 | Supported Data Stream Transport Configuration | {Configs:[{TransportType:0}]} | pr | tlv8 |  |  |  |  |  |  |  |  |
| 354 | 131 | Setup Data Stream Transport | (vide) | pr,pw,wr | tlv8 |  |  |  |  |  |  |  |  |
| 355 | 37 | Version | "1.0" | pr | string |  |  |  |  |  |  |  |  |

### [1.928] Camera Operating Mode (21A)

| iid | type | nom | valeur | droits | format | unité | min | max | pas | valides | plage | maxLen | autres |
|---|---|---|---|---|---|---|---|---|---|---|---|---|---|
| 936 | 23 | Name | "Operating Mode (AAAA)" | pr | string |  |  |  |  |  |  |  | description="Operating Mode" |
| 949 | 223 | Event Snapshots Active | 1 | pr,pw,ev | uint8 |  | 0 | 1 | 1 |  |  |  | description="Event Snapshots Active" |
| 950 | 225 | Periodic Snapshots Active | 1 | pr,pw,ev | uint8 |  | 0 | 1 | 1 |  |  |  | description="Periodic Snapshots Active" |
| 947 | 21B | HomeKit Camera Active | 1 | pr,pw,ev | uint8 |  | 0 | 1 | 1 |  |  |  | description="Homekit Active" |
| 948 | 21D | Camera Operating Mode Indicator | 0 | pr,pw,ev | uint8 |  | 0 | 1 | 1 |  |  |  | description="Mode Indicator" |
| 945 | 11B | Night Vision | 1 | pr,pw,ev | bool |  |  |  |  |  |  |  | description="\"Night Vision LED\"" |
| 951 | 227 | Manually Disabled | 0 | pr,ev | uint8 |  | 0 | 1 | 1 |  |  |  | description="\"Manually Disabled\"" |

### [1.1536] Light Sensor (84)

| iid | type | nom | valeur | droits | format | unité | min | max | pas | valides | plage | maxLen | autres |
|---|---|---|---|---|---|---|---|---|---|---|---|---|---|
| 1553 | 6B | Current Ambient Light Level | 0.3856595 | pr,ev | float | lux | 0.0001 | 100000 | 0 |  |  |  |  |
| 1554 | 75 | Status Active | 1 | pr,ev | bool |  |  |  |  |  |  |  | description="Ambient Light Sensor Active" |

### [1.1025] inconnu, fabricant (68163DDE-599E-C8BC-7B45-77DA153E1AE7)

| iid | type | nom | valeur | droits | format | unité | min | max | pas | valides | plage | maxLen | autres |
|---|---|---|---|---|---|---|---|---|---|---|---|---|---|
| 1032 | 23 | Name | "General Settings" | pr | string |  |  |  |  |  |  |  | description="Settings Name" |
| 1041 | 84E14E8A-CF28-3DA8-A949-C505248B9CD9 | inconnu (fabricant) | 58 | pr,ev | uint8 |  | 0 | 100 | 1 |  |  |  | description="WiFi Signal Strength" |
| 1042 | 2C063F78-398F-DBB0-9E48-8F2DC1FBDA2E | inconnu (fabricant) | "(masqué)" | pr | string |  |  |  |  |  |  | 4 | description="WiFi Country Code" |
| 1046 | E9DB48C2-954C-4BBC-8A4B-43BD90131F81 | inconnu (fabricant) | 11 | pr,ev | uint16 |  |  |  |  |  |  |  | description="WiFi Channel" |
| 1043 | 27BE48D7-AB48-1684-1540-1F1E3C9BA0FA | inconnu (fabricant) | 60 | pr,pw | uint8 |  | 0 | 100 | 1 |  |  |  | description="Motion Detector Sensitivity" |
| 1044 | F55B9EB2-6120-74B9-544C-D40A55E5CC5A | inconnu (fabricant) | — | pw | bool |  |  |  |  |  |  |  | description="Submit Diagnostics to Logitech" |
| 1045 | A9896218-6661-D7B1-954D-7CDEEAB302D8 | inconnu (fabricant) | 0 | pr,pw | bool |  |  |  |  |  |  |  | description="Persistent Log" |
| 1047 | 3BF3E754-1DE6-BA9E-6243-D25C272A72F5 | inconnu (fabricant) | "02:00:00:00:00:0D" | pr | string |  |  |  |  |  |  | 18 | description="MAC Address" |
| 1048 | AB9B9865-4D5C-42AB-0F4B-C5F0EC84E5EB | inconnu (fabricant) | — | pw | bool |  |  |  |  |  |  |  | description="Check for New Firmware" |
| 1049 | 4A7C9629-4F4F-55B0-B540-CF545AAEB961 | inconnu (fabricant) | "(masqué)" | pr | string |  |  |  |  |  |  | 32 | description="Last Firmware Update Check Time" |
| 1050 | 0840EBC4-8B90-E8B6-294C-C242CFFEF1F1 | inconnu (fabricant) | "(masqué)" | pr | string |  |  |  |  |  |  | 32 | description="Next Firmware Update Check Time" |

### [1.1057] inconnu, fabricant (10FCD34C-62E9-2495-2E45-0D2BC698F563)

| iid | type | nom | valeur | droits | format | unité | min | max | pas | valides | plage | maxLen | autres |
|---|---|---|---|---|---|---|---|---|---|---|---|---|---|
| 1064 | 23 | Name | "Video Settings" | pr | string |  |  |  |  |  |  |  | description="Video Settings Name" |
| 1073 | 56964A81-42F2-F983-7B45-947851ECC249 | inconnu (fabricant) | 20 | pr,pw | int |  | 0 | 30 | 1 |  |  |  | description="IQ Minimum Day Frame Rate" |
| 1074 | F6C475B0-8645-D686-E74D-E089A338138C | inconnu (fabricant) | 20 | pr,pw | int |  | 0 | 30 | 1 |  |  |  | description="IQ Minimum Night Frame Rate" |

## cam2

device_id `02:00:00:00:0C:02` · firmware `"8.2.64"` · adresse jointe `192.0.2.45:49152` (mDNS) · URL go2rtc `192.0.2.45:49152`

### [1.1] Accessory Information (3E)

| iid | type | nom | valeur | droits | format | unité | min | max | pas | valides | plage | maxLen | autres |
|---|---|---|---|---|---|---|---|---|---|---|---|---|---|
| 2 | 14 | Identify | — | pw | bool |  |  |  |  |  |  |  |  |
| 3 | 20 | Manufacturer | "Logitech" | pr | string |  |  |  |  |  |  |  |  |
| 4 | 21 | Model | "Circle View Camera" | pr | string |  |  |  |  |  |  |  |  |
| 5 | 23 | Name | "Circle View BBBB" | pr | string |  |  |  |  |  |  |  |  |
| 6 | 30 | Serial Number | "SERIE-COURS" | pr | string |  |  |  |  |  |  |  |  |
| 7 | 52 | Firmware Revision | "8.2.64" | pr | string |  |  |  |  |  |  |  |  |
| 8 | 53 | Hardware Revision | "8" | pr | string |  |  |  |  |  |  |  |  |
| 289 | 34AB8811-AC7F-4340-BAC3-FD6A85F9943B | inconnu (fabricant) | "3.0;17A126" | pr,hd | string |  |  |  |  |  |  |  |  |
| 290 | 220 | Product Data | 8 o : (masqué) | pr,hd | data |  |  |  |  |  |  |  | maxDataLen=8 |

### [1.305] Protocol Information (A2)

| iid | type | nom | valeur | droits | format | unité | min | max | pas | valides | plage | maxLen | autres |
|---|---|---|---|---|---|---|---|---|---|---|---|---|---|
| 307 | 37 | Version | "1.1.0" | pr | string |  |  |  |  |  |  |  |  |

### [1.9] Camera RTP Stream Management (110) · principal

| iid | type | nom | valeur | droits | format | unité | min | max | pas | valides | plage | maxLen | autres |
|---|---|---|---|---|---|---|---|---|---|---|---|---|---|
| 322 | B0 | Active | 1 | pr,pw,ev,tw | uint8 |  | 0 | 1 | 1 |  |  |  |  |
| 10 | 120 | Streaming Status | {Status:1} | pr,ev | tlv8 |  |  |  |  |  |  |  |  |
| 11 | 114 | Supported Video Stream Configuration | {Codecs:[{CodecType:0 CodecParams:[{ProfileID:[1] Level:[0 2] PacketizationMode:0 CVOEnabled:[0]}] VideoAttrs:[{Width:1280 Height:720 Framerate:30} {Width:640 Height:480 Framerate:30} {Width:640 Height:360 Framerate:30} {Width:480 Height:360 Framerate:30} {Width:480 Height:270 Framerate:30} {Width:320 Height:240 Framerate:30} {Width:320 Height:180 Framerate:30}]}]} | pr | tlv8 |  |  |  |  |  |  |  |  |
| 12 | 115 | Supported Audio Stream Configuration | {Codecs:[{CodecType:3 CodecParams:[{Channels:1 BitrateMode:0 SampleRate:[0 1]}]}] ComfortNoiseSupport:0} | pr | tlv8 |  |  |  |  |  |  |  |  |
| 13 | 116 | Supported RTP Configuration | {SRTPCryptoType:[0 1 2]} | pr | tlv8 |  |  |  |  |  |  |  |  |
| 14 | 118 | Setup Endpoints | (vide) | pr,pw | tlv8 |  |  |  |  |  |  |  |  |
| 15 | 117 | Selected RTP Stream Configuration | (vide) | pr,pw | tlv8 |  |  |  |  |  |  |  |  |

### [1.368] Camera Recording Management (204) · liés : [352 40]

| iid | type | nom | valeur | droits | format | unité | min | max | pas | valides | plage | maxLen | autres |
|---|---|---|---|---|---|---|---|---|---|---|---|---|---|
| 369 | B0 | Active | 0 | pr,pw,ev,tw | uint8 |  | 0 | 1 | 1 |  |  |  |  |
| 370 | 205 | Supported Camera Recording Configuration | {PrebufferLength:8000 EventTriggerOptions:1 MediaContainerConfigurations:{MediaContainerType:0 MediaContainerParameters:{FragmentLength:4000}}} | pr,ev | tlv8 |  |  |  |  |  |  |  |  |
| 371 | 206 | Supported Video Recording Configuration | {CodecConfigs:[{CodecType:0 CodecParams:{ProfileID:1 Level:2 Bitrate:0 IFrameInterval:0} CodecAttrs:{Width:640 Height:360 Framerate:15}}]} | pr,ev | tlv8 |  |  |  |  |  |  |  |  |
| 372 | 207 | Supported Audio Recording Configuration | {CodecConfigs:[{CodecType:0 CodecParams:[{Channels:1 BitrateMode:[0] SampleRate:[3]}]}]} | pr,ev | tlv8 |  |  |  |  |  |  |  |  |
| 373 | 209 | Selected Camera Recording Configuration | (vide) | pr,pw,ev | tlv8 |  |  |  |  |  |  |  |  |
| 374 | 226 | Recording Audio Active | 0 | pr,pw,ev,tw | uint8 |  | 0 | 1 | 1 |  |  |  |  |

### [1.34] Microphone (112)

| iid | type | nom | valeur | droits | format | unité | min | max | pas | valides | plage | maxLen | autres |
|---|---|---|---|---|---|---|---|---|---|---|---|---|---|
| 257 | 23 | Name | "Microphone" | pr | string |  |  |  |  |  |  |  |  |
| 35 | 11A | Mute | 0 | pr,pw,ev | bool |  |  |  |  |  |  |  |  |
| 36 | 119 | Volume | 79 | pr,pw,ev | uint8 | percentage | 0 | 100 | 1 |  |  |  |  |

### [1.37] Speaker (113)

| iid | type | nom | valeur | droits | format | unité | min | max | pas | valides | plage | maxLen | autres |
|---|---|---|---|---|---|---|---|---|---|---|---|---|---|
| 273 | 23 | Name | "Speaker" | pr | string |  |  |  |  |  |  |  |  |
| 38 | 11A | Mute | 0 | pr,pw,ev | bool |  |  |  |  |  |  |  |  |
| 39 | 119 | Volume | 88 | pr,pw,ev | uint8 | percentage | 0 | 100 | 1 |  |  |  |  |

### [1.40] Motion Sensor (85)

| iid | type | nom | valeur | droits | format | unité | min | max | pas | valides | plage | maxLen | autres |
|---|---|---|---|---|---|---|---|---|---|---|---|---|---|
| 41 | 22 | Motion Detected | 0 | pr,ev | bool |  |  |  |  |  |  |  |  |
| 42 | 75 | Status Active | 1 | pr,ev | bool |  |  |  |  |  |  |  |  |

### [1.352] Data Stream Transport Management (129)

| iid | type | nom | valeur | droits | format | unité | min | max | pas | valides | plage | maxLen | autres |
|---|---|---|---|---|---|---|---|---|---|---|---|---|---|
| 353 | 130 | Supported Data Stream Transport Configuration | {Configs:[{TransportType:0}]} | pr | tlv8 |  |  |  |  |  |  |  |  |
| 354 | 131 | Setup Data Stream Transport | (vide) | pr,pw,wr | tlv8 |  |  |  |  |  |  |  |  |
| 355 | 37 | Version | "1.0" | pr | string |  |  |  |  |  |  |  |  |

### [1.928] Camera Operating Mode (21A)

| iid | type | nom | valeur | droits | format | unité | min | max | pas | valides | plage | maxLen | autres |
|---|---|---|---|---|---|---|---|---|---|---|---|---|---|
| 936 | 23 | Name | "Operating Mode (BBBB)" | pr | string |  |  |  |  |  |  |  | description="Operating Mode" |
| 949 | 223 | Event Snapshots Active | 1 | pr,pw,ev | uint8 |  | 0 | 1 | 1 |  |  |  | description="Event Snapshots Active" |
| 950 | 225 | Periodic Snapshots Active | 1 | pr,pw,ev | uint8 |  | 0 | 1 | 1 |  |  |  | description="Periodic Snapshots Active" |
| 947 | 21B | HomeKit Camera Active | 1 | pr,pw,ev | uint8 |  | 0 | 1 | 1 |  |  |  | description="Homekit Active" |
| 948 | 21D | Camera Operating Mode Indicator | 0 | pr,pw,ev | uint8 |  | 0 | 1 | 1 |  |  |  | description="Mode Indicator" |
| 945 | 11B | Night Vision | 1 | pr,pw,ev | bool |  |  |  |  |  |  |  | description="\"Night Vision LED\"" |
| 951 | 227 | Manually Disabled | 0 | pr,ev | uint8 |  | 0 | 1 | 1 |  |  |  | description="\"Manually Disabled\"" |

### [1.1536] Light Sensor (84)

| iid | type | nom | valeur | droits | format | unité | min | max | pas | valides | plage | maxLen | autres |
|---|---|---|---|---|---|---|---|---|---|---|---|---|---|
| 1553 | 6B | Current Ambient Light Level | 0.0001 | pr,ev | float | lux | 0.0001 | 100000 | 0 |  |  |  |  |
| 1554 | 75 | Status Active | 1 | pr,ev | bool |  |  |  |  |  |  |  | description="Ambient Light Sensor Active" |

### [1.1025] inconnu, fabricant (68163DDE-599E-C8BC-7B45-77DA153E1AE7)

| iid | type | nom | valeur | droits | format | unité | min | max | pas | valides | plage | maxLen | autres |
|---|---|---|---|---|---|---|---|---|---|---|---|---|---|
| 1032 | 23 | Name | "General Settings" | pr | string |  |  |  |  |  |  |  | description="Settings Name" |
| 1041 | 84E14E8A-CF28-3DA8-A949-C505248B9CD9 | inconnu (fabricant) | 45 | pr,ev | uint8 |  | 0 | 100 | 1 |  |  |  | description="WiFi Signal Strength" |
| 1042 | 2C063F78-398F-DBB0-9E48-8F2DC1FBDA2E | inconnu (fabricant) | "(masqué)" | pr | string |  |  |  |  |  |  | 4 | description="WiFi Country Code" |
| 1046 | E9DB48C2-954C-4BBC-8A4B-43BD90131F81 | inconnu (fabricant) | 6 | pr,ev | uint16 |  |  |  |  |  |  |  | description="WiFi Channel" |
| 1043 | 27BE48D7-AB48-1684-1540-1F1E3C9BA0FA | inconnu (fabricant) | 60 | pr,pw | uint8 |  | 0 | 100 | 1 |  |  |  | description="Motion Detector Sensitivity" |
| 1044 | F55B9EB2-6120-74B9-544C-D40A55E5CC5A | inconnu (fabricant) | — | pw | bool |  |  |  |  |  |  |  | description="Submit Diagnostics to Logitech" |
| 1045 | A9896218-6661-D7B1-954D-7CDEEAB302D8 | inconnu (fabricant) | 0 | pr,pw | bool |  |  |  |  |  |  |  | description="Persistent Log" |
| 1047 | 3BF3E754-1DE6-BA9E-6243-D25C272A72F5 | inconnu (fabricant) | "02:00:00:00:00:0C" | pr | string |  |  |  |  |  |  | 18 | description="MAC Address" |
| 1048 | AB9B9865-4D5C-42AB-0F4B-C5F0EC84E5EB | inconnu (fabricant) | — | pw | bool |  |  |  |  |  |  |  | description="Check for New Firmware" |
| 1049 | 4A7C9629-4F4F-55B0-B540-CF545AAEB961 | inconnu (fabricant) | "(masqué)" | pr | string |  |  |  |  |  |  | 32 | description="Last Firmware Update Check Time" |
| 1050 | 0840EBC4-8B90-E8B6-294C-C242CFFEF1F1 | inconnu (fabricant) | "(masqué)" | pr | string |  |  |  |  |  |  | 32 | description="Next Firmware Update Check Time" |

### [1.1057] inconnu, fabricant (10FCD34C-62E9-2495-2E45-0D2BC698F563)

| iid | type | nom | valeur | droits | format | unité | min | max | pas | valides | plage | maxLen | autres |
|---|---|---|---|---|---|---|---|---|---|---|---|---|---|
| 1064 | 23 | Name | "Video Settings" | pr | string |  |  |  |  |  |  |  | description="Video Settings Name" |
| 1073 | 56964A81-42F2-F983-7B45-947851ECC249 | inconnu (fabricant) | 20 | pr,pw | int |  | 0 | 30 | 1 |  |  |  | description="IQ Minimum Day Frame Rate" |
| 1074 | F6C475B0-8645-D686-E74D-E089A338138C | inconnu (fabricant) | 20 | pr,pw | int |  | 0 | 30 | 1 |  |  |  | description="IQ Minimum Night Frame Rate" |

