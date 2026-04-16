const consoleApiURL = import.meta.env.VITE_CONSOLE_API_URL ?? '/api/console';

export function getConsoleApiURL(): string {
  return consoleApiURL;
}

export function buildConsoleRequestURL(origin: string, logQuery: string, jobQuery: string, pluginID: string): URL {
  const requestURL = new URL(consoleApiURL, origin);
  const normalizedLogQuery = logQuery.trim();
  const normalizedJobQuery = jobQuery.trim();
  const normalizedPluginID = pluginID.trim();

  if (normalizedLogQuery !== '') {
    requestURL.searchParams.set('log_query', normalizedLogQuery);
  }
  if (normalizedJobQuery !== '') {
    requestURL.searchParams.set('job_query', normalizedJobQuery);
  }
  if (normalizedPluginID !== '') {
    requestURL.searchParams.set('plugin_id', normalizedPluginID);
  }

  return requestURL;
}
