import { describe, expect, it } from 'vitest';
import { checkHasPermission, normalizeRoleSlug, PermissionSlug, UserRole } from './roles';

describe('roles & permissions mapping', () => {
  it('maps backend organization role slugs accurately', () => {
    expect(normalizeRoleSlug('owner')).toBe(UserRole.OWNER);
    expect(normalizeRoleSlug('admin')).toBe(UserRole.ADMIN);
    expect(normalizeRoleSlug('developer')).toBe(UserRole.DEVELOPER);
    expect(normalizeRoleSlug('agent_manager')).toBe(UserRole.AGENT_MANAGER);
    expect(normalizeRoleSlug('finops_manager')).toBe(UserRole.FINOPS_MANAGER);
    expect(normalizeRoleSlug('auditor')).toBe(UserRole.AUDITOR);
    expect(normalizeRoleSlug('viewer')).toBe(UserRole.VIEWER);
  });

  it('matches canonical and legacy PermissionSlug aliases', () => {
    const backendPerms = ['credential:create', 'routing:read', 'governance:update'];
    expect(checkHasPermission(UserRole.DEVELOPER, backendPerms, 'credential:create')).toBe(true);
    expect(checkHasPermission(UserRole.DEVELOPER, backendPerms, 'credentials:write')).toBe(true);
    expect(checkHasPermission(UserRole.DEVELOPER, backendPerms, PermissionSlug.POLICIES_READ)).toBe(true);
    expect(checkHasPermission(UserRole.DEVELOPER, backendPerms, 'governance:manage')).toBe(true);
    expect(checkHasPermission(UserRole.VIEWER, ['logs:read'], 'credential:create')).toBe(false);
    expect(checkHasPermission(UserRole.OWNER, [], 'credential:create')).toBe(true);
    // DEF-012 & DEF-008: update does not grant create, and unseeded admin has no bypass.
    expect(checkHasPermission(UserRole.DEVELOPER, ['model:update'], 'model:create')).toBe(false);
    expect(checkHasPermission(UserRole.DEVELOPER, ['credential:update'], 'credential:create')).toBe(false);
    expect(checkHasPermission(UserRole.ADMIN, [], 'credential:create')).toBe(false);
  });
});
