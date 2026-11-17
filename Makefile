.PHONY: run test generate fetch build
run:
	go run ./cmd/server
build:
	mkdir -p bin
	go build -o bin/ecoli-server ./cmd/server
generate:
	python3 tools/generate_scene.py
	python3 tools/generate_demo_trace.py
test:
	go test ./...
	python3 -m unittest discover -s tests -v
fetch:
	python3 tools/fetch_structures.py 4YBB
