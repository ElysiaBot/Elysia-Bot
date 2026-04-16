import { describe, expect, it } from 'vitest';
import { parseConsolePayload } from './consolePayload';

const basePlugin = {
  id: 'plugin-echo',
  name: 'Echo Plugin',
  version: '0.1.0',
  apiVersion: 'v0',
  mode: 'subprocess',
  permissions: ['message:read'],
  entry: { module: 'plugins/plugin-echo' },
};

const basePayload = {
  status: { adapters: 1, plugins: 1, jobs: 1, schedules: 1 },
  plugins: [basePlugin],
  jobs: [
    {
      id: 'job-console',
      type: 'ai.call',
      status: 'pending',
      retryCount: 0,
      maxRetries: 1,
      timeout: 30_000_000_000,
      lastError: '',
      createdAt: '2026-04-06T00:00:00Z',
      deadLetter: false,
      correlation: 'corr-console',
    },
  ],
  schedules: [
    {
      id: 'schedule-console',
      kind: 'delay',
      source: 'runtime-demo-scheduler',
      eventType: 'message.received',
      cronExpr: '',
      delayMs: 30_000,
      executeAt: null,
      dueAt: '2026-04-06T00:00:30Z',
      createdAt: '2026-04-06T00:00:00Z',
      updatedAt: '2026-04-06T00:00:00Z',
    },
  ],
  logs: ['runtime started'],
  config: {
    Runtime: {
      Environment: 'development',
      LogLevel: 'debug',
      HTTPPort: 8080,
    },
  },
  meta: {
    runtime_entry: 'apps/runtime',
    demo_paths: ['/demo/onebot/message'],
    sqlite_path: 'data/dev/runtime.sqlite',
    scheduler_interval_ms: 100,
    console_mode: 'read-only',
  },
  observability: {
    jobDispatchReady: 1,
    scheduleDueReady: 0,
  },
};

describe('parseConsolePayload', () => {
  it('accepts plugin lastDispatchAt when it is a string timestamp', () => {
    const parsed = parseConsolePayload({
      ...basePayload,
      plugins: [{ ...basePlugin, lastDispatchAt: '2026-04-05T23:59:00Z' }],
    });

    expect(parsed.plugins[0]?.lastDispatchAt).toBe('2026-04-05T23:59:00Z');
  });

  it('keeps plugin lastDispatchAt optional when it is missing', () => {
    const parsed = parseConsolePayload(basePayload);

    expect(parsed.plugins[0]?.lastDispatchAt).toBeUndefined();
  });

  it('rejects plugin lastDispatchAt when it is not a string', () => {
    expect(() =>
      parseConsolePayload({
        ...basePayload,
        plugins: [{ ...basePlugin, lastDispatchAt: 123 }],
      }),
    ).toThrow('Console API payload shape is incompatible with Console Web v0');
  });
});
