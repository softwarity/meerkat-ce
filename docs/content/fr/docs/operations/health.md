---
title: Sondes de santé
section: Exploitation
order: 212
summary: Laquelle de /healthz et /readyz répond quoi, et pourquoi la sonde de vivacité ne touche jamais la base.
---

# Sondes de santé

Deux sondes, servies sur les **deux** plans, qui répondent à deux questions différentes. Pointer un
orchestrateur sur la mauvaise est l'erreur la plus coûteuse disponible ici, donc elles sont séparées
exprès (OBS-02).

## /healthz est la vivacité

Elle répond `UP` sans condition, avec la version.

C'est la bonne réponse, pas une réponse paresseuse. La vivacité décide de **tuer** le processus. Une
sonde de vivacité qui tomberait parce que la base est injoignable transformerait une coupure de base
en redémarrage simultané de tous les noeuds - chacun tué pour une faute qu'aucun n'a, et qu'aucun ne
répare en mourant. Ce qui appartient à une dépendance appartient à la disponibilité.

```json
{"status":"UP","version":"dev"}
```

## /readyz est la disponibilité

Elle décide d'**envoyer du trafic**, donc elle pose les deux questions qui rendent un noeud inutile :

- **la base répond-elle ?** Un ping, borné à deux secondes. Une sonde de disponibilité qui pend est
  une sonde qui ne dit rien, et c'est alors le délai de l'orchestrateur qui tranche, sur aucune
  information.
- **le routeur a-t-il compilé sa table au moins une fois ?** Un noeud qui accepte des connexions sans
  avoir jamais compilé répond `404` à tout, ce qu'une mise à jour glissante lit volontiers comme une
  instance saine, et qu'elle nourrit en trafic réel.

En échec, c'est un `503` **avec la raison**, parce qu'un exploitant qui lit un échec de sonde a besoin
de savoir laquelle des deux.

```json
{"status":"DOWN","reason":"the store is not answering","version":"dev"}
```

```json
{"status":"DOWN","reason":"the routing table has not been compiled yet","version":"dev"}
```

Un noeud dont la base est tombée garde sa table compilée et répond encore, alors qu'il ne peut plus
résoudre une session, lire un réglage ni recharger une route. C'est exactement l'état qui se déclarait
prêt.

## Les deux échappent à la redirection HTTPS

Là où le port en clair redirige vers HTTPS, ces deux chemins sont exemptés : un `308` se lit comme
« pas prêt » pour la plupart des sondes, et une passerelle serait sortie de rotation pour être
correctement configurée.

## En cluster

Le répartiteur sonde `/readyz` et **jamais** `/healthz`. Il n'y a aucune affinité de session à
demander - les sessions vivent en base, et le cache de cinq secondes est invalidé par le bus de
changement - et aucun noeud primaire à viser, puisque le verrou consultatif garde ce qui ne doit
se faire qu'une fois.
