package fakeacc

// Sample imite la base d'accessoires d'une caméra HomeKit : informations,
// gestion du flux RTP, micro, mode de fonctionnement, et un service de
// fabricant avec un champ hors spécification. Les valeurs sont inventées.
const Sample = `{"accessories":[{"aid":1,"services":[
{"iid":1,"type":"3E","characteristics":[
 {"iid":2,"type":"14","perms":["pw"],"format":"bool","description":"Identify"},
 {"iid":3,"type":"20","perms":["pr"],"format":"string","value":"Logitech","maxLen":64},
 {"iid":4,"type":"21","perms":["pr"],"format":"string","value":"Circle View Camera"},
 {"iid":5,"type":"23","perms":["pr"],"format":"string","value":"Circle View AAAA"},
 {"iid":6,"type":"30","perms":["pr"],"format":"string","value":"2111AB0CDEF8"},
 {"iid":7,"type":"52","perms":["pr"],"format":"string","value":"5.0.0"}]},
{"iid":16,"type":"A2","characteristics":[
 {"iid":17,"type":"37","perms":["pr"],"format":"string","value":"1.1.0"}]},
{"iid":32,"type":"110","primary":true,"characteristics":[
 {"iid":33,"type":"120","perms":["pr","ev"],"format":"tlv8","value":"AQEA"},
 {"iid":34,"type":"117","perms":["pr","pw"],"format":"tlv8","value":""},
 {"iid":35,"type":"118","perms":["pr","pw"],"format":"tlv8","value":"AQEB"},
 {"iid":36,"type":"B0","perms":["pr","pw","ev"],"format":"uint8","value":1,"minValue":0,"maxValue":1,"minStep":1,"valid-values":[0,1]}]},
{"iid":48,"type":"112","characteristics":[
 {"iid":49,"type":"11A","perms":["pr","pw","ev"],"format":"bool","value":false},
 {"iid":50,"type":"119","perms":["pr","pw","ev"],"format":"uint8","value":100,"unit":"percentage","minValue":0,"maxValue":100,"minStep":1}]},
{"iid":64,"type":"21A","linked":[32],"characteristics":[
  {"iid":66,"type":"21B","perms":["pr","pw","ev"],"format":"bool","value":true},
 {"iid":67,"type":"11B","perms":["pr","pw","ev","tw"],"format":"bool","value":true},
 {"iid":68,"type":"21D","perms":["pr","pw","ev"],"format":"uint8","value":0,"minValue":0,"maxValue":1,"minStep":1}]},
{"iid":80,"type":"8C6E0A3D-1A2B-4C5D-8E9F-0A1B2C3D4E5F","hidden":true,"characteristics":[
 {"iid":81,"type":"8C6E0A3E-1A2B-4C5D-8E9F-0A1B2C3D4E5F","perms":["pr"],"format":"data","value":"AAEC","x-vendor":"z"}]}
]}]}`
