---
title: Ce qu'est Meerkat
section: Demarrer
order: 1
summary: Une porte unique devant vos applications internes, qui prend en charge tout ce qui n'est pas le metier de vos equipes.
---

# Ce qu'est Meerkat

Meerkat est une **app-gateway** : une porte unique devant les applications de
votre organisation. Les requetes arrivent sur Meerkat, il decide quoi en faire,
puis il les transmet.

Ce qu'il prend en charge, pour que vos services n'aient pas a le faire :

- **Qui appelle** - pages de connexion, SSO, double facteur, jetons d'API, sessions.
- **Qui a le droit de passer** - roles, groupes, organisations, regles par route et par endpoint.
- **Comment la requete voyage** - routage, reecriture, en-tetes, limites de debit, TLS.
- **Ce qui se passe** - trafic, journal d'audit, metriques, sante des amonts.

## Pourquoi une gateway

Une application interne demarre en general sans rien de tout cela. Puis il lui
faut une page de connexion, alors quelqu'un l'ecrit. Puis une deuxieme
application a besoin de la meme, et les deux ne s'accordent pas sur ce qu'est
une session. Meerkat est l'endroit ou ces questions sont tranchees une fois.

> [!NOTE]
> Meerkat proxifie vos applications telles qu'elles sont. Il ne leur demande ni
> d'embarquer une bibliotheque, ni de parler un protocole a lui.

## Un seul binaire

La gateway est un unique binaire Go sans dependance : elle sert le plan de
donnees sur un port et sa console d'administration sur un autre. Rien d'autre
n'a besoin d'etre installe pour la faire tourner.
