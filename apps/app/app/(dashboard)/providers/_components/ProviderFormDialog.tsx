"use client";

import { useEffect } from "react";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { toast } from "sonner";

import { Button } from "@/components/atoms/Button";
import { Input } from "@/components/atoms/Input";
import { Switch } from "@/components/atoms/Switch";
import { Select, SelectTrigger, SelectValue, SelectContent, SelectItem } from "@/components/molecules/Select";
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
import { useCreateProvider, useUpdateProvider } from "@/hooks/mutations/useProviderMutations";
import { ApiProvider } from "@/lib/api";
import { parseApiError } from "@/lib/http/errors";
import { providerSchema, ProviderFormValues } from "@/features/providers/schemas/create-provider.schema";

const defaultValues: ProviderFormValues = {
  name: "",
  type: "custom",
  baseUrl: "",
  enabled: true,
};

interface ProviderFormDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  editingProvider?: ApiProvider | null;
}

export function ProviderFormDialog({ open, onOpenChange, editingProvider }: ProviderFormDialogProps) {
  const createMutation = useCreateProvider();
  const updateMutation = useUpdateProvider();
  const isEditing = Boolean(editingProvider);
  const isPending = createMutation.isPending || updateMutation.isPending;

  const form = useForm<ProviderFormValues>({
    resolver: zodResolver(providerSchema),
    defaultValues,
  });

  const { reset } = form;

  useEffect(() => {
    if (!open) return;
    if (editingProvider) {
      reset({
        name: editingProvider.name || "",
        type: editingProvider.type || "custom",
        baseUrl: editingProvider.baseUrl || "",
        enabled: editingProvider.enabled ?? true,
      });
    } else {
      reset(defaultValues);
    }
  }, [open, editingProvider, reset]);

  const onSubmit = async (values: ProviderFormValues) => {
    try {
      if (editingProvider) {
        await updateMutation.mutateAsync({ id: editingProvider.id, data: values });
        toast.success(`Provider "${values.name}" updated`);
      } else {
        await createMutation.mutateAsync(values);
        toast.success(`Provider "${values.name}" connected`);
      }
      onOpenChange(false);
    } catch (err: unknown) {
      toast.error(parseApiError(err, isEditing ? "Failed to update provider" : "Failed to create provider").message);
    }
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>{isEditing ? "Edit Custom Provider" : "Connect New Provider"}</DialogTitle>
          <DialogDescription>
            {isEditing
              ? "Update endpoint URL, adapter type, or status for this custom provider."
              : "Add a new AI provider adapter to your workspace."}
          </DialogDescription>
        </DialogHeader>
        <Form {...form}>
          <form
            onSubmit={form.handleSubmit(onSubmit)}
            className="space-y-4 py-4"
          >
            <FormField
              control={form.control}
              name="name"
              render={({ field }) => (
                <FormItem className="space-y-2">
                  <FormLabel required>Provider Name</FormLabel>
                  <FormControl>
                    <Input
                      {...field}
                      placeholder="e.g., OpenAI Production"
                      disabled={isPending}
                    />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name="type"
              render={({ field }) => (
                <FormItem className="space-y-2">
                  <FormLabel required>Provider Type</FormLabel>
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
                        <SelectItem value="custom">Custom (OpenAI-Compatible)</SelectItem>
                        <SelectItem value="openai">OpenAI</SelectItem>
                        <SelectItem value="anthropic">Anthropic</SelectItem>
                        <SelectItem value="google">Google Gemini</SelectItem>
                        <SelectItem value="opencode">OpenCode</SelectItem>
                      </SelectContent>
                    </Select>
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name="baseUrl"
              render={({ field }) => (
                <FormItem className="space-y-2">
                  <FormLabel required>Base URL</FormLabel>
                  <FormControl>
                    <Input
                      {...field}
                      placeholder="https://api.openai.com/v1"
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
                    <FormLabel className="text-sm font-medium">Enabled</FormLabel>
                    <p className="text-xs text-muted-foreground">Allow routing requests to this provider.</p>
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
                disabled={isPending}
              >
                {isPending
                  ? isEditing ? "Saving..." : "Creating..."
                  : isEditing ? "Save Changes" : "Create Provider"}
              </Button>
            </DialogFooter>
          </form>
        </Form>
      </DialogContent>
    </Dialog>
  );
}
