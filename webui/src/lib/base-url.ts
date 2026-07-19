const baseURL = new URL('.', document.baseURI)

function relativePath(path: string): string {
  return path.replace(/^\/+/, '')
}

// applicationURL keeps all browser requests below api.base_path when one is set.
export function applicationURL(path = ''): string {
  return new URL(relativePath(path), baseURL).toString()
}

export function applicationBasePath(): string {
  return baseURL.pathname
}

export function apiURL(path = ''): string {
  return applicationURL(path ? `api/${relativePath(path)}` : 'api')
}

export function websocketURL(path: string): string {
  const url = new URL(applicationURL(path))
  url.protocol = url.protocol === 'https:' ? 'wss:' : 'ws:'
  return url.toString()
}

export function assetURL(path: string): string {
  return applicationURL(path)
}
