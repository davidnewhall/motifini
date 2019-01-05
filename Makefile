# binary name.
NAME=motifini
# library folders. so they can be tested and linted.
LIBRARYS=./encode ./messages ./exp ./subscribe
# used for plist file name on macOS.
ID=pro.sleepers
CONF=motifini.conf

# dont change this one.
PACKAGES=`find ./cmd -mindepth 1 -maxdepth 1 -type d`

all: clean build test
	@echo Finished.

clean:
	@echo "Cleaning Local Build"
	for p in $(PACKAGES); do rm -f `echo $${p}|cut -d/ -f3`{,.1,.1.gz}; done

build:
	@echo "Building Binary"
	for p in $(PACKAGES); do go build -ldflags "-w -s" $${p}; done

linux:
	for p in $(PACKAGES); do GOOS=linux go build -ldflags "-w -s" $${p}; done

install:
	@echo "If you get errors, you may need sudo."
	GOBIN=/usr/local/bin go install -ldflags "-w -s" ./...
	mkdir -p /usr/local/etc/$(NAME) /usr/local/var/lib/$(NAME)
	test -f /usr/local/etc/$(NAME)/${CONF} || cp ${CONF}.example /usr/local/etc/$(NAME)/${CONF}
	test -d ~/Library/LaunchAgents && cp installparts/launchd/$(ID).$(NAME).plist ~/Library/LaunchAgents || true
	test -d ~/SecuritySpy/Scripts && cp installparts/MotifiniCameraEvents.scpt ~/SecuritySpy/Scripts || true
	test -d ~/"Library/Application Scripts/com.apple.iChat" && cp installparts/MotifiniHandler.applescript ~/"Library/Application Scripts/com.apple.iChat" || true
	test -d /etc/systemd/system && cp installparts/systemd/$(NAME).service /etc/systemd/system || true

uninstall:
	@echo "If you get errors, you may need sudo."
	test -f ~/Library/LaunchAgents/$(ID).$(NAME).plist && launchctl unload ~/Library/LaunchAgents/$(ID).$(NAME).plist || true
	test -f /etc/systemd/system/$(NAME).service && systemctl stop $(NAME) || true
	rm -f ~/Library/LaunchAgents/$(ID).$(NAME).plist
	rm -f /etc/systemd/system/$(NAME).service
	rm -f ~/"Library/Application Scripts/com.apple.iChat/MotifiniHandler.applescript"
	rm -f ~/SecuritySpy/Scripts/MotifiniCameraEvents.scpt
	rm -f /usr/local/bin/$(NAME)

test: lint
	@echo "Running Go Tests"
	for p in $(PACKAGES) $(LIBRARYS); do go test -race -covermode=atomic $${p}; done

# TODO: look into gometalinter
lint:
	@echo "Running Go Linters"
	goimports -l $(PACKAGES) $(LIBRARYS)
	gofmt -l $(PACKAGES) $(LIBRARYS)
	errcheck $(PACKAGES) $(LIBRARYS)
	golint $(PACKAGES) $(LIBRARYS)
	go vet $(PACKAGES) $(LIBRARYS)

deps:
	@echo "Gathering Vendors"
	dep ensure -update
	dep status
