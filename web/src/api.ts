const base = '/api'

async function request<T>(method: string, path: string, body?: any): Promise<T> {
  const headers: Record<string, string> = {}
  const opts: RequestInit = {
    method,
    credentials: 'same-origin',
    headers,
  }
  if (body !== undefined) {
    if (body instanceof FormData) {
      opts.body = body
    } else {
      headers['Content-Type'] = 'application/json'
      opts.body = JSON.stringify(body)
    }
  }
  const resp = await fetch(base + path, opts)
  const text = await resp.text()
  const data = text ? JSON.parse(text) : null
  if (!resp.ok) {
    throw new Error(data?.error || resp.statusText)
  }
  return data as T
}

export const api = {
  get: <T>(p: string) => request<T>('GET', p),
  post: <T>(p: string, body?: any) => request<T>('POST', p, body),
  put: <T>(p: string, body?: any) => request<T>('PUT', p, body),
  del: <T>(p: string) => request<T>('DELETE', p),
  raw: (method: string, path: string, body?: BodyInit) => fetch(base + path, { method, body, credentials: 'same-origin' }),
}