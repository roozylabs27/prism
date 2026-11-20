import { z } from 'zod';

export const providerSchema = z.object({
  name: z.string().trim().min(1, 'Provider name is required'),
  type: z.string().trim().min(1, 'Provider type is required'),
  baseUrl: z
    .string()
    .trim()
    .min(1, 'Base URL is required')
    .url('Must be a valid URL')
    .refine(
      (val) => val.startsWith('http://') || val.startsWith('https://'),
      'Base URL must use http:// or https://'
    ),
  enabled: z.boolean().default(true),
});

export type ProviderFormValues = z.infer<typeof providerSchema>;
export const createProviderSchema = providerSchema;
export type CreateProviderFormValues = ProviderFormValues;
