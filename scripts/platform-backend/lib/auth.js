import crypto from 'node:crypto';
import { generateId, nowISO } from './ids.js';

export function hashPassword(password) {
  const salt = crypto.randomBytes(16).toString('hex');
  const hash = crypto.createHash('sha256').update(salt + password).digest('hex');
  return `${salt}:${hash}`;
}

export function verifyPassword(password, stored) {
  const [salt, hash] = stored.split(':');
  const computed = crypto.createHash('sha256').update(salt + password).digest('hex');
  return computed === hash;
}

export function createSessionToken() {
  return crypto.randomBytes(32).toString('hex');
}

export class AuthService {
  constructor(store) {
    this.store = store;
  }

  register(display_name, role, password) {
    const existing = this.store.findOne('users', u => u.display_name === display_name);
    if (existing) {
      return { error: 'display_name already taken', status: 409 };
    }

    const validRoles = ['data_owner', 'data_requester', 'verifier', 'admin'];
    if (!validRoles.includes(role)) {
      return { error: `invalid role: ${role}`, status: 400 };
    }

    const user = {
      user_id: generateId(),
      display_name,
      role,
      password_hash: hashPassword(password),
      created_at: nowISO(),
      updated_at: nowISO(),
    };
    this.store.create('users', user);

    const { password_hash: _, ...safe } = user;
    return { user: safe };
  }

  login(display_name, password) {
    const user = this.store.findOne('users', u => u.display_name === display_name);
    if (!user) {
      return { error: 'invalid credentials', status: 401 };
    }

    if (!verifyPassword(password, user.password_hash)) {
      return { error: 'invalid credentials', status: 401 };
    }

    const token = createSessionToken();
    this.store.create('sessions', {
      token,
      user_id: user.user_id,
      created_at: nowISO(),
    });

    const { password_hash: _, ...safe } = user;
    return { user: safe, token };
  }

  authenticate(token) {
    if (!token) return null;
    const session = this.store.findOne('sessions', s => s.token === token);
    if (!session) return null;
    const user = this.store.findOne('users', u => u.user_id === session.user_id);
    if (!user) return null;
    const { password_hash: _, ...safe } = user;
    return safe;
  }
}
