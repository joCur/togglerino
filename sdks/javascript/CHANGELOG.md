# Changelog

## [0.3.0](https://github.com/joCur/togglerino/compare/sdk-v0.2.2...sdk-v0.3.0) (2026-03-17)


### Features

* add server-side SDK evaluation with per-request context ([#146](https://github.com/joCur/togglerino/issues/146)) ([0773d8f](https://github.com/joCur/togglerino/commit/0773d8f02b4ce86f68275157af0953316093077a))

## [0.2.2](https://github.com/joCur/togglerino/compare/sdk-v0.2.1...sdk-v0.2.2) (2026-03-15)


### Bug Fixes

* SSE events trigger SDK re-fetch instead of caching wrong value ([#142](https://github.com/joCur/togglerino/issues/142)) ([15a45d9](https://github.com/joCur/togglerino/commit/15a45d913fb46378e62a99a92102a2928c85d36e))

## [0.2.1](https://github.com/joCur/togglerino/compare/sdk-v0.2.0...sdk-v0.2.1) (2026-02-27)


### Bug Fixes

* correct repository URL in SDK package.json for OIDC provenance ([#21](https://github.com/joCur/togglerino/issues/21)) ([9c77539](https://github.com/joCur/togglerino/commit/9c77539be5821586102b49437a1a194cf6ba25d3))

## [0.2.0](https://github.com/joCur/togglerino/compare/sdk-v0.1.1...sdk-v0.2.0) (2026-02-27)


### Features

* @togglerino/sdk — JS/TS client with SSE and polling ([2157559](https://github.com/joCur/togglerino/commit/2157559a2b58691b7df79fe45c2f28d39a834ff1))
* add SSE reconnection with exponential backoff to JS SDK ([1de5ba9](https://github.com/joCur/togglerino/commit/1de5ba9f05ab9264b8a3100655943129c044f7b7))
* add useTogglerinoContext hook for reactive context updates ([#8](https://github.com/joCur/togglerino/issues/8)) ([9821897](https://github.com/joCur/togglerino/commit/9821897de0cec979d2c8fd6367e2c95abe119852))
* archive and delete feature flags ([c04b355](https://github.com/joCur/togglerino/commit/c04b3555d3f54396fe478fb487ccac8b6d1e1675))
* **sdk:** handle flag_deleted SSE events ([2945cbf](https://github.com/joCur/togglerino/commit/2945cbf3b064e2820f6967c82e1f296a41c5fe71))


### Bug Fixes

* address code review feedback ([2439f0d](https://github.com/joCur/togglerino/commit/2439f0dbb98963e69fa2ca7fb4f323c2c2edab42))


### Code Refactoring

* simplify SDK init to only require sdkKey ([031b3f5](https://github.com/joCur/togglerino/commit/031b3f5f9a5ee5657dd450f807c26ec391ba36ab))
