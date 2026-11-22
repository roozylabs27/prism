export enum UserRole {
  OWNER = 'owner',
  ADMIN = 'admin',
  DEVELOPER = 'developer',
  AGENT_MANAGER = 'agent_manager',
  FINOPS_MANAGER = 'finops_manager',
  AUDITOR = 'auditor',
  MEMBER = 'member',
  VIEWER = 'viewer',
}

export enum PermissionSlug {
  CREDENTIALS_READ = 'credential:read',
  CREDENTIALS_WRITE = 'credential:create',
  MODELS_READ = 'model:read',
  MODELS_WRITE = 'model:create',
  POLICIES_READ = 'routing:read',
  POLICIES_WRITE = 'routing:update',
  GATEWAY_KEYS_WRITE = 'api_key:create',
  AUDIT_EXPORT = 'audit:export',
  BILLING_MANAGE = 'billing:manage',
  GOVERNANCE_MANAGE = 'governance:update',
}

const PERMISSION_ALIASES: Record<string, string[]> = {
  'credential:read': ['credentials:read'],
  'credentials:read': ['credential:read'],
  'credential:create': ['credentials:write', 'credential:update'],
  'credentials:write': ['credential:create', 'credential:update'],
  'model:read': ['models:read'],
  'models:read': ['model:read'],
  'model:create': ['models:write', 'model:update'],
  'models:write': ['model:create', 'model:update'],
  'routing:read': ['policies:read'],
  'policies:read': ['routing:read'],
  'routing:update': ['policies:write'],
  'policies:write': ['routing:update'],
  'api_key:create': ['gateway_keys:write'],
  'gateway_keys:write': ['api_key:create'],
  'governance:update': ['governance:manage', 'governance:create'],
  'governance:manage': ['governance:update', 'governance:create'],
};

export function normalizeRoleSlug(rawRole?: string): UserRole {
  switch ((rawRole || '').toLowerCase().trim()) {
    case 'owner':
      return UserRole.OWNER;
    case 'admin':
      return UserRole.ADMIN;
    case 'developer':
      return UserRole.DEVELOPER;
    case 'agent_manager':
      return UserRole.AGENT_MANAGER;
    case 'finops_manager':
      return UserRole.FINOPS_MANAGER;
    case 'auditor':
      return UserRole.AUDITOR;
    case 'viewer':
      return UserRole.VIEWER;
    default:
      return UserRole.MEMBER;
  }
}

export function checkHasPermission(role: UserRole, permissions: string[], requested: string): boolean {
  if (role === UserRole.OWNER || role === UserRole.ADMIN) return true;
  if (permissions.includes('*') || permissions.includes(requested)) return true;
  const aliases = PERMISSION_ALIASES[requested];
  return Boolean(aliases?.some((a) => permissions.includes(a)));
}
