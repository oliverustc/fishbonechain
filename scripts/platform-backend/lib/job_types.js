export const SUPPORTED_EXEC_TYPES = new Set(['proof_generation']);

export const KNOWN_TYPES = new Set([
  'proof_generation',
  'data_preprocessing',
  'anonymization',
  'verification',
  'training',
]);

export function isKnownType(jobType) {
  return KNOWN_TYPES.has(jobType);
}

export function isSupportedExecution(jobType) {
  return SUPPORTED_EXEC_TYPES.has(jobType);
}

export function validateProofGenerationInputs(inputRefs) {
  const byType = {};
  for (const ref of inputRefs) {
    if (!ref.artifact_type) continue;
    byType[ref.artifact_type] = ref;
  }
  const missing = [];
  for (const required of ['workflow_run', 'profile', 'dataset', 'request']) {
    if (!byType[required]) {
      missing.push(required);
    }
  }
  if (missing.length > 0) {
    return { valid: false, error: `missing required inputs: ${missing.join(', ')}` };
  }
  return { valid: true, byType };
}
