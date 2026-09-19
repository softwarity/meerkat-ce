---
title: Tarifs
section: Le produit
order: 6
summary: Une édition est gratuite et le reste. L'autre se facture par instance de production, et commence par une conversation.
---

# Tarifs

Deux façons de faire tourner Meerkat. L'une ne coûte rien, et ce n'est pas un
essai.

::: cards
### Communautaire - gratuite

Toute la passerelle pour une organisation sur une instance. Le routage, les
pages de connexion, les rôles et les règles d'accès, TLS, le coffre, le second
facteur et les passkeys, le journal d'audit, les écrans de trafic, la console,
le point d'entrée agent.

Gratuite pour tout usage, **y compris en production et en entreprise**, sous
Functional Source License. Aucun compte à créer, aucune clé, aucune expiration.

```bash
docker run -p 8080:8080 -p 9090:9090 \
  -e MEERKAT_ADMIN_PASSWORD=choisissez-en-un softwarity/meerkat
```

[Démarrer](/docs/start/quick-start)

### Enterprise - parlons-en

Tout ce qui précède, plus ce dont une installation a besoin quand elle
grandit : plusieurs organisations, LDAP et Active Directory, les rôles accordés
par votre annuaire, plusieurs passerelles derrière une seule entrée,
l'exposition Prometheus, le tunnel de développement, les pages intégrées sans
notre marque - et le support.

**Le support fait partie de l'accord** : vous joignez ceux qui ont écrit la
passerelle, pas un palier. Ce qu'il couvre et en combien de temps est en cours
d'écriture et sera sur cette page.

Facturée **par instance de production**. Pas par utilisateur, pas par requête,
pas par route.

[Ce qu'ajoute Enterprise](/product/editions)
:::

## Comment c'est facturé

- **Par instance de production.** L'unité est l'installation que vous mettez
  devant vos utilisateurs, pas le nombre de personnes derrière. Personne ne
  compte de sièges, et passer de trente à trois mille utilisateurs ne change
  pas la facture.
- **Rien à mesurer.** La passerelle ne nous appelle jamais : il n'y a ni
  rapport d'usage ni vérification de licence nulle part dedans. Ce que vous
  payez, c'est l'image et l'accord qui l'ouvre.
- **Rien à activer.** Aucune clé à installer, aucun droit à renouveler, rien
  qui puisse expirer au milieu d'une nuit.

> [!NOTE]
> Les montants, les conditions et ce que couvre un contrat de support sont en
> cours d'écriture, et ils seront sur cette page. D'ici là, la réponse à
> « combien » est une conversation - et elle est courte, parce que l'unité est
> simple.

## Les questions qui viennent en premier

**Peut-on utiliser l'édition gratuite en production, en entreprise, sur un
produit commercial ?** Oui. La Functional Source License autorise explicitement
l'usage interne et en production. La seule chose qu'elle interdit est d'en
construire une passerelle concurrente.

**Peut-on lire et modifier le code ?** Oui, le tronc est public et modifiable.
Et **deux ans après chaque version, celle-ci devient de l'Apache 2.0** sans la
moindre condition : ce que vous déployez aujourd'hui ne peut pas vous être
repris plus tard.

**Voyez-vous notre trafic, nos utilisateurs ou notre configuration ?** Non. Il
n'y a aucune télémétrie dans le produit, et rien dedans ne nous envoie quoi que
ce soit.

**Que devient une installation si un accord se termine ?** C'est une des
conditions en cours d'écriture ; demandez-nous et nous répondrons simplement
plutôt que contractuellement. Ce que fait le produit lui-même n'est pas une
falaise : une capacité qu'il ne porte plus est refusée par une phrase, et tout
ce qui est déjà en place continue d'être servi.

**Vendez-vous du support sur l'édition gratuite ?** Demandez-nous. L'image
communautaire est soutenue par le dépôt : une issue sur
[softwarity/meerkat-ce](https://github.com/softwarity/meerkat-ce/issues) est lue.

::: cta
### Parlons-en

Le canal de contact commercial est en cours de mise en place et son adresse
sera ici. D'ici là, ouvrez une issue sur
[softwarity/meerkat-ce](https://github.com/softwarity/meerkat-ce/issues) en disant ce
que vous construisez et à quelle taille, et nous prendrons le relais.
:::
