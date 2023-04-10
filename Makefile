.DEFAULT_GOAL := help

##@ Help
.PHONY: help
help:  ## Display this help.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z0-9_-]+:.*?##/ { printf "  \033[36m%-25s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

##@ Commands
.PHONY: start
start:  ## Start docker stack
	docker-compose up -d --remove-orphans

.PHONY: start-clean
start-clean: clean start ## Stop, clean, rebuild, and start docker stack

.PHONY: clean
clean: ## Stop & clean docker stack
	docker-compose stop  && docker-compose rm -f && docker-compose build --no-cache --force-rm --parallel

.PHONY: stop
stop:  ## Stop docker stack
	docker-compose down

.PHONY: show-containers
show-containers: ## Show running container information
	docker ps

.PHONY: logs
logs:  ## Live tail logs of load test script
	docker logs -f fileserver-challenge-load_tester-1

.PHONY: stats
stats: ## Show container CPU / Memory / IO Utilization
	docker stats

.PHONY: load-test
load-test: ## Manually execute python load test. REQUIRES PYTHON INSTALLATION
	./load_test/run.sh