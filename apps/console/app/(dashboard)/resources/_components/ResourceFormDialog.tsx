"use client";

import { useEffect } from "react";
import { useForm, useFieldArray, useWatch } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";

import { Button } from "@/components/atoms/Button";
import { Input } from "@/components/atoms/Input";
import { Textarea } from "@/components/atoms/Textarea";
import { Label } from "@/components/atoms/Label";
import { Switch } from "@/components/atoms/Switch";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/molecules/Select";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogFooter,
  DialogTitle,
  DialogDescription,
} from "@/components/molecules/Dialog";
import {
  Form,
  FormField,
  FormItem,
  FormLabel,
  FormControl,
  FormMessage,
} from "@/components/molecules/Form";
import {
  useCreateResource,
  useUpdateResource,
  useResourceDetailQuery,
} from "@/hooks/queries/useResourcesQuery";
import type { ApiResource, ApiCreateResourceBackend } from "@/lib/api";
import { toast } from "sonner";
import { getErrorMessage } from "@/types/ui";
import { Plus, Trash2, Loader2 } from "lucide-react";

import {
  resourceSchema,
  ResourceFormValues,
} from "@/features/resources/schemas/create-resource.schema";

const defaultValues: ResourceFormValues = {
  name: "",
  displayName: "",
  description: "",
  enabled: true,
  parametersSchema: "{}",
  backends: [],
};

function formatSchemaJSON(schema: Record<string, unknown> | null | undefined): string {
  if (!schema || Object.keys(schema).length === 0) return "{}";
  return JSON.stringify(schema, null, 2);
}

interface ResourceFormDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  editingResource: ApiResource | null;
}

