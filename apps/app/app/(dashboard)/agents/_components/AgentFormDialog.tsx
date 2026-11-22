"use client";

import { useEffect, useState } from "react";
import { useForm, useWatch } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";

import { Button } from "@/components/atoms/Button";
import { Input } from "@/components/atoms/Input";
import { Switch } from "@/components/atoms/Switch";
import { Textarea } from "@/components/atoms/Textarea";
import { Select, SelectTrigger, SelectValue, SelectContent, SelectItem } from "@/components/molecules/Select";
import { Dialog, DialogContent, DialogHeader, DialogFooter, DialogTitle, DialogDescription } from "@/components/molecules/Dialog";
import {
  Form,
  FormField,
  FormItem,
  FormLabel,
  FormControl,
  FormMessage,
} from "@/components/molecules/Form";
import { MultiSelect, MultiSelectOption } from "@/components/molecules/MultiSelect";
import { NumberInput } from "@/components/molecules/NumberInput";
import { useCreateAgent, useUpdateAgent } from "@/hooks/queries/useAgentsQuery";
import { ApiAgent, ApiCreateAgentRequest } from "@/lib/api";
import { Pencil, Link as LinkIcon } from "lucide-react";
import { toast } from "sonner";
import { getErrorMessage } from "@/types/ui";

const AGENT_TYPES = ["general", "code", "research", "ops", "custom"];

function slugify(value: string): string {
  return value
    .toLowerCase()
    .trim()
    .replace(/[^a-z0-9\s-]/g, "")
    .replace(/\s+/g, "-")
    .replace(/-+/g, "-")
    .replace(/^-+|-+$/g, "");
}

import { agentSchema, AgentFormValues } from "@/features/agents/schemas/agent.schema";

interface AgentFormDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  editingAgent: ApiAgent | null;
  modelOptions?: MultiSelectOption[];
  toolOptions: MultiSelectOption[];
  resourceOptions: MultiSelectOption[];
  mcpServerOptions: MultiSelectOption[];
}

const defaultValues: AgentFormValues = {
  name: "",
  displayName: "",
  description: "",
  agentType: "general",
  systemPromptOverride: "",
  maxBudgetCents: 0,
  allowedModels: [],
  allowedTools: [],
  allowedResources: [],
  allowedMcpServers: [],
  enabled: true,
};

