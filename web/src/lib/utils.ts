import { type ClassValue, clsx } from 'clsx'
import { twMerge } from 'tailwind-merge'

export function cn(...inputs: ClassValue[]) {
	return twMerge(clsx(inputs))
}

/**
 * A panel's classes without corners, shadows or a fill of its own: panels
 * are square, on the ground, parted by hairlines (docs/dev/ui-identity-proposal.md).
 */
export function flatPanel(className?: string): string | undefined {
	return className
		?.replace(/(?<![\w-])(?:[\w-]+:)*(?:rounded(?:-[\w-]+)?|shadow(?:-[\w-]+)?|bg-card|bg-muted(?:\/\d+)?|bg-gradient-[\w-]+)(?![\w-])/g, '') // ui-check: pattern
		.replace(/\s+/g, ' ')
		.trim()
}

export function formatBytes(bytes: number, decimals = 2): string {
	if (bytes === 0) return '0 Bytes'
	
	const k = 1024
	const dm = decimals < 0 ? 0 : decimals
	const sizes = ['Bytes', 'KB', 'MB', 'GB', 'TB', 'PB', 'EB', 'ZB', 'YB']
	
	const i = Math.floor(Math.log(bytes) / Math.log(k))
	
	return parseFloat((bytes / Math.pow(k, i)).toFixed(dm)) + ' ' + sizes[i]
}


