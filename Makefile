APPNAME = starghlist
APPVER = 1.0
LDFLAGS = "-s -w -X 'main.version=$(APPVER)'"

.PHONY: all

build: main.main
all: clean build dist

%.main: main.go
	GOOS=linux GOARCH=amd64 go build $(BULD_FLAGS) -ldflags $(LDFLAGS) -o $(APPNAME)-$(APPVER)-linux-amd64
	GOOS=windows GOARCH=amd64 go build $(BULD_FLAGS) -ldflags $(LDFLAGS) -o $(APPNAME)-$(APPVER)-windows-amd64.exe
	GOOS=darwin GOARCH=arm64 go build -ldflags $(LDFLAGS) -o $(APPNAME)-$(APPVER)-darwin-aarch64

.PHONY: dist
dist:
	@test -d dist || mkdir -p dist/
	@mv $(APPNAME)-* dist/
	@cp README.md dist/

.PHONY: clean
clean:
	@-rm -f $(APPNAME)-*
	-@test -d dist && rm -f dist/$(APPNAME)-*
	-@test -d dist && rm -f dist/README.md
	-@test -d dist && rmdir dist
