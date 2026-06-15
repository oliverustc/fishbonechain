// This file is part of Substrate.

// Copyright (C) Parity Technologies (UK) Ltd.
// SPDX-License-Identifier: Apache-2.0

// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// 	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

use crate::{AccountId, BalancesConfig, ChainProfileConfig, RuntimeGenesisConfig, SudoConfig};
use alloc::{vec, vec::Vec};
use frame_support::build_struct_json_patch;
use pallet_chain_profile::{ChainProfileInfo, SceneKind, SettlementMode};
use serde_json::Value;
use sp_consensus_aura::sr25519::AuthorityId as AuraId;
use sp_consensus_grandpa::AuthorityId as GrandpaId;
use sp_genesis_builder::{self, PresetId};
use sp_keyring::Sr25519Keyring;

/// Default chain profile based on compile-time feature.
fn default_chain_profile() -> ChainProfileInfo {
	#[cfg(feature = "scene-data-trade")]
	{
		return ChainProfileInfo {
			chain_id: 6,
			scene: SceneKind::DataTrade,
			settlement: SettlementMode::MainEscrow,
			params_hash: Default::default(),
		};
	}

	#[cfg(feature = "scene-crowdsource")]
	{
		return ChainProfileInfo {
			chain_id: 0,
			scene: SceneKind::Crowdsource,
			settlement: SettlementMode::FmcTaskBill,
			params_hash: Default::default(),
		};
	}

	#[cfg(all(not(feature = "scene-data-trade"), not(feature = "scene-crowdsource")))]
	{
		ChainProfileInfo {
			chain_id: 0,
			scene: SceneKind::PlatformOnly,
			settlement: SettlementMode::None,
			params_hash: Default::default(),
		}
	}
}

// ── AURA genesis (default, non-babe builds) ─────────────────────────────────

#[cfg(not(feature = "babe"))]
fn testnet_genesis(
	initial_authorities: Vec<(AuraId, GrandpaId)>,
	endowed_accounts: Vec<AccountId>,
	root: AccountId,
) -> Value {
	build_struct_json_patch!(RuntimeGenesisConfig {
		balances: BalancesConfig {
			balances: endowed_accounts
				.iter()
				.cloned()
				.map(|k| (k, 1u128 << 60))
				.collect::<Vec<_>>(),
		},
		aura: pallet_aura::GenesisConfig {
			authorities: initial_authorities.iter().map(|x| x.0.clone()).collect::<Vec<_>>(),
		},
		grandpa: pallet_grandpa::GenesisConfig {
			authorities: initial_authorities.iter().map(|x| (x.1.clone(), 1)).collect::<Vec<_>>(),
		},
		chain_profile: ChainProfileConfig { profile: default_chain_profile() },
		sudo: SudoConfig { key: Some(root) },
	})
}

#[cfg(not(feature = "babe"))]
pub fn development_config_genesis() -> Value {
	testnet_genesis(
		vec![(
			sp_keyring::Sr25519Keyring::Alice.public().into(),
			sp_keyring::Ed25519Keyring::Alice.public().into(),
		)],
		vec![
			Sr25519Keyring::Alice.to_account_id(),
			Sr25519Keyring::Bob.to_account_id(),
			Sr25519Keyring::AliceStash.to_account_id(),
			Sr25519Keyring::BobStash.to_account_id(),
			Sr25519Keyring::Charlie.to_account_id(),
		],
		sp_keyring::Sr25519Keyring::Alice.to_account_id(),
	)
}

#[cfg(not(feature = "babe"))]
pub fn local_config_genesis() -> Value {
	testnet_genesis(
		vec![
			(
				sp_keyring::Sr25519Keyring::Alice.public().into(),
				sp_keyring::Ed25519Keyring::Alice.public().into(),
			),
			(
				sp_keyring::Sr25519Keyring::Bob.public().into(),
				sp_keyring::Ed25519Keyring::Bob.public().into(),
			),
		],
		Sr25519Keyring::iter()
			.filter(|v| v != &Sr25519Keyring::One && v != &Sr25519Keyring::Two)
			.map(|v| v.to_account_id())
			.collect::<Vec<_>>(),
		Sr25519Keyring::Alice.to_account_id(),
	)
}

