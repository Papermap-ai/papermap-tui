# Changelog

## [0.8.2](https://github.com/Papermap-ai/papermap-tui/compare/v0.8.1...v0.8.2) (2026-06-16)


### Bug Fixes

* **app:** tidy invalid session startup ([2cd3cc7](https://github.com/Papermap-ai/papermap-tui/commit/2cd3cc778e98b87c7463adde4d5343e877694969))
* **app:** tidy invalid session startup ([5013db2](https://github.com/Papermap-ai/papermap-tui/commit/5013db24bdee3117a7677aa9d680cf0182f713b3))

## [0.8.1](https://github.com/Papermap-ai/papermap-tui/compare/v0.8.0...v0.8.1) (2026-06-16)


### Bug Fixes

* **auth:** restore Keychain credential storage ([12c4da6](https://github.com/Papermap-ai/papermap-tui/commit/12c4da611ad6162bbc21c62b1fe19e3c7e592c0f))
* **auth:** restore Keychain credential storage ([2e8bb28](https://github.com/Papermap-ai/papermap-tui/commit/2e8bb28dec4469aca6ee57fe6f721fa11dd79eb0))

## [0.8.0](https://github.com/Papermap-ai/papermap-tui/compare/v0.7.2...v0.8.0) (2026-06-11)


### Features

* **config:** persist thinking toggle ([9df9e62](https://github.com/Papermap-ai/papermap-tui/commit/9df9e62f947acbcc8b62e85ed247809d4111cd71))
* **workspace:** add picker search ([1c783bf](https://github.com/Papermap-ai/papermap-tui/commit/1c783bf9b78c6c962531bce46732f134e6dbbbc1))


### Bug Fixes

* **app:** return to palette from pickers ([cd01767](https://github.com/Papermap-ai/papermap-tui/commit/cd017673b0416a1c455a9f4ab89414010b5890f7))

## [0.7.2](https://github.com/Papermap-ai/papermap-tui/compare/v0.7.1...v0.7.2) (2026-06-11)


### Bug Fixes

* **app:** refresh workspaces on picker open ([346517c](https://github.com/Papermap-ai/papermap-tui/commit/346517c913b3f401b6f05787e190e75d0e30a00f))
* **config:** prefer config file URLs ([1a3f655](https://github.com/Papermap-ai/papermap-tui/commit/1a3f6555da01eec703d41c0b2b5513e49ed0405a))

## [0.7.1](https://github.com/Papermap-ai/papermap-tui/compare/v0.7.0...v0.7.1) (2026-06-10)


### Documentation

* mention Homebrew tui alias ([2091574](https://github.com/Papermap-ai/papermap-tui/commit/2091574c6c3b4b8c13e1c93828e973469e62a47b))
* mention Homebrew tui alias ([b9be5b4](https://github.com/Papermap-ai/papermap-tui/commit/b9be5b40a2b5b66c13537287a22ad01070c5761a))

## [0.7.0](https://github.com/Papermap-ai/papermap-tui/compare/v0.6.3...v0.7.0) (2026-06-09)


### Features

* **release:** publish Homebrew formula to tap ([aa37565](https://github.com/Papermap-ai/papermap-tui/commit/aa375653e39e970d00baf196beb6dcdc8a6c90d1))
* **release:** publish Homebrew formula to tap ([cba16db](https://github.com/Papermap-ai/papermap-tui/commit/cba16dbcadb53d0290f878fd302b4d4ecbaa4319))


### Documentation

* add MIT license ([3a43b43](https://github.com/Papermap-ai/papermap-tui/commit/3a43b43349c9585845fed1609f5aedf96af225c2))
* add MIT license ([42307b3](https://github.com/Papermap-ai/papermap-tui/commit/42307b3793063ab28107a434348029cec101db32))

## [0.6.3](https://github.com/Papermap-ai/papermap-tui/compare/v0.6.2...v0.6.3) (2026-06-09)


### Bug Fixes

* **config:** set default API URL to prod host ([94245d7](https://github.com/Papermap-ai/papermap-tui/commit/94245d7a645e07b9818246de7723b40c4ee20adf))
* **config:** set default API URL to prod host ([298c7b6](https://github.com/Papermap-ai/papermap-tui/commit/298c7b6be29e924df7f566651ed0dba2d5b80648))

## [0.6.1](https://github.com/Papermap-ai/papermap-tui/compare/v0.6.0...v0.6.1) (2026-06-09)


### Security

* **api:** fail closed on partial refresh responses ([d23bf28](https://github.com/Papermap-ai/papermap-tui/commit/d23bf28f87bfa6f3151c49a591f542d98ea1332c))
* **api:** strip Authorization on cross-host redirects ([0b94ac9](https://github.com/Papermap-ai/papermap-tui/commit/0b94ac97ad030dacd78fda71df6eeacc912743d6))

## [0.6.0](https://github.com/Papermap-ai/papermap-tui/compare/v0.5.0...v0.6.0) (2026-05-08)


### Features

* **charts:** render line, area, scatter, and radar natively ([1a71f4d](https://github.com/Papermap-ai/papermap-tui/commit/1a71f4dfc7096c3633ea8b2171c634ff01ff2dcc))


### Refactors

* **dialog:** unify approval and quit confirm into generic primitive ([02e0b01](https://github.com/Papermap-ai/papermap-tui/commit/02e0b01a124741d9ecd29d7015579adc71735263))

## [0.5.0](https://github.com/Papermap-ai/papermap-tui/compare/v0.4.0...v0.5.0) (2026-05-06)


### Features

* **approval:** add tool-call confirmation modal ([919d466](https://github.com/Papermap-ai/papermap-tui/commit/919d4662e9f84bcca0c851c56a9e08b96f00951f))
* **auth:** browser-based login with already-signed-in short-circuit ([d5ea04c](https://github.com/Papermap-ai/papermap-tui/commit/d5ea04c354416858f824f69b4335a1a30da591cf))
* **cli:** add 'workspace create' and 'workspace list' subcommands ([628f5dd](https://github.com/Papermap-ai/papermap-tui/commit/628f5dd964cb4b13a1cf6250667222c7b0065ff8))
* **shell:** default Windows "!" to PowerShell with cmd opt-out ([5470280](https://github.com/Papermap-ai/papermap-tui/commit/547028025a14a33cf75ebb299c6496fa287daa4e))
* **shell:** port "!" shell mode to Windows ([af1a131](https://github.com/Papermap-ai/papermap-tui/commit/af1a131586d6626649ce7909dea2393fb7e13e80))
* **shell:** port "!" shell mode to Windows ([ffbc7d2](https://github.com/Papermap-ai/papermap-tui/commit/ffbc7d2891ae8281e778510e1cf495d5d8223a1a))


### Bug Fixes

* **chat:** scroll to bottom after shell result append ([9e4ad09](https://github.com/Papermap-ai/papermap-tui/commit/9e4ad095c09f4095cbde649c680ce1957422dfa6))


### Documentation

* **readme:** document "!" shell mode and Windows pwsh/cmd config ([26596cc](https://github.com/Papermap-ai/papermap-tui/commit/26596ccf0d4f8e24eb0f0604c2994109dfb60003))
* **readme:** document "!" shell mode and Windows pwsh/cmd config ([61293dc](https://github.com/Papermap-ai/papermap-tui/commit/61293dc9fcd25756c94ef8e49891745147a4d786))

## [0.4.0](https://github.com/Papermap-ai/papermap-tui/compare/v0.3.0...v0.4.0) (2026-04-30)


### Features

* **chat:** "!" shell mode with sandboxed one-shot exec ([6901920](https://github.com/Papermap-ai/papermap-tui/commit/69019202a7627ffc7bb58f13c23d268d17f992ef))
* **chat:** "!" shell mode with sandboxed one-shot exec ([da10c0c](https://github.com/Papermap-ai/papermap-tui/commit/da10c0c7ca991898411b346e4983ca48870ede53))

## [0.3.0](https://github.com/Papermap-ai/papermap-tui/compare/v0.2.0...v0.3.0) (2026-04-29)


### Features

* add LLM model picker with TAB cycle and persisted selection ([e11bd15](https://github.com/Papermap-ai/papermap-tui/commit/e11bd151303f511f29e57f62c88dcbc10b65c42f))
* **chat:** breathe trace and visualizations apart from body ([2068415](https://github.com/Papermap-ai/papermap-tui/commit/206841544b630a3deace1c94ebb72855a10f819b))
* **chat:** cancel in-flight insights with inline error rendering ([f3645d0](https://github.com/Papermap-ai/papermap-tui/commit/f3645d0a0446de6ff1583ee7227c4aab0944c0fd))
* **chat:** collapse large pastes into removable chips ([fad5d75](https://github.com/Papermap-ai/papermap-tui/commit/fad5d75ee13bd6f12d109d698536cf3e5c2f4e1c))
* **chat:** conversation history with command palette ([d34b0cf](https://github.com/Papermap-ai/papermap-tui/commit/d34b0cf461e819cedadad3a8b265a40137cbb119))
* **chat:** ctrl+l clears prompt textarea ([17f8e12](https://github.com/Papermap-ai/papermap-tui/commit/17f8e125259c60dfe433812c4ece3d24a068da68))
* **chat:** drag-to-select transcript with OSC52 copy + toast ([e58a101](https://github.com/Papermap-ai/papermap-tui/commit/e58a101414229db5164a75f6f8508554303f4e7d))
* **chat:** paint selection via cell buffer + banner toast ([a249212](https://github.com/Papermap-ai/papermap-tui/commit/a2492125f2842d03f46e70992f99bf48d47077f9))
* **chat:** sticky thinking toggle with muted streaming preview ([c7eb02f](https://github.com/Papermap-ai/papermap-tui/commit/c7eb02ff21ce834e9aedf67e7f864efa6a9ee3b8))


### Bug Fixes

* **chat:** show conversations overlay on Ctrl+P ([5b9b351](https://github.com/Papermap-ai/papermap-tui/commit/5b9b3518db8ca288e0cb02edb4b86ff5d1f094f3))
* **chat:** show conversations overlay on Ctrl+P ([12ee33f](https://github.com/Papermap-ai/papermap-tui/commit/12ee33f868e9ec170c784b758485a56d6d3506ce))


### Documentation

* update tagline to Papermap Data Platform ([72c446b](https://github.com/Papermap-ai/papermap-tui/commit/72c446b3a6376eff1e7bd0e9040589c4cfd94b41))

## [0.2.0](https://github.com/Papermap-ai/papermap-tui/compare/v0.1.1...v0.2.0) (2026-04-24)


### Features

* **charts:** native bar and pie rendering with right-aligned values ([209b8ea](https://github.com/Papermap-ai/papermap-tui/commit/209b8ea6a577f6707ee36e762893185c1554c13e))
* **chat:** persist streaming trace, wrap thoughts/tool output, drop dead code ([099c497](https://github.com/Papermap-ai/papermap-tui/commit/099c4970ecbb77c90fb4b8a5c5e34478649b0f52))


### Bug Fixes

* keep HTTP body alive when SSE complete sentinel arrives ([42e6a9e](https://github.com/Papermap-ai/papermap-tui/commit/42e6a9eb6826348e2daac880a10df0b0f01ce364))
* replace shape of m on the papermap log ([c6458cb](https://github.com/Papermap-ai/papermap-tui/commit/c6458cb905af2401aeb8d4998c80ca395aee3140))
* replace shape of m on the papermap log ([8fdebc4](https://github.com/Papermap-ai/papermap-tui/commit/8fdebc40252a73ace35d93c67b099628250e25e3))


### Refactors

* address code-quality followups across app/api/theme ([e9940e5](https://github.com/Papermap-ai/papermap-tui/commit/e9940e5bfb4f0690d4adecded8608c24d59b757e))

## [0.1.1](https://github.com/Papermap-ai/papermap-tui/compare/v0.1.0...v0.1.1) (2026-04-22)


### Documentation

* adds tasks in phase two of papermap tui ([d11ceec](https://github.com/Papermap-ai/papermap-tui/commit/d11ceecfacee6025233f2a7ef752347e7067f64f))

## [0.1.0](https://github.com/Papermap-ai/papermap-tui/compare/v0.0.1...v0.1.0) (2026-04-22)


### Features

* fixes scrolling with mouse on viewport ([7fc0109](https://github.com/Papermap-ai/papermap-tui/commit/7fc01093d9b00de8856dc696f00c4139e9e728ac))
* **install:** improve installer robustness for release downloads ([2e40b19](https://github.com/Papermap-ai/papermap-tui/commit/2e40b19ff68f198a4f987eb9d6e8e7790341b833))


### Documentation

* **install:** simplify install one-liner ([eb6c1f1](https://github.com/Papermap-ai/papermap-tui/commit/eb6c1f169490c181ff9235589c78142aa48518e4))
