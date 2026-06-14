# sp-io 28.x (polkadot-sdk 2512) + Rust >= 1.82 需要此标志
# 待升级到修复版 SDK 后可移除
WASM_FLAGS := -C link-arg=--allow-undefined

.PHONY: build build-release build-node build-runtime build-main build-crowdsource-child build-data-trade-child build-2s build-1s build-10mb build-babe check test clean

build:
	WASM_BUILD_RUSTFLAGS="$(WASM_FLAGS)" cargo build -p fishbone-node

build-release:
	mkdir -p deploy/bin
	WASM_BUILD_RUSTFLAGS="$(WASM_FLAGS)" cargo build --release -p fishbone-node --features fishbone-runtime/role-main
	cp target/release/fishbone-node deploy/bin/fishbone-node

build-runtime:
	WASM_BUILD_RUSTFLAGS="$(WASM_FLAGS)" cargo build --release -p fishbone-runtime

build-node:
	SKIP_WASM_BUILD=1 cargo build --release -p fishbone-node

build-main:
	mkdir -p deploy/bin
	WASM_BUILD_RUSTFLAGS="$(WASM_FLAGS)" cargo build --release -p fishbone-node --features fishbone-runtime/role-main
	cp target/release/fishbone-node deploy/bin/fishbone-node

build-crowdsource-child:
	mkdir -p deploy/bin
	WASM_BUILD_RUSTFLAGS="$(WASM_FLAGS)" cargo build --release -p fishbone-node --features fishbone-runtime/scene-crowdsource
	cp target/release/fishbone-node deploy/bin/fishbone-node-crowdsource

build-data-trade-child:
	mkdir -p deploy/bin
	WASM_BUILD_RUSTFLAGS="$(WASM_FLAGS)" cargo build --release -p fishbone-node --features fishbone-runtime/scene-data-trade
	cp target/release/fishbone-node deploy/bin/fishbone-node-data-trade

# 快出块变体：编译后复制到 deploy/bin/
build-2s:
	mkdir -p deploy/bin
	WASM_BUILD_RUSTFLAGS="$(WASM_FLAGS)" cargo build --release -p fishbone-node --features fishbone-runtime/scene-crowdsource,fishbone-runtime/block-2s
	cp target/release/fishbone-node deploy/bin/fishbone-node-2s

build-1s:
	mkdir -p deploy/bin
	WASM_BUILD_RUSTFLAGS="$(WASM_FLAGS)" cargo build --release -p fishbone-node --features fishbone-runtime/scene-crowdsource,fishbone-runtime/block-1s
	cp target/release/fishbone-node deploy/bin/fishbone-node-1s

# 大区块变体（子链 3：医疗影像标注，10 MB 区块）
build-10mb:
	mkdir -p deploy/bin
	WASM_BUILD_RUSTFLAGS="$(WASM_FLAGS)" cargo build --release -p fishbone-node --features fishbone-runtime/scene-crowdsource,fishbone-runtime/block-10mb
	cp target/release/fishbone-node deploy/bin/fishbone-node-10mb

# BABE 共识版本（子链 6：去中心化数据市场）
# 与默认 binary 相同，但 chain spec 用 --chain child6-babe 启动
build-babe:
	mkdir -p deploy/bin
	WASM_BUILD_RUSTFLAGS="$(WASM_FLAGS)" cargo build --release -p fishbone-node --features fishbone-runtime/scene-crowdsource,fishbone-runtime/babe
	cp target/release/fishbone-node deploy/bin/fishbone-node-babe

check:
	SKIP_WASM_BUILD=1 cargo check -p fishbone-node

test:
	WASM_BUILD_RUSTFLAGS="$(WASM_FLAGS)" cargo test

clean:
	cargo clean
