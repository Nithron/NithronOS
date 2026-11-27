import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import http from '@/lib/nos-client'

export interface SMBConfig {
  enabled: boolean
  guest?: boolean
  time_machine?: boolean
  recycle?: { enabled: boolean; directory?: string }
}

export interface NFSConfig {
  enabled: boolean
  /** hosts, addresses or networks that may mount it */
  networks?: string[]
  read_only?: boolean
}

/**
 * A share as GET /api/v1/shares lists it: /srv/shares/<name>, exported over
 * SMB and/or NFS. owners read and write, readers read; a principal is
 * "user:<name>" or "group:<name>" ("group:users" is everyone).
 */
export interface Share {
  id: string
  name: string
  path: string
  description?: string
  smb?: SMBConfig
  nfs?: NFSConfig
  owners?: string[]
  readers?: string[]
  /** installed apps (by id) that may read it, through their own group */
  apps?: string[]
  created_at: string
  updated_at: string
}

/** What a create or update sends; an update changes only what it sends. */
export interface ShareInput {
  description?: string
  smb?: SMBConfig
  nfs?: NFSConfig
  owners?: string[]
  readers?: string[]
  apps?: string[]
}

export type CreateShareRequest = ShareInput & { name: string }
export type UpdateShareRequest = ShareInput

export interface ShareProblem {
  code: string
  message: string
  field?: string
}

export interface TestShareResponse {
  valid: boolean
  errors?: ShareProblem[]
}

export interface Policy {
  shares: { guest_access_forbidden: boolean }
}

export const sharesApi = {
  list: () => http.get<Share[]>('/v1/shares'),
  create: (data: CreateShareRequest) => http.post<Share>('/v1/shares', data),
  update: (name: string, data: UpdateShareRequest) =>
    http.patch<Share>(`/v1/shares/${encodeURIComponent(name)}`, data),
  remove: (name: string) => http.del<void>(`/v1/shares/${encodeURIComponent(name)}`),
  /** Checks a share's settings without saving them. */
  test: (name: string, config: ShareInput) =>
    http.post<TestShareResponse>(`/v1/shares/${encodeURIComponent(name)}/test`, { config }),
  policy: () => http.get<Policy>('/v1/policy'),
}

const sharesKey = ['shares'] as const

export function useShares() {
  return useQuery({ queryKey: sharesKey, queryFn: sharesApi.list })
}

/** One share by name (case-insensitive, like the backend). */
export function useShare(name?: string) {
  return useQuery({
    queryKey: [...sharesKey, name],
    queryFn: async () => {
      const list = await sharesApi.list()
      return list.find((s) => s.name.toLowerCase() === name!.toLowerCase()) ?? null
    },
    enabled: !!name,
  })
}

export function useShareMutations() {
  const qc = useQueryClient()
  const refresh = () => qc.invalidateQueries({ queryKey: sharesKey })
  return {
    create: useMutation({ mutationFn: sharesApi.create, onSuccess: refresh }),
    update: useMutation({
      mutationFn: ({ name, data }: { name: string; data: UpdateShareRequest }) =>
        sharesApi.update(name, data),
      onSuccess: refresh,
    }),
    remove: useMutation({ mutationFn: sharesApi.remove, onSuccess: refresh }),
  }
}

export function usePolicy() {
  return useQuery({ queryKey: ['policy'], queryFn: sharesApi.policy, staleTime: 60_000 })
}
