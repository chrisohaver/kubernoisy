DOCKER:=chrisohaver
all:
	docker buildx use kubernoisy-builder || docker buildx create --use --name kubernoisy-builder
	docker buildx build -t $(DOCKER)/kubernoisy --platform=linux/amd64 . --push