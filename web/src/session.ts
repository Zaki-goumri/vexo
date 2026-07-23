import { reactive } from 'vue'
import { getSession } from './api'

export const session = reactive<{ authenticated: boolean; username: string }>({
  authenticated: false,
  username: '',
})

export async function refreshSession(): Promise<boolean> {
  try {
    const data = await getSession()
    session.authenticated = data.authenticated
    session.username = data.username || ''
  } catch {
    session.authenticated = false
    session.username = ''
  }
  return session.authenticated
}

export function clearSession() {
  session.authenticated = false
  session.username = ''
}
