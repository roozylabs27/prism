'use client';

import { useEffect, useState } from 'react';
import Link from 'next/link';
import { AuthBackgroundAnimation } from '@/components/molecules/AuthBackgroundAnimation';
import { ThemeToggle } from '@/components/molecules/ThemeToggle';
import { Card, CardHeader, CardTitle, CardDescription, CardContent, CardFooter } from '@/components/molecules/Card';
import { Button } from '@/components/atoms/Button';
import { Badge } from '@/components/atoms/Badge';
import { PrismLogo } from '@/components/icons';
import { AlertTriangle, RefreshCw, Home, ChevronDown, ChevronUp, Activity } from 'lucide-react';

export default function ErrorBoundaryPage({
  error,
  reset,
}: {
  error: Error & { digest?: string };
  reset: () => void;
}) {
  const [showDetails, setShowDetails] = useState(false);

  useEffect(() => {
    // Log unexpected client exceptions to console/telemetry
    console.error('[Prism App Crash Caught]', error);
  }, [error]);

  return (
    <div className="relative flex min-h-screen flex-col items-center justify-center bg-background p-4 sm:p-6 lg:p-8 overflow-hidden">
      {/* Dynamic AI & Mesh Animated Background */}
      <AuthBackgroundAnimation />

      {/* Top Header Controls */}
      <div className="absolute right-4 top-4 z-20 flex items-center gap-2">
        <ThemeToggle />
      </div>

      {/* Brand Header */}
      <div className="relative z-10 mb-6 flex items-center gap-2.5">
        <PrismLogo className="h-7 w-7 shrink-0 text-[#6D28D9] dark:text-[#B49CFB]" />
        <div className="flex flex-col">
          <span className="font-mono text-lg font-bold tracking-tight text-foreground">
            RoozyLabs <span className="text-red-400">Prism</span>
          </span>
          <span className="text-[10px] font-semibold uppercase tracking-wider text-muted-foreground font-mono">
            Gateway Resilience System Active
          </span>
        </div>
      </div>

      {/* Main Error Recovery Card */}
      <div className="relative z-10 w-full max-w-lg">
        <Card className="border-red-500/30 shadow-2xl backdrop-blur-sm bg-card/95">
          <CardHeader className="text-center space-y-2 pb-4">
            <div className="flex justify-center mb-1">
              <Badge variant="destructive" className="font-mono text-xs px-3 py-0.5 gap-1.5">
                <AlertTriangle className="h-3.5 w-3.5" />
                <span>RUNTIME EXCEPTION • 500</span>
              </Badge>
            </div>
            <CardTitle className="text-xl font-bold tracking-tight text-foreground">
              Gateway Execution Interrupted
            </CardTitle>
            <CardDescription className="text-xs text-muted-foreground max-w-sm mx-auto">
              An unexpected operational fault was intercepted. System telemetry has logged this event with trace isolation.
            </CardDescription>
          </CardHeader>

          <CardContent className="space-y-4">
            {/* Error Message Box */}
            <div className="rounded-lg border border-rose-500/20 bg-rose-500/5 p-3.5 font-mono text-xs">
              <div className="flex items-center justify-between text-[11px] text-muted-foreground mb-1.5">
                <span className="uppercase font-semibold text-rose-600 dark:text-rose-400">Error Digest:</span>
                <span className="text-muted-foreground">{error.digest || 'ERR_RUNTIME_FAULT'}</span>
              </div>
              <p className="font-semibold text-foreground break-all bg-background/80 p-2 rounded-md border border-border/60">
                {error.message || 'An unexpected client-side error occurred.'}
              </p>

              {/* Stack Details Toggle */}
              {error.stack && (
                <div className="mt-2.5 pt-2 border-t border-border/40">
                  <button
                    type="button"
                    onClick={() => setShowDetails(!showDetails)}
                    className="flex items-center gap-1 text-[11px] text-muted-foreground hover:text-foreground transition-colors cursor-pointer"
                  >
                    {showDetails ? <ChevronUp className="h-3 w-3" /> : <ChevronDown className="h-3 w-3" />}
                    <span>{showDetails ? 'Hide Stack Trace' : 'Inspect Technical Stack Trace'}</span>
                  </button>

                  {showDetails && (
                    <pre className="mt-2 max-h-36 overflow-auto rounded-md bg-console text-emerald-400 p-2 text-[10px] leading-relaxed border border-border">
                      {error.stack}
                    </pre>
                  )}
                </div>
              )}
            </div>

            {/* Quick Diagnostic Tips */}
            <div className="rounded-lg border border-border bg-muted/30 p-3 text-xs space-y-1.5">
              <p className="font-semibold font-mono text-[11px] text-foreground flex items-center gap-1.5">
                <Activity className="h-3.5 w-3.5 text-muted-foreground" />
                Automatic Recovery Suggestions:
              </p>
              <ul className="list-disc list-inside text-muted-foreground space-y-0.5 text-[11px]">
                <li>Re-executing the component tree may resolve transient state desynchronization.</li>
                <li>Verify your network connectivity to the proxy upstream backend.</li>
              </ul>
            </div>
          </CardContent>

          <CardFooter className="flex flex-col sm:flex-row gap-2 pt-2 border-t border-border/50">
            <Button
              type="button"
              variant="outline"
              className="w-full sm:w-1/2 gap-2 text-xs"
              onClick={() => reset()}
            >
              <RefreshCw className="h-4 w-4 text-muted-foreground" />
              Try Again
            </Button>
            <Button asChild variant="prismViolet" className="w-full sm:w-1/2 gap-2 text-xs">
              <Link href="/">
                <Home className="h-4 w-4" />
                Return to Dashboard
              </Link>
            </Button>
          </CardFooter>
        </Card>
      </div>

      {/* Footer System Info */}
      <footer className="relative z-10 mt-8 text-center text-xs text-muted-foreground font-mono">
        <span>© 2026 RoozyLabs Prism. Fault-Tolerant AI Control Plane.</span>
      </footer>
    </div>
  );
}
