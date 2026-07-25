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
  patch: <T>(p: string, body?: any) => request<T>('PATCH', p, body),
  del: <T>(p: string) => request<T>('DELETE', p),
  raw: (method: string, path: string, body?: BodyInit) => fetch(base + path, { method, body, credentials: 'same-origin' }),
}

// ---- Typed domain types (field casing verified against Go json tags) ----

export interface Session {
  authenticated: boolean
  username?: string
}

export interface Bucket {
  Name: string
  Versioning: boolean
  Public: boolean
  Lifecycle: number[] | null
  Tags: Record<string, string> | null
  CreatedAt: string
  CreatedBy: string
}

export interface ObjectMeta {
  id: string
  bucket: string
  key: string
  size: number
  etag: string
  contentType: string
  tier: string
  createdAt: string
  lastAccessedAt: string
  accessCount: number
}

export interface User {
  username: string
  status: string
  groups: string[]
  policies: string[]
  createdAt: string
  updatedAt: string
}

export interface KeyInfo {
  accessKey: string
  username: string
  status: string
  createdAt: string
}

export interface CreatedKey {
  accessKey: string
  secret: string
}

export interface PolicyStatement {
  Effect: string
  Action: string[]
  Resource: string[]
}

export interface Policy {
  Version: string
  Name: string
  Statement: PolicyStatement[]
}

// ---- Typed endpoint functions ----

export const login = (username: string, password: string) =>
  api.post<{ username: string }>('/login', { username, password })
export const logout = () => api.post<{ status: string }>('/logout')
export const getSession = () => api.get<Session>('/session')

export const listBuckets = () => api.get<Bucket[] | null>('/buckets').then((v) => v || [])
export const createBucket = (name: string) => api.post<Bucket>('/buckets', { name })
export const getBucket = (name: string) => api.get<Bucket>(`/buckets/${encodeURIComponent(name)}`)
export const deleteBucket = (name: string) => api.del<{ status: string }>(`/buckets/${encodeURIComponent(name)}`)
export const setBucketPublic = (name: string, isPublic: boolean) =>
  api.patch<Bucket>(`/buckets/${encodeURIComponent(name)}/visibility`, { public: isPublic })

export const listObjects = (bucket: string, prefix: string) =>
  api.get<ObjectMeta[] | null>(`/buckets/${encodeURIComponent(bucket)}/objects?prefix=${encodeURIComponent(prefix)}`).then((v) => v || [])
export const putObject = (bucket: string, key: string, file: File) =>
  api.raw('PUT', `/buckets/${encodeURIComponent(bucket)}/objects/${key}`, file)
export const getObject = (bucket: string, key: string) =>
  api.raw('GET', `/buckets/${encodeURIComponent(bucket)}/objects/${key}`)
export const deleteObject = (bucket: string, key: string) => {
  const encodedKey = encodeURIComponent(key).replace(/%2F/g, '/')
  return api.del<{ status: string }>(`/buckets/${encodeURIComponent(bucket)}/objects/${encodedKey}`)
}
export const renameObject = (bucket: string, oldKey: string, newKey: string) =>
  api.post<ObjectMeta>(`/buckets/${encodeURIComponent(bucket)}/objects/rename`, { oldKey, newKey })
export const shareObject = (bucket: string, key: string, expiresInSeconds: number) =>
  api.post<{ url: string; expiresAt: string }>(`/buckets/${encodeURIComponent(bucket)}/objects/share`, { key, expiresInSeconds })

export const listUsers = () => api.get<User[] | null>('/users').then((v) => v || [])
export const createUser = (username: string, password: string) => api.post<User>('/users', { username, password })
export const deleteUser = (name: string) => api.del<{ status: string }>(`/users/${encodeURIComponent(name)}`)

export const listKeys = (username: string) => api.get<KeyInfo[] | null>(`/users/${encodeURIComponent(username)}/keys`).then((v) => v || [])
export const createKey = (username: string) => api.post<CreatedKey>(`/users/${encodeURIComponent(username)}/keys`)
export const deleteKey = (accessKey: string) => api.del<{ status: string }>(`/keys/${encodeURIComponent(accessKey)}`)
export const setUserPolicies = (username: string, policies: string[]) =>
  api.patch<{ status: string }>(`/users/${encodeURIComponent(username)}/policies`, { policies })

export const listPolicies = () => api.get<Policy[] | null>('/policies').then((v) => v || [])
export const createPolicy = (name: string, statement: PolicyStatement[]) =>
  api.post<Policy>('/policies', { name, statement })
export const updatePolicy = (name: string, statement: PolicyStatement[]) =>
  api.put<Policy>(`/policies/${encodeURIComponent(name)}`, { statement })
export const deletePolicy = (name: string) => api.del<{ status: string }>(`/policies/${encodeURIComponent(name)}`)
