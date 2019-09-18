# This Makefile is written as generic as possible.
# Setting the variables in .metadata.sh and creating the paths in the repo makes this work.
# See more: https://github.com/golift/application-builder

# Suck in our application information.
IGNORED:=$(shell bash -c "source .metadata.sh ; env | grep -iv 'func_' | sed 's/=/:=/;s/^/export /' > .metadata.make")

# md2roff turns markdown into man files and html files.
MD2ROFF_BIN=github.com/github/hub/md2roff-bin

# Travis CI passes the version in. Local builds get it from the current git tag.
ifeq ($(VERSION),)
	include .metadata.make
else
	# Preserve the passed-in version & iteration (homebrew).
	_VERSION:=$(VERSION)
	_ITERATION:=$(ITERATION)
	include .metadata.make
	VERSION:=$(_VERSION)
	ITERATION:=$(_ITERATION)
endif

# Makefile targets follow.

all: build

# Prepare a release. Called in Travis CI.
release: clean macos
	# Prepareing a release!
	mkdir -p $@
	mv $(BINARY).*.macos $@/
	gzip -9r $@/
	# Generating File Hashes
	openssl dgst -r -sha256 $@/* | sed 's#release/##' | tee $@/checksums.sha256.txt

# Delete all build assets.
clean:
	# Cleaning up.
	rm -f $(BINARY) $(BINARY).*.{macos,linux,exe}{,.gz,.zip} $(BINARY).1{,.gz} $(BINARY).rb
	rm -f $(BINARY){_,-}*.{deb,rpm} v*.tar.gz.sha256 examples/MANUAL .metadata.make
	rm -f cmd/$(BINARY)/README{,.html} README{,.html} ./$(BINARY)_manual.html
	rm -rf package_build_* release

# Build a man page from a markdown file using md2roff.
# This also turns the repo readme into an html file.
# md2roff is needed to build the man file and html pages from the READMEs.
man: $(BINARY).1.gz
$(BINARY).1.gz: md2roff
	# Building man page. Build dependency first: md2roff
	go run $(MD2ROFF_BIN) --manual $(BINARY) --version $(VERSION) --date "$(DATE)" examples/MANUAL.md
	gzip -9nc examples/MANUAL > $@
	mv examples/MANUAL.html $(BINARY)_manual.html

md2roff:
	go get $(MD2ROFF_BIN)

# TODO: provide a template that adds the date to the built html file.
readme: README.html
README.html: md2roff
	# This turns README.md into README.html
	go run $(MD2ROFF_BIN) --manual $(BINARY) --version $(VERSION) --date "$(DATE)" README.md

# Binaries

build: $(BINARY)
$(BINARY): *.go */*/*.go
	go build -o $(BINARY) -ldflags "-w -s -X $(VERSION_PATH)=$(VERSION)-$(ITERATION)"

macos: $(BINARY).amd64.macos
$(BINARY).amd64.macos: *.go */*/*.go
	# Building darwin 64-bit x86 binary.
	GOOS=darwin GOARCH=amd64 go build -o $@ -ldflags "-w -s -X $(VERSION_PATH)=$(VERSION)-$(ITERATION)"

# This builds a Homebrew formula file that can be used to install this app from source.
# The source used comes from the released version on GitHub. This will not work with local source.
# This target is used by Travis CI to update the released Forumla when a new tag is created.
formula: $(BINARY).rb
v$(VERSION).tar.gz.sha256:
	# Calculate the SHA from the Github source file.
	curl -sL $(URL)/archive/v$(VERSION).tar.gz | openssl dgst -r -sha256 | tee $@
$(BINARY).rb: v$(VERSION).tar.gz.sha256 init/homebrew/$(FORMULA).rb.tmpl
	# Creating formula from template using sed.
	sed -e "s/{{Version}}/$(VERSION)/g" \
		-e "s/{{Iter}}/$(ITERATION)/g" \
		-e "s/{{SHA256}}/$(shell head -c64 $<)/g" \
		-e "s/{{Desc}}/$(DESC)/g" \
		-e "s%{{URL}}%$(URL)%g" \
		-e "s%{{IMPORT_PATH}}%$(IMPORT_PATH)%g" \
		-e "s%{{SOURCE_PATH}}%$(SOURCE_PATH)%g" \
		-e "s%{{CONFIG_FILE}}%$(CONFIG_FILE)%g" \
		-e "s%{{Class}}%$(shell echo $(BINARY) | perl -pe 's/(?:\b|-)(\p{Ll})/\u$$1/g')%g" \
		init/homebrew/$(FORMULA).rb.tmpl | tee $(BINARY).rb
# That perl line turns hello-world into HelloWorld, etc.

# Extras

# Run code tests and lint.
test: lint
	# Testing.
	go test -race -covermode=atomic ./...
lint:
	# Checking lint.
	golangci-lint run $(GOLANGCI_LINT_ARGS)

# This is safe; recommended even.
dep: vendor
vendor: Gopkg.*
	dep ensure --vendor-only

# Don't run this unless you're ready to debug untested vendored dependencies.
deps:
	dep ensure --update

# Homebrew stuff. macOS only.

# Used for Homebrew only. Other distros can create packages.
install: man readme $(BINARY)
	@echo -  Done Building!  -
	@echo -  Local installation with the Makefile is only supported on macOS.
	@echo If you wish to install the application manually on Linux, check out the wiki: https://$(SOURCE_URL)/wiki/Installation
	@echo -  Otherwise, build and install a package: make rpm -or- make deb
	@echo See the Package Install wiki for more info: https://$(SOURCE_URL)/wiki/Package-Install
	@[ "$(shell uname)" = "Darwin" ] || (echo "Unable to continue, not a Mac." && false)
	@[ "$(PREFIX)" != "" ] || (echo "Unable to continue, PREFIX not set. Use: make install PREFIX=/usr/local ETC=/usr/local/etc" && false)
	@[ "$(ETC)" != "" ] || (echo "Unable to continue, ETC not set. Use: make install PREFIX=/usr/local ETC=/usr/local/etc" && false)
	# Copying the binary, config file, unit file, and man page into the env.
	/usr/bin/install -m 0755 -d $(PREFIX)/bin $(PREFIX)/share/man/man1 $(ETC)/$(BINARY) $(PREFIX)/share/doc/$(BINARY)
	/usr/bin/install -m 0755 -cp $(BINARY) $(PREFIX)/bin/$(BINARY)
	/usr/bin/install -m 0644 -cp $(BINARY).1.gz $(PREFIX)/share/man/man1
	/usr/bin/install -m 0644 -cp examples/$(CONFIG_FILE).example $(ETC)/
	[ -f $(ETC)/$(CONFIG_FILE) ] || /usr/bin/install -m 0644 -cp  examples/$(CONFIG_FILE).example $(ETC)/$(CONFIG_FILE)
	/usr/bin/install -m 0644 -cp LICENSE *.html examples/* $(PREFIX)/share/doc/$(BINARY)/
