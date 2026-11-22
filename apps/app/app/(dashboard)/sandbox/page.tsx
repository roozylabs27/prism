'use client';

import React, { useState } from 'react';
import { useForm } from 'react-hook-form';
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
import { getErrorMessage } from '@/types/ui';
import { toast } from 'sonner';

export default function SandboxPage() {
  const { data: modelsData } = useModelsListQuery();
  const { data: agentsData } = useAgentsQuery();
  const { data: keysData } = useAllGatewayKeysQuery();
  const { data: policiesData } = usePoliciesQuery();

  const modelsList: ApiModel[] = Array.isArray(modelsData)
    ? modelsData
    : (modelsData as unknown as { data?: ApiModel[] })?.data ?? [];
  const agentsList = Array.isArray(agentsData) ? agentsData : [];
  const keysList = keysData ?? [];
  const policiesList = Array.isArray(policiesData) ? policiesData : [];

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

  const typewriterRef = React.useRef<ReturnType<typeof setTimeout> | null>(null);
  const completedJobRef = React.useRef<string | null>(null);
  const jobStartTimeRef = React.useRef<number>(0);

  const typewrite = React.useCallback((text: string, onComplete?: () => void) => {
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

  React.useEffect(() => {
    if (policiesList.length === 0) return;
    const current = form.getValues('routingPolicy');
    if (policiesList.some((p) => p.id === current)) return;
    const preferred = policiesList.find((p) => p.isDefault) ?? policiesList[0];
    form.setValue('routingPolicy', preferred.id);
  }, [policiesList, form]);

  const handleJobCompleted = React.useCallback(
    (jobId: string, result: Record<string, unknown> | null | undefined, model?: string, startTime?: number) => {
      if (completedJobRef.current === jobId) return;
      completedJobRef.current = jobId;

      const choices = result?.choices as Array<{ message?: { content?: string } }> | undefined;
      const usage = result?.usage as { prompt_tokens?: number; completion_tokens?: number } | undefined;
      const content = choices?.[0]?.message?.content || (result ? JSON.stringify(result, null, 2) : '');

      if (usage) {
        setTokenStats({
          input: usage.prompt_tokens || 0,
          output: usage.completion_tokens || 0,
        });
      }
      if (model) setRoutedModel(model);
      if (startTime) setLatencyMs(Date.now() - startTime);

      toast.success('Async job completed');
      setActiveJobId(null);
      setIsAsyncExecuting(false);
      form.setValue('userPrompt', '');
      typewrite(content);
    },
    [typewrite, form]
  );

  const handleJobFailed = React.useCallback((jobId: string, errorMessage: string) => {
    if (completedJobRef.current === jobId) return;
    completedJobRef.current = jobId;
    setExecutionOutput(`Execution Failed:\n${errorMessage}`);
    toast.error(`Async Job ${jobId} failed: ${errorMessage}`);
    setActiveJobId(null);
    setIsAsyncExecuting(false);
  }, []);

  React.useEffect(() => {
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

  React.useEffect(() => {
    if (typeof window === 'undefined' || agentsList.length === 0) return;
    const requestedAgent = new URLSearchParams(window.location.search).get('agent');
    if (!requestedAgent) return;
    const matched = agentsList.find((a) => a.id === requestedAgent || a.name === requestedAgent);
    if (matched) form.setValue('agentId', matched.name);
  }, [agentsList, form]);

  React.useEffect(() => {
    if (keysList.length > 0) {
      const currentPrefix = form.getValues('keyPrefix');
      if (!currentPrefix || currentPrefix === 'auto' || !keysList.some((k) => k.keyPrefix === currentPrefix)) {
        form.setValue('keyPrefix', keysList[0].keyPrefix);
      }
    }
  }, [keysList, form]);

  const selectedModel = form.watch('model');

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
        const reader = res.body.getReader();
        const decoder = new TextDecoder('utf-8');
        let accumulatedContent = '';
        let accumulatedReasoning = '';
        const toolCallsMap: Record<number, { name: string; args: string }> = {};
        let buffer = '';
        let doneReading = false;

        const updateDisplay = () => {
          let output = '';
          if (accumulatedReasoning) {
            output += `> **Thinking:**\n> ${accumulatedReasoning.replace(/\n/g, '\n> ')}\n\n`;
          }
          if (accumulatedContent) output += accumulatedContent;
          const toolCallEntries = Object.values(toolCallsMap);
          if (toolCallEntries.length > 0) {
            output += (output ? '\n\n' : '') + '### 🔧 Tool Invocations\n';
            for (const tc of toolCallEntries) {
              output += `\`\`\`json\n// Invoking ${tc.name || 'Tool'}\n${tc.args || '{}'}\n\`\`\`\n`;
            }
          }
          setExecutionOutput(output);
        };

        while (!doneReading) {
          const { value, done } = await reader.read();
          if (done) break;
          buffer += decoder.decode(value, { stream: true });
          const lines = buffer.split('\n');
          buffer = lines.pop() || '';

          for (const line of lines) {
            const trimmed = line.trim();
            if (!trimmed || !trimmed.startsWith('data: ')) continue;
            const dataStr = trimmed.replace(/^data:\s*/, '');
            if (dataStr === '[DONE]') {
              doneReading = true;
              break;
            }
            try {
              const parsed = JSON.parse(dataStr);
              if (parsed.model) setRoutedModel(parsed.model);
              const delta = parsed.choices?.[0]?.delta;
              if (delta) {
                if (typeof delta.content === 'string') accumulatedContent += delta.content;
                if (typeof delta.reasoning_content === 'string') accumulatedReasoning += delta.reasoning_content;
                if (typeof delta.thinking === 'string') accumulatedReasoning += delta.thinking;
                if (Array.isArray(delta.tool_calls)) {
                  for (const tc of delta.tool_calls) {
                    const idx = tc.index ?? 0;
                    if (!toolCallsMap[idx]) toolCallsMap[idx] = { name: '', args: '' };
                    if (tc.function?.name) toolCallsMap[idx].name += tc.function.name;
                    if (tc.function?.arguments) toolCallsMap[idx].args += tc.function.arguments;
                  }
                }
                updateDisplay();
              }
              if (parsed.usage) {
                setTokenStats({
                  input: parsed.usage.prompt_tokens || 0,
                  output: parsed.usage.completion_tokens || 0,
                });
              }
            } catch {
              // Ignore partial SSE JSON chunks
            }
          }
        }

        if (buffer.trim().startsWith('data: ')) {
          const dataStr = buffer.trim().replace(/^data:\s*/, '');
          if (dataStr !== '[DONE]') {
            try {
              const parsed = JSON.parse(dataStr);
              if (parsed.model) setRoutedModel(parsed.model);
              const delta = parsed.choices?.[0]?.delta;
              if (delta?.content) accumulatedContent += delta.content;
              if (delta?.reasoning_content) accumulatedReasoning += delta.reasoning_content;
              updateDisplay();
            } catch {
              /* ignore malformed trailing frame */
            }
          }
        }

        updateDisplay();
        if (!accumulatedContent && !accumulatedReasoning && Object.keys(toolCallsMap).length === 0) {
          setExecutionOutput('No output content returned by model stream.');
        }
        setLatencyMs(Date.now() - startTime);
        form.setValue('userPrompt', '');
        toast.success('Stream execution completed');
      } else {
        const data = await res.json();
        if (data.model) setRoutedModel(data.model);
        let choice = data.choices?.[0]?.message?.content;
        const toolCalls = data.choices?.[0]?.message?.tool_calls;
        if (!choice && toolCalls && Array.isArray(toolCalls)) {
          choice =
            '### 🔧 Tool Invocations\n' +
            toolCalls
              .map(
                (tc: { function?: { name?: string; arguments?: string } }) =>
                  `\`\`\`json\n// Invoking ${tc.function?.name || 'Tool'}\n${tc.function?.arguments || '{}'}\n\`\`\``
              )
              .join('\n\n');
        }
        if (!choice) choice = JSON.stringify(data, null, 2);
        if (data.usage) {
          setTokenStats({
            input: data.usage.prompt_tokens || 0,
            output: data.usage.completion_tokens || 0,
          });
        }
        setLatencyMs(Date.now() - startTime);
        form.setValue('userPrompt', '');
        toast.success('Sandbox execution completed');
        typewrite(choice);
      }
    } catch (err: unknown) {
      setIsAsyncExecuting(false);
      setExecutionOutput(`[System Exception]\n${getErrorMessage(err)}`);
      form.setValue('userPrompt', savedPrompt);
      toast.error('Execution failed');
    }
  };

  const isAsyncMode = form.watch('enableAsync') || activeJobId !== null;

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
