import { z } from 'zod';

const slugPattern = /^[a-z0-9]+(?:-[a-z0-9]+)*$/;

export const createWorkspaceSchema = z.object({
  name: z.string().trim().min(1, 'Workspace name is required').max(120, 'Max 120 characters'),
  slug: z
    .string()
    .trim()
    .max(64, 'Max 64 characters')
    .refine((val) => val === '' || slugPattern.test(val), 'Use lowercase letters, numbers, and hyphens only')
    .optional(),
});

export type CreateWorkspaceValues = z.infer<typeof createWorkspaceSchema>;

export const createProjectSchema = z.object({
  name: z.string().trim().min(1, 'Project name is required').max(120, 'Max 120 characters'),
  slug: z
    .string()
    .trim()
    .max(64, 'Max 64 characters')
    .refine((val) => val === '' || slugPattern.test(val), 'Use lowercase letters, numbers, and hyphens only')
    .optional(),
});

export type CreateProjectValues = z.infer<typeof createProjectSchema>;

export const renameEntitySchema = z.object({
  name: z.string().trim().min(1, 'Name cannot be empty').max(120, 'Max 120 characters'),
});

export type RenameEntityValues = z.infer<typeof renameEntitySchema>;
