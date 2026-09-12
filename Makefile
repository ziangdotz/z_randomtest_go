ifeq ($(OS), Windows_NT)
	FIX_PATH = $(subst /,\,$(1))
	RM = if exist $(call FIX_PATH, $(1)) rd /s /q $(call FIX_PATH, $(1))
	RF = if exist $(call FIX_PATH, $(1)) del /f /q $(call FIX_PATH, $(1))
	MD = if not exist $(call FIX_PATH, $(1)) mkdir $(call FIX_PATH, $(1))
else
	FIX_PATH = $(1)
	RM = rm -rf $(1)
	RF = rm -f $(1)
	MD = mkdir -p $(1)
endif


MUSL_CC_PATH = C:/Tools/cross-tools/x86_64-linux-musl/bin/x86_64-linux-musl-gcc.exe
MUSL_CXX_PATH = C:/Tools/cross-tools/x86_64-linux-musl/bin/x86_64-linux-musl-g++.exe
ZIG_PATH = C:/Tools/zig-x86_64-windows-0.16.0-dev.2490+fce7878a9/zig.exe

WEB_CODE_PATH = ./randomtest_web_gin
WEB_MAIN = main.go
WEB_MAIN_PATH = $(WEB_CODE_PATH)/$(WEB_MAIN)
STDTEST_CODE_PATH = ./randomtest_stdtest
STDTEST_MAIN = randomtest_stdtest.go
STDTEST_MAIN_PATH = $(STDTEST_CODE_PATH)/$(STDTEST_MAIN)
STDTEST_VUE3_WAILSV2_CODE_PATH = ./randomtest_stdtest_vue3_wailsv2

BUILDDIR = release

RELEASE_WEB_GIN_WINDOWS_AMD64 = $(BUILDDIR)/z_randomtest_web_gin_windows_amd64.exe
RELEASE_WEB_GIN_LINUX_AMD64 = $(BUILDDIR)/z_randomtest_web_gin_linux_amd64
RELEASE_STDTEST_WINDOWS_AMD64 = $(BUILDDIR)/z_randomtest_stdtest_windows_amd64.exe
RELEASE_STDTEST_LINUX_AMD64 = $(BUILDDIR)/z_randomtest_stdtest_linux_amd64
RELEASE_STDTEST_DARWIN_ARM64 = $(BUILDDIR)/z_randomtest_stdtest_darwin_arm64

all: help

.PHONY: prepare clean help test_core test_core_race update_core_golden

test_core:
	cd ./randomtest_core \
	&& go mod download \
	&& go test -timeout 20m -v ./...

test_core_race:
	cd ./randomtest_core \
	&& go mod download \
	&& go test -race -timeout 20m ./...

update_core_golden:
	cd ./randomtest_core \
	&& go mod download \
	&& go test . -v -run TestRegressionBaseline -args -update-golden

prepare:
	@$(call MD, $(BUILDDIR))

run_web_gin:
	cd $(WEB_CODE_PATH) \
	&& go mod download \
	&& go run $(WEB_MAIN)

build_web_gin_windows_amd64: prepare clean
	@echo "Build started: web Windows amd64 version..."
	@go mod download
	@set CGO_ENABLED=1&& set GOOS=windows&& set GOARCH=amd64&&\
	set CC=gcc&&set CXX=g++&&\
	go build -ldflags "-extldflags -static -s -w" -o $(call FIX_PATH, $(RELEASE_WEB_GIN_WINDOWS_AMD64)) $(call FIX_PATH, $(WEB_MAIN_PATH))
	@echo "Build completed: $(RELEASE_WEB_GIN_WINDOWS_AMD64)"

build_web_gin_linux_amd64: prepare clean
	@echo "Build started: web Linux amd64 version..."
	@go mod download
	@set CGO_ENABLED=1&& set GOOS=linux&& set GOARCH=amd64&&\
	set CC=$(MUSL_CC_PATH)&&set CXX=$(MUSL_CXX_PATH)&&\
	go build -trimpath -ldflags "-linkmode external -extldflags -static -s -w" -o $(call FIX_PATH, $(RELEASE_WEB_GIN_LINUX_AMD64)) $(call FIX_PATH, $(WEB_MAIN_PATH))
	@echo "Build completed: $(RELEASE_WEB_GIN_LINUX_AMD64)"