// ── BABE genesis (babe feature) ─────────────────────────────────────────────

#[cfg(feature = "babe")]
use sp_consensus_babe::AuthorityId as BabeId;

#[cfg(feature = "babe")]
fn testnet_genesis(
	initial_authorities: Vec<(BabeId, GrandpaId)>,
	endowed_accounts: Vec<AccountId>,
	root: AccountId,
) -> Value {
	build_struct_json_patch!(RuntimeGenesisConfig {
		balances: BalancesConfig {
			balances: endowed_accounts
				.iter()
				.cloned()
				.map(|k| (k, 1u128 << 60))
				.collect::<Vec<_>>(),
		},
		// AURA pallet is always compiled in; empty authorities = no AURA consensus
		aura: pallet_aura::GenesisConfig { authorities: vec![] },
		babe: pallet_babe::GenesisConfig {
			authorities: initial_authorities.iter().map(|x| (x.0.clone(), 1)).collect::<Vec<_>>(),
			epoch_config: sp_consensus_babe::BabeEpochConfiguration {
				c: (1, 4),
				allowed_slots: sp_consensus_babe::AllowedSlots::PrimaryAndSecondaryVRFSlots,
			},
		},
		grandpa: pallet_grandpa::GenesisConfig {
			authorities: initial_authorities.iter().map(|x| (x.1.clone(), 1)).collect::<Vec<_>>(),
		},
		chain_profile: ChainProfileConfig { profile: default_chain_profile() },
		sudo: SudoConfig { key: Some(root) },
	})
}

#[cfg(feature = "babe")]
pub fn development_config_genesis() -> Value {
	use sp_core::crypto::ByteArray;
	testnet_genesis(
		vec![(
			BabeId::from_slice(&sp_keyring::Sr25519Keyring::Alice.public().0).expect("valid key"),
			sp_keyring::Ed25519Keyring::Alice.public().into(),
		)],
		vec![
			Sr25519Keyring::Alice.to_account_id(),
			Sr25519Keyring::Bob.to_account_id(),
			Sr25519Keyring::AliceStash.to_account_id(),
			Sr25519Keyring::BobStash.to_account_id(),
		],
		sp_keyring::Sr25519Keyring::Alice.to_account_id(),
	)
}

#[cfg(feature = "babe")]
pub fn local_config_genesis() -> Value {
	use sp_core::crypto::ByteArray;
	testnet_genesis(
		vec![
			(
				BabeId::from_slice(&sp_keyring::Sr25519Keyring::Alice.public().0)
					.expect("valid key"),
				sp_keyring::Ed25519Keyring::Alice.public().into(),
			),
			(
				BabeId::from_slice(&sp_keyring::Sr25519Keyring::Bob.public().0).expect("valid key"),
				sp_keyring::Ed25519Keyring::Bob.public().into(),
			),
		],
		Sr25519Keyring::iter()
			.filter(|v| v != &Sr25519Keyring::One && v != &Sr25519Keyring::Two)
			.map(|v| v.to_account_id())
			.collect::<Vec<_>>(),
		Sr25519Keyring::Alice.to_account_id(),
	)
}

// ── shared ───────────────────────────────────────────────────────────────────

/// Provides the JSON representation of predefined genesis config for given `id`.
pub fn get_preset(id: &PresetId) -> Option<Vec<u8>> {
	let patch = match id.as_ref() {
		sp_genesis_builder::DEV_RUNTIME_PRESET => development_config_genesis(),
		sp_genesis_builder::LOCAL_TESTNET_RUNTIME_PRESET => local_config_genesis(),
		_ => return None,
	};
	Some(
		serde_json::to_string(&patch)
			.expect("serialization to json is expected to work. qed.")
			.into_bytes(),
	)
}

/// List of supported presets.
pub fn preset_names() -> Vec<PresetId> {
	vec![
		PresetId::from(sp_genesis_builder::DEV_RUNTIME_PRESET),
		PresetId::from(sp_genesis_builder::LOCAL_TESTNET_RUNTIME_PRESET),
	]
}
