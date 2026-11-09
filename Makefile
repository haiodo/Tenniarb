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
BLOG ?= ../haiodo.blog
.PHONY: help build test test-only golden swift-png perf lint lint-fix format format-check ci clean web web-test go-test app go-mcp go-mcp-install go-cli-install opencode-mcp codex-mcp claude-mcp agent-install skill-install kill-running install go-fuzz go-pkg-mac go-pkg-win go-pkg-linux go-pkg tenn-bench embed view render tenn-fuzz md-fuzz blog web-demo

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

go-test: ## Go port (go/): vet + tests
	cd go && go vet ./... && go test ./...

go-mcp: ## MCP for agents: build/go/bin/tenniarb-mcp (proxy) and tenniarb-service, side by side
	cd go && go build -trimpath -ldflags '-s -w' -o ../build/go/bin/ ./cmd/tenniarb-mcp ./cmd/tenniarb-service

go-mcp-install: ## MCP for agents: go install tenniarb-mcp and tenniarb-service into the same GOBIN
	cd go && go install -trimpath -ldflags '-s -w' ./cmd/tenniarb-mcp ./cmd/tenniarb-service

go-cli-install: ## CLI for agents: go install tenniarb (tenniarb check FILE...) into GOBIN
	cd go && CGO_ENABLED=$(GO_CGO) go install $(GO_TAGS) -trimpath -ldflags '$(GO_LDFLAGS)' ./cmd/tenniarb

GO_PKG_BIN = $(shell cd go && p=$$(go env GOBIN); [ -n "$$p" ] || p=$$(go env GOPATH)/bin; echo $$p)

# Where the MCP proxy writes the calls of the agents, and the address of the service profiler: the two
# things a run is taken apart with. Both are set in the agent config by the targets below, so a session
# started by opencode or codex writes them without anyone remembering a variable.
MCP_LOG   ?= $(HOME)/Library/Logs/Tenniarb/mcp-calls.log
MCP_PPROF ?= 127.0.0.1:6061

agent-install: go-cli-install ## Skill and MCP server in every agent (opencode, codex, Claude Code)
	$(GO_PKG_BIN)/tenniarb install --mcp --agents --log $(MCP_LOG) --pprof $(MCP_PPROF)

opencode-mcp: go-cli-install ## Skill and MCP server in opencode only
	$(GO_PKG_BIN)/tenniarb install --mcp --opencode --log $(MCP_LOG) --pprof $(MCP_PPROF)

codex-mcp: go-cli-install ## Skill and MCP server in codex only
	$(GO_PKG_BIN)/tenniarb install --mcp --codex --log $(MCP_LOG) --pprof $(MCP_PPROF)

claude-mcp: go-cli-install ## Skill and MCP server in Claude Code only
	$(GO_PKG_BIN)/tenniarb install --mcp --claude --log $(MCP_LOG) --pprof $(MCP_PPROF)

install: app go-mcp-install go-cli-install agent-install kill-running ## Install locally: the app into /Applications (macOS), tenniarb CLI and MCP into GOBIN, the agent skill and the tenniarb MCP in opencode, codex and Claude Code, with the call log and the profiler
ifeq ($(shell uname),Darwin)
	rm -rf /Applications/Tenniarb.app
	cp -R $(GOPKG)/app/Tenniarb.app /Applications/
	/System/Library/Frameworks/CoreServices.framework/Frameworks/LaunchServices.framework/Support/lsregister -f /Applications/Tenniarb.app
endif

skill-install: go-cli-install ## Write the tenniarb-design skill where Claude Code and opencode read it
	$(GO_PKG_BIN)/tenniarb install

kill-running: ## Stop the app, the MCP proxy and the service that keep the old binaries in memory
	@pkill -x tenniarb || true
	@pkill -x tenniarb-mcp || true
	@pkill -x tenniarb-service || true

# Go app packaging (go/packaging). Unsigned: no codesign identity, no notarization.
GOPKG := build/go
# The Go app is versioned here, apart from the Swift release tags (v1.x) that trigger release.yml.
GO_SHORT := 2.1.0
GO_VERSION = $(GO_SHORT)-$(shell git describe --always --dirty --exclude '*')
GO_BUILD_NO = $(shell git rev-list --count HEAD)
GO_LDFLAGS = -s -w -X github.com/haiodo/tenniarb/go/app.Version=$(GO_VERSION)
GOWT_DIR = $(shell cd go && go mod download github.com/haiodo/gowt 2>/dev/null; go list -m -f '{{.Dir}}' github.com/haiodo/gowt)

# CGO=1 is the default: the app builds with gowt's native cgo bindings (CGO_ENABLED=1 -tags gowtcgo),
# about eight times faster native calls, and it needs a C toolchain. CGO=0 keeps the purego backends;
# the packaging targets below always cross-compile on the purego ones.
CGO ?= 1
ifeq ($(CGO),1)
GO_CGO := 1
GO_TAGS := -tags gowtcgo
else
GO_CGO := 0
GO_TAGS :=
endif

app: $(EMBED) ## Go app for this machine: build/go/app/Tenniarb.app on macOS, build/go/app/tenniarb elsewhere (unsigned); CGO=0 for purego
	rm -rf $(GOPKG)/app && mkdir -p $(GOPKG)/app
	cd go && CGO_ENABLED=$(GO_CGO) go build $(GO_TAGS) -trimpath -ldflags '$(GO_LDFLAGS)' -o ../$(GOPKG)/app/tenniarb ./cmd/tenniarb
