"use client";

import { useEffect } from "react";
import Link from "next/link";
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
import { useProvidersQuery } from "@/hooks/queries/useProvidersQuery";
import { useCreateCredentialMutation } from "@/hooks/mutations/useCredentialMutations";
import { toast } from "sonner";
import { getErrorMessage } from "@/types/ui";
import { AppRoutes } from "@/constants/routes";

import { credentialSchema, CredentialFormValues } from "@/features/credentials/schemas/credential.schema";

const defaultValues: CredentialFormValues = {
  providerId: "",
  name: "",
  apiKey: "",
};

interface CredentialFormDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
}

export function CredentialFormDialog({ open, onOpenChange }: CredentialFormDialogProps) {
  const { data: providersData } = useProvidersQuery();
  const providers = Array.isArray(providersData) ? providersData : [];
  const createMutation = useCreateCredentialMutation();

  const form = useForm<CredentialFormValues>({
    resolver: zodResolver(credentialSchema),
    defaultValues,
  });

  const { reset } = form;

  useEffect(() => {
    if (open) reset(defaultValues);
  }, [open, reset]);

  const onSubmit = (values: CredentialFormValues) => {
    createMutation.mutate(
      { providerId: values.providerId, name: values.name, apiKey: values.apiKey },
      {
        onSuccess: () => {
          toast.success("Credential created successfully");
          onOpenChange(false);
        },
        onError: (err) => {
          toast.error(`Failed to create credential: ${getErrorMessage(err)}`);
        },
      },
    );
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>Add Provider Credential</DialogTitle>
          <DialogDescription>
            Configure a new API key or secret credential for upstream AI routing.
          </DialogDescription>
        </DialogHeader>
        <Form {...form}>
          <form
            onSubmit={form.handleSubmit(onSubmit)}
            className="space-y-4 py-4"
          >
            <FormField
              control={form.control}
              name="providerId"
              render={({ field }) => (
                <FormItem className="space-y-2">
                  <FormLabel required>Provider</FormLabel>
                  <FormControl>
                    <Select
                      value={field.value}
                      onValueChange={field.onChange}
                      disabled={createMutation.isPending || !providers || providers.length === 0}
                    >
                      <SelectTrigger>
                        <SelectValue
                          placeholder={
                            providers && providers.length > 0
                              ? "Select Provider"
                              : "No providers available"
                          }
                        />
                      </SelectTrigger>
                      <SelectContent>
                        {providers && providers.length > 0 ? (
                          providers.map((p) => (
                            <SelectItem key={p.id} value={p.id}>
                              {p.name} ({p.type})
                            </SelectItem>
                          ))
                        ) : (
                          <SelectItem value="__none__" disabled>
                            No providers available
                          </SelectItem>
                        )}
                      </SelectContent>
                    </Select>
                  </FormControl>
                  {(!providers || providers.length === 0) && (
                    <p className="text-[11px] text-amber-500">
                      No upstream providers found.{" "}
                      <Link href={AppRoutes.PROVIDERS} className="underline hover:text-amber-400">
                        Configure a provider first &rarr;
                      </Link>
                    </p>
                  )}
                  <FormMessage />
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name="name"
              render={({ field }) => (
                <FormItem className="space-y-2">
                  <FormLabel required>Credential Label</FormLabel>
                  <FormControl>
                    <Input
                      {...field}
                      placeholder="e.g., Production Key"
                      disabled={createMutation.isPending}
                    />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name="apiKey"
              render={({ field }) => (
                <FormItem className="space-y-2">
                  <FormLabel required>API Key</FormLabel>
                  <FormControl>
                    <Input
                      {...field}
                      type="password"
                      placeholder="sk-..."
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
                {createMutation.isPending ? "Creating..." : "Add Credential"}
              </Button>
            </DialogFooter>
          </form>
        </Form>
      </DialogContent>
    </Dialog>
  );
}