run_stdtest:
	cd $(STDTEST_CODE_PATH) \
	&& go mod download \
	&& go run $(STDTEST_MAIN)

build_stdtest_windows_amd64: prepare clean
	@echo "Build started: stdtest Windows amd64 version..."
	@go mod download
	@set CGO_ENABLED=1&&set GOOS=windows&&set GOARCH=amd64&&\
	set CC=gcc&& set CXX=g++&&\
	go build -ldflags "-extldflags -static -s -w" -o $(call FIX_PATH, $(RELEASE_STDTEST_WINDOWS_AMD64)) $(call FIX_PATH, $(STDTEST_MAIN_PATH))
	@echo "Build completed: $(RELEASE_STDTEST_WINDOWS_AMD64)"

build_stdtest_linux_amd64: prepare clean
	@echo "Build started: stdtest Linux amd64 version..."
	@go mod download
	@set CGO_ENABLED=1&&set GOOS=linux&&set GOARCH=amd64&&\
	set CC=$(MUSL_CC_PATH)&&set CXX=$(MUSL_CXX_PATH)&&\
	go build -trimpath -ldflags "-linkmode external -extldflags -static -s -w" -o $(call FIX_PATH, $(RELEASE_STDTEST_LINUX_AMD64)) $(call FIX_PATH, $(STDTEST_MAIN_PATH))
	@echo "Build completed: $(RELEASE_STDTEST_LINUX_AMD64)"

run_stdtest_vue3_wailsv2_test:
	cd $(STDTEST_VUE3_WAILSV2_CODE_PATH) \
	&& go mod download \
	&& go test -v -run TestRunSTDTestFromIniConfigOptional . -args -run-config-flow

run_stdtest_vue3_wailsv2:
	cd $(STDTEST_VUE3_WAILSV2_CODE_PATH) && \
	make run_dev

build_stdtest_vue3_wailsv2_windows_amd64:
	cd $(STDTEST_VUE3_WAILSV2_CODE_PATH) && \
	make build_windows_amd64

build_all: build_web_gin_windows_amd64 build_web_gin_linux_amd64 build_stdtest_windows_amd64 build_stdtest_linux_amd64 build_stdtest_vue3_wailsv2_windows_amd64

clean:
	$(call RF, $(RELEASE_WEB_GIN_WINDOWS_AMD64))
	$(call RF, $(RELEASE_WEB_GIN_LINUX_AMD64))
	$(call RF, $(RELEASE_STDTEST_WINDOWS_AMD64))
	$(call RF, $(RELEASE_STDTEST_LINUX_AMD64))
	$(call RF, $(RELEASE_STDTEST_DARWIN_ARM64))
	go clean

help:
	@echo ------------------------
	@echo "make run_web_gin:                              run web gin server"
	@echo "make build_web_gin_windows_amd64:              build main.go -- web windows amd64 version"
	@echo "make build_web_gin_linux_amd64:                build main.go -- web linux amd64 version"
	@echo "make test_core:                                run randomtest_core golden/regression tests"
	@echo "make test_core_race:                           run randomtest_core tests with race detector"
	@echo "make update_core_golden:                       rebuild testdata/baseline.json regression baseline"
	@echo "make run_stdtest:                              run stdtest"
	@echo "make build_stdtest_windows_amd64:              build randomtest_stdtest.go -- stdtest windows amd64 version"
	@echo "make build_stdtest_linux_amd64:                build randomtest_stdtest.go -- stdtest linux amd64 version"
	@echo "make run_stdtest_vue3_wailsv2_test:            test randomtest_stdtest_vue3_wailsv2"
	@echo "make run_stdtest_vue3_wailsv2:                 run randomtest_stdtest_vue3_wailsv2"
	@echo "make build_stdtest_vue3_wailsv2_windows_amd64: build randomtest_stdtest_vue3_wailsv2 -- stdtest vue3 wailsv2 windows amd64 version"
	@echo "make build_all:                                build all works"
	@echo "make clean:                                    clean middle files"
	@echo "make help:                                     see custom commands"
	@echo "make:                                          like 'make help'"
	@echo ------------------------

