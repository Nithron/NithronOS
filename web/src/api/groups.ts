import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import http from '@/lib/nos-client'

/** A group of users (the Unix group nos-<name>), for share access. */
export interface Group {
	name: string
	description?: string
	/** usernames */
	members: string[]
	created_at: string
	updated_at: string
}

export interface GroupInput {
	name?: string
	description?: string
	members?: string[]
}

/** Lowercase letters, digits, '_' and '-', starting with a letter or '_'; not "users" or "app-...". */
export const GROUP_NAME_PATTERN = /^[a-z_][a-z0-9_-]{0,27}$/

export function groupNameProblem(name: string): string | null {
	if (!GROUP_NAME_PATTERN.test(name)) return "1-28 lowercase letters, digits, '_' or '-', starting with a letter"
	if (name === 'users') return '"users" is every user already'
	if (name.startsWith('app-')) return 'Names starting with "app-" are for apps'
	return null
}

export const groupsApi = {
	list: () => http.get<Group[]>('/v1/groups'),
	create: (g: GroupInput & { name: string }) => http.post<Group>('/v1/groups', g),
	/** Replaces the description and members (send both). */
	update: (name: string, g: { description: string; members: string[] }) =>
		http.put<Group>(`/v1/groups/${encodeURIComponent(name)}`, g),
	remove: (name: string) => http.del<void>(`/v1/groups/${encodeURIComponent(name)}`),
}

const groupsKey = ['groups'] as const

export function useGroups() {
	return useQuery({ queryKey: groupsKey, queryFn: groupsApi.list })
}

export function useGroupMutations() {
	const qc = useQueryClient()
	// A group change reapplies share access; the shares list shows it.
	const refresh = () => {
		qc.invalidateQueries({ queryKey: groupsKey })
		qc.invalidateQueries({ queryKey: ['shares'] })
	}
	return {
		create: useMutation({ mutationFn: groupsApi.create, onSuccess: refresh }),
		update: useMutation({
			mutationFn: ({ name, data }: { name: string; data: { description: string; members: string[] } }) =>
				groupsApi.update(name, data),
			onSuccess: refresh,
		}),
		remove: useMutation({ mutationFn: groupsApi.remove, onSuccess: refresh }),
	}
}
