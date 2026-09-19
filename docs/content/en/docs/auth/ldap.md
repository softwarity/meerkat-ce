---
title: LDAP and Active Directory
section: Authentication
order: 106
summary: Ask a directory directly, with the username and password typed in the ordinary sign-in form.
---

# LDAP and Active Directory

> [!NOTE] **Enterprise edition.**
> Directories are part of the Enterprise edition. The community image refuses to
> save an LDAP authority (`403`), and the *Directory* choice is dimmed in the
> console.

A directory authority takes the username and password from the **ordinary
sign-in form** and asks the directory itself. No button appears on the sign-in
page: there is nowhere to send the browser.

The order is: the local password first, and if that says no, each enabled
directory in turn. Someone whose local and directory passwords happen to match
comes in through the directory only once local accounts are disabled.

> [!WARNING]
> A directory row can reach a community image through an imported
> configuration, or a database first written by an Enterprise image. The
> community binary carries **no directory driver**, so such an authority
> authenticates nobody - the only trace is a `directory unavailable` line in the
> log, and the person sees the ordinary "wrong username or password". If local
> accounts are disabled as well, the sign-in form is still drawn and nobody can
> get in at all.

## The fields

**Infra > Authentication > New authority > Directory.**

| Field | What to put in it |
|---|---|
| Dialect | *Directory (OpenLDAP and friends)* or *Active Directory*. It picks the defaults below |
| Server URL | the scheme is chosen, not typed: `ldaps://dc1.acme.io:636` or `ldap://...` |
| Search base | where the search starts, `dc=acme,dc=io` |
| Service account | the DN that performs the **search**, never a sign-in: `cn=meerkat,ou=services,dc=acme,dc=io`. Leave it empty for an anonymous search |
| Service password | its password, or a vault reference |
| User filter | `%s` is replaced by what the person typed, escaped. Empty uses the dialect default |
| Follow nested groups | on by default: a group that contains a group counts |
| Skip the certificate check | for a self-signed directory only |

What a dialect decides, when you leave the matching field empty:

| | Directory | Active Directory |
|---|---|---|
| User filter | `(&(objectClass=inetOrgPerson)(uid=%s))` | `(&(objectClass=user)(sAMAccountName=%s))` |
| Username attribute | `uid` | `sAMAccountName` |
| Name attribute | `cn` | `displayName` |
| E-mail attribute | `mail` | `mail` |
| Groups read from | a search by `member` or `uniqueMember` | the `memberOf` attribute, then a nested-membership search |

> [!NOTE]
> The attribute names, the group search base, the group filter and the group
> name attribute all exist as configuration (`usernameAttr`, `emailAttr`,
> `nameAttr`, `groupBaseDn`, `groupFilter`, `groupIdAttr`, `memberOfAttr`) but
> have no box in the console: they are set through the admin API or an imported
> configuration file. Everything else is on the screen.

## Search, then bind

That is the whole sequence, and it matters because the service account never
sees anybody's password:

1. an empty username **or an empty password** is refused immediately - an empty password is an anonymous bind, and an anonymous bind succeeds;
2. connect;
3. bind as the service account, if one is configured;
4. search the subtree from the search base with the user filter, at most two entries. No entry means "wrong username or password". **Two or more entries and the sign-in is refused**, naming the ambiguity, rather than picking one;
5. bind as the person, with the DN just found and the password they typed. That is the only use the password is put to;
6. bind back as the service account before reading groups;
7. read the identity: the entry's DN becomes the stable subject, plus username, full name, e-mail and groups.

An address that comes from a directory is treated as **verified**: a directory
is authoritative about its own people. That is what lets a directory sign-in
adopt an existing local account holding the same address.

Group names are the leaf of the DN, not the DN: `cn=developer,ou=groups,dc=acme,dc=io`
is reported as `developer`. They become memberships and roles through a group
rule on an organisation.

## TLS

TLS happens when, and only when, the URL starts with `ldaps://`. The minimum
version is TLS 1.2. There is no StartTLS on port `389`, no client certificate
and no custom certificate authority: for a directory with a self-signed
certificate, the honest option on the screen is *Skip the certificate check*.

## Test the connection

The button connects, binds the service account, and runs your user filter once
against a name nobody has. It therefore proves the URL, the certificate, the
service account and that the search base and filter parse - which is most of
what a first setup gets wrong.

## Two things a directory does that a redirect authority cannot

**It re-validates.** When someone signs in with a passkey, Meerkat asks the
directory whether it still knows that person before letting them through. A
clear "no such object" revokes the sign-in; any other error means "could not
ask", and nobody is signed out because a server was briefly unreachable.
Identity providers reached by redirect have no such question, so they never
revoke a passkey sign-in.

**It can be the only authority.** With local accounts disabled and a directory
enabled, the sign-in form is still the way in - it is the directory's form as
much as the local one.
