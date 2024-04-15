
CRS_URL := https://github.com/coreruleset/coreruleset/archive/refs/tags/v4.1.0.tar.gz
CRS_FILE := v4.1.0.tar.gz
CRS_DIR := ./crs

prepare:
	@test -f $(CRS_FILE) || wget $(CRS_URL) -O $(CRS_FILE)
	@if [ ! -d "$(CRS_DIR)" ]; then \
    		mkdir $(CRS_DIR); \
    		tar -xzvf $(CRS_FILE) --strip-components 1 -C $(CRS_DIR); \
    fi

clean:
	rm -rf $(CRS_DIR)
	rm $(CRS_FILE)
rm: clean

up: prepare
	docker-compose up -d --force-recreate
	docker logs -f api-firewall
start: up
run: up

down:
	docker-compose down
stop: down

.PHONY: prepare clean up start run down stop rm