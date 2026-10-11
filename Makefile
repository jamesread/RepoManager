.PHONY: all generate build test lint run migrate service frontend dev-service dev-frontend integration-test docs clean install setup

all: build

generate:
	cd protocol/proto && buf generate --template ../buf.gen.yaml

DB_DRIVER ?= sqlite

migrate:
ifeq ($(DB_DRIVER),mysql)
	$(MAKE) -C database/mysql
else
	DB_PATH="$(CURDIR)/tmp/repomanager.db" $(MAKE) -C database/sqlite
endif

build: generate migrate service frontend

service:
	go build -o plist-wui ./service
	go build -o plist ./tui

frontend:
	$(MAKE) -wC frontend build

test: generate
	go test ./...
	$(MAKE) -wC frontend test

lint:
	go vet ./...
	$(MAKE) -wC frontend lint

run: service
	./plist-wui

dev-service: service
	REPOMANAGER_SECURE_COOKIES=false ./plist-wui -configdir var/config

dev-frontend:
	$(MAKE) -wC frontend run

install:
	cd frontend && npm install
	go mod download

setup: install generate migrate

integration-test: build
	cd integration-tests && npm install && node run-tests.js

docs:
	mkdocs build

clean:
	rm -f plist plist-wui
	rm -rf frontend/dist frontend/gen service/gen tmp/repomanager.db tmp/repomanager-test.db

PREFIX ?= /usr/local

install-bin: build
	install -d $(DESTDIR)$(PREFIX)/bin
	install -m 755 plist $(DESTDIR)$(PREFIX)/bin/plist
	install -m 755 plist-wui $(DESTDIR)$(PREFIX)/bin/plist-wui
