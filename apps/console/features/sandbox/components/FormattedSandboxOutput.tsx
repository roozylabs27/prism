'use client';

import React from 'react';
import { Card, CardHeader, CardTitle, CardContent } from '@/components/molecules/Card';
import { Button } from '@/components/atoms/Button';
import { Badge } from '@/components/atoms/Badge';
import { Label } from '@/components/atoms/Label';
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/atoms/Tooltip';
import {
  Check as CheckIcon,
  Clock as ClockIcon,
  Code2 as Code2Icon,
  Copy as CopyIcon,
  HelpCircle as HelpCircleIcon,
  Layers as LayersIcon,
  Loader2 as Loader2Icon,
  Terminal as TerminalIcon,
  Zap as ZapIcon,
} from 'lucide-react';
import { toast } from 'sonner';

export function FormTooltipLabel({
  label,
  tooltip,
  required,
}: {
  label: string;
  tooltip: string;
  required?: boolean;
}) {
  return (
    <div className="flex items-center gap-1.5 mb-1">
      <Label required={required} className="text-xs font-semibold text-foreground">
        {label}
      </Label>
      <Tooltip>
        <TooltipTrigger asChild>
          <button
            type="button"
            className="text-muted-foreground hover:text-primary transition-colors focus:outline-none"
          >
            <HelpCircleIcon className="h-3.5 w-3.5" />
          </button>
        </TooltipTrigger>
        <TooltipContent
          side="top"
          className="max-w-xs text-[11px] leading-relaxed bg-console-header border-slate-800 text-slate-200 shadow-xl"
        >
          {tooltip}
        </TooltipContent>
      </Tooltip>
    </div>
  );
}

function renderInlineMarkdown(text: string): React.ReactNode {
  if (!text) return text;
  const tokens = text.split(/(\*\*\*.*?\*\*\*|\*\*.*?\*\*|__.*?__|`.*?`|\*.*?\*|_.*?_)/g);
  return tokens.map((token, i) => {
    if (token.startsWith('***') && token.endsWith('***') && token.length >= 6) {
      return (
        <strong key={i} className="font-semibold italic text-slate-100">
          {token.slice(3, -3)}
        </strong>
      );
    }
    if (
      (token.startsWith('**') && token.endsWith('**') && token.length >= 4) ||
      (token.startsWith('__') && token.endsWith('__') && token.length >= 4)
    ) {
      return (
        <strong key={i} className="font-semibold text-slate-100">
          {token.slice(2, -2)}
        </strong>
      );
    }
    if (token.startsWith('`') && token.endsWith('`') && token.length >= 2) {
      return (
        <code key={i} className="px-1.5 py-0.5 rounded bg-slate-800 text-prism-cyan font-mono text-[11px]">
          {token.slice(1, -1)}
        </code>
      );
    }
    if (
      (token.startsWith('*') && token.endsWith('*') && token.length >= 2) ||
      (token.startsWith('_') && token.endsWith('_') && token.length >= 2)
    ) {
      return (
        <em key={i} className="italic opacity-90">
          {token.slice(1, -1)}
        </em>
      );
    }
    return token;
  });
}

