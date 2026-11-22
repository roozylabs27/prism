'use client';

import Link from 'next/link';
import { UseFormReturn } from 'react-hook-form';
import {
  Card,
  CardHeader,
  CardTitle,
  CardContent,
  CardDescription,
} from '@/components/molecules/Card';
import { Button } from '@/components/atoms/Button';
import { Switch } from '@/components/atoms/Switch';
import { Textarea } from '@/components/atoms/Textarea';
import {
  Select,
  SelectTrigger,
  SelectValue,
  SelectContent,
  SelectItem,
} from '@/components/molecules/Select';
import { SearchableSelect } from '@/components/molecules/SearchableSelect';
import {
  Form,
  FormField,
  FormItem,
  FormLabel,
  FormControl,
  FormMessage,
} from '@/components/molecules/Form';
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/atoms/Tooltip';
import { PermissionGuard } from '@/components/PermissionProvider';
import { FormTooltipLabel } from './FormattedSandboxOutput';
import { AppRoutes } from '@/constants/routes';
import type { SandboxFormValues } from '@/features/sandbox/schemas/sandbox.schema';
import type { ApiAgent, ApiGatewayKey, ApiModel, ApiRoutingPolicy } from '@/lib/api';
import {
  Code2 as Code2Icon,
  HelpCircle as HelpCircleIcon,
  Play as PlayIcon,
  RefreshCw as RefreshCwIcon,
  KeyRound as KeyRoundIcon,
} from 'lucide-react';

interface SandboxControlsFormProps {
  form: UseFormReturn<SandboxFormValues>;
  onSubmit: (values: SandboxFormValues) => Promise<void>;
  modelsList: ApiModel[];
  policiesList: ApiRoutingPolicy[];
  keysList: ApiGatewayKey[];
  agentsList: ApiAgent[];
  isExecuting: boolean;
  isAsyncMode: boolean;
  selectedModel: string;
}

