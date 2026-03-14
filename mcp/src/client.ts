export class TogglerinoError extends Error {
  status: number

  constructor(status: number, message: string) {
    super(message)
    this.name = 'TogglerinoError'
    this.status = status
  }
}

export class TogglerinoClient {
  private baseUrl: string
  private apiKey: string

  constructor(baseUrl: string, apiKey: string) {
    this.baseUrl = baseUrl.replace(/\/+$/, '')
    this.apiKey = apiKey
  }

  async get<T>(path: string): Promise<T> {
    return this.request<T>(path, { method: 'GET' })
  }

  async post<T>(path: string, body?: unknown): Promise<T> {
    return this.request<T>(path, {
      method: 'POST',
      body: body ? JSON.stringify(body) : undefined,
    })
  }

  async put<T>(path: string, body?: unknown): Promise<T> {
    return this.request<T>(path, {
      method: 'PUT',
      body: body ? JSON.stringify(body) : undefined,
    })
  }

  async del<T>(path: string): Promise<T> {
    return this.request<T>(path, { method: 'DELETE' })
  }

  private async request<T>(path: string, options: RequestInit): Promise<T> {
    const url = `${this.baseUrl}/api/v1${path}`
    const res = await fetch(url, {
      ...options,
      headers: {
        'Content-Type': 'application/json',
        Authorization: `Bearer ${this.apiKey}`,
        ...options.headers,
      },
    })

    if (!res.ok) {
      const error = await res.json().catch(() => ({ error: res.statusText }))
      throw new TogglerinoError(res.status, error.error || res.statusText)
    }

    if (res.status === 204) {
      return undefined as T
    }

    return res.json()
  }
}
