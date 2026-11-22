import { z } from 'zod';

export const resourceBackendSchema = z.object({
  id: z.string().optional(),
  name: z.string().min(1, 'Backend name is required'),
  backendType: z.enum(['http', 'postgres', 's3', 'blob']).default('http'),
  endpointUrl: z.string().optional().default(''),
  httpMethod: z.string().optional().default('POST'),
  authToken: z.string().optional().default(''),
  connectionString: z.string().optional().default(''),
  queryTemplate: z.string().optional().default(''),
  sqlQuery: z.string().optional().default(''),
  timeoutMs: z.coerce.number().int().positive().optional(),
  priority: z.coerce.number().int().positive().optional(),
});

export const resourceSchema = z.object({
  name: z.string().min(1, 'Resource name is required'),
  displayName: z.string().default(''),
  description: z.string().default(''),
  enabled: z.boolean().default(true),
  parametersSchema: z
    .string()
    .default('{}')
    .refine((value) => {
      try {
        JSON.parse(value);
        return true;
      } catch {
        return false;
      }
    }, 'Parameters schema must be valid JSON'),
  backends: z.array(resourceBackendSchema).default([]),
});

export type ResourceFormValues = z.infer<typeof resourceSchema>;
export const createResourceSchema = resourceSchema;
export type CreateResourceFormValues = ResourceFormValues;
