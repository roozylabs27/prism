import { z } from 'zod';

export const clientOnboardingSchema = z.object({
  clientName: z.string().trim().min(1, 'Client workspace name is required').max(80, 'Max 80 characters'),
  projectName: z.string().trim().min(1, 'Initial project name is required').max(80, 'Max 80 characters'),
  monthlyLimitUsd: z.coerce.number().min(0, 'Must be 0 or greater'),
  dailyLimitUsd: z.coerce.number().min(0, 'Must be 0 or greater'),
  dailyRequestLimit: z.coerce.number().int().min(0, 'Must be 0 or greater'),
  rateLimitRpm: z.coerce.number().int().min(1, 'Rate limit must be at least 1 RPM').max(100_000, 'Max 100,000 RPM'),
});

export type ClientOnboardingFormValues = z.infer<typeof clientOnboardingSchema>;
