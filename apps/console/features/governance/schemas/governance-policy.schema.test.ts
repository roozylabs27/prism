import { describe, expect, it } from 'vitest';
import { governancePolicySchema } from './governance-policy.schema';
import { toolSchema } from '../../tools/schemas/create-tool.schema';
import { AppRoutes } from '@/constants/routes';

describe('Wave 5 HITL Approval Contracts', () => {
  it('accepts require_approval effect in governance policy schema', () => {
    const parsed = governancePolicySchema.parse({
      name: 'Require Approval on Prod Deploy',
      effect: 'require_approval',
      toolPattern: 'deploy_*',
    });
    expect(parsed.effect).toBe('require_approval');
  });

  it('supports requireApproval flag in tool schema', () => {
    const defaults = toolSchema.parse({ name: 'read_status' });
    expect(defaults.requireApproval).toBe(false);

    const enabled = toolSchema.parse({ name: 'wipe_cache', requireApproval: true });
    expect(enabled.requireApproval).toBe(true);
  });

  it('exposes canonical approvals route constant', () => {
    expect(AppRoutes.APPROVALS).toBe('/approvals');
  });
});
