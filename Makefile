.PHONY: build image test test-js test-browser test-race test-mutations test-container check demo

build:
	CGO_ENABLED=0 go build -trimpath -ldflags "-X main.version=$$(cat VERSION)" -o dist/folkweave ./cmd/folkweave

image: export GOOS=linux
image: build
	docker build -t folkweave:dev .

test-container: image
	python3 scripts/comfy.py container-lifecycle --image folkweave:dev --version development

test:
	go test ./...

test-js:
	node --test internal/folkweave/static/app.test.cjs internal/folkweave/static/theme.test.cjs

test-browser: build
	npm --prefix scripts/browser test

test-race:
	go test -race ./...

test-mutations:
	python3 scripts/comfy.py mutate --jobs 2 mutations/core.json mutations/previews.json

check: test-suite-assets test-config test-cli
	python3 scripts/comfy.py test-containers
	python3 scripts/release/test_config.py
	python3 scripts/test_project.py
	go run ./tools/check-format
	go vet ./...
	$(MAKE) test test-js

demo: build
	./dist/folkweave demo

.PHONY: test-containers test-container-registry
test-containers:
	python3 scripts/comfy.py test-containers
test-container-registry:
	python3 scripts/comfy.py container-registry

.PHONY: test-cli
test-cli: build
	python3 "$$(go list -m -f '{{.Dir}}' github.com/airencracken/comfylib)/tools/check_password_cli.py" --binary dist/folkweave --command create-owner --data-env FOLKWEAVE_DATA_DIR --prompt "Password: " --confirmation "Confirm password: "
	python3 "$$(go list -m -f '{{.Dir}}' github.com/airencracken/comfylib)/tools/check_setup.py" --binary dist/folkweave --app folkweave

.PHONY: test-config
test-config:
	python3 scripts/comfy.py service-config

.PHONY: test-suite-assets
test-suite-assets: ## Verify shared module integration and deployment contracts
	python3 scripts/comfy.py check
	python3 scripts/comfy.py suite-deployment
	python3 scripts/comfy.py release-dispatch

.PHONY: release-preflight test-metrics-browser
release-preflight:
	python3 scripts/comfy.py release-plan --require-clean --cold-cache --output .release/plan.json

test-metrics-browser:
	python3 scripts/comfy.py metrics-browser
