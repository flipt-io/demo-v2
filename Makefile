DOCKER_COMPOSE_CMD ?= docker compose

.PHONY: build
build:
	$(DOCKER_COMPOSE_CMD) build --pull --no-cache
	@echo ""
	@echo "Done."

.PHONY: start
start:
	$(DOCKER_COMPOSE_CMD) up --force-recreate --remove-orphans --detach
	$(DOCKER_COMPOSE_CMD) exec --user git  -t gitea  bash -c 'gitea admin regenerate hooks'

	@echo ""
	@echo ""
	@echo ""
	@echo "Demo is running. Use admin:password as auth credentials"
	@echo "Go to http://localhost:8080 for the Flipt UI."
	@echo "Go to http://localhost:3000 for the Gitea."
	@echo "Go to http://localhost:16686 for the Jaeger UI."
	@echo "Go to http://localhost:9090 for the Prometheus UI."


.PHONY: stop
stop:
	$(DOCKER_COMPOSE_CMD) down --remove-orphans --volumes
	@echo ""
	@echo "Demo is stopped."

