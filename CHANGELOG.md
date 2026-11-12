# Changelog

## 1.4.0 (2026-11-12)

### Features

* **core:** implement go-native repokit design uplifts (#29) (cdaa0ab)
* **propagation:** implement fast-failing poller, issue routing, and bot PR auto-rebase (#28) (afe17c2)

## 1.3.0 (2026-11-06)

### Features

* **propagation:** wait for downstream PR auto-merge and fail propagation on CI errors (#26) (5d6cad5)

## 1.2.0 (2026-11-06)

### Features

* **propagation:** adopt bot/repokit-sync branch and embed release notes in sync PRs (#24) (28a2994)

## 1.1.2 (2026-11-06)

### Bug Fixes

* **propagation:** configure git author identity for automated sync PR commits (#22) (aae3f29)

## 1.1.1 (2026-11-06)

### Bug Fixes

* **ruleset:** align branch protection status checks for reusable workflow callers (#20) (dca1233)

## 1.1.0 (2026-11-05)

### Features

* **workflows:** implement bifurcated release CI and unified quality gate (#19) (c6c0053)
* **release:** pre-certify required status checks on release staging PRs (#18) (efcea42)
* **propagation:** implement release-driven direct propagation and v1 workflow pinning (#17) (20cf61e)
* **release:** implement native release staging engine and resolve CI issues (#14) (eedfdd4)
* **templates:** embed PR audit trail, add operability sections, and exempt AgentVault (#12) (34458de)

### Bug Fixes

* **ci:** remove invalid secrets context from release workflow caller (#15) (597a56c)

## 1.0.0 (2026-11-03)


### Features

* **archetypes:** add python, java, typescript, and bash solution templates ([#1](https://github.com/Cooper-Wallace/repokit/issues/1)) ([765c242](https://github.com/Cooper-Wallace/repokit/commit/765c24293fbe5dc6471a0ebb1bc8a60253b21784))
* **audit:** enforce CI PR event guard in audit and configure commands ([#6](https://github.com/Cooper-Wallace/repokit/issues/6)) ([484b658](https://github.com/Cooper-Wallace/repokit/commit/484b658614b3ff012a3751ec9c2b61f80895d22e))
* **core:** implement workflow permissions governance and propagate engine ([#11](https://github.com/Cooper-Wallace/repokit/issues/11)) ([2184d1b](https://github.com/Cooper-Wallace/repokit/commit/2184d1b435728088da63771d1c747a668d51b449))
* **core:** port repokit to Go with embedded templates and reusable CI ([#8](https://github.com/Cooper-Wallace/repokit/issues/8)) ([8e00c25](https://github.com/Cooper-Wallace/repokit/commit/8e00c257e33d49cd22e1971f532d675a7d2a0470))
* **governance:** enforce human-gated PR merge flow and strict status check rulesets ([#3](https://github.com/Cooper-Wallace/repokit/issues/3)) ([8b5dcec](https://github.com/Cooper-Wallace/repokit/commit/8b5dcec6f485f528d6b2a65fac63914b135bda06))
* **governance:** evolve gh-standardise into repokit ([#2](https://github.com/Cooper-Wallace/repokit/issues/2)) ([3bee9b1](https://github.com/Cooper-Wallace/repokit/commit/3bee9b19f1434fe8394cca24166627d5ce1ccb57))
* **release:** implement central release automation workflows and archetype governance ([#9](https://github.com/Cooper-Wallace/repokit/issues/9)) ([d36a758](https://github.com/Cooper-Wallace/repokit/commit/d36a7587fa6a7a62364d4e3a9ec29dfb2652bfaf))


### Bug Fixes

* **ci:** filter non-shell files and enable external source following in reusable bash CI ([#7](https://github.com/Cooper-Wallace/repokit/issues/7)) ([d6fbdb2](https://github.com/Cooper-Wallace/repokit/commit/d6fbdb2b05eb4b3b7a0fe26cf02a5db1e9ca8640))
* **ci:** guard semantic-pr execution against push events on main ([#5](https://github.com/Cooper-Wallace/repokit/issues/5)) ([f2b9a34](https://github.com/Cooper-Wallace/repokit/commit/f2b9a348483d2cb9e3ac1357fe189f300bae870d))
* **ruleset:** align required status check contexts for hub and consumer repos ([#4](https://github.com/Cooper-Wallace/repokit/issues/4)) ([61fc7ac](https://github.com/Cooper-Wallace/repokit/commit/61fc7ac546e52f3f7bbfdce5fc01619861936e8b))
