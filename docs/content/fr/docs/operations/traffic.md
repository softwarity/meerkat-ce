---
title: L'écran de trafic
section: Exploitation
order: 203
summary: Ce que montre la vue de trafic intégrée, ce qu'elle laisse volontairement de côté, et combien de temps elle garde ses données.
---

# L'écran de trafic

La console a toujours montré ce qui est **configuré**. Cet écran montre ce qui a
réellement été **servi**, et c'est la moitié qui compte quand vous êtes d'astreinte :
dans une configuration, rien ne distingue une route en panne d'une route que personne
n'appelle.

Il se trouve à l'adresse `/traffic` du plan de contrôle, sous l'entrée **Metrics** du
rail, et il exige le compte root ou la capacité gateway-admin.

![L'écran de trafic](img/console/traffic.webp)

## Ce qu'il montre

- Deux courbes sur la dernière heure, à raison d'un point toutes les `5s` : les
 réponses servies et les échecs.
- Cinq chiffres sur la **dernière minute**, dont le p95. Une moyenne sur une heure
 laisserait un incident de cinq minutes en tête d'écran longtemps après sa fin : la
 période est donc écrite à l'écran plutôt que sous-entendue.
- Un classement des routes selon trois axes : les plus lentes, celles qui échouent, et
 celles qui consomment le plus de temps au total.
- L'ouverture d'une route affiche **ses endpoints**, sur la même période que le
 tableau. Avec deux horloges sur un même écran, la somme des lignes ne retombe jamais
 sur la ligne du dessus.

![Le même écran, défilé jusqu'au classement par route](img/console/metrics.webp)

L'écran est alimenté par le canal temps réel et non par interrogation périodique : la
gateway pousse chaque intervalle dès qu'il est mesuré.

## L'unité d'un endpoint est un gabarit

Un endpoint, c'est `GET /orders/{id}`, jamais `GET /orders/1042`. Compter les chemins
bruts reviendrait à créer une série par commande, conservée pendant toute la vie du
processus, et **choisie par celui qui envoie les requêtes** - une simple boucle sur
`curl` ferait tomber la gateway au moyen de son propre instrument de mesure.

Un gabarit a deux origines possibles, et l'écran précise laquelle :

- **déclaré** - une spécification OpenAPI déposée sur la route, les règles par
 endpoint que quelqu'un a écrites, ou une spécification que le plan de contrôle a
 obtenue auprès du service lui-même. Quelqu'un l'a écrit noir sur blanc : le gabarit
 est donc exact.
- **déduit** - la forme d'un chemin que cette gateway a vu passer, où les segments
 qui ressemblent à des identifiants sont ramenés à `{id}`. La déduction se trompe
 parfois : dans `/files/2024/report`, l'année n'est pas un identifiant. Ces lignes
 portent la mention **deduced** partout où elles apparaissent.

La déduction est bornée deux fois : par ce regroupement, et par un budget de deux cents
gabarits par route, au-delà duquel tout le reste partage une seule et même case.

## Ce qu'il ne montre pas

- **Les requêtes individuelles.** Les compteurs sont des agrégats. Une ligne par
 appel, c'est le rôle du [journal d'accès](/docs/operations/logs), désactivé par
 défaut.
- **Les traces.** Les compteurs sont des agrégats : pour savoir où sont passées les
 secondes d'UNE requête, il vous faut les [traces](/docs/operations/tracing).
- **Qui a appelé.** Aucune étiquette n'est jamais un utilisateur, une adresse ou un
 chemin brut. C'est ce qui garde la cardinalité bornée.

## Rétention

Une heure, en mémoire, dans un tampon circulaire. Un redémarrage ouvre une nouvelle
heure et rien n'est écrit nulle part. Les endpoints conservent leur propre historique,
plus grossier : un point par minute, sur soixante-dix minutes, écrit uniquement quand il
s'est réellement passé quelque chose - un endpoint que personne n'appelle ne coûte donc
strictement rien.

## En cluster

Les courbes sont la somme de tous les nœuds, et l'écran indique combien de nœuds
l'intervalle couvre.

La différence est calculée **par nœud**, puis les différences sont additionnées, jamais
l'inverse. Additionner d'abord les totaux pour calculer ensuite la différence de la
somme semble équivalent, mais ne l'est pas : dès qu'un nœud disparaît - mise à jour
progressive, plantage - la somme chute de tout ce que ce nœud avait compté depuis son
démarrage, et cette chute se lit comme un pic où tout l'historique des nœuds restants
arriverait en cinq secondes.

Un endpoint est compté sur le nœud qui lui a répondu.
