import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import http from '@/lib/nos-client'

/** A role: admins manage the system, users reach what they are given. */
export type Role = 'admin' | 'user'

/** A user as GET /api/v1/users returns it. */
export interface UserAccount {
	id: string
	username: string
	roles: Role[]
	created_at: string
	updated_at: string
	last_login_at?: string
	two_factor_enabled: boolean
	/** Set while failed sign-ins keep the user out. */
	locked_until?: string
}

export interface CreateUserRequest {
	username: string
	password: string
	roles: Role[]
}

/** 3-32 characters: lowercase letters, digits, '-' and '_', starting with a letter or '_'. */
export const USERNAME_PATTERN = /^[a-z_][a-z0-9_-]{2,31}$/

/** What is wrong with a new password, or null (the backend's rule). */
export function passwordProblem(p: string): string | null {
	if (p.length < 8) return 'At least 8 characters'
	if (p.length > 256) return 'At most 256 characters'
	if (/[\r\n\0]/.test(p)) return "Can't contain line breaks"
	return null
}

export const usersApi = {
	list: () => http.get<UserAccount[]>('/v1/users'),
	create: (body: CreateUserRequest) => http.post<UserAccount>('/v1/users', body),
	setRoles: (id: string, roles: Role[]) => http.put<UserAccount>(`/v1/users/${id}`, { roles }),
	resetPassword: (id: string, password: string) => http.post<void>(`/v1/users/${id}/reset-password`, { password }),
	disableTwoFactor: (id: string) => http.post<UserAccount>(`/v1/users/${id}/2fa/disable`),
	remove: (id: string) => http.delete<void>(`/v1/users/${id}`),
	/** The signed-in user's own password. */
	changeOwnPassword: (current_password: string, new_password: string) =>
		http.post<{ success: boolean }>('/v1/auth/password', { current_password, new_password }),
}

const usersKey = ['users'] as const

export function useUserAccounts() {
	return useQuery({ queryKey: usersKey, queryFn: usersApi.list })
}

/** Mutations that refresh the user list when they succeed. */
export function useUserMutations() {
	const qc = useQueryClient()
	const refresh = () => qc.invalidateQueries({ queryKey: usersKey })
	return {
		create: useMutation({ mutationFn: usersApi.create, onSuccess: refresh }),
		setRoles: useMutation({
			mutationFn: ({ id, roles }: { id: string; roles: Role[] }) => usersApi.setRoles(id, roles),
			onSuccess: refresh,
		}),
		resetPassword: useMutation({
			mutationFn: ({ id, password }: { id: string; password: string }) => usersApi.resetPassword(id, password),
			onSuccess: refresh,
		}),
		disableTwoFactor: useMutation({ mutationFn: usersApi.disableTwoFactor, onSuccess: refresh }),
		remove: useMutation({ mutationFn: usersApi.remove, onSuccess: refresh }),
	}
}