export function SandboxControlsForm({
  form,
  onSubmit,
  modelsList,
  policiesList,
  keysList,
  agentsList,
  isExecuting,
  isAsyncMode,
  selectedModel,
}: SandboxControlsFormProps) {
  return (
    <Card className="flex flex-col min-w-0 overflow-hidden border-border/70 shadow-lg">
      <CardHeader className="pb-3 border-b border-border/40 bg-card/50">
        <CardTitle className="text-sm font-semibold flex items-center gap-2">
          <Code2Icon className="h-4 w-4 text-primary" />
          <span>Sandbox Execution Controls</span>
        </CardTitle>
        <CardDescription className="text-xs">
          Configure target AI model, API key context, agent boundaries, and execution mode.
        </CardDescription>
      </CardHeader>

      <CardContent className="space-y-4 flex-1 flex flex-col justify-between pt-4">
        <Form {...form}>
          <form
            onSubmit={form.handleSubmit(onSubmit)}
            className="space-y-4 flex-1 flex flex-col justify-start"
          >
            <div className="space-y-4">
              <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
                <FormField
                  control={form.control}
                  name="model"
                  render={({ field }) => (
                    <FormItem>
                      <FormTooltipLabel
                        label="Target AI Model"
                        tooltip="Select a specific AI model (e.g. Gemini, Claude, OpenAI) or use 'prism-auto' for automatic optimal routing."
                      />
                      <FormControl>
                        <SearchableSelect
                          value={field.value}
                          onValueChange={field.onChange}
                          disabled={isExecuting}
                          placeholder="Select target model"
                          searchPlaceholder="Search models..."
                          options={[
                            { value: 'prism-auto', label: 'prism-auto (Smart Routing / Auto Selection)' },
                            ...modelsList
                              .filter((m: ApiModel) => m.slug !== 'prism-auto' && m.id !== 'prism-auto' && m.name !== 'prism-auto')
                              .map((m: ApiModel) => ({
                                value: m.slug,
                                label: `${m.displayName || m.name} (${m.providerName || m.slug})`,
                              })),
                          ]}
                        />
                      </FormControl>
                      <FormMessage />
                    </FormItem>
                  )}
                />

                <FormField
                  control={form.control}
                  name="routingPolicy"
                  render={({ field }) => (
                    <FormItem>
                      <FormTooltipLabel
                        label="Smart Router Policy"
                        tooltip="Rules for automatic model selection. E.g. 'Balanced' for speed/cost balance, 'Quality' for highest accuracy, or 'Cheap' for lowest cost."
                      />
                      <FormControl>
                        <Select
                          value={field.value}
                          onValueChange={field.onChange}
                          disabled={isExecuting || selectedModel !== 'prism-auto'}
                        >
                          <SelectTrigger className="w-full min-w-0">
                            <SelectValue placeholder="Select routing policy" />
                          </SelectTrigger>
                          <SelectContent>
                            {policiesList.length > 0 ? (
                              policiesList.map((p) => (
                                <SelectItem key={p.id} value={p.id}>
                                  {p.name} {p.isDefault ? '(Default)' : ''}
                                </SelectItem>
                              ))
                            ) : (
                              <SelectItem value="__none__" disabled>
                                No routing policies yet; the built-in balanced policy applies
                              </SelectItem>
                            )}
                          </SelectContent>
                        </Select>
                      </FormControl>
                      <FormMessage />
                    </FormItem>
                  )}
                />

                <FormField
                  control={form.control}
                  name="keyPrefix"
                  render={({ field }) => (
                    <FormItem>
                      <FormTooltipLabel
                        label="Gateway API Key Context"
                        required
                        tooltip="Your organization's API Key used to evaluate quota limits, budget policies, and access rights."
                      />
                      <FormControl>
                        <Select
                          value={field.value}
                          onValueChange={field.onChange}
                          disabled={isExecuting || keysList.length === 0}
                        >
                          <SelectTrigger className="w-full min-w-0">
                            <SelectValue
                              placeholder={
                                keysList.length === 0
                                  ? 'No gateway keys available'
                                  : 'Select API key'
                              }
                            />
                          </SelectTrigger>
                          <SelectContent>
                            {keysList.length > 0 ? (
                              keysList.map((k) => (
                                <SelectItem key={k.id} value={k.keyPrefix}>
                                  <span className="truncate block max-w-[220px]">
                                    {k.name} ({k.keyPrefix}...)
                                  </span>
                                </SelectItem>
                              ))
                            ) : (
                              <SelectItem value="__none__" disabled>
                                No gateway keys found
                              </SelectItem>
                            )}
                          </SelectContent>
                        </Select>
                      </FormControl>
                      {keysList.length === 0 && (
                        <div className="flex items-center gap-1.5 pt-1 text-[11px] text-amber-500">
                          <KeyRoundIcon className="h-3 w-3 shrink-0" />
                          <span>No keys found.</span>
                          <Link
                            href={AppRoutes.GATEWAY_KEYS}
                            className="underline font-medium hover:text-amber-400"
                          >
                            Create a Gateway API key &rarr;
                          </Link>
                        </div>
                      )}
                      <FormMessage />
                    </FormItem>
                  )}
                />

                <FormField
                  control={form.control}
                  name="agentId"
                  render={({ field }) => (
                    <FormItem>
                      <FormTooltipLabel
                        label="Agent Context Boundary"
                        tooltip="Bound AI Agent identity restricting security rules (RBAC) and allowed tool executions."
                      />
                      <FormControl>
                        <Select value={field.value} onValueChange={field.onChange} disabled={isExecuting}>
                          <SelectTrigger className="w-full min-w-0">
                            <SelectValue placeholder="Select AI agent" />
                          </SelectTrigger>
                          <SelectContent>
                            <SelectItem value="default">Default Gateway Identity</SelectItem>
                            {agentsList.map((a) => (
                              <SelectItem key={a.id} value={a.name}>
                                <span className="truncate block max-w-[220px]">{a.displayName || a.name}</span>
                              </SelectItem>
                            ))}
                          </SelectContent>
                        </Select>
                      </FormControl>
                      <FormMessage />
                    </FormItem>
                  )}
                />
              </div>

              <FormField
                control={form.control}
                name="enableStream"
                render={({ field }) => (
                  <FormItem className="flex items-center justify-between p-3.5 border border-border bg-card/60 hover:bg-card transition-colors space-y-0">
                    <div className="space-y-0.5">
                      <div className="flex items-center gap-1.5">
                        <FormLabel className="text-xs font-semibold cursor-pointer">
                          Live Response Streaming (SSE)
                        </FormLabel>
                        <Tooltip>
                          <TooltipTrigger asChild>
                            <button type="button" className="text-muted-foreground hover:text-primary">
                              <HelpCircleIcon className="h-3.5 w-3.5" />
                            </button>
                          </TooltipTrigger>
                          <TooltipContent
                            side="top"
                            className="max-w-xs text-[11px] bg-console-header border-slate-800 text-slate-200"
                          >
                            Receive AI responses word-by-word in real time (SSE streaming) without waiting for full
                            completion.
                          </TooltipContent>
                        </Tooltip>
                      </div>
                      <p className="text-[11px] text-muted-foreground">
                        Display response text live as it is generated by the AI model
                      </p>
                    </div>
                    <FormControl>
                      <Switch
                        checked={field.value}
                        onCheckedChange={field.onChange}
                        disabled={isExecuting || form.watch('enableAsync')}
                      />
                    </FormControl>
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name="enableAsync"
                render={({ field }) => (
                  <FormItem className="flex items-center justify-between p-3.5 border border-border bg-card/60 hover:bg-card transition-colors space-y-0">
                    <div className="space-y-0.5">
                      <div className="flex items-center gap-1.5">
                        <FormLabel className="text-xs font-semibold cursor-pointer">
                          Background Queue Execution (Async)
                        </FormLabel>
                        <Tooltip>
                          <TooltipTrigger asChild>
                            <button type="button" className="text-muted-foreground hover:text-primary">
                              <HelpCircleIcon className="h-3.5 w-3.5" />
                            </button>
                          </TooltipTrigger>
                          <TooltipContent
                            side="top"
                            className="max-w-xs text-[11px] bg-console-header border-slate-800 text-slate-200"
                          >
                            Offloads requests to a background Redis queue. Ideal for long-running prompts without
                            keeping the browser connection waiting.
                          </TooltipContent>
                        </Tooltip>
                      </div>
                      <p className="text-[11px] text-muted-foreground">
                        Dispatch requests asynchronously to background workers without holding browser connection
                      </p>
                    </div>
                    <FormControl>
                      <Switch checked={field.value} onCheckedChange={field.onChange} disabled={isExecuting} />
                    </FormControl>
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name="userPrompt"
                render={({ field }) => (
                  <FormItem>
                    <FormTooltipLabel
                      label="Prompt / Code Instruction"
                      required
                      tooltip="Enter the instruction, prompt text, or code block for the AI sandbox to evaluate."
                    />
                    <FormControl>
                      <Textarea
                        {...field}
                        disabled={isExecuting}
                        className="h-28 text-xs font-mono custom-scrollbar"
                        placeholder="Type your prompt or code instructions here..."
                      />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />
            </div>

            <PermissionGuard
              permission="playground:execute"
              disabledTooltip={
                keysList.length === 0
                  ? 'A Gateway API Key is required to run the sandbox container'
                  : 'Playground execution permission required to run sandbox inference'
              }
            >
              <Button
                type="submit"
                variant="prismViolet"
                className="w-full gap-2 mt-4 py-2.5 font-semibold text-xs shadow-md transition-all duration-200"
                disabled={isExecuting || keysList.length === 0}
              >
                {isExecuting ? (
                  <RefreshCwIcon className="h-4 w-4 animate-spin text-white" />
                ) : (
                  <PlayIcon className="h-4 w-4 fill-white" />
                )}
                {isExecuting
                  ? isAsyncMode
                    ? 'Processing in Background Queue...'
                    : 'Running AI Instruction...'
                  : 'Run Sandbox Container'}
              </Button>
            </PermissionGuard>
          </form>
        </Form>
      </CardContent>
    </Card>
  );
}
