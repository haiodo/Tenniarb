PROJECT     := Tenniarb.xcodeproj
SCHEME      := Tenniarb
PERF_SCHEME := Tenniarb-Performance
CONFIG      := Debug
DEST        := platform=macOS
DERIVED     := build
NOSIGN      := CODE_SIGNING_ALLOWED=NO CODE_SIGNING_REQUIRED=NO CODE_SIGN_IDENTITY="" DEVELOPMENT_TEAM=""

XCB := xcodebuild -project $(PROJECT) -destination '$(DEST)' -configuration $(CONFIG) -derivedDataPath $(DERIVED) $(NOSIGN)

SWIFT_SRC := $(shell find Tenniarb TenniarbTests TenniarbUITests -name '*.swift' -not -path '*/Preview Content/*' -not -path '*/Experiments/*')

# Via xcrun, not `swift format`: a toolchain on PATH (e.g. swift-actions/setup-swift)
# would shadow it with a different version and report bogus diffs.
SWIFT_FORMAT := xcrun swift-format

# Bundled into the app as a resource, so every xcodebuild target needs it.
EMBED := Tenniarb/web/tenniarb-embed.min.js
EMBED_SRC := $(shell find web/packages/core/src web/packages/markdown/src web/packages/render/src web/packages/embed/src web/packages/embed/fonts -type f 2>/dev/null)

.DEFAULT_GOAL := help
N ?= 2000
.PHONY: help build test test-only golden swift-png perf lint lint-fix format format-check ci clean web web-test app-dev app embed view render tenn-fuzz md-fuzz

help: ## Show available targets
	@grep -hE '^[a-z-]+:.*?## ' $(MAKEFILE_LIST) | awk -F':.*?## ' '{printf "  \033[36m%-10s\033[0m %s\n", $$1, $$2}'

build: $(EMBED) ## Build the app (Debug)
	$(XCB) -scheme $(SCHEME) build

test: $(EMBED) ## Run unit tests (performance tests excluded)
	$(XCB) -scheme $(SCHEME) test

test-only: $(EMBED) ## Run a single suite: make test-only T=TenniarbTests/LexerTests
	$(XCB) -scheme $(SCHEME) -only-testing:$(T) test

golden: $(EMBED) ## Regenerate web/fixtures/*.parse.json from the Swift parser
	TEST_RUNNER_TENN_GOLDEN_UPDATE=1 $(XCB) -scheme $(SCHEME) -only-testing:TenniarbTests/TennGoldenTests test

swift-png: $(EMBED) ## Export Swift reference PNGs (F=file.tenn O=dir; default web/fixtures)
	TEST_RUNNER_TENN_SWIFT_PNG=$(abspath $(or $(O),.work/web-stage5/swift-png)) $(if $(F),TEST_RUNNER_TENN_SWIFT_IN=$(abspath $(F))) $(XCB) -scheme $(SCHEME) -only-testing:TenniarbTests/TennGoldenTests/testExportSwiftPNG test

perf: $(EMBED) ## Run performance tests only
	$(XCB) -scheme $(PERF_SCHEME) test

lint: ## Run SwiftLint
	swiftlint lint --config .swiftlint.yml

lint-fix: ## Apply SwiftLint autocorrections
	swiftlint --fix --config .swiftlint.yml

format: ## Reformat sources in place (swift-format from Xcode toolchain)
	$(SWIFT_FORMAT) --in-place --configuration .swift-format $(SWIFT_SRC)

format-check: ## Fail if sources are not formatted
	$(SWIFT_FORMAT) lint --strict --configuration .swift-format $(SWIFT_SRC)

ci: lint build test ## What CI runs: lint + build + unit tests

web/node_modules: web/package-lock.json
	cd web && npm ci
	@touch $@

web: web/node_modules ## Build web packages (web/)
	cd web && npm run build

web-test: web/node_modules ## Typecheck and test web packages
	cd web && npm run typecheck && npm test

app-dev: web/node_modules ## Run the Tauri app in dev mode (web/packages/app)
	cd web/packages/app && npx tauri dev

app: web/node_modules ## Build the Tauri .app, unsigned (web/packages/app/src-tauri/target/release/bundle/macos)
	cd web/packages/app && npx tauri build

embed: $(EMBED) ## Build Tenniarb/web/tenniarb-embed.min.js (self-contained, not in git)

$(EMBED): web/node_modules $(EMBED_SRC)
	cd web && node packages/embed/scripts/build.ts --swift

view: web/node_modules ## Open a .tenn in the browser viewer: make view F=tenniarb.tenn
	cd web && npm run build -w @tenniarb/viewer
	node web/packages/viewer/scripts/serve.ts $(F)

render: web/node_modules ## Render a .tenn file: make render F=docs/Example.tenn O=out.pdf
	node web/packages/render/src/cli.ts $(F) -o $(O)

tenn-fuzz: web/node_modules ## Differential fuzz: Swift vs TS .tenn parser
	node web/packages/core/fuzz/fuzz.ts $(N)

md-fuzz: web/node_modules ## Differential fuzz: Swift vs TS markdown
	node web/packages/markdown/fuzz/fuzz.ts $(N)

clean: ## Remove derived data and lint cache
	rm -rf $(DERIVED) .swiftlint-cache
