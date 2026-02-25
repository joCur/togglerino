# Changelog

## [0.3.0](https://github.com/joCur/togglerino/compare/v0.2.1...v0.3.0) (2026-02-25)


### Features

* **web:** add contextual help text to flag configuration screen ([85ebf7e](https://github.com/joCur/togglerino/commit/85ebf7e436839f90b967be672928364a59bfc530))
* **web:** add inline description to rollout slider ([41360d7](https://github.com/joCur/togglerino/commit/41360d72c9b6b23c9acf5aea50827c8933af641d))
* **web:** add toggle and default variant inline descriptions ([edaefb8](https://github.com/joCur/togglerino/commit/edaefb8911167dc86b4bbfd16f7acdc07e283f72))
* **web:** add type-specific descriptions to variant editor ([b464ff5](https://github.com/joCur/togglerino/commit/b464ff55e5ce58e1e9b2b465d3391159406da669))
* **web:** expand operators to 14 with optgroups, add rule descriptions and contextual placeholders ([1e86895](https://github.com/joCur/togglerino/commit/1e868958b1a560e61bd492e2a9d5d31976ff1bc4))


### Bug Fixes

* **web:** use unique IDs for rollout checkboxes, clear value on exists/not_exists ([21fe753](https://github.com/joCur/togglerino/commit/21fe7539183412f555205bc680dec6f8c69244bc))

## [0.2.1](https://github.com/joCur/togglerino/compare/v0.2.0...v0.2.1) (2026-02-25)


### Bug Fixes

* replace setState-in-useEffect with keyed components ([7e5a1a0](https://github.com/joCur/togglerino/commit/7e5a1a05584ae09ca3bf73dd427f8198b5ad887f))

## [0.2.0](https://github.com/joCur/togglerino/compare/v0.1.0...v0.2.0) (2026-02-25)


### Features

* @togglerino/react — provider and useFlag hook ([629f066](https://github.com/joCur/togglerino/commit/629f066e30abd53f27bf855278d9ba91ab5a4c9d))
* @togglerino/sdk — JS/TS client with SSE and polling ([2157559](https://github.com/joCur/togglerino/commit/2157559a2b58691b7df79fe45c2f28d39a834ff1))
* add accept-invite and reset-password pages ([93d7f0b](https://github.com/joCur/togglerino/commit/93d7f0b467ea614673d1d292068dd89ee5bfde2f))
* add accept-invite endpoint for new user onboarding ([f876ea2](https://github.com/joCur/togglerino/commit/f876ea2d0f54b5b889ba78432e2e489dc8b7cf87))
* add admin-initiated password reset flow ([c2eb001](https://github.com/joCur/togglerino/commit/c2eb0017c0453c334e170bccc90030cea53e163d))
* add graceful shutdown with signal handling ([03c3da6](https://github.com/joCur/togglerino/commit/03c3da6f7979a51b94e890bbfdb9ec21d4cb89e8))
* add invite store with create, find, accept, list ([1ddde5c](https://github.com/joCur/togglerino/commit/1ddde5c89dc98edf48825ebe33ed1a4e77dada53))
* add invites table migration ([d43d59f](https://github.com/joCur/togglerino/commit/d43d59ff18fb2a8d0f27c1e38cb1bf5ac7dbe221))
* add OrgLayout for organization-level pages ([495427f](https://github.com/joCur/togglerino/commit/495427f5e10d8a34d0b6536c9da2fd6ad845b35a))
* add ProjectLayout with project-scoped sidebar and switcher ([b49935e](https://github.com/joCur/togglerino/commit/b49935eb7fc2f5bfca8434b2740c0e7b10142901))
* add ProjectSettingsPage with edit, members placeholder, and delete ([7260573](https://github.com/joCur/togglerino/commit/7260573921a41daeb9fd1e1c0d9a417092b68798))
* add ProjectSwitcher dropdown component ([ac24114](https://github.com/joCur/togglerino/commit/ac241147cf17819218d389f358e931ffcf221a25))
* add rate limiting middleware for auth endpoints ([5da0c02](https://github.com/joCur/togglerino/commit/5da0c024d458fc8fe05e0a7c24d299e7b7e03940))
* add SSE reconnection with exponential backoff to JS SDK ([1de5ba9](https://github.com/joCur/togglerino/commit/1de5ba9f05ab9264b8a3100655943129c044f7b7))
* add structured logging with slog and request middleware ([0e8f362](https://github.com/joCur/togglerino/commit/0e8f362c40c247989dd74b4d90cb20c9eef46b30))
* add user management endpoints (list, invite, delete) ([c4200d4](https://github.com/joCur/togglerino/commit/c4200d4a69fe1f7498950c014fcf50bbbb889218))
* audit log store, handler, and integration into mutation endpoints ([b724b7c](https://github.com/joCur/togglerino/commit/b724b7c18589fcd6111141ef47ae334107d6eef0))
* auth handlers for setup, login, logout, me, status ([2897a69](https://github.com/joCur/togglerino/commit/2897a690ca9ae63309ae65fec9edff3cfc26e2c2))
* auth package with bcrypt password hashing and session middleware ([d63fa5d](https://github.com/joCur/togglerino/commit/d63fa5d17fc4e6b432b30f390e00c277473b4b6f))
* client evaluation API with SDK key auth ([7649dbb](https://github.com/joCur/togglerino/commit/7649dbbb44bf4ebfbef20518306c8750b195ee60))
* dashboard core pages — projects, flags, rule builder, rollout ([414940e](https://github.com/joCur/togglerino/commit/414940e0b14ecbee1def906fb830824514b13443))
* dashboard login and setup wizard ([4d6755f](https://github.com/joCur/togglerino/commit/4d6755f91cbf155bf6580bf28bd82e2893c6eba2))
* dashboard settings — environments, team, SDK keys, audit log ([6dc9646](https://github.com/joCur/togglerino/commit/6dc96460765bcaa36bf75655218b324595b0e88c))
* database connection, migration framework, initial schema ([19e54f0](https://github.com/joCur/togglerino/commit/19e54f019a0fdf270a2a472f673a840cdaae618f))
* Dockerfile and docker-compose for self-hosted deployment ([e9c6b6c](https://github.com/joCur/togglerino/commit/e9c6b6ceca45b54cf519abcb7009d0ded9a056d3))
* domain model types for users, projects, environments, flags, audit ([5b4014a](https://github.com/joCur/togglerino/commit/5b4014ae812cae279b25d8d68a8e6a9cde12952b))
* enforce admin role on destructive operations ([322ac86](https://github.com/joCur/togglerino/commit/322ac861a582c3a81146405d056aad907acd0b55))
* environment store and handlers, auto-create default environments ([28ce706](https://github.com/joCur/togglerino/commit/28ce70637858937fbb59446e6b046ec89c99e469))
* evaluation engine with operators, consistent hashing, targeting rules ([ce3229a](https://github.com/joCur/togglerino/commit/ce3229a0c525181113f1ca5b4e6c3aaa88b8d08a))
* extract shared Topbar component from Layout ([73790cb](https://github.com/joCur/togglerino/commit/73790cb34fe859a5997edaf5c4f41d3b7b2d3239))
* flag store and management API handlers ([26aed3a](https://github.com/joCur/togglerino/commit/26aed3a73d020b79627c11d9fd531b4aabc6c016))
* in-memory flag config cache with concurrent-safe access ([0c328c7](https://github.com/joCur/togglerino/commit/0c328c7181b7f35d1df1c55ec0321a67d56724ed))
* make CORS origins configurable via CORS_ORIGINS env var ([27d8def](https://github.com/joCur/togglerino/commit/27d8def8a01d2bd2666c50be436a97893bf9ffa6))
* project scaffolding with Go module, config, health endpoint, docker-compose ([7b18315](https://github.com/joCur/togglerino/commit/7b1831539b39a7cade640c2121358e7d8c114908))
* project store and management API handlers ([2053e82](https://github.com/joCur/togglerino/commit/2053e82bc27f8746615a16e12996cf3f61c0abb1))
* React dashboard scaffolding with Vite, TanStack Query, React Router ([959be3f](https://github.com/joCur/togglerino/commit/959be3fb646b37740a94a323d8105f968188bb73))
* redesign dashboard with Ink & Ember theme ([3e0182b](https://github.com/joCur/togglerino/commit/3e0182bf4b4c2537cccd7392d910e57e3856ec2a))
* replace team page placeholder with member management UI ([208a799](https://github.com/joCur/togglerino/commit/208a7994612317ec394fcb36a30d760f4d919313))
* SDK key store and handlers ([67f463a](https://github.com/joCur/togglerino/commit/67f463a6d8c9e2a63d19007c2d5a361f872cbe2d))
* SSE streaming hub and handler with real-time flag updates ([d36f053](https://github.com/joCur/togglerino/commit/d36f053ea2a700b94a0e6e2899bab51b7cc73954))
* user and session stores with tests ([7e4eeaf](https://github.com/joCur/togglerino/commit/7e4eeafef4d20f9d78f43067c8adbc4781547672))
* wire all handlers into router, full server assembly ([f4c8bff](https://github.com/joCur/togglerino/commit/f4c8bff686c22c85ce791524b337bc53bc94c108))
* wire two-layout architecture, remove old Layout ([0b848e5](https://github.com/joCur/togglerino/commit/0b848e5a15b0742c7e4e353eaaf533ac7aed8357))


### Bug Fixes

* enforce SDK key project/environment scope on client API endpoints ([e65b9dd](https://github.com/joCur/togglerino/commit/e65b9dd6a430d02d11e3a05770c48c11a250238e))
* logout button and post-logout loading screen ([c448f95](https://github.com/joCur/togglerino/commit/c448f95937d24f662ba4bab0add5fea656234caf))
* prevent race condition in concurrent invite acceptance ([6529a07](https://github.com/joCur/togglerino/commit/6529a07fddf486bcbcad9a50e0bfea4d5db40b90))
* run database migrations before Go tests in CI ([b8c49b6](https://github.com/joCur/togglerino/commit/b8c49b62b4466e54659cc893a9240ec99171547c))
