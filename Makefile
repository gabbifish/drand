USER := gabbi
NAME := drand
VERSION := 2018.5.1-test

.PHONY: build-container
build-container:
	docker build -f Dockerfile -t docker-registry.cfdata.org/u/${USER}/${NAME}:${VERSION} .

.PHONY: publish-container
publish-container: build-container
	docker push docker-registry.cfdata.org/u/${USER}/${NAME}:${VERSION}

.PHONY: deploy
deploy: publish-container
	kubectl -n drand apply -f kubernetes.yaml
