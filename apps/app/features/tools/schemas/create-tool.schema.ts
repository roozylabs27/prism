import { z } from 'zod';

export const backendSchema = z.object({
  id: z.string().optional(),
  name: z.string().min(1, 'Backend name is required'),
  endpointUrl: z
    .string()
    .min(1, 'Endpoint URL is required')
    .url('Must be a valid URL')
    .refine(
      (v) => v.startsWith('http://') || v.startsWith('https://'),
      'Endpoint URL must start with http:// or https://'
    ),
  authToken: z.string().optional().default(''),
  timeoutMs: z.coerce.number().int().positive().optional(),
  priority: z.coerce.number().int().positive().optional(),
});

export const toolSchema = z.object({
  name: z.string().min(1, 'Function name is required'),
  displayName: z.string().default(''),
  description: z.string().default(''),
  enabled: z.boolean().default(true),
  inputSchema: z
    .string()
    .default('{}')
    .refine((value) => {
      try {
        JSON.parse(value);
        return true;
      } catch {
        return false;
      }
    }, 'Input schema must be valid JSON'),
  backends: z.array(backendSchema).default([]),
});

export type ToolFormValues = z.infer<typeof toolSchema>;
export const createToolSchema = toolSchema;
export type CreateToolFormValues = ToolFormValues;
