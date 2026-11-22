'use client';

import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { useForm, useWatch } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { ApiEndpoints } from '@/constants/routes';
import { api, apiExecuteSandboxChatAsync, type ApiModel } from '@/lib/api';
import { AppLayout } from '@/components/AppLayout';
import { PageHeader } from '@/components/molecules/PageHeader';
import { TooltipProvider } from '@/components/atoms/Tooltip';
import { useModelsListQuery } from '@/hooks/queries/useModelsListQuery';
import { useAgentsQuery } from '@/hooks/queries/useAgentsQuery';
import { useAllGatewayKeysQuery } from '@/hooks/queries/useGatewayKeysQuery';
import { usePoliciesQuery } from '@/hooks/queries/usePoliciesQuery';
import { useSandboxExecutionMutation } from '@/hooks/mutations/useSandboxMutation';
import { useSSE } from '@/hooks/useSSE';
import { sandboxSchema, SandboxFormValues } from '@/features/sandbox/schemas/sandbox.schema';
import { SandboxControlsForm } from '@/features/sandbox/components/SandboxControlsForm';
import { SandboxOutputConsole } from '@/features/sandbox/components/FormattedSandboxOutput';
import {
  consumeSandboxStream,
  extractAsyncJobContent,
  formatSandboxJsonResult,
  parseSandboxTokenStats,
} from '@/features/sandbox/utils/parse-sandbox-stream';
import { getErrorMessage } from '@/types/ui';
import { toast } from 'sonner';

