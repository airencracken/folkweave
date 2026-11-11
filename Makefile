.PHONY: build image test test-js test-browser test-race test-mutations test-container check demo

build:
	CGO_ENABLED=0 go build -trimpath -ldflags "-X main.version=$$(cat VERSION)" -o dist/folkweave ./cmd/folkweave

image: export GOOS=linux
image: build
	docker build -t folkweave:dev .

test-container: image
	python3 scripts/containers/lifecycle.py --image folkweave:dev --version development

test:
	go test ./...

test-js:
	node --test internal/folkweave/static/app.test.cjs

test-browser: build
	npm --prefix scripts/browser test

test-race:
	go test -race ./...

test-mutations:
	python3 tools/mutate.py --jobs 2 mutations/core.json mutations/previews.json

check: test-suite-assets test-config test-cli
	python3 -m unittest discover -s scripts/containers -p "test_*.py"
	python3 scripts/release/test_config.py
	python3 scripts/test_project.py
	go run ./tools/check-format
	go vet ./...
	$(MAKE) test test-js

demo: build
	./dist/folkweave demo

.PHONY: test-containers test-container-registry
test-containers:
	python3 -m unittest discover -s scripts/containers -p 'test_*.py'
test-container-registry:
	python3 scripts/containers/check_registry.py

.PHONY: test-cli
test-cli: build
	python3 "$$(go list -m -f '{{.Dir}}' github.com/airencracken/comfylib)/tools/check_password_cli.py" --binary dist/folkweave --command create-owner --data-env FOLKWEAVE_DATA_DIR --prompt "Password: " --confirmation "Confirm password: "
	python3 "$$(go list -m -f '{{.Dir}}' github.com/airencracken/comfylib)/tools/check_setup.py" --binary dist/folkweave --app folkweave

.PHONY: test-config
test-config:
	python3 scripts/test_service_config.py

.PHONY: test-suite-assets
test-suite-assets: ## Verify synchronized suite source and assets
	python3 scripts/verify_suite.py
	python3 scripts/test_suite_deployment.py
