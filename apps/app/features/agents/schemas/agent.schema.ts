import { z } from 'zod';

export const agentSchema = z.object({
  name: z.string().min(1, 'Agent name is required'),
  displayName: z.string().default(''),
  description: z.string().default(''),
  agentType: z.string().default('general'),
  systemPromptOverride: z.string().default(''),
  maxBudgetCents: z.number().min(0).default(0),
  allowedModels: z.array(z.string()).default([]),
  allowedTools: z.array(z.string()).default([]),
  allowedResources: z.array(z.string()).default([]),
  allowedMcpServers: z.array(z.string()).default([]),
  enabled: z.boolean().default(true),
});

export type AgentFormValues = z.infer<typeof agentSchema>;
