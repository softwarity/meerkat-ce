---
title: L'écran de trafic
section: Exploitation
order: 203
summary: Ce que la vue de trafic intégrée montre, ce qu'elle ne montre délibérément pas, et combien de temps elle se souvient.
---

# L'écran de trafic

La console a toujours montré ce qui est **configuré**. Cet écran montre ce qui a réellement été
**servi**, et c'est la moitié dont on a besoin quand on est d'astreinte : dans une configuration, une
route qui échoue et une route que personne n'appelle se ressemblent exactement.

Il vit sur `/traffic` sur le plan de contrôle, sous l'entrée **Metrics** du rail, et il demande root
ou la capacité gateway-admin.

![L'écran de trafic](img/console/traffic.webp)

## Ce qu'il montre

- Deux courbes sur la dernière heure, un point toutes les `5s` : ce qui a été répondu et ce qui a
  échoué.
- Quatre chiffres sur la **dernière minute**. Moyenner une heure garderait un incident de cinq
  minutes dans le titre longtemps après sa fin, donc la période est écrite sur l'écran plutôt que
  supposée.
- Un classement des routes sur trois axes : les plus lentes, celles qui échouent, et celles qui
  coûtent le plus de temps au total.
- Ouvrir une route liste **ses endpoints**, sur la même période que celle que dessine la table. Deux
  horloges sur un écran, c'est un écran dont les lignes ne font pas la somme de celle du dessus.

![Le même écran, déroulé sur le classement par route](img/console/metrics.webp)

Il est alimenté par le canal live et non interrogé en boucle : la passerelle pousse chaque intervalle
au moment où il est mesuré.

## L'unité d'un endpoint est un gabarit

Un endpoint, c'est `GET /orders/{id}`, jamais `GET /orders/1042`. Compter les chemins bruts ferait une
série par commande, gardée pour la vie du processus, et **choisie par celui qui envoie
les requêtes** - une boucle sur `curl` ferait tomber la passerelle par son propre
instrument.

Un gabarit vient de l'un de deux endroits, et l'écran dit lequel :

- **déclaré** - une spec OpenAPI déposée sur la route, les règles par endpoint que quelqu'un a
  écrites, ou une spec que le plan de contrôle a résolue depuis le service. Quelqu'un l'a écrit, donc
  c'est exact.
- **déduit** - la forme d'un chemin que cette passerelle a vu, les segments qui ressemblent à un
  identifiant pliés en `{id}`. Ça se trompe parfois : l'année de `/files/2024/report` n'est pas un
  identifiant. Ces lignes portent la mention **déduit** partout où elles s'affichent.

La déduction est bornée deux fois : par le pliage, et par un budget de deux cents gabarits par route,
tout ce qui dépasse partageant un seul seau.

## Ce qu'il ne montre pas

- **Les requêtes une par une.** Les compteurs sont des agrégats ; rien n'enregistre un appel
  (OBS-03).
- **Les percentiles.** L'histogramme de latence est collecté et exposé, mais aucune courbe de p95
  n'est tracée ici.
- **Les traces.** `traceparent` n'est pas propagé vers les amonts (OBS-04).
- **Qui a appelé.** Aucune étiquette n'est jamais un utilisateur, une adresse ou un chemin brut.
  C'est ce qui borne la cardinalité.
- **Le verdict d'un appel gRPC.** Un appel gRPC répond toujours `200` et met son verdict dans
  `grpc-status`, que les compteurs ne lisent pas : une route gRPC en échec se lit donc comme saine ici
  (ROUTE-20).

## La rétention

Une heure, en mémoire, dans un anneau. Un redémarrage commence une nouvelle heure et rien n'est écrit
sur disque. Les endpoints gardent leur propre historique, plus grossier : un point par minute,
soixante-dix minutes, et écrit seulement quand quelque chose s'est passé - donc un endpoint que
personne n'appelle ne coûte rien du tout.

## En cluster

Les courbes sont sommées sur tous les noeuds, et l'écran dit combien de noeuds l'intervalle couvre.

La différence est prise **par noeud** et les différences sont ensuite additionnées, jamais l'inverse.
Sommer les totaux puis différencier la somme a l'air équivalent et ne l'est pas : à l'instant où un
noeud s'en va - une mise à jour glissante, un crash - la somme tombe de tout ce que ce noeud avait
jamais compté, et la chute se lit comme un pic de tout l'historique des survivants atterrissant en
cinq secondes.

Un endpoint est compté sur le noeud qui a répondu.
