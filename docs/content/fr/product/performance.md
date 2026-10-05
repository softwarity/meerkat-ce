---
title: Performance
section: Le produit
order: 8
widget: benchmark
summary: Ce que la gateway coûte à une requête, mesuré à côté de Kong, APISIX et Traefik sur la même machine, dans la même exécution, et lu en direct dans le dernier benchmark de la CI.
---

# Performance

Les chiffres ci-dessous proviennent du dernier commit que la CI a mesuré, et
sont lus en direct. Meerkat est mesuré à côté de trois gateways open source,
chacune disposant du même CPU unique, devant le même service, sous la même
charge et dans la même exécution : ces chiffres comparent des produits, pas des
machines.

Rien sur cette page n'est écrit à la main. Le tableau est récupéré sur la
branche de benchmark chaque fois que quelqu'un ouvre la page : il suit donc le
code, et non la dernière fois où quelqu'un a pensé à mettre une présentation à
jour.
