import { describe, expect, it, vi } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';

const mutateSpy = vi.fn();

vi.mock('@/hooks/queries/useBudgetsQuery', () => ({
  useCreateBudget: () => ({
    mutate: mutateSpy,
    isPending: false,
  }),
}));

vi.mock('sonner', () => ({ toast: { success: vi.fn(), error: vi.fn() } }));

import { BudgetFormSheet } from './BudgetFormSheet';

describe('BudgetFormSheet (DEF-001)', () => {
  it('converts UI percentage thresholds (80 / 95) into 0..1 fractions for the API', async () => {
    mutateSpy.mockReset();
    const user = userEvent.setup();
    render(<BudgetFormSheet open={true} onOpenChange={() => {}} />);

    await user.type(screen.getByPlaceholderText('e.g., Organization Monthly Cap'), 'Smoke Monthly Budget');
    await user.type(screen.getByPlaceholderText('1000.00'), '50');
    await user.click(screen.getByRole('button', { name: 'Create Budget' }));

    await waitFor(() => expect(mutateSpy).toHaveBeenCalledTimes(1));
    const [payload] = mutateSpy.mock.calls[0];
    expect(payload).toMatchObject({
      name: 'Smoke Monthly Budget',
      monthlyLimit: 50,
      dailyLimit: 0,
      hardLimit: true,
      warningThreshold: 0.8,
      criticalThreshold: 0.95,
      enabled: true,
    });
  });
});
