let counter = 0;

function uniqueId(): string {
  counter++;
  return `${Date.now().toString(36)}${counter.toString(36)}`;
}

export function uniqueProjectKey(): string {
  return `test-proj-${uniqueId()}`;
}

export function uniqueFlagKey(): string {
  return `test-flag-${uniqueId()}`;
}

export function uniqueEmail(): string {
  return `test-${uniqueId()}@example.com`;
}

/** Fixed admin credentials used across all smoke tests. */
export const ADMIN_EMAIL = 'admin@e2e-test.com';
export const ADMIN_PASSWORD = 'TestPassword123!';
