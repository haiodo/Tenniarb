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
.PHONY: help build test test-only perf lint lint-fix format format-check ci clean

help: ## Show available targets
	@grep -hE '^[a-z-]+:.*?## ' $(MAKEFILE_LIST) | awk -F':.*?## ' '{printf "  \033[36m%-10s\033[0m %s\n", $$1, $$2}'

build: ## Build the app (Debug)
	$(XCB) -scheme $(SCHEME) build

test: ## Run unit tests (performance tests excluded)
	$(XCB) -scheme $(SCHEME) test

test-only: ## Run a single suite: make test-only T=TenniarbTests/LexerTests
	$(XCB) -scheme $(SCHEME) -only-testing:$(T) test

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

clean: ## Remove derived data and lint cache
	rm -rf $(DERIVED) .swiftlint-cache
