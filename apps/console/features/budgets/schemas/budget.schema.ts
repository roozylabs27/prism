import { z } from 'zod';

const isPositiveFinite = (v: string) => {
  const n = Number(v);
  return Number.isFinite(n) && n > 0;
};

const isThresholdPct = (v: string) => {
  const n = Number(v);
  return Number.isFinite(n) && n > 0 && n <= 100;
};

export const budgetSchema = z
  .object({
    name: z.string().trim().min(1, 'Budget name is required'),
    monthlyLimit: z
      .string()
      .min(1, 'Monthly limit is required')
      .refine(isPositiveFinite, 'Monthly limit must be greater than 0'),
    dailyLimit: z
      .string()
      .default('')
      .refine((v) => v.trim() === '' || (Number.isFinite(Number(v)) && Number(v) >= 0), 'Daily limit must be 0 or greater'),
    hardLimit: z.boolean().default(true),
    warningThreshold: z
      .string()
      .default('80')
      .refine(isThresholdPct, 'Warning threshold must be between 1 and 100%'),
    criticalThreshold: z
      .string()
      .default('95')
      .refine(isThresholdPct, 'Critical threshold must be between 1 and 100%'),
  })
  .refine((data) => Number(data.warningThreshold) <= Number(data.criticalThreshold), {
    message: 'Warning threshold must not exceed critical threshold',
    path: ['warningThreshold'],
  });

export type BudgetFormValues = z.infer<typeof budgetSchema>;