export default function SandboxPage() {
  const { data: modelsData } = useModelsListQuery();
  const { data: agentsData } = useAgentsQuery();
  const { data: keysData } = useAllGatewayKeysQuery();
  const { data: policiesData } = usePoliciesQuery();

  const modelsList = useMemo<ApiModel[]>(
    () =>
      Array.isArray(modelsData)
        ? modelsData
        : ((modelsData as unknown as { data?: ApiModel[] })?.data ?? []),
    [modelsData]
  );
  const agentsList = useMemo(() => (Array.isArray(agentsData) ? agentsData : []), [agentsData]);
  const keysList = useMemo(() => keysData ?? [], [keysData]);
  const policiesList = useMemo(() => (Array.isArray(policiesData) ? policiesData : []), [policiesData]);

  const sandboxMutation = useSandboxExecutionMutation();
  const { subscribeToSSE } = useSSE();

  const [executionOutput, setExecutionOutput] = useState<string>('');
  const [latencyMs, setLatencyMs] = useState<number | null>(null);
  const [tokenStats, setTokenStats] = useState<{ input: number; output: number } | null>(null);
  const [routedModel, setRoutedModel] = useState<string | null>(null);
  const [routedPolicy, setRoutedPolicy] = useState<string | null>(null);
  const [copied, setCopied] = useState(false);
  const [activeJobId, setActiveJobId] = useState<string | null>(null);
  const [isAsyncExecuting, setIsAsyncExecuting] = useState(false);
  const [isTyping, setIsTyping] = useState(false);

  const isExecuting = sandboxMutation.isPending || activeJobId !== null || isAsyncExecuting || isTyping;

  const typewriterRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const completedJobRef = useRef<string | null>(null);
  const jobStartTimeRef = useRef<number>(0);

  const typewrite = useCallback((text: string, onComplete?: () => void) => {
    if (typewriterRef.current) clearTimeout(typewriterRef.current);
    setIsTyping(true);
    let i = 0;
    const step = () => {
      i += Math.min(text.length - i, Math.floor(Math.random() * 4) + 2);
      setExecutionOutput(text.slice(0, i));
      if (i < text.length) {
        typewriterRef.current = setTimeout(step, 16 + Math.random() * 8);
      } else {
        setIsTyping(false);
        if (onComplete) onComplete();
      }
    };
    step();
  }, []);

  const form = useForm<SandboxFormValues>({
    resolver: zodResolver(sandboxSchema),
    defaultValues: {
      model: 'prism-auto',
      routingPolicy: '',
      keyPrefix: keysList[0]?.keyPrefix || '',
      agentId: 'default',
      enableStream: true,
      enableAsync: false,
      userPrompt: '',
    },
  });

  useEffect(() => {
    if (policiesList.length === 0) return;
    const current = form.getValues('routingPolicy');
    if (policiesList.some((p) => p.id === current)) return;
    const preferred = policiesList.find((p) => p.isDefault) ?? policiesList[0];
    form.setValue('routingPolicy', preferred.id);
  }, [policiesList, form]);

  useEffect(() => {
    if (keysList.length === 0) return;
    const current = form.getValues('keyPrefix');
    if (keysList.some((k) => k.keyPrefix === current)) return;
    form.setValue('keyPrefix', keysList[0].keyPrefix);
  }, [keysList, form]);

  const handleJobCompleted = useCallback(
    (jobId: string, result: Record<string, unknown> | null | undefined, model?: string, startTime?: number) => {
      if (completedJobRef.current === jobId) return;
      completedJobRef.current = jobId;

      const stats = parseSandboxTokenStats(
        result?.usage as { prompt_tokens?: number; completion_tokens?: number } | undefined
      );
      if (stats) setTokenStats(stats);
      if (model) setRoutedModel(model);
      if (startTime) setLatencyMs(Date.now() - startTime);

      toast.success('Async job completed');
      setActiveJobId(null);
      setIsAsyncExecuting(false);
      form.setValue('userPrompt', '');
      typewrite(extractAsyncJobContent(result));
    },
    [typewrite, form]
  );

  const handleJobFailed = useCallback((jobId: string, errorMessage: string) => {
    if (completedJobRef.current === jobId) return;
    completedJobRef.current = jobId;
    setExecutionOutput(`Execution Failed:\n${errorMessage}`);
    toast.error(`Async Job ${jobId} failed: ${errorMessage}`);
    setActiveJobId(null);
    setIsAsyncExecuting(false);
  }, []);

  useEffect(() => {
    if (!activeJobId) return;

    let cancelled = false;
    let pollTimer: ReturnType<typeof setTimeout> | null = null;
    let attempts = 0;
    const maxAttempts = 60;

    const unsubscribe = subscribeToSSE((event) => {
      if (!event || !event.payload) return;
      const payload = event.payload as Record<string, unknown>;
      const evtType = (payload.type as string) || event.type;
      const data = (payload.data as Record<string, unknown>) || payload;

      if (evtType === 'async_job_updated' || payload.type === 'async_job_updated') {
        const jobId = data.jobId || data.job_id || payload.jobId || payload.job_id;
        const status = data.status || payload.status;
        if (jobId === activeJobId) {
          if (status === 'completed') {
            handleJobCompleted(
              activeJobId,
              (data.result || payload.result) as Record<string, unknown> | null | undefined,
              (data.model || payload.model) as string | undefined,
              jobStartTimeRef.current
            );
          } else if (status === 'failed') {
            handleJobFailed(activeJobId, String(data.error || payload.error || 'Unknown error'));
          }
        }
      }
    });

    const pollJobStatus = async () => {
      if (cancelled || completedJobRef.current === activeJobId) return;
      attempts += 1;
      try {
        const res = await api.get<Record<string, unknown>>(`${ApiEndpoints.JOBS}/${activeJobId}`);
        if (cancelled || completedJobRef.current === activeJobId) return;
        const job = res.data;
        if (job?.status === 'completed') {
          handleJobCompleted(
            activeJobId,
            job.result as Record<string, unknown> | null | undefined,
            job.model as string | undefined,
            jobStartTimeRef.current
          );
          return;
        }
        if (job?.status === 'failed') {
          handleJobFailed(activeJobId, String(job.error || 'Unknown error'));
          return;
        }
      } catch (pollErr: unknown) {
        if (cancelled || completedJobRef.current === activeJobId) return;
        if ((pollErr as { status?: number })?.status === 404) {
          handleJobFailed(activeJobId, 'Job expired or not found');
          return;
        }
      }
      if (attempts >= maxAttempts) {
        handleJobFailed(activeJobId, 'Timed out waiting for async job completion');
        return;
      }
      pollTimer = setTimeout(pollJobStatus, 2000);
    };

    pollTimer = setTimeout(pollJobStatus, 1500);
    return () => {
      cancelled = true;
      if (pollTimer) clearTimeout(pollTimer);
      unsubscribe();
    };
  }, [activeJobId, subscribeToSSE, handleJobCompleted, handleJobFailed]);

  useEffect(() => {
    if (typeof window === 'undefined' || agentsList.length === 0) return;
    const requestedAgent = new URLSearchParams(window.location.search).get('agent');
    if (!requestedAgent) return;
    const matched = agentsList.find((a) => a.id === requestedAgent || a.name === requestedAgent);
    if (matched) form.setValue('agentId', matched.name);
  }, [agentsList, form]);

  useEffect(() => {
    if (keysList.length > 0) {
      const currentPrefix = form.getValues('keyPrefix');
      if (!currentPrefix || currentPrefix === 'auto' || !keysList.some((k) => k.keyPrefix === currentPrefix)) {
        form.setValue('keyPrefix', keysList[0].keyPrefix);
      }
    }
  }, [keysList, form]);

  const selectedModel = useWatch({ control: form.control, name: 'model' });
  const enableAsyncWatched = useWatch({ control: form.control, name: 'enableAsync' });
  const isAsyncMode = Boolean(enableAsyncWatched) || activeJobId !== null;

  const handleCopyOutput = () => {
    if (!executionOutput) return;
    navigator.clipboard.writeText(executionOutput);
    setCopied(true);
    toast.success('Result copied to clipboard');
    setTimeout(() => setCopied(false), 2000);
  };

  const onSubmit = async (values: SandboxFormValues) => {
    const savedPrompt = values.userPrompt;
    completedJobRef.current = null;
    setExecutionOutput('');
    setLatencyMs(null);
    setTokenStats(null);
    setRoutedModel(null);
    const startTime = Date.now();

    try {
      if (values.enableAsync) {
        jobStartTimeRef.current = startTime;
        setIsAsyncExecuting(true);
        const asyncJob = await apiExecuteSandboxChatAsync({
          keyPrefix: values.keyPrefix,
          routingPolicy: values.routingPolicy,
          agentId: values.agentId,
          model: values.model,
          messages: [{ role: 'user', content: savedPrompt }],
          temperature: 0.7,
          stream: false,
        }).catch((asyncErr) => {
          setIsAsyncExecuting(false);
          form.setValue('userPrompt', savedPrompt);
          throw asyncErr;
        });
        setActiveJobId(asyncJob.job_id);
        toast.info(`Async Job ${asyncJob.job_id} queued successfully`);
        return;
      }

      const res = await sandboxMutation.mutateAsync({
        keyPrefix: values.keyPrefix,
        routingPolicy: values.routingPolicy,
        agentId: values.agentId,
        model: values.model,
        messages: [{ role: 'user', content: savedPrompt }],
        temperature: 0.7,
        stream: values.enableStream,
      });

      const selectedHeaderModel = res.headers?.get('X-Prism-Model');
      if (selectedHeaderModel) setRoutedModel(selectedHeaderModel);
      setRoutedPolicy(res.headers?.get('X-Prism-Routing-Policy') || null);

      if (values.enableStream && res.body) {
        const { hasOutput } = await consumeSandboxStream(res.body, {
          onModel: setRoutedModel,
          onTokenStats: setTokenStats,
          onOutputUpdate: setExecutionOutput,
        });
        if (!hasOutput) {
          setExecutionOutput('No output content returned by model stream.');
        }
        setLatencyMs(Date.now() - startTime);
        form.setValue('userPrompt', '');
        toast.success('Stream execution completed');
      } else {
        const data = await res.json();
        if (data.model) setRoutedModel(data.model);
        const stats = parseSandboxTokenStats(data.usage);
        if (stats) setTokenStats(stats);
        setLatencyMs(Date.now() - startTime);
        form.setValue('userPrompt', '');
        toast.success('Sandbox execution completed');
        typewrite(formatSandboxJsonResult(data));
      }
    } catch (err: unknown) {
      setIsAsyncExecuting(false);
      setExecutionOutput(`[System Exception]\n${getErrorMessage(err)}`);
      form.setValue('userPrompt', savedPrompt);
      toast.error('Execution failed');
    }
  };

  return (
    <TooltipProvider>
      <AppLayout>
        <PageHeader
          title="Developer Web Sandbox"
          description="Isolated execution container for prompt evaluation, real-time AI responses, and boundary safety testing."
        />
        <div className="grid grid-cols-1 xl:grid-cols-2 gap-6 min-w-0 flex-1">
          <SandboxControlsForm
            form={form}
            onSubmit={onSubmit}
            modelsList={modelsList}
            policiesList={policiesList}
            keysList={keysList}
            agentsList={agentsList}
            isExecuting={isExecuting}
            isAsyncMode={isAsyncMode}
            selectedModel={selectedModel}
          />
          <SandboxOutputConsole
            executionOutput={executionOutput}
            routedModel={routedModel}
            routedPolicy={routedPolicy}
            latencyMs={latencyMs}
            tokenStats={tokenStats}
            isExecuting={isExecuting}
            isAsyncMode={isAsyncMode}
            copied={copied}
            onCopyOutput={handleCopyOutput}
          />
        </div>
      </AppLayout>
    </TooltipProvider>
  );
}
