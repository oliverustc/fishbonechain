import { readFileSync } from "node:fs";

const DEFAULT_PATH = "scripts/profiles/chains.json";

export function loadProfiles(path = DEFAULT_PATH) {
  const raw = JSON.parse(readFileSync(path, "utf8"));
  if (!raw.trade_profiles || typeof raw.trade_profiles !== "object") {
    throw new Error(`missing trade_profiles in ${path}`);
  }
  return raw.trade_profiles;
}

export function loadTradeProfile(id, path = DEFAULT_PATH) {
  const profiles = loadProfiles(path);
  const profile = profiles[id];
  if (!profile) {
    throw new Error(`unknown trade profile: ${id}`);
  }
  const required = ["chain", "main_ws", "child_ws", "settlement_mode", "verifier_mode"];
  for (const key of required) {
    if (!profile[key]) throw new Error(`trade profile ${id} missing ${key}`);
  }
  if (!profile.proof || typeof profile.proof !== "object") {
    throw new Error(`trade profile ${id} missing proof config`);
  }
  for (const key of ["system", "constraint_kind", "ro_depth"]) {
    if (!profile.proof[key]) throw new Error(`trade profile ${id} missing proof.${key}`);
  }
  if (!Number.isInteger(profile.proof.ro_depth) || profile.proof.ro_depth <= 0) {
    throw new Error(`trade profile ${id} proof.ro_depth must be a positive integer`);
  }
  return { id, ...profile };
}

export function parseProfileArg(argv = process.argv) {
  const idx = argv.indexOf("--profile");
  return idx === -1 ? null : argv[idx + 1];
}
