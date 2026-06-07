.PHONY: up down down-v logs load-traffic load-synthetic

STREAM ?= app
COUNT ?= 1000000

up:
	docker compose up -d

down:
	docker compose down

down-v:
	docker compose down -v

logs:
	docker compose logs -f

load-traffic:
	go run ./cmd/loadgen -mode traffic -rate 200 -concurrency 50 -duration 30s

load-synthetic:
	docker compose run --rm loadgen -mode synthetic -stream $(STREAM) -count $(COUNT)