export function AgentFormDialog({
  open,
  onOpenChange,
  editingAgent,
  modelOptions = [],
  toolOptions,
  resourceOptions,
  mcpServerOptions,
}: AgentFormDialogProps) {
  const createMutation = useCreateAgent();
  const updateMutation = useUpdateAgent();
  const lockScopeKey = open ? (editingAgent?.id ?? "new") : "closed";
  const defaultNameLocked =
    !editingAgent || editingAgent.name === slugify(editingAgent.displayName || editingAgent.name);
  const [nameLockOverride, setNameLockOverride] = useState<{ scope: string; locked: boolean } | null>(null);
  const nameLocked =
    nameLockOverride?.scope === lockScopeKey ? nameLockOverride.locked : defaultNameLocked;

  const form = useForm<AgentFormValues>({
    resolver: zodResolver(agentSchema),
    defaultValues,
  });

  const { reset } = form;
  const formName = useWatch({ control: form.control, name: "name" });
  const formDisplayName = useWatch({ control: form.control, name: "displayName" });

  useEffect(() => {
    if (!open) return;
    if (!editingAgent) {
      reset(defaultValues);
      return;
    }
    reset({
      name: editingAgent.name,
      displayName: editingAgent.displayName,
      description: editingAgent.description,
      agentType: editingAgent.agentType,
      systemPromptOverride: editingAgent.systemPromptOverride ?? "",
      maxBudgetCents: editingAgent.maxBudgetCents,
      allowedModels: editingAgent.allowedModels ?? [],
      allowedTools: editingAgent.allowedTools ?? [],
      allowedResources: editingAgent.allowedResources ?? [],
      allowedMcpServers: editingAgent.allowedMcpServers ?? [],
      enabled: editingAgent.enabled ?? true,
    });
  }, [open, editingAgent, reset]);

  const handleDisplayNameChange = (v: string) => {
    form.setValue("displayName", v, { shouldValidate: true });
    if (nameLocked) {
      form.setValue("name", slugify(v));
    }
  };

  const handleToggleNameLock = (locked: boolean) => {
    setNameLockOverride({ scope: lockScopeKey, locked });
    if (locked) {
      form.setValue("name", slugify(formDisplayName));
    }
  };

  const isPending = createMutation.isPending || updateMutation.isPending;

  const onSubmit = (values: AgentFormValues) => {
    const payload: ApiCreateAgentRequest = {
      name: values.name,
      displayName: values.displayName || undefined,
      description: values.description || undefined,
      agentType: values.agentType,
      systemPromptOverride: values.systemPromptOverride,
      maxBudgetCents: values.maxBudgetCents,
      allowedModels: values.allowedModels,
      allowedTools: values.allowedTools,
      allowedResources: values.allowedResources,
      allowedMcpServers: values.allowedMcpServers,
      enabled: values.enabled,
    };

    if (editingAgent) {
      updateMutation.mutate(
        { id: editingAgent.id, data: payload },
        {
          onSuccess: () => {
            toast.success(`Agent "${values.name}" updated`);
            onOpenChange(false);
          },
          onError: (err) =>
            toast.error(`Failed to update: ${getErrorMessage(err)}`),
        },
      );
    } else {
      createMutation.mutate(payload, {
        onSuccess: () => {
          toast.success(`Agent "${values.name}" created`);
          onOpenChange(false);
        },
        onError: (err) =>
          toast.error(`Failed to create: ${getErrorMessage(err)}`),
      });
    }
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>
            {editingAgent ? "Configure Agent" : "Instantiate New Agent"}
          </DialogTitle>
          <DialogDescription>
            {editingAgent
              ? `Update settings for "${editingAgent.name}".`
              : "Provision a new AI agent identity in this workspace."}
          </DialogDescription>
        </DialogHeader>
        <Form {...form}>
          <form
            onSubmit={form.handleSubmit(onSubmit)}
            className="space-y-4 py-4"
          >
            <FormField
              control={form.control}
              name="displayName"
              render={({ field }) => (
                <FormItem className="space-y-2">
                  <FormLabel required>Display Name</FormLabel>
                  <FormControl>
                    <Input
                      {...field}
                      onChange={(e) => handleDisplayNameChange(e.target.value)}
                      placeholder="e.g., Code Reviewer"
                      disabled={isPending}
                    />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name="name"
              render={({ field }) => (
                <FormItem className="space-y-2">
                  <FormLabel required>Agent Name</FormLabel>
                  <FormControl>
                    <div className="relative">
                      <Input
                        {...field}
                        readOnly={nameLocked}
                        onChange={(e) => field.onChange(e.target.value)}
                        placeholder="auto-from-display-name"
                        className={
                          nameLocked
                            ? "pr-10 bg-muted/50 text-muted-foreground"
                            : "pr-10"
                        }
                        disabled={isPending}
                      />
                      <button
                        type="button"
                        aria-label={
                          nameLocked
                            ? "Edit agent name"
                            : "Auto-generate from display name"
                        }
                        onClick={() => handleToggleNameLock(!nameLocked)}
                        className="absolute inset-y-0 right-0 flex w-9 items-center justify-center text-muted-foreground transition-colors hover:text-foreground focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring"
                      >
                        {nameLocked ? (
                          <Pencil className="h-3.5 w-3.5" />
                        ) : (
                          <LinkIcon className="h-3.5 w-3.5" />
                        )}
                      </button>
                    </div>
                  </FormControl>
                  {nameLocked && (
                    <p className="text-[10px] text-muted-foreground">
                      Auto-generated from Display Name. Click the pencil to edit.
                    </p>
                  )}
                  <FormMessage />
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name="description"
              render={({ field }) => (
                <FormItem className="space-y-2">
                  <FormLabel>Description</FormLabel>
                  <FormControl>
                    <Input
                      {...field}
                      placeholder="What this agent does..."
                      disabled={isPending}
                    />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name="systemPromptOverride"
              render={({ field }) => (
                <FormItem className="space-y-2">
                  <FormLabel>System Prompt Override</FormLabel>
                  <FormControl>
                    <Textarea
                      {...field}
                      rows={3}
                      placeholder="Optional system prompt injected ahead of conversation messages..."
                      disabled={isPending}
                    />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name="agentType"
              render={({ field }) => (
                <FormItem className="space-y-2">
                  <FormLabel>Agent Type</FormLabel>
                  <FormControl>
                    <Select
                      value={field.value}
                      onValueChange={field.onChange}
                      disabled={isPending}
                    >
                      <SelectTrigger>
                        <SelectValue />
                      </SelectTrigger>
                      <SelectContent>
                        {AGENT_TYPES.map((t) => (
                          <SelectItem key={t} value={t}>
                            {t.charAt(0).toUpperCase() + t.slice(1)}
                          </SelectItem>
                        ))}
                      </SelectContent>
                    </Select>
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name="maxBudgetCents"
              render={({ field }) => (
                <FormItem className="space-y-2">
                  <FormLabel>Max Budget (cents/mo)</FormLabel>
                  <FormControl>
                    <NumberInput
                      value={field.value}
                      onValueChange={field.onChange}
                      min={0}
                      step={100}
                      placeholder="0"
                      disabled={isPending}
                    />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name="allowedModels"
              render={({ field }) => (
                <FormItem className="space-y-2">
                  <FormLabel>Allowed Models</FormLabel>
                  <FormControl>
                    <MultiSelect
                      value={field.value}
                      onValueChange={field.onChange}
                      options={modelOptions}
                      placeholder="All workspace models allowed..."
                      searchPlaceholder="Search models..."
                      emptyMessage="No models available."
                      disabled={isPending}
                    />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name="allowedTools"
              render={({ field }) => (
                <FormItem className="space-y-2">
                  <FormLabel>Allowed Tools</FormLabel>
                  <FormControl>
                    <MultiSelect
                      value={field.value}
                      onValueChange={field.onChange}
                      options={toolOptions}
                      placeholder="Select tools..."
                      searchPlaceholder="Search tools..."
                      emptyMessage="No tools available."
                      disabled={isPending}
                    />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name="allowedResources"
              render={({ field }) => (
                <FormItem className="space-y-2">
                  <FormLabel>Allowed Resources</FormLabel>
                  <FormControl>
                    <MultiSelect
                      value={field.value}
                      onValueChange={field.onChange}
                      options={resourceOptions}
                      placeholder="Select resources..."
                      searchPlaceholder="Search resources..."
                      emptyMessage="No resources available."
                      disabled={isPending}
                    />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name="allowedMcpServers"
              render={({ field }) => (
                <FormItem className="space-y-2">
                  <FormLabel>Allowed MCP Servers</FormLabel>
                  <FormControl>
                    <MultiSelect
                      value={field.value}
                      onValueChange={field.onChange}
                      options={mcpServerOptions}
                      placeholder="Select MCP servers..."
                      searchPlaceholder="Search MCP servers..."
                      emptyMessage="No MCP servers available."
                      disabled={isPending}
                    />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name="enabled"
              render={({ field }) => (
                <FormItem className="flex items-center justify-between rounded-lg border border-border p-3">
                  <div>
                    <FormLabel className="text-sm font-medium">Agent Enabled</FormLabel>
                    <p className="text-xs text-muted-foreground">Allow gateway requests under this agent identity.</p>
                  </div>
                  <FormControl>
                    <Switch
                      checked={field.value}
                      onCheckedChange={field.onChange}
                      disabled={isPending}
                    />
                  </FormControl>
                </FormItem>
              )}
            />
            <DialogFooter>
              <Button
                type="button"
                variant="outline"
                onClick={() => onOpenChange(false)}
                disabled={isPending}
              >
                Cancel
              </Button>
              <Button
                type="submit"
                variant="prismViolet"
                disabled={!formName.trim() || isPending}
              >
                {isPending
                  ? "Saving..."
                  : editingAgent
                    ? "Update Agent"
                    : "Create Agent"}
              </Button>
            </DialogFooter>
          </form>
        </Form>
      </DialogContent>
    </Dialog>
  );
}
