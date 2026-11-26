import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import http from '@/lib/nos-client'

export interface HostnameConfig {
	hostname: string
	pretty_hostname?: string
}

export interface TimezoneConfig {
	timezone: string
	time: string
	utc: boolean
}

export interface NTPConfig {
	enabled: boolean
	servers: string[]
	/** synchronized, not_synchronized or unknown */
	status: string
}

/** The parts of GET /api/v1/about the UI shows. */
export interface AboutInfo {
	system: {
		hostname: string
		platform: string
		architecture: string
		kernel: string
		/** seconds */
		uptime: number
		timezone: string
	}
	hardware: {
		cpu: { model: string; cores: number; threads: number }
		memory: { total: number }
	}
	software: {
		nos_version: string
		webui_version: string
		agent_version: string
		docker_version?: string
		components: Record<string, string>
	}
}

/** Same rule as the agent: letters, digits and '-', labels of 1-63. */
export const HOSTNAME_PATTERN = /^(?=.{1,253}$)[A-Za-z0-9](?:[A-Za-z0-9-]{0,61}[A-Za-z0-9])?(?:\.[A-Za-z0-9](?:[A-Za-z0-9-]{0,61}[A-Za-z0-9])?)*$/

export const systemApi = {
	hostname: () => http.get<HostnameConfig>('/v1/system/hostname'),
	setHostname: (hostname: string) => http.post<HostnameConfig>('/v1/system/hostname', { hostname }),
	timezone: () => http.get<TimezoneConfig>('/v1/system/timezone'),
	timezones: () => http.get<{ timezones: string[] }>('/v1/system/timezones'),
	setTimezone: (timezone: string) => http.post<TimezoneConfig>('/v1/system/timezone', { timezone }),
	ntp: () => http.get<NTPConfig>('/v1/system/ntp'),
	setNTP: (cfg: Pick<NTPConfig, 'enabled' | 'servers'>) => http.post<NTPConfig>('/v1/system/ntp', cfg),
	about: () => http.get<AboutInfo>('/v1/about'),
}

export function useHostname() {
	return useQuery({ queryKey: ['system', 'hostname'], queryFn: systemApi.hostname })
}

export function useTimezone() {
	return useQuery({ queryKey: ['system', 'timezone'], queryFn: systemApi.timezone })
}

export function useTimezoneList() {
	return useQuery({ queryKey: ['system', 'timezones'], queryFn: systemApi.timezones, staleTime: Infinity })
}

export function useNTP() {
	return useQuery({ queryKey: ['system', 'ntp'], queryFn: systemApi.ntp })
}

export function useAbout() {
	return useQuery({ queryKey: ['system', 'about'], queryFn: systemApi.about, staleTime: 60_000 })
}

export function useSystemMutations() {
	const qc = useQueryClient()
	const refresh = () => qc.invalidateQueries({ queryKey: ['system'] })
	return {
		setHostname: useMutation({ mutationFn: systemApi.setHostname, onSuccess: refresh }),
		setTimezone: useMutation({ mutationFn: systemApi.setTimezone, onSuccess: refresh }),
		setNTP: useMutation({ mutationFn: systemApi.setNTP, onSuccess: refresh }),
	}
}
