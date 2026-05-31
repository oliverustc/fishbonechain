# sp-io 28.x (polkadot-sdk 2512) + Rust >= 1.82 需要此标志
# 待升级到修复版 SDK 后可移除
WASM_FLAGS := -C link-arg=--allow-undefined

.PHONY: build build-release build-node build-runtime check test clean

build:
	WASM_BUILD_RUSTFLAGS="$(WASM_FLAGS)" cargo build -p fishbone-node

build-release:
	WASM_BUILD_RUSTFLAGS="$(WASM_FLAGS)" cargo build --release -p fishbone-node

build-runtime:
	WASM_BUILD_RUSTFLAGS="$(WASM_FLAGS)" cargo build --release -p fishbone-runtime

build-node:
	SKIP_WASM_BUILD=1 cargo build --release -p fishbone-node

check:
	SKIP_WASM_BUILD=1 cargo check -p fishbone-node

test:
	WASM_BUILD_RUSTFLAGS="$(WASM_FLAGS)" cargo test

clean:
	cargo clean
