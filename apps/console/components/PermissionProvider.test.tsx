import { describe, it, expect, vi } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import { PermissionGuard, usePermission } from './PermissionProvider';
import { UserRole } from '@/types/roles';

function Probe() {
  const { role, hasPermission } = usePermission();
  return (
    <div>
      <span data-testid="role">{role}</span>
      <span data-testid="can-create">{String(hasPermission('model:create'))}</span>
    </div>
  );
}

describe('PermissionProvider & PermissionGuard', () => {
  it('fails closed to VIEWER and denies privileged permissions by default', () => {
    render(<Probe />);
    expect(screen.getByTestId('role').textContent).toBe(UserRole.VIEWER);
    expect(screen.getByTestId('can-create').textContent).toBe('false');
  });

  it('blocks click and keyboard activation when PermissionGuard denies action with disabledTooltip', () => {
    const onClick = vi.fn();
    render(
      <PermissionGuard permission="model:create" disabledTooltip="Requires permission">
        <button type="button" onClick={onClick}>
          Create Model
        </button>
      </PermissionGuard>
    );

    const guardTrigger = screen.getByRole('button', { name: 'Create Model', hidden: true }).closest('[aria-disabled="true"]');
    expect(guardTrigger).not.toBeNull();
    fireEvent.click(screen.getByText('Create Model'));
    if (guardTrigger) {
      fireEvent.keyDown(guardTrigger, { key: 'Enter' });
    }
    expect(onClick).not.toHaveBeenCalled();
  });
});
