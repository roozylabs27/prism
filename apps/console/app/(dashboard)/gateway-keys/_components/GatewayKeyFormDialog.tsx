"use client";

import { useEffect, useState } from "react";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";

import { Button } from "@/components/atoms/Button";
import { Input } from "@/components/atoms/Input";
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
import { useCreateGatewayKey } from "@/hooks/queries/useGatewayKeysQuery";
import type { ApiAgent, ApiProvider } from "@/lib/api";
import { toast } from "sonner";
import { SecretReveal } from "@/components/molecules/SecretReveal";
import { getErrorMessage } from "@/types/ui";

import { gatewayKeySchema, GatewayKeyFormValues } from "@/features/gateway-keys/schemas/create-gateway-key.schema";

const defaultValues: GatewayKeyFormValues = {
  name: "",
  providerId: "",
  agentId: "",
  rateLimit: "100",
};

interface GatewayKeyFormDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  providers?: ApiProvider[];
  agents?: ApiAgent[];
}

export function GatewayKeyFormDialog({
  open,
  onOpenChange,
  providers,
  agents,
}: GatewayKeyFormDialogProps) {
  const createMutation = useCreateGatewayKey();
  // The API returns the secret only in the create response; it is shown here
  // once and dropped when the dialog closes.
  const [createdKey, setCreatedKey] = useState<string | null>(null);

  const form = useForm<GatewayKeyFormValues>({
    resolver: zodResolver(gatewayKeySchema),
    defaultValues,
  });

  const { reset } = form;

  useEffect(() => {
    if (open) {
      reset(defaultValues);
    }
  }, [open, reset]);

  const handleOpenChange = (nextOpen: boolean) => {
    if (!nextOpen) {
      setCreatedKey(null);
    }
    onOpenChange(nextOpen);
  };

  const onSubmit = (values: GatewayKeyFormValues) => {
    setCreatedKey(null);
    createMutation.mutate(
      {
        name: values.name.trim(),
        providerId: values.providerId === "global" ? "" : values.providerId,
        rateLimit: Number(values.rateLimit) || undefined,
        ...(values.agentId?.trim() ? { agentId: values.agentId.trim() } : {}),
      },
      {
        onSuccess: (created) => {
          toast.success("Gateway key created");
          setCreatedKey(created.key);
        },
        onError: (err) => {
          toast.error(`Failed to create gateway key: ${getErrorMessage(err)}`);
        },
      },
    );
  };

  return (
    <Dialog open={open} onOpenChange={handleOpenChange}>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>{createdKey ? "Gateway Key Created" : "Create Gateway Key"}</DialogTitle>
          <DialogDescription>
            {createdKey
              ? "Use this key as the bearer token for requests to the gateway."
              : "Generate a new API key to authenticate requests through the gateway."}
          </DialogDescription>
        </DialogHeader>
        {createdKey ? (
          <div className="space-y-4 py-4">
            <SecretReveal label="Gateway Secret Key" secret={createdKey} />
            <DialogFooter>
              <Button type="button" variant="prismViolet" onClick={() => onOpenChange(false)}>
                Done
              </Button>
            </DialogFooter>
          </div>
        ) : (
        <Form {...form}>
          <form onSubmit={form.handleSubmit(onSubmit)} className="space-y-4 py-4">
            <FormField
              control={form.control}
              name="name"
              render={({ field }) => (
                <FormItem className="space-y-2">
                  <FormLabel required>Key Name</FormLabel>
                  <FormControl>
                    <Input
                      {...field}
                      placeholder="e.g. production-key"
                      disabled={createMutation.isPending}
                    />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name="providerId"
              render={({ field }) => (
                <FormItem className="space-y-2">
                  <FormLabel required>Provider / Routing Scope</FormLabel>
                  <FormControl>
                    <Select
                      value={field.value}
                      onValueChange={field.onChange}
                      disabled={createMutation.isPending}
                    >
                      <SelectTrigger>
                        <SelectValue placeholder="Select a provider scope" />
                      </SelectTrigger>
                      <SelectContent>
                        <SelectItem value="global">
                          All Providers (Universal / prism-auto Smart Router)
                        </SelectItem>
                        {providers && providers.length > 0 && (
                          providers.map((p) => (
                            <SelectItem key={p.id} value={p.id}>
                              {p.name}
                            </SelectItem>
                          ))
                        )}
                      </SelectContent>
                    </Select>
                  </FormControl>
                  <p className="text-[11px] text-muted-foreground">
                    Select <strong>All Providers</strong> for automatic multi-model routing via <code>prism-auto</code>, or pick a specific provider to pin this key.
                  </p>
                  <FormMessage />
                </FormItem>
              )}
            />
            {agents && agents.length > 0 && (
              <FormField
                control={form.control}
                name="agentId"
                render={({ field }) => (
                  <FormItem className="space-y-2">
                    <FormLabel>Bound Agent (Optional)</FormLabel>
                    <FormControl>
                      <Select
                        value={field.value || "__none__"}
                        onValueChange={(v) => field.onChange(v === "__none__" ? "" : v)}
                        disabled={createMutation.isPending}
                      >
                        <SelectTrigger>
                          <SelectValue placeholder="No pinned agent" />
                        </SelectTrigger>
                        <SelectContent>
                          <SelectItem value="__none__">No pinned agent</SelectItem>
                          {agents.map((ag) => (
                            <SelectItem key={ag.id} value={ag.id}>
                              {ag.displayName || ag.name}
                            </SelectItem>
                          ))}
                        </SelectContent>
                      </Select>
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />
            )}
            <FormField
              control={form.control}
              name="rateLimit"
              render={({ field }) => (
                <FormItem className="space-y-2">
                  <FormLabel>Rate Limit (req/min)</FormLabel>
                  <FormControl>
                    <Input
                      {...field}
                      type="number"
                      placeholder="100"
                      disabled={createMutation.isPending}
                    />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
            <DialogFooter>
              <Button
                type="button"
                variant="outline"
                onClick={() => onOpenChange(false)}
                disabled={createMutation.isPending}
              >
                Cancel
              </Button>
              <Button
                type="submit"
                variant="prismViolet"
                disabled={createMutation.isPending}
              >
                {createMutation.isPending ? "Creating..." : "Create Key"}
              </Button>
            </DialogFooter>
          </form>
        </Form>
        )}
      </DialogContent>
    </Dialog>
  );
}
