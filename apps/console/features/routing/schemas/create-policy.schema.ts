import { z } from 'zod';

export const policySchema = z.object({
  name: z.string().trim().min(1, 'Policy name is required'),
  task_match: z.number().min(0).max(100).default(35),
  quality: z.number().min(0).max(100).default(35),
  cost: z.number().min(0).max(100).default(15),
  speed: z.number().min(0).max(100).default(15),
});

export type PolicyFormValues = z.infer<typeof policySchema>;
export const createPolicySchema = policySchema;
export type CreatePolicyFormValues = PolicyFormValues;
