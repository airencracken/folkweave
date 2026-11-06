.PHONY: build image test test-js test-browser test-race test-mutations test-container check demo

build:
	CGO_ENABLED=0 go build -trimpath -o dist/folkweave ./cmd/folkweave

image: export GOOS=linux
image: build
	docker build -t folkweave:dev .

test-container: image
	python3 scripts/test-container.py

test:
	go test ./...

test-js:
	node --test internal/folkweave/static/app.test.cjs

test-browser: build
	npm --prefix scripts/browser test

test-race:
	go test -race ./...

test-mutations:
	python3 tools/mutate.py --jobs 2 mutations/core.json

check:
	python3 scripts/test_project.py
	go run ./tools/check-format
	go vet ./...
	$(MAKE) test test-js

demo: build
	./dist/folkweave demo
