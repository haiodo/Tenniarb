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

.DEFAULT_GOAL := help
N ?= 2000
.PHONY: help build test test-only golden perf lint lint-fix format format-check ci clean web web-test tenn-fuzz

help: ## Show available targets
	@grep -hE '^[a-z-]+:.*?## ' $(MAKEFILE_LIST) | awk -F':.*?## ' '{printf "  \033[36m%-10s\033[0m %s\n", $$1, $$2}'

build: ## Build the app (Debug)
	$(XCB) -scheme $(SCHEME) build

test: ## Run unit tests (performance tests excluded)
	$(XCB) -scheme $(SCHEME) test

test-only: ## Run a single suite: make test-only T=TenniarbTests/LexerTests
	$(XCB) -scheme $(SCHEME) -only-testing:$(T) test

golden: ## Regenerate web/fixtures/*.parse.json from the Swift parser
	TEST_RUNNER_TENN_GOLDEN_UPDATE=1 $(XCB) -scheme $(SCHEME) -only-testing:TenniarbTests/TennGoldenTests test

perf: ## Run performance tests only
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

tenn-fuzz: web/node_modules ## Differential fuzz: Swift vs TS .tenn parser
	node web/packages/core/fuzz/fuzz.ts $(N)

clean: ## Remove derived data and lint cache
	rm -rf $(DERIVED) .swiftlint-cache
