---
title: Performance
section: Le produit
order: 8
widget: benchmark
summary: Ce que la passerelle coûte à une requête, mesurée à côté de Kong, APISIX et Traefik sur la même machine, dans la même exécution, lu en direct depuis le dernier benchmark de la CI.
---

# Performance

Les chiffres ci-dessous viennent du dernier commit mesuré par la CI, lus en
direct. Meerkat est mesuré à côté de trois gateways libres, chacune avec le même
CPU unique, devant le même service, sous la même charge, dans la même exécution :
ils comparent des produits, pas des machines.

Rien sur cette page n'est écrit à la main. Le tableau est récupéré sur la branche
de benchmark à chaque ouverture, donc il suit le code plutôt que la dernière fois
où quelqu'un a pensé à mettre un transparent à jour.