export function ResourceFormDialog({
  open,
  onOpenChange,
  editingResource,
}: ResourceFormDialogProps) {
  const createMutation = useCreateResource();
  const updateMutation = useUpdateResource();

  const { data: detail, isLoading: detailLoading } = useResourceDetailQuery(
    open && editingResource?.id ? editingResource.id : undefined
  );

  const form = useForm<ResourceFormValues>({
    resolver: zodResolver(resourceSchema),
    defaultValues,
  });

  const { reset } = form;
  const watchedBackends = useWatch({ control: form.control, name: "backends" });
  const { fields, append, remove } = useFieldArray({
    control: form.control,
    name: "backends",
  });

  useEffect(() => {
    if (!open) return;
    if (!editingResource) {
      reset(defaultValues);
      return;
    }
    if (detail) {
      reset({
        name: detail.resource?.name ?? detail.name ?? editingResource.name,
        displayName:
          detail.resource?.displayName ??
          detail.displayName ??
          editingResource.displayName,
        description:
          detail.resource?.description ??
          detail.description ??
          editingResource.description,
        enabled:
          detail.resource?.enabled ?? detail.enabled ?? editingResource.enabled,
        parametersSchema: formatSchemaJSON(
          detail.resource?.parametersSchema ??
            detail.parametersSchema ??
            editingResource.parametersSchema
        ),
        backends: (detail.backends || []).map((b) => {
          const bExtra = b as unknown as {
            endpointUrl?: string;
            httpMethod?: string;
            sqlQuery?: string;
          };
          return {
            id: b.id,
            name: b.name,
            backendType: b.backendType === "postgres" || b.backendType === "s3" || b.backendType === "blob" ? b.backendType : "http",
            endpointUrl: bExtra.endpointUrl ?? "",
            httpMethod: bExtra.httpMethod ?? "POST",
            authToken: "",
            connectionString: "",
            queryTemplate: b.queryTemplate ?? "",
            sqlQuery: bExtra.sqlQuery ?? "",
            timeoutMs: b.timeoutMs,
            priority: b.priority,
          };
        }),
      });
    } else {
      reset({
        name: editingResource.name,
        displayName: editingResource.displayName,
        description: editingResource.description,
        enabled: editingResource.enabled,
        parametersSchema: formatSchemaJSON(editingResource.parametersSchema),
        backends: [],
      });
    }
  }, [open, editingResource, detail, reset]);

  const isSubmitting = createMutation.isPending || updateMutation.isPending;

  const onSubmit = (values: ResourceFormValues) => {
    let parsedSchema: Record<string, unknown>;
    try {
      parsedSchema = JSON.parse(values.parametersSchema);
    } catch {
      toast.error("Parameters schema must be valid JSON");
      return;
    }

    const backends: ApiCreateResourceBackend[] = values.backends
      .filter((b) => b.name.trim())
      .map((b) => ({
        id: b.id,
        name: b.name.trim(),
        backendType: b.backendType,
        endpointUrl: b.endpointUrl?.trim() || undefined,
        httpMethod: b.httpMethod?.trim() || "POST",
        authToken: b.authToken?.trim() || undefined,
        connectionString: b.connectionString?.trim() || undefined,
        queryTemplate: b.queryTemplate?.trim() || undefined,
        sqlQuery: b.sqlQuery?.trim() || undefined,
        timeoutMs: b.timeoutMs || 30000,
        priority: b.priority || 1,
      }));

    const payload = {
      name: values.name.trim(),
      displayName: values.displayName.trim() || values.name.trim(),
      description: values.description.trim(),
      enabled: values.enabled,
      parametersSchema: parsedSchema,
      backends,
    };

    if (editingResource) {
      updateMutation.mutate(
        { id: editingResource.id, data: payload },
        {
          onSuccess: () => {
            toast.success(`Resource "${payload.name}" updated`);
            onOpenChange(false);
          },
          onError: (err) =>
            toast.error(`Failed to update resource: ${getErrorMessage(err)}`),
        }
      );
    } else {
      createMutation.mutate(payload, {
        onSuccess: () => {
          toast.success(`Resource "${payload.name}" created`);
          onOpenChange(false);
        },
        onError: (err) =>
          toast.error(`Failed to create resource: ${getErrorMessage(err)}`),
      });
    }
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-2xl">
        <DialogHeader>
          <DialogTitle>
            {editingResource ? "Edit Resource" : "Add Resource Source"}
          </DialogTitle>
          <DialogDescription>
            {editingResource
              ? "Update the resource parameters schema and execution backends."
              : "Connect a new document store, API endpoint, or PostgreSQL query resource."}
          </DialogDescription>
        </DialogHeader>
        <Form {...form}>
          <form onSubmit={form.handleSubmit(onSubmit)} className="space-y-4 py-4">
            <div className="space-y-4 overflow-y-auto max-h-[65vh] pr-1">
              <FormField
                control={form.control}
                name="name"
                render={({ field }) => (
                  <FormItem className="space-y-2">
                    <FormLabel required>Resource Name</FormLabel>
                    <FormControl>
                      <Input
                        {...field}
                        placeholder="e.g., product-knowledge-base"
                        disabled={isSubmitting || detailLoading}
                      />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />
              <FormField
                control={form.control}
                name="displayName"
                render={({ field }) => (
                  <FormItem className="space-y-2">
                    <FormLabel>Display Name</FormLabel>
                    <FormControl>
                      <Input
                        {...field}
                        placeholder="e.g., Product Knowledge Base"
                        disabled={isSubmitting || detailLoading}
                      />
                    </FormControl>
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
                        placeholder="Describe what this resource provides..."
                        disabled={isSubmitting || detailLoading}
                      />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />
              <FormField
                control={form.control}
                name="parametersSchema"
                render={({ field }) => (
                  <FormItem className="space-y-2">
                    <FormLabel>Parameters Schema (JSON)</FormLabel>
                    <FormControl>
                      <Textarea
                        {...field}
                        rows={4}
                        className="font-mono text-xs"
                        disabled={isSubmitting || detailLoading}
                      />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <div className="space-y-2">
                <div className="flex items-center justify-between">
                  <Label className="text-xs font-semibold">
                    Execution Backends
                  </Label>
                  <Button
                    type="button"
                    variant="outline"
                    size="sm"
                    className="h-7 gap-1 text-xs"
                    disabled={isSubmitting || detailLoading}
                    onClick={() =>
                      append({
                        name: `backend-${fields.length + 1}`,
                        backendType: "http",
                        endpointUrl: "",
                        httpMethod: "POST",
                        authToken: "",
                        connectionString: "",
                        queryTemplate: "",
                        sqlQuery: "",
                        timeoutMs: 30000,
                        priority: fields.length + 1,
                      })
                    }
                  >
                    <Plus className="h-3.5 w-3.5" /> Add Backend
                  </Button>
                </div>

                {fields.length === 0 ? (
                  <p className="text-xs text-muted-foreground rounded border border-dashed border-border p-3">
                    No resource backends attached yet. Add at least one HTTP or Postgres backend to make this resource queryable.
                  </p>
                ) : (
                  <div className="space-y-3">
                    {fields.map((item, index) => {
                      const bType = watchedBackends?.[index]?.backendType ?? "http";
                      return (
                        <div
                          key={item.id}
                          className="rounded border border-border bg-muted/20 p-3 space-y-2"
                        >
                          <div className="flex items-center justify-between">
                            <Label className="text-xs font-semibold text-muted-foreground">
                              Backend {index + 1}
                            </Label>
                            <Button
                              type="button"
                              variant="ghost"
                              size="sm"
                              className="h-7 w-7 p-0 text-muted-foreground hover:text-destructive"
                              onClick={() => remove(index)}
                              disabled={isSubmitting || detailLoading}
                            >
                              <Trash2 className="h-3.5 w-3.5" />
                            </Button>
                          </div>
                          <div className="grid grid-cols-1 gap-2 sm:grid-cols-2">
                            <div className="space-y-1">
                              <Label className="text-[11px] text-muted-foreground">Name</Label>
                              <Input
                                placeholder="e.g., primary"
                                {...form.register(`backends.${index}.name`)}
                                disabled={isSubmitting || detailLoading}
                              />
                            </div>
                            <div className="space-y-1">
                              <Label className="text-[11px] text-muted-foreground">Backend Type</Label>
                              <Select
                                value={bType}
                                onValueChange={(v) =>
                                  form.setValue(
                                    `backends.${index}.backendType`,
                                    v as "http" | "postgres" | "s3" | "blob"
                                  )
                                }
                                disabled={isSubmitting || detailLoading}
                              >
                                <SelectTrigger className="h-9 text-xs">
                                  <SelectValue />
                                </SelectTrigger>
                                <SelectContent>
                                  <SelectItem value="http">HTTP / REST</SelectItem>
                                  <SelectItem value="postgres">PostgreSQL</SelectItem>
                                </SelectContent>
                              </Select>
                            </div>
                            {bType === "postgres" ? (
                              <>
                                <div className="space-y-1 sm:col-span-2">
                                  <Label className="text-[11px] text-muted-foreground">Connection String</Label>
                                  <Input
                                    type="password"
                                    placeholder={detail ? "Leave blank to keep existing DSN" : "postgres://user:pass@host:5432/db"}
                                    {...form.register(`backends.${index}.connectionString`)}
                                    disabled={isSubmitting || detailLoading}
                                  />
                                </div>
                                <div className="space-y-1 sm:col-span-2">
                                  <Label className="text-[11px] text-muted-foreground">SQL Query</Label>
                                  <Textarea
                                    rows={2}
                                    className="font-mono text-xs"
                                    placeholder="SELECT * FROM docs WHERE topic = $1"
                                    {...form.register(`backends.${index}.sqlQuery`)}
                                    disabled={isSubmitting || detailLoading}
                                  />
                                </div>
                              </>
                            ) : (
                              <>
                                <div className="space-y-1 sm:col-span-2">
                                  <Label className="text-[11px] text-muted-foreground">Endpoint URL</Label>
                                  <Input
                                    placeholder="https://api.example.com/query"
                                    {...form.register(`backends.${index}.endpointUrl`)}
                                    disabled={isSubmitting || detailLoading}
                                  />
                                </div>
                                <div className="space-y-1 sm:col-span-2">
                                  <Label className="text-[11px] text-muted-foreground">Auth Token (optional)</Label>
                                  <Input
                                    type="password"
                                    placeholder={detail ? "Leave blank to keep existing token" : "Bearer token"}
                                    {...form.register(`backends.${index}.authToken`)}
                                    disabled={isSubmitting || detailLoading}
                                  />
                                </div>
                              </>
                            )}
                          </div>
                        </div>
                      );
                    })}
                  </div>
                )}
              </div>

              <FormField
                control={form.control}
                name="enabled"
                render={({ field }) => (
                  <FormItem className="flex items-center justify-between space-y-0 p-3 rounded border border-border bg-card">
                    <div className="space-y-0.5">
                      <FormLabel>Enabled</FormLabel>
                      <p className="text-xs text-muted-foreground">
                        Allow agents to access this resource
                      </p>
                    </div>
                    <FormControl>
                      <Switch
                        checked={field.value}
                        onCheckedChange={field.onChange}
                        disabled={isSubmitting || detailLoading}
                      />
                    </FormControl>
                  </FormItem>
                )}
              />

              {detailLoading && (
                <div className="flex items-center gap-2 text-xs text-muted-foreground">
                  <Loader2 className="h-3.5 w-3.5 animate-spin" />
                  <span>Loading resource configuration...</span>
                </div>
              )}
            </div>

            <DialogFooter>
              <Button
                type="button"
                variant="outline"
                onClick={() => onOpenChange(false)}
                disabled={isSubmitting}
              >
                Cancel
              </Button>
              <Button type="submit" variant="prismViolet" disabled={isSubmitting || detailLoading}>
                {isSubmitting
                  ? editingResource
                    ? "Saving..."
                    : "Creating..."
                  : editingResource
                    ? "Save Changes"
                    : "Create Resource"}
              </Button>
            </DialogFooter>
          </form>
        </Form>
      </DialogContent>
    </Dialog>
  );
}
