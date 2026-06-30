#!/usr/bin/env node

import http from 'node:http';
import { JsonStore } from './lib/json_store.js';
import { AuthService } from './lib/auth.js';
import { setupRoutes } from './lib/routes.js';

const USAGE = `
Usage: node scripts/platform-backend/server.js [options]

Options:
  --host <addr>   Bind address (default: 127.0.0.1)
  --port <n>      Port number; use 0 for an ephemeral port (default: 3000)
  --data-dir <p>  File-backed store directory (required)
  --help          Print this help and exit
`.trim();

function parseArgs() {
  const args = process.argv.slice(2);
  const opts = { host: '127.0.0.1', port: 3000, dataDir: null };
  for (let i = 0; i < args.length; i++) {
    if (args[i] === '--help') {
      console.log(USAGE);
      process.exit(0);
    } else if (args[i] === '--host' && i + 1 < args.length) {
      opts.host = args[++i];
    } else if (args[i] === '--port' && i + 1 < args.length) {
      opts.port = parseInt(args[++i], 10);
    } else if (args[i] === '--data-dir' && i + 1 < args.length) {
      opts.dataDir = args[++i];
    }
  }
  return opts;
}

function main() {
  const opts = parseArgs();

  if (!opts.dataDir) {
    console.error('Error: --data-dir is required');
    console.log(USAGE);
    process.exit(1);
  }

  const store = new JsonStore(opts.dataDir);
  store.init();
  store.auth = new AuthService(store);

  const handle = setupRoutes(store);

  const server = http.createServer(async (req, res) => {
    await handle(req, res);
  });

  server.listen(opts.port, opts.host, () => {
    const addr = server.address();
    const url = `http://${addr.address}:${addr.port}`;
    console.log(`platform-backend listening on ${url}`);
    console.log(`data-dir: ${opts.dataDir}`);

    if (opts.port === 0) {
      console.log(url);
    }
  });

  server.on('error', (err) => {
    console.error('server error:', err.message);
    process.exit(1);
  });
}

main();
