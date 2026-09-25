.PHONY: build frontend install test vet clean

PREFIX ?= /usr/local

build: frontend
	go build -o plist ./tui
	go build -o plist-wui ./service

frontend:
	$(MAKE) -wC frontend build

install: build
	install -d $(DESTDIR)$(PREFIX)/bin
	install -m 755 plist $(DESTDIR)$(PREFIX)/bin/plist
	install -m 755 plist-wui $(DESTDIR)$(PREFIX)/bin/plist-wui

test:
	go test ./...

vet:
	go vet ./...

clean:
	rm -f plist plist-wui
	rm -rf frontend/dist
