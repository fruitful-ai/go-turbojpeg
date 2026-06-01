.PHONY: test tag

test:
	go test ./...

tag:
	# Using: https://github.com/caarlos0/svu
	git tag $$(svu next)
