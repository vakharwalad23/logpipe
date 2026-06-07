.PHONY: up down logs load-traffic load-synthetic

up:
	docker compose up -d

down:
	docker compose down

logs:
	docker compose logs -f

load-traffic:
	go run ./cmd/loadgen -mode traffic -rate 200 -concurrency 50 -duration 30s

load-synthetic:
	docker compose run --rm loadgen -mode synthetic -stream app -count 1000000
