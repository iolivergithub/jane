# Jane documentation

Jane is a remote attestation engine: it holds known-good values for machines (elements), asks their trust agents for evidence, and checks it with rules. tarzan is its trust agent, jp registers and attests an element, addjaneuser manages Jane's users.

## Start here

1. [Quick start](quickstart.md): one Debian machine from nothing to its first attestation, with the defaults (no keys, no logins).
2. [Getting started](gettingstarted.md): a tour of the Web UI: elements, sessions, claims, results, expected values, rules.
3. [Hardening](hardening.md): TLS, logins, database and broker accounts, secrets, tarzan, the systemd sandbox. Read before exposing Jane to a network.

## Reference

**Installing and running**

- [Installing with dpkg/apt](deb.md): what the packages install, upgrade and removal, running under systemd.
- [Running Jane, tarzan and jp](running.md): every option, flag and environment variable; running by hand and on other systems.
- [Authentication](authentication.md): logins for the Web UI, REST API and x3270, and Jane's users.
- [Compiling](compiling.md): building from source, tests, building the packages.

**Data structures**

- [Elements](elements.md): the machines or other things being attested.
- [Intents](intents.md): what to ask a trust agent for. [Standard intents](standardintents.md): the common set shipped with Jane.
- [Expected values](expectedvalues.md): the known-good values rules compare with.
- [Claims](claims.md): what a trust agent returned, with its context.
- [Results](results.md): a rule's verdict on a claim.
- [Sessions](sessions.md): claims and results of one piece of work.
- [Data model](datamodels.md): how they link.

**Attestation and verification**

- [Functions](functions.md): each intent function's parameters and results.
- [Protocols](protocols.md): how Jane talks to trust agents.
- [Trust agents](trustagents.md): tarzan, ratsd, and writing your own.
- [Rules](rules.md): every rule, what it checks and what it needs.
- [Signing](signing.md): the signed footers on claims, results and sessions.

## Appendices

- [History](history.md)
- [Publications](publications.md)
- [Changelog](../CHANGELOG.md)
