import { useEffect } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import http from '@/lib/nos-client'
import { applyTheme, isThemeChoice, type ThemeChoice } from '@/lib/theme'

/**
 * GET/PUT /api/v1/settings/appearance. The UI uses the theme; the other
 * fields are kept as they are when the theme changes (PUT replaces the
 * whole record).
 */
export interface AppearanceSettings {
  theme: ThemeChoice
  [field: string]: unknown
}

const KEY = ['settings', 'appearance'] as const

export const appearanceApi = {
  get: () => http.get<AppearanceSettings>('/v1/settings/appearance'),
  put: (s: AppearanceSettings) => http.put<AppearanceSettings>('/v1/settings/appearance', s),
}

export function useAppearance() {
  return useQuery({ queryKey: KEY, queryFn: appearanceApi.get, staleTime: 5 * 60 * 1000 })
}

/** Applies the stored theme once it is known (signed-in pages). */
export function useAppliedTheme(): void {
  const { data } = useAppearance()
  const theme = data?.theme
  useEffect(() => {
    if (isThemeChoice(theme)) applyTheme(theme)
  }, [theme])
}

export function useSetTheme() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async (theme: ThemeChoice) => {
      const current = await appearanceApi.get()
      return appearanceApi.put({ ...current, theme })
    },
    onSuccess: (saved) => {
      qc.setQueryData(KEY, saved)
      if (isThemeChoice(saved.theme)) applyTheme(saved.theme)
    },
  })
}