ifeq ($(shell uname),Darwin)
	go/packaging/macos.sh $(GOPKG)/app/tenniarb $(GOPKG)/app $(GOWT_DIR) $(GO_SHORT) $(GO_BUILD_NO) $(EMBED)
	rm $(GOPKG)/app/tenniarb
else
	cp $(EMBED) $(GOPKG)/app/
endif

go-pkg-mac: $(EMBED) ## Go app: build/go/Tenniarb-<ver>-arm64.{app.zip,dmg} for Apple silicon (unsigned)
	rm -rf $(GOPKG)/mac && mkdir -p $(GOPKG)/mac/dmg
	cd go && CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -trimpath -ldflags '$(GO_LDFLAGS)' -o ../$(GOPKG)/mac/tenniarb ./cmd/tenniarb
	cd go && CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -trimpath -ldflags '$(GO_LDFLAGS)' -o ../$(GOPKG)/mac/tenniarb-mcp ./cmd/tenniarb-mcp
	go/packaging/macos.sh $(GOPKG)/mac/tenniarb $(GOPKG)/mac/dmg $(GOWT_DIR) $(GO_SHORT) $(GO_BUILD_NO) $(EMBED) $(GOPKG)/mac/tenniarb-mcp
	ditto -c -k --keepParent $(GOPKG)/mac/dmg/Tenniarb.app $(GOPKG)/Tenniarb-$(GO_VERSION)-arm64.app.zip
	ln -sfn /Applications $(GOPKG)/mac/dmg/Applications
	rm -f $(GOPKG)/Tenniarb-$(GO_VERSION)-arm64.dmg
	hdiutil create -quiet -volname Tenniarb -srcfolder $(GOPKG)/mac/dmg -format UDZO $(GOPKG)/Tenniarb-$(GO_VERSION)-arm64.dmg

go-pkg-win: $(EMBED) ## Go app: build/go/tenniarb-<ver>-windows-amd64.zip (cross-built, no icon resource yet)
	rm -rf $(GOPKG)/win && mkdir -p $(GOPKG)/win
	cd go && CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -trimpath -ldflags '$(GO_LDFLAGS) -H=windowsgui' -o ../$(GOPKG)/win/tenniarb.exe ./cmd/tenniarb
	cd go && CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -trimpath -ldflags '$(GO_LDFLAGS)' -o ../$(GOPKG)/win/tenniarb-mcp.exe ./cmd/tenniarb-mcp
	cp $(EMBED) $(GOPKG)/win/
	rm -f $(GOPKG)/tenniarb-$(GO_VERSION)-windows-amd64.zip
	cd $(GOPKG)/win && zip -q -r ../tenniarb-$(GO_VERSION)-windows-amd64.zip .

go-pkg-linux: $(EMBED) ## Go app: build/go/tenniarb-<ver>-linux-amd64.tar.gz (binary, .desktop, mime xml, install.sh)
	rm -rf $(GOPKG)/linux && mkdir -p $(GOPKG)/linux
	cd go && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags '$(GO_LDFLAGS)' -o ../$(GOPKG)/linux/tenniarb ./cmd/tenniarb
	cd go && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags '$(GO_LDFLAGS)' -o ../$(GOPKG)/linux/tenniarb-mcp ./cmd/tenniarb-mcp
	go/packaging/linux.sh $(GOPKG)/linux/tenniarb $(GOPKG)/linux/tenniarb-$(GO_VERSION) $(GOWT_DIR) $(EMBED)
	cp $(GOPKG)/linux/tenniarb-mcp $(GOPKG)/linux/tenniarb-$(GO_VERSION)/
	tar -C $(GOPKG)/linux -czf $(GOPKG)/tenniarb-$(GO_VERSION)-linux-amd64.tar.gz tenniarb-$(GO_VERSION)

go-pkg: go-pkg-mac go-pkg-win go-pkg-linux ## Go app: all packages

go-fuzz: ## Differential fuzz: Swift vs Go .tenn parser: make go-fuzz N=20000 SEED=1
	cd go && go run ./cmd/tennfuzz -n $(or $(N),2000) -seed $(or $(SEED),1)

tenn-bench: ## Compare Tenn lexer/parser speed: Go vs Swift vs TS: make tenn-bench RUNS=15
	cd go && go run ./cmd/tennbench -runs $(or $(RUNS),15)

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

blog: web/node_modules ## Build embed + editor and copy the live demo into a blog: make blog BLOG=../haiodo.blog
	cd web && npm run build -w @tenniarb/embed -w @tenniarb/editor
	mkdir -p $(BLOG)/public/tenniarb/fonts
	cp web/packages/editor/dist/editor.js web/packages/embed/dist/tenniarb-embed.min.js $(BLOG)/public/tenniarb/
	cp web/packages/embed/dist/Inter-*.woff2 $(BLOG)/public/tenniarb/fonts/
	$(MAKE) web-demo

web-demo: ## Bundle tenniarb.tenn into the single file the blog demo loads: make web-demo BLOG=../haiodo.blog
	mkdir -p $(BLOG)/public/tenniarb
	cd go && go run ./cmd/tenniarb bundle -root $(abspath .) -o $(abspath $(BLOG))/public/tenniarb/tenniarb.tenn $(abspath tenniarb.tenn)

clean: ## Remove derived data and lint cache
	rm -rf $(DERIVED) .swiftlint-cache
