.PHONY: format lint build-web

format:
	go fmt ./...
	biome format ./pkg/wasm

lint:
	go vet ./...
	biome lint ./pkg/wasm

build-web:
	mkdir -p build
	tinygo build -o build/lcp.wasm -target wasm ./pkg/wasm/wasm.go
	cp pkg/wasm/{index.html,main.js,styles.css} $$(tinygo env TINYGOROOT)/targets/wasm_exec.js ./build/