export function FormattedSandboxOutput({ content }: { content: string }) {
  if (!content) return null;
  const parts = content.split(/(```[\s\S]*?```)/g);

  return (
    <div className="space-y-2.5 text-xs leading-relaxed text-slate-200">
      {parts.map((part, index) => {
        if (part.startsWith('```')) {
          const firstLineEnd = part.indexOf('\n');
          const language = part.slice(3, firstLineEnd).trim() || 'code';
          const codeBody = part.slice(firstLineEnd + 1, -3).trim();

          return (
            <div
              key={index}
              className="my-3 rounded-md border border-slate-800 bg-console-card overflow-hidden shadow-md"
            >
              <div className="flex items-center justify-between px-3 py-1.5 bg-console-header border-b border-slate-800 text-[11px]">
                <div className="flex items-center gap-1.5 text-slate-100 font-semibold font-mono">
                  <Code2Icon className="h-3.5 w-3.5" />
                  <span className="uppercase tracking-wider">{language}</span>
                </div>
                <Button
                  type="button"
                  variant="ghost"
                  size="sm"
                  className="h-6 px-1.5 text-[10px] text-muted-foreground hover:text-foreground gap-1"
                  onClick={() => {
                    navigator.clipboard.writeText(codeBody);
                    toast.success(`Copied ${language} code block`);
                  }}
                >
                  <CopyIcon className="h-3 w-3 text-slate-100" />
                  <span>Copy</span>
                </Button>
              </div>
              <pre className="p-3 overflow-x-auto text-prism-cyan whitespace-pre font-mono text-[11px] leading-relaxed custom-scrollbar">
                {codeBody}
              </pre>
            </div>
          );
        }

        const lines = part.split('\n');
        return (
          <div key={index} className="space-y-1.5">
            {lines.map((line, lineIdx) => {
              const trimmed = line.trim();
              if (/^#{1,6}\s+/.test(trimmed)) {
                const headingText = trimmed.replace(/^#{1,6}\s+/, '');
                return (
                  <h4
                    key={lineIdx}
                    className="text-slate-100 font-bold text-xs pt-3 pb-1 border-b border-slate-800 flex items-center gap-1.5"
                  >
                    <span className="text-prism-cyan">#</span>
                    <span>{renderInlineMarkdown(headingText)}</span>
                  </h4>
                );
              }
              if (trimmed === '---' || trimmed === '***') {
                return <hr key={lineIdx} className="border-slate-800 my-3" />;
              }
              const numMatch = trimmed.match(/^(\d+)\.\s+(.*)$/);
              if (numMatch) {
                return (
                  <div key={lineIdx} className="flex items-start gap-2 pl-2 text-slate-200">
                    <span className="text-slate-100 font-semibold min-w-[16px] text-[11px] font-mono">
                      {numMatch[1]}.
                    </span>
                    <div className="flex-1">{renderInlineMarkdown(numMatch[2])}</div>
                  </div>
                );
              }
              if (trimmed.startsWith('* ') || trimmed.startsWith('- ')) {
                const itemText = trimmed.replace(/^[*|-]\s*/, '');
                return (
                  <div key={lineIdx} className="flex items-start gap-2 pl-2 text-slate-200">
                    <span className="text-slate-100 font-bold mt-0.5">•</span>
                    <div className="flex-1">{renderInlineMarkdown(itemText)}</div>
                  </div>
                );
              }
              if (!trimmed) {
                return <div key={lineIdx} className="h-1" />;
              }
              return (
                <p key={lineIdx} className="text-slate-300 text-xs leading-normal">
                  {renderInlineMarkdown(line)}
                </p>
              );
            })}
          </div>
        );
      })}
    </div>
  );
}

interface SandboxOutputConsoleProps {
  executionOutput: string;
  routedModel: string | null;
  routedPolicy: string | null;
  latencyMs: number | null;
  tokenStats: { input: number; output: number } | null;
  isExecuting: boolean;
  isAsyncMode: boolean;
  copied: boolean;
  onCopyOutput: () => void;
}

export function SandboxOutputConsole({
  executionOutput,
  routedModel,
  routedPolicy,
  latencyMs,
  tokenStats,
  isExecuting,
  isAsyncMode,
  copied,
  onCopyOutput,
}: SandboxOutputConsoleProps) {
  return (
    <Card className="flex flex-col min-w-0 overflow-hidden border-border/70 shadow-lg">
      <CardHeader className="pb-3 border-b border-border/40 bg-card/50">
        <div className="flex items-center justify-between">
          <CardTitle className="text-sm font-semibold flex items-center gap-2">
            <TerminalIcon className="h-4 w-4 text-primary" />
            <span>Execution Output Console</span>
          </CardTitle>
          <div className="flex items-center gap-2">
            {routedModel && (
              <Badge variant="violet" className="font-mono text-[10px] gap-1 border-slate-800">
                <LayersIcon className="h-3 w-3" />
                Model: {routedModel}
              </Badge>
            )}
            {routedPolicy && (
              <Badge variant="outline" className="font-mono text-[10px] gap-1">
                Policy: {routedPolicy}
              </Badge>
            )}
            {latencyMs != null && (
              <Badge variant="violet" className="font-mono text-[10px] gap-1">
                <ClockIcon className="h-3 w-3" />
                {latencyMs} ms
              </Badge>
            )}
            {tokenStats && (
              <Badge variant="outline" className="font-mono text-[10px] gap-1">
                <ZapIcon className="h-3 w-3 text-amber-400" />
                {tokenStats.input + tokenStats.output} tokens
              </Badge>
            )}
            {Boolean(executionOutput) && (
              <Button
                variant="outline"
                size="sm"
                className="h-7 px-2.5 text-xs gap-1.5 border-slate-800 hover:border-slate-700"
                onClick={onCopyOutput}
                disabled={isExecuting}
              >
                {copied ? (
                  <CheckIcon className="h-3.5 w-3.5 text-emerald-700 dark:text-emerald-400" />
                ) : (
                  <CopyIcon className="h-3.5 w-3.5 text-primary" />
                )}
                <span>{copied ? 'Copied!' : 'Copy Result'}</span>
              </Button>
            )}
          </div>
        </div>
      </CardHeader>

      <CardContent className="flex-1 flex flex-col pt-4">
        <div className="flex-1 w-full max-h-[580px] min-h-[420px] p-4 border border-border/80 bg-console font-mono text-xs overflow-y-auto custom-scrollbar shadow-inner relative">
          {isExecuting && !executionOutput ? (
            <div className="h-full flex flex-col items-center justify-center space-y-4 p-6 text-center">
              <Loader2Icon className="h-8 w-8 text-slate-100 animate-spin" />
              <div className="space-y-1">
                <p className="text-xs font-medium text-foreground">
                  {isAsyncMode ? 'Processing in background...' : 'Generating response...'}
                </p>
                <p className="text-[11px] text-muted-foreground">
                  {isAsyncMode
                    ? 'Your request is queued and being processed.'
                    : 'Connecting to AI model and waiting for output.'}
                </p>
              </div>
            </div>
          ) : executionOutput ? (
            <div>
              <FormattedSandboxOutput content={executionOutput} />
              {isExecuting && (
                <span className="inline-block w-2 h-4 bg-slate-100 animate-pulse ml-1 align-middle rounded-sm" />
              )}
            </div>
          ) : (
            <div className="h-full min-h-[360px] flex flex-col items-center justify-center text-center p-6 text-muted-foreground space-y-3">
              <TerminalIcon className="h-10 w-10 text-muted-foreground/30" />
              <div className="space-y-1 max-w-xs">
                <p className="text-xs font-semibold text-slate-300">Console Ready</p>
                <p className="text-[11px] text-muted-foreground">
                  Type your prompt in the left panel and click &quot;Run Sandbox Container&quot; to view AI output.
                </p>
              </div>
            </div>
          )}
        </div>
      </CardContent>
    </Card>
  );
}
