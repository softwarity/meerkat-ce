---
title: Organisations
section: Access control
order: 138
summary: Membership, the choice at sign-in, the owner, and which settings an organisation can override.
---

# Organisations

An organisation - a tenant - is who somebody is working as. It is the scope roles
live in: groups belong to an organisation, and a session with no active
organisation holds no roles.

Every installation owns one from its first start, and one is often all it will
ever need.

> [!NOTE] **Enterprise edition.**
> More than one organisation is part of the Enterprise edition. The community
> image serves a **single** organisation, which no screen ever names: its groups,
> members and rules are administered from **Application**, and the notion does not
> appear anywhere else. Switching to the multi-organisation shape is a root
> decision on an Enterprise image.

Switching back to single deletes nothing: the other organisations simply stop
being served, and the one that is served is the oldest.

## Members

A membership is one person in one organisation, typed **ADMIN** or **USER**, and
enabled or not. That is the whole enumeration - there is no OWNER membership.

**Ownership is a property of the organisation**, not a membership: an
organisation always has an owner, the owner can be transferred, and the owner does
not have to be a member. Someone who creates an organisation owns it.

An **ADMIN** membership, or being the owner, is what lets a person administer that
organisation from the console: the organisation itself, its members, its groups,
its group rules, resetting a member's password, reading a member's sign-in
history. It grants nothing outside that organisation, and in particular not the
global role catalogue.

A person can also **leave** the organisation they are in, from their profile:
*Leave this organisation*, beside its name, then a confirmation saying that only
an administrator can add them back. The membership goes, the session stops
carrying the organisation at once, and the line `member.leave` lands in the
organisation's [audit trail](/docs/operations/audit). Multi-organisation
installations only: a single one has nothing to leave.

![The members of an organisation, and what each one is in it](img/console/members.webp)

## The choice at sign-in

Once the first factor and the second factor are done:

| Memberships | What happens |
|---|---|
| none | the session is issued with no organisation. Someone who administers nothing lands in `/account-pending`, the waiting room |
| one | it is stamped on the session, silently |
| several | they choose, on `/select-tenant` |

Only enabled memberships of enabled organisations count.

A session that has no organisation and whose owner later gains exactly one
membership adopts it on the next request - being added to an organisation takes
effect without signing out.

Changing organisation afterwards is done from the user button, and re-runs the
group step: a new organisation may have a different group mode and different
groups.

## Working hours

An organisation can restrict when its people may come in: weekdays, time ranges,
a start and end date, in a named timezone.

> [!NOTE] **Enterprise edition.**
> Business hours are part of the Enterprise edition.

The hours that apply come from the most precise level that sets some, and they
apply **whole**: the membership, then the group, then the account, then the
organisation, then the installation. A group's hours count only in an
organisation in **exclusive** group mode, where one group is chosen per session
(a day shift, a night shift); in cumulative mode a person holds several groups
at once, so a group has no hours of its own there. Hours set at a level are a different rule for whoever it
covers, not a narrower one: a member given evenings gets in at 20:00 even when
the organisation closes at 18:00. The membership's start and end dates hold
whichever level the hours come from. The default opens every day, around the
clock.

The account's hours are set by the application administrator, on the account;
the membership's by the organisation's administrator, in the member's drawer.
Without an organisation, the account's apply, else the installation's. Root is
never held to hours: the person who repairs the gateway is never locked out of
it.

With several organisations, the chooser shows the ones whose hours are closed
for this person greyed out, marked *Outside working hours*; a sign-in when all
of them are closed is refused. The group chooser does the same with closed
groups, and the session is then bound by the hours of the group chosen.

The hours are checked at sign-in and when entering or switching organisation, and
the session **ends when they close**: one opened at 17:55 in a window that closes
at 18:00 ends at 18:01, the minute 18:00 being part of the window, however long
its lifetime. The page goes back to sign-in, which then says the hours are closed.
Hours changed during the day apply from the next sign-in.

Refusing on hours says so plainly - it never looks like a wrong password.

## What an organisation can override

| Setting | Levels, narrowest first | Editable where |
|---|---|---|
| Session lifetime | membership, organisation, installation | installation in the console; the two others through the admin API |
| Working hours | membership, group (exclusive mode), account, organisation, installation | all of them in the console: the member's drawer, the group's drawer, the account's drawer (Security), the organisation, Application, General |
| Group mode | organisation only | the organisation's screen |
| Two-factor | **account, then installation** | both in the console |

Two-factor deliberately has **no organisation level**: the second factor is asked
before the organisation is known, so a rule per organisation could not be read in
time.

Everything else - the password policy, the throttle, passkeys, the theme, the
languages, the relay - is installation-wide.

## Isolation

Every request carries the organisation it is being made in, and that is what
route rules, endpoint rules and the identity forwarded upstream are all answered
against. Your service receives the organisation as a fact about the caller rather
than as something the caller asked for.

One thing to know: **disabling an organisation, or a membership, does not end the
sessions already working there.** It stops new sign-ins from adopting it, but a
session that already carries that organisation keeps it, and keeps its roles in
it, until the session expires.

What does take effect at once is removing somebody from a **group**: their roles
are recomputed on the next request. So the quickest way to take access away
immediately is to empty the groups, not to disable the membership.
