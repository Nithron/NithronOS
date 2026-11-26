import http from '@/lib/nos-client'
import type { paths, components } from './schema'

type GetResponse<Path extends keyof paths> = paths[Path] extends { get: any }
	? paths[Path]['get'] extends {
			responses: { 200: { content: { 'application/json': infer T } } }
	  }
		? T
		: never
	: never

export type Health = GetResponse<'/api/health'>
export type Disks = GetResponse<'/api/disks'>
export type Pools = GetResponse<'/api/pools'>
export type PoolCandidates = GetResponse<'/api/pools/candidates'>
export type Share = components['schemas']['Share']
export type Shares = GetResponse<'/api/shares'>
export type Apps = GetResponse<'/api/apps'>
export type RemoteStatus = GetResponse<'/api/remote/status'>

// Additional types for shares
export interface SMBConfig {
	enabled: boolean
	guest?: boolean
	time_machine?: boolean
	recycle?: {
		enabled: boolean
		directory?: string
	}
}

export interface NFSConfig {
	enabled: boolean
	networks?: string[]
	read_only?: boolean
}

export interface CreateShareRequest {
	name: string
	smb?: SMBConfig
	nfs?: NFSConfig
	owners?: string[]
	readers?: string[]
	description?: string
}

export interface UpdateShareRequest {
	smb?: SMBConfig
	nfs?: NFSConfig
	owners?: string[]
	readers?: string[]
	description?: string
}

export interface TestShareResponse {
	valid: boolean
	errors?: Array<{
		code: string
		message: string
		field?: string
	}>
}

export interface User {
	username: string
	uid?: number
	groups?: string[]
}

export interface Group {
	name: string
	gid?: number
	members?: string[]
}

export interface Policy {
	shares?: {
		guest_access_forbidden?: boolean
		max_shares?: number
	}
}

type PostResponse<Path extends keyof paths> = paths[Path] extends { post: any }
	? paths[Path]['post'] extends {
			responses: { 200: { content: { 'application/json': infer T } } }
		}
		? T
		: never
	: never

export const api = {
	health: {
		get: () => http.get<Health>('/v1/health'),
	},
	disks: {
		get: () => http.get<Disks>('/v1/disks'),
	},
	pools: {
		get: () => http.get<Pools>('/v1/pools'),
		planCreateV1: (body: any) => http.post<any>('/v1/pools/plan-create', body),
		applyCreateV1: (body: any) => http.post<any>('/v1/pools/apply-create', body),
		txStatus: (id: string) => http.get<any>(`/v1/pools/tx/${id}/status`),
		txLog: (id: string, cursor = 0, max = 1000) => http.get<any>(`/v1/pools/tx/${id}/log?cursor=${cursor}&max=${max}`),
		planCreate: (body: any) => http.post<PostResponse<'/api/pools/plan-create'>>('/v1/pools/plan-create', body),
		create: (body: any) => http.post<PostResponse<'/api/pools/create'>>('/v1/pools/create', body),
		candidates: () => http.get<PoolCandidates>('/v1/pools/candidates'),
		import: (deviceOrUuid: string) => http.post<PostResponse<'/api/pools/import'>>('/v1/pools/import', { device_or_uuid: deviceOrUuid }),
		discoverV1: () => http.get<any>('/v1/pools/discover'),
		importV1: (body: { uuid: string; label?: string; mountpoint: string }) => http.post<any>('/v1/pools/import', body),
		scrubStart: (mount: string) => http.post<any>('/v1/pools/scrub/start', { mount }),
		scrubStatus: (mount: string) => http.get<any>(`/v1/pools/scrub/status?mount=${encodeURIComponent(mount)}`),
	},
	shares: {
		get: () => http.get<Share[]>('/v1/shares'),
		create: (data: CreateShareRequest) => http.post<Share>('/v1/shares', data),
		update: (name: string, data: UpdateShareRequest) => http.patch<Share>(`/v1/shares/${name}`, data),
		delete: (name: string) => http.del(`/v1/shares/${name}`),
		test: (name: string, config: any) => http.post<TestShareResponse>(`/v1/shares/${name}/test`, { config }),
	},
	remote: {
		status: () => http.get<RemoteStatus>('/v1/remote/status'),
	},
	auth: {
		me: () => http.get<any>('/v1/auth/me'),
	},
	users: {
		get: () => http.get<User[]>('/v1/users'),
	},
	groups: {
		get: () => http.get<Group[]>('/v1/groups'),
	},
	policy: {
		get: () => http.get<Policy>('/v1/policy'),
	},
	support: {
		bundle: () => http.support.bundle(),
	},
}


