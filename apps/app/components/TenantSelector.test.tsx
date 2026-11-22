import { QueryClient } from '@tanstack/react-query';
import { render } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import { purgeTenantQueries } from './TenantSelector';
import { usePermission } from './PermissionProvider';
import { UserRole } from '@/types/roles';

function PermissionProbe({ onCapture }: { onCapture: (p: ReturnType<typeof usePermission>) => void }) {
  onCapture(usePermission());
  return null;
}

describe('Tenant cache purge & fail-closed PermissionContext', () => {
  it('purgeTenantQueries removes tenant-scoped queries while preserving global organizations and feature-flags', () => {
    const qc = new QueryClient();
    qc.setQueryData(['organizations'], [{ id: 'org_1', name: 'Org 1' }]);
    qc.setQueryData(['feature-flags'], { billing: true });
    qc.setQueryData(['dashboard-stats', 'org_1', 'all', 'all', '30d'], { totalRequests: 42 });
    qc.setQueryData(['credentials', 'org_1', 'all', 'all', 'openai'], [{ id: 'cred_1' }]);
    qc.setQueryData(['request-logs', 'org_1', 'all', 'all'], { data: [] });

    purgeTenantQueries(qc);

    expect(qc.getQueryData(['organizations'])).toBeDefined();
    expect(qc.getQueryData(['feature-flags'])).toBeDefined();
    expect(qc.getQueryData(['dashboard-stats', 'org_1', 'all', 'all', '30d'])).toBeUndefined();
    expect(qc.getQueryData(['credentials', 'org_1', 'all', 'all', 'openai'])).toBeUndefined();
    expect(qc.getQueryData(['request-logs', 'org_1', 'all', 'all'])).toBeUndefined();
  });

  it('PermissionContext defaults fail-closed to VIEWER with hasPermission() === false when unmounted', () => {
    let captured: ReturnType<typeof usePermission> | null = null;
    render(<PermissionProbe onCapture={(p) => { captured = p; }} />);
    expect(captured).not.toBeNull();
    expect(captured!.role).toBe(UserRole.VIEWER);
    expect(captured!.hasPermission('credential:delete')).toBe(false);
    expect(captured!.hasPermission('organization:update')).toBe(false);
  });
});
