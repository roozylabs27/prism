export interface SandboxStreamCallbacks {
  onModel: (model: string) => void;
  onTokenStats: (stats: { input: number; output: number }) => void;
  onOutputUpdate: (output: string) => void;
}

export function buildSandboxOutputMarkdown(
  accumulatedReasoning: string,
  accumulatedContent: string,
  toolCallsMap: Record<number, { name: string; args: string }>
): string {
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
  return output;
}

export async function consumeSandboxStream(
  body: ReadableStream<Uint8Array>,
  callbacks: SandboxStreamCallbacks
): Promise<{ hasOutput: boolean }> {
  const reader = body.getReader();
  const decoder = new TextDecoder('utf-8');
  let accumulatedContent = '';
  let accumulatedReasoning = '';
  const toolCallsMap: Record<number, { name: string; args: string }> = {};
  let buffer = '';
  let doneReading = false;

  const emitUpdate = () => {
    callbacks.onOutputUpdate(
      buildSandboxOutputMarkdown(accumulatedReasoning, accumulatedContent, toolCallsMap)
    );
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
        if (parsed.model) callbacks.onModel(parsed.model);
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
          emitUpdate();
        }
        if (parsed.usage) {
          callbacks.onTokenStats({
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
        if (parsed.model) callbacks.onModel(parsed.model);
        const delta = parsed.choices?.[0]?.delta;
        if (delta?.content) accumulatedContent += delta.content;
        if (delta?.reasoning_content) accumulatedReasoning += delta.reasoning_content;
        emitUpdate();
      } catch {
        /* ignore malformed trailing frame */
      }
    }
  }

  emitUpdate();
  const hasOutput = Boolean(accumulatedContent || accumulatedReasoning || Object.keys(toolCallsMap).length > 0);
  return { hasOutput };
}

export function formatSandboxJsonResult(data: {
  choices?: Array<{
    message?: {
      content?: string | null;
      tool_calls?: Array<{ function?: { name?: string; arguments?: string } }>;
    };
  }>;
}): string {
  let choice = data.choices?.[0]?.message?.content;
  const toolCalls = data.choices?.[0]?.message?.tool_calls;
  if (!choice && toolCalls && Array.isArray(toolCalls)) {
    choice =
      '### 🔧 Tool Invocations\n' +
      toolCalls
        .map(
          (tc) =>
            `\`\`\`json\n// Invoking ${tc.function?.name || 'Tool'}\n${tc.function?.arguments || '{}'}\n\`\`\``
        )
        .join('\n\n');
  }
  return choice || JSON.stringify(data, null, 2);
}

export function parseSandboxTokenStats(
  usage?: { prompt_tokens?: number; completion_tokens?: number } | null
): { input: number; output: number } | null {
  if (!usage) return null;
  return {
    input: usage.prompt_tokens || 0,
    output: usage.completion_tokens || 0,
  };
}

export function extractAsyncJobContent(
  result: Record<string, unknown> | null | undefined
): string {
  const choices = result?.choices as Array<{ message?: { content?: string } }> | undefined;
  return choices?.[0]?.message?.content || (result ? JSON.stringify(result, null, 2) : '');
}
