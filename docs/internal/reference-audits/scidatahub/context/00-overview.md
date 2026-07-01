# SciDataHub Deterministic Context

- generated_at: 2026-07-01T16:45:26+08:00
- reference_repo: references/SciDataHub
- audit_dir: docs/internal/reference-audits/scidatahub

## Git
- branch: dev
- commit: d75aebcddd87
- status:

## Package Scripts
### package.json
{
  "name": null,
  "scripts": {},
  "dependencies": [
    "uuid"
  ],
  "devDependencies": []
}

### backend/package.json
{
  "name": "backend",
  "scripts": {
    "test": "node --experimental-vm-modules node_modules/jest/bin/jest.js",
    "test:watch": "node --experimental-vm-modules node_modules/jest/bin/jest.js --watch",
    "test:manual": "node test/blockchainList.test.mjs",
    "start": "node src/app.mjs",
    "ipfs": "node test/ipfs_test.mjs",
    "chaincode": "node test/chaincode_test.mjs",
    "log": "node test/log_test.mjs"
  },
  "dependencies": [
    "@grpc/grpc-js",
    "@hyperledger/fabric-gateway",
    "body-parser",
    "cors",
    "express",
    "kubo-rpc-client",
    "multer",
    "sqlite3"
  ],
  "devDependencies": [
    "@eslint/js",
    "eslint",
    "globals",
    "jest",
    "supertest"
  ]
}

### frontend/package.json
{
  "name": "vue-material-kit-2",
  "scripts": {
    "dev": "vite",
    "build": "vite build",
    "preview": "vite preview --port 4173",
    "lint": "eslint . --ext .vue,.js,.jsx,.cjs,.mjs --fix --ignore-path .gitignore"
  },
  "dependencies": [
    "@popperjs/core",
    "axios",
    "bootstrap",
    "crypto-js",
    "elliptic",
    "pinia",
    "prismjs",
    "typed.js",
    "vue",
    "vue-clipboard3",
    "vue-count-to",
    "vue-prism-editor",
    "vue-router"
  ],
  "devDependencies": [
    "@rushstack/eslint-patch",
    "@vitejs/plugin-vue",
    "@vue/eslint-config-prettier",
    "eslint",
    "eslint-plugin-vue",
    "prettier",
    "sass",
    "sass-loader",
    "vite"
  ]
}

### caliper/package.json
{
  "name": "caliper",
  "scripts": {
    "test": "echo \"Error: no test specified\" && exit 1"
  },
  "dependencies": [
    "@grpc/grpc-js",
    "@hyperledger/caliper-cli",
    "@hyperledger/fabric-gateway"
  ],
  "devDependencies": []
}

