# https://makefiletutorial.com/
SHELL=/bin/bash

up:
	@docker-compose up -d --force-recreate
start: up
run: up

down:
	@docker-compose down
stop: down

log: logs
logs: follow
follow:
	@docker-compose logs -f

.PHONY: up start run down stop follow
