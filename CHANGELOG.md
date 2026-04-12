# Changelog

## [0.3.5](https://github.com/manmartgarc/negent/compare/v0.3.4...v0.3.5) (2026-04-12)


### Bug Fixes

* **plugin:** use explicit Copilot hook launchers ([f12c284](https://github.com/manmartgarc/negent/commit/f12c284d9fb2bae42c7acadcabc853b04e5792b6))

## [0.3.4](https://github.com/manmartgarc/negent/compare/v0.3.3...v0.3.4) (2026-04-12)


### Features

* **plugin:** add Copilot CLI plugin support ([a9eeecf](https://github.com/manmartgarc/negent/commit/a9eeecf0295e47d6a53189a69493c6ec280bf11b))

## [0.3.3](https://github.com/manmartgarc/negent/compare/v0.3.2...v0.3.3) (2026-04-12)


### Features

* **copilot:** sync session-state data ([9c3d066](https://github.com/manmartgarc/negent/commit/9c3d0661ccfd85e150a647641d14c1631405ad1b))

## [0.3.2](https://github.com/manmartgarc/negent/compare/v0.3.1...v0.3.2) (2026-04-05)


### Features

* **agent:** add GitHub Copilot CLI support ([b41891d](https://github.com/manmartgarc/negent/commit/b41891dafedb86e3731587617767e6d97297b767))


### Bug Fixes

* **agent:** skip symlinks during file collection ([c2e2f5e](https://github.com/manmartgarc/negent/commit/c2e2f5e0145452e1a680eaeb28c02bbdc9e64b84))

## [0.3.1](https://github.com/manmartgarc/negent/compare/v0.3.0...v0.3.1) (2026-04-05)


### Bug Fixes

* **git:** auto-resolve rebase conflicts by preferring remote content ([9dbc8ae](https://github.com/manmartgarc/negent/commit/9dbc8ae3cbb3281ed6bd42b0970e12036a7c4858))

## [0.3.0](https://github.com/manmartgarc/negent/compare/v0.2.7...v0.3.0) (2026-04-05)


### ⚠ BREAKING CHANGES

* Agent interface adds SupportedSyncTypes, DefaultSyncTypes, NormalizeSyncTypes, and SyncTypeForPath; removes DefaultCategories and CategorizePath. SyncFile.Category is now SyncFile.Type. Orchestrator Push/Pull accept map[string][]SyncType instead of map[string][]Category.
* negent now supports only Linux and macOS.

### Features

* add conflict detection on pull, cross-platform paths, and fetch/fetchedFiles ([e0e85dc](https://github.com/manmartgarc/negent/commit/e0e85dc030ae780e4aef5d49d1acb38a8e33e7fc))
* add git backend tests, integration tests, and implement Status() ([9f269bd](https://github.com/manmartgarc/negent/commit/9f269bd27097644f373a65cef9699dd7b8a79662))
* add home dir matching, category-scoped deletions, and remove settings.json sync ([3d9ea31](https://github.com/manmartgarc/negent/commit/3d9ea31b62783dffd4334528dff719374788cffe))
* add session merge, push deletions, category-scoped pull, and output-styles sync ([58b5449](https://github.com/manmartgarc/negent/commit/58b544934ce5f9ab9ec9223b258ab4e545a2dc07))
* add sync plan preview, dry-run, plugins sync type, and CLI improvements ([eaf55f2](https://github.com/manmartgarc/negent/commit/eaf55f24c1ffd6c447301e53a8b4ba9a44a0ef74))
* **ci:** add automated versioning with release-please ([62247e8](https://github.com/manmartgarc/negent/commit/62247e8ccaa1f6923013a5e589b6427bae8098e6))
* **cli:** add --quiet flag to push and pull ([fc8d023](https://github.com/manmartgarc/negent/commit/fc8d02392307151db20dc039feeb2e146ebcd0b0))
* **hooks:** add auto-sync integration with Claude Code session lifecycle ([0e6e054](https://github.com/manmartgarc/negent/commit/0e6e05456599deaabbfc4b388c4e5870a51975be))
* implement pull logic, project matching, status, and link commands ([058ab0d](https://github.com/manmartgarc/negent/commit/058ab0d58ebe7f8348ebb8d1924fd3d92e64e223))
* inital work ([15b54de](https://github.com/manmartgarc/negent/commit/15b54de6b7a6eb06601c73e095c7c95f4d701833))
* **plugin:** add Claude Code plugin and npm distribution ([9e47342](https://github.com/manmartgarc/negent/commit/9e473420277f38c17bd5e2305dbddab409388ded))
* **sync:** add conflicts command and structured pull result ([66216ac](https://github.com/manmartgarc/negent/commit/66216ac5a73006d710714889c23fff94ee540b05))


### Bug Fixes

* **agent:** restrict KnownAgents to implemented agents only ([d94b9a7](https://github.com/manmartgarc/negent/commit/d94b9a7f1314d0a214615096d59a3134c1f775e7))
* **ci:** chain release build into release-please and revert to pre-1.0 ([3f8b534](https://github.com/manmartgarc/negent/commit/3f8b534624812ef99a1df15d2a169041789355ce))
* **claude:** exclude plugin marketplace catalog and install-counts cache from sync ([1119b66](https://github.com/manmartgarc/negent/commit/1119b6635388767a1db6401bb235c733b9cdd36a))
* **git:** remove pre-push pull that fails on dirty worktree ([8d305db](https://github.com/manmartgarc/negent/commit/8d305db592a1fbabc05ae5dce686d74aef343472))
* **git:** reset dirty staging directory before pull ([873c5db](https://github.com/manmartgarc/negent/commit/873c5db0d5c98be32c46d35654d44a7396130db8))
* **plugin:** convert negent exit codes to exit 2 for Claude Code visibility ([bf2c3de](https://github.com/manmartgarc/negent/commit/bf2c3debe492eca4b83f05e9c446f59227ddd447))
* **plugin:** exit 2 on failure so Claude Code surfaces the error ([9e4e4bf](https://github.com/manmartgarc/negent/commit/9e4e4bfe5e1a522ef583fb09af98cb66fe8732a3))
* **plugin:** remove extra top-level field from hooks.json ([4185f18](https://github.com/manmartgarc/negent/commit/4185f18e33a05c9fb24312d4c22c973a814c2d9a))
* **plugin:** surface actionable error when auto-install fails ([6231c55](https://github.com/manmartgarc/negent/commit/6231c55b469b2728e5f783086162aa5e825c38f1))
* **plugin:** use exit 0 + stdout for hook errors and remove npm lookup ([a8495da](https://github.com/manmartgarc/negent/commit/a8495dae4be3ebe60515fa8dd244adb99f5d878a))
* **plugin:** use SessionEnd instead of Stop for push-on-exit hook ([19ce473](https://github.com/manmartgarc/negent/commit/19ce4731895f531e44958c46c8c2c1ef51744b03))
* **plugin:** write hook errors to stderr with exit 2 for user visibility ([4fb4c49](https://github.com/manmartgarc/negent/commit/4fb4c498984adf8269c73082ccba679d5bb14cf0))
* **sync:** classify git conflicts and improve sync guidance ([51de120](https://github.com/manmartgarc/negent/commit/51de120679624fd889fe85c01ef500b02308e48d))
* **sync:** persist keep-local conflict resolutions ([d2953d4](https://github.com/manmartgarc/negent/commit/d2953d47a573a6ba91b5e4080ebc8cb98e7fa172))
* **sync:** stabilize append-only history and rebase recovery ([445e344](https://github.com/manmartgarc/negent/commit/445e3442811924ef519428071362e873743fe961))


### Code Refactoring

* drop Windows support ([578db01](https://github.com/manmartgarc/negent/commit/578db01b389f854ed45c31f43aebeb1ccb973636))
* replace coarse Category enum with agent-defined SyncType taxonomy ([2cb42d1](https://github.com/manmartgarc/negent/commit/2cb42d17a4107dd403584fe958cc77d1cc05c076))

## Changelog
