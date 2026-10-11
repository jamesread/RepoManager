import { createClient } from '@connectrpc/connect'
import { createConnectTransport } from '@connectrpc/connect-web'
import { RepoManagerService } from '../../gen/repomanager/v1/repomanager_pb.ts'

const transport = createConnectTransport({
  baseUrl: '',
  credentials: 'include',
  fetch: (input, init) => {
    const headers = new Headers(init?.headers)
    const token = localStorage.getItem('repomanager-token')
    if (token) {
      headers.set('Session-Token', token)
    }
    return fetch(input, { ...init, headers })
  },
})

export const client = createClient(RepoManagerService, transport)
