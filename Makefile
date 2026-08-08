.PHONY: install patch banner docs

install:
	go install ./cmd/tpd

docs:
	go run ./cmd/gen-catalog

patch:
	@git diff --quiet && git diff --cached --quiet || { echo "error: uncommitted changes; commit or stash before releasing"; exit 1; }
	@[ "$$(git branch --show-current)" = "main" ] || { echo "error: must be on main to release (on $$(git branch --show-current))"; exit 1; }
	version=$$(mise exec -- svu patch) && git tag "$$version" && git push && git push --tags
