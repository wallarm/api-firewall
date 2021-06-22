# https://makefiletutorial.com/
SHELL=/bin/bash

include .env

build:
	@docker-compose build --parallel --compress

rebuild:
	@docker-compose build --parallel --compress --no-cache

start:
	@docker-compose up --no-start
	@docker-compose start kubernetes
	@docker-compose start dns
	@docker-compose exec -w /root kubernetes bash -c 'cat $${CLUSTER_MANIFEST_TEMPLATE} | envsubst > $${KIND_CLUSTER_MANIFEST}'
	@docker-compose exec -w /root kubernetes routines.py create
	@docker-compose start
	@make backend $(MAKEFLAGS)
	@make chart $(MAKEFLAGS)

stop:
	@docker-compose down

exec bash shell:
	@docker-compose exec -w /root kubernetes bash

backend:
	@docker-compose exec -w /root kubernetes bash -c "set +x; cd ./init-backend; ls -1 | egrep '.(yml|yaml)$$' | sort | xargs -L1 kubectl create -f || true"

chart:
	@docker-compose exec -w /root kubernetes bash -c "set +x; helm install api-firewall ./chart/. -f ./helm/api-firewall.yaml"
