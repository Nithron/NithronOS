import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { z } from 'zod'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import {
	Server,
	Network,
	Users,
	Clock,
	Trash2,
	AlertCircle,
	Info,
	Shield,
	X,
	Cloud,
	FolderSync,
} from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Switch } from '@/components/ui/switch'
import { Card } from '@/components/ui/card'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Separator } from '@/components/ui/separator'
import { PermissionsEditor } from '@/components/shares/PermissionsEditor'
import { usePolicy } from '@/hooks/use-shares'
import type { Share, CreateShareRequest, UpdateShareRequest } from '@/api/client'

const shareNameSchema = z.string()
	.min(2, 'Name must be at least 2 characters')
	.max(32, 'Name must be at most 32 characters')
	.regex(/^[a-z0-9][a-z0-9-_]*$/, 'Name must start with a-z or 0-9, and contain only a-z, 0-9, -, _')

const shareFormSchema = z.object({
	name: shareNameSchema,
	description: z.string().optional(),
	smb: z.object({
		enabled: z.boolean(),
		guest: z.boolean(),
		time_machine: z.boolean(),
		recycle: z.object({
			enabled: z.boolean(),
			directory: z.string().optional(),
		}),
	}),
	nfs: z.object({
		enabled: z.boolean(),
		networks: z.array(z.string()).optional(),
		read_only: z.boolean(),
	}),
	sync: z.object({
		enabled: z.boolean(),
		max_size_gb: z.number().optional(),
		exclude_patterns: z.array(z.string()).optional(),
	}),
	owners: z.array(z.string()),
	readers: z.array(z.string()),
})

type ShareFormData = z.infer<typeof shareFormSchema>

interface ShareFormProps {
	share?: Share
	onSubmit: (data: CreateShareRequest | UpdateShareRequest) => Promise<void>
	onCancel: () => void
	mode: 'create' | 'edit'
}

export function ShareForm({ share, onSubmit, onCancel, mode }: ShareFormProps) {
	const navigate = useNavigate()
	const { data: policy } = usePolicy()
	const [isSubmitting, setIsSubmitting] = useState(false)
	const [activeTab, setActiveTab] = useState('general')
	
	const guestAccessForbidden = policy?.shares?.guest_access_forbidden || false

	const {
		register,
		handleSubmit,
		watch,
		setValue,
		formState: { errors },
	} = useForm<ShareFormData>({
		resolver: zodResolver(shareFormSchema),
		defaultValues: {
			name: share?.name || '',
			description: share?.description || '',
			smb: {
				enabled: share?.smb?.enabled || true,
				guest: share?.smb?.guest || false,
				time_machine: share?.smb?.time_machine || false,
				recycle: {
					enabled: share?.smb?.recycle?.enabled || false,
					directory: share?.smb?.recycle?.directory || '.recycle',
				},
			},
			nfs: {
				enabled: share?.nfs?.enabled || false,
				networks: share?.nfs?.networks || ['192.168.0.0/16', '10.0.0.0/8'],
				read_only: share?.nfs?.read_only || false,
			},
			sync: {
				enabled: (share as any)?.sync?.enabled || false,
				max_size_gb: (share as any)?.sync?.max_size_gb || 0,
				exclude_patterns: (share as any)?.sync?.exclude_patterns || ['*.tmp', '*.bak', '.git'],
			},
			owners: share?.owners || [],
			readers: share?.readers || [],
		},
	})

	const watchName = watch('name')
	const watchSmbEnabled = watch('smb.enabled')
	const watchSmbGuest = watch('smb.guest')
	const watchSmbRecycleEnabled = watch('smb.recycle.enabled')
	const watchNfsEnabled = watch('nfs.enabled')
	const watchOwners = watch('owners')
	const watchReaders = watch('readers')
	const watchSyncEnabled = watch('sync.enabled')
	const watchSyncExcludePatterns = watch('sync.exclude_patterns')

	const pathPreview = watchName ? `/srv/shares/${watchName}` : '/srv/shares/<name>'

	const handleFormSubmit = async (data: ShareFormData) => {
		setIsSubmitting(true)
		try {
			if (mode === 'create') {
				await onSubmit(data as CreateShareRequest)
			} else {
				const { name, ...updateData } = data
				await onSubmit(updateData as UpdateShareRequest)
			}
			navigate('/shares')
		} catch (error) {
			console.error('Failed to save share:', error)
		} finally {
			setIsSubmitting(false)
		}
	}

	return (
		<form onSubmit={handleSubmit(handleFormSubmit)} className="space-y-6">
			<Tabs value={activeTab} onValueChange={setActiveTab}>
				<TabsList className="grid w-full grid-cols-4">
					<TabsTrigger value="general">General</TabsTrigger>
					<TabsTrigger value="protocols">Protocols</TabsTrigger>
					<TabsTrigger value="sync">Sync</TabsTrigger>
					<TabsTrigger value="permissions">Permissions</TabsTrigger>
				</TabsList>

				<TabsContent value="general" className="space-y-6">
					<Card className="p-6 space-y-6">
						<div className="space-y-4">
							<div className="space-y-2">
								<Label htmlFor="name">Share Name</Label>
								<Input
									id="name"
									{...register('name')}
									placeholder="documents"
									disabled={mode === 'edit'}
									className={errors.name ? 'border-destructive' : ''}
								/>
								{errors.name && (
									<p className="text-sm text-destructive">{errors.name.message}</p>
								)}
								<p className="text-sm text-muted-foreground">
									Path will be: <code className="bg-muted px-1 py-0.5 rounded">{pathPreview}</code>
								</p>
							</div>

							<div className="space-y-2">
								<Label htmlFor="description">Description</Label>
								<Input
									id="description"
									{...register('description')}
									placeholder="Shared documents and files"
								/>
							</div>
						</div>
					</Card>
				</TabsContent>

				<TabsContent value="protocols" className="space-y-6">
					{/* SMB Configuration */}
					<Card className="p-6 space-y-6">
						<div className="flex items-center justify-between">
							<div className="flex items-center gap-3">
								<Server className="h-5 w-5 text-muted-foreground" />
								<div>
									<h3 className="font-medium">SMB/CIFS</h3>
									<p className="text-sm text-muted-foreground">Windows file sharing</p>
								</div>
							</div>
							<Switch
								checked={watchSmbEnabled}
								        onCheckedChange={(checked: boolean) => setValue('smb.enabled', checked)}
							/>
						</div>

						{watchSmbEnabled && (
							<>
								<Separator />
								<div className="space-y-4">
									<div className="flex items-center justify-between">
										<div className="flex items-center gap-3">
											<Users className="h-4 w-4 text-muted-foreground" />
											<div>
												<Label>Guest Access</Label>
												<p className="text-sm text-muted-foreground">
													Allow anonymous access without authentication
												</p>
											</div>
										</div>
										<Switch
											checked={watchSmbGuest}
											           onCheckedChange={(checked: boolean) => setValue('smb.guest', checked)}
											disabled={guestAccessForbidden}
										/>
									</div>

									{guestAccessForbidden && watchSmbGuest && (
										<Alert variant="destructive">
											<AlertCircle className="h-4 w-4" />
											<AlertDescription>
												Guest access is forbidden by policy but this share has it enabled.
												Please disable guest access or contact your administrator.
											</AlertDescription>
										</Alert>
									)}

									<div className="flex items-center justify-between">
										<div className="flex items-center gap-3">
											<Clock className="h-4 w-4 text-muted-foreground" />
											<div>
												<Label>Time Machine</Label>
												<p className="text-sm text-muted-foreground">
													Enable macOS Time Machine backups
												</p>
											</div>
										</div>
										<Switch
											checked={watch('smb.time_machine')}
											           onCheckedChange={(checked: boolean) => setValue('smb.time_machine', checked)}
										/>
									</div>

									<div className="flex items-center justify-between">
										<div className="flex items-center gap-3">
											<Trash2 className="h-4 w-4 text-muted-foreground" />
											<div>
												<Label>Recycle Bin</Label>
												<p className="text-sm text-muted-foreground">
													Keep deleted files in a hidden directory
												</p>
											</div>
										</div>
										<Switch
											checked={watchSmbRecycleEnabled}
											           onCheckedChange={(checked: boolean) => setValue('smb.recycle.enabled', checked)}
										/>
									</div>

									{watchSmbRecycleEnabled && (
										<div className="ml-7 space-y-2">
											<Label htmlFor="recycle-dir">Recycle Directory</Label>
											<Input
												id="recycle-dir"
												{...register('smb.recycle.directory')}
												placeholder=".recycle"
												className="max-w-xs"
											/>
										</div>
									)}
								</div>
							</>
						)}
					</Card>

					{/* NFS Configuration */}
					<Card className="p-6 space-y-6">
						<div className="flex items-center justify-between">
							<div className="flex items-center gap-3">
								<Network className="h-5 w-5 text-muted-foreground" />
								<div>
									<h3 className="font-medium">NFS</h3>
									<p className="text-sm text-muted-foreground">Unix/Linux network filesystem</p>
								</div>
							</div>
							<Switch
								checked={watchNfsEnabled}
								        onCheckedChange={(checked: boolean) => setValue('nfs.enabled', checked)}
							/>
						</div>

						{watchNfsEnabled && (
							<>
								<Separator />
								<div className="space-y-4">
									<div className="flex items-center justify-between">
										<div className="flex items-center gap-3">
											<Shield className="h-4 w-4 text-muted-foreground" />
											<div>
												<Label>Read Only</Label>
												<p className="text-sm text-muted-foreground">
													Prevent write access via NFS
												</p>
											</div>
										</div>
										<Switch
											checked={watch('nfs.read_only')}
											           onCheckedChange={(checked: boolean) => setValue('nfs.read_only', checked)}
										/>
									</div>

									<div className="space-y-2">
										<Label>Allowed Networks</Label>
										<div className="flex flex-wrap gap-2">
											{watch('nfs.networks')?.map((network, index) => (
												<Badge key={index} variant="secondary">
													{network}
													<button
														type="button"
														onClick={() => {
															const networks = watch('nfs.networks') || []
															setValue('nfs.networks', networks.filter((_, i) => i !== index))
														}}
														className="ml-1"
													>
														<X className="h-3 w-3" />
													</button>
												</Badge>
											))}
										</div>
										<Input
											placeholder="Add network (e.g., 192.168.1.0/24)"
											onKeyDown={(e) => {
												if (e.key === 'Enter') {
													e.preventDefault()
													const input = e.currentTarget
													const value = input.value.trim()
													if (value) {
														const networks = watch('nfs.networks') || []
														setValue('nfs.networks', [...networks, value])
														input.value = ''
													}
												}
											}}
											className="max-w-xs"
										/>
									</div>
								</div>
							</>
						)}
					</Card>

					{!watchSmbEnabled && !watchNfsEnabled && (
						<Alert>
							<Info className="h-4 w-4" />
							<AlertDescription>
								At least one protocol (SMB or NFS) must be enabled for the share to be accessible.
							</AlertDescription>
						</Alert>
					)}
				</TabsContent>

				{/* Sync Configuration (NithronSync) */}
				<TabsContent value="sync" className="space-y-6">
					<Card className="p-6 space-y-6">
						<div className="flex items-center justify-between">
							<div className="flex items-center gap-3">
								<Cloud className="h-5 w-5 text-blue-500" />
								<div>
									<h3 className="font-medium">NithronSync</h3>
									<p className="text-sm text-muted-foreground">Enable file synchronization to devices</p>
								</div>
							</div>
							<Switch
								checked={watchSyncEnabled}
								onCheckedChange={(checked: boolean) => setValue('sync.enabled', checked)}
							/>
						</div>

						{watchSyncEnabled && (
							<>
								<Separator />
								<div className="space-y-4">
									<Alert>
										<FolderSync className="h-4 w-4" />
										<AlertDescription>
											When enabled, this share will be available for sync on Windows, Linux, and mobile devices via the NithronSync client.
										</AlertDescription>
									</Alert>

									<div className="space-y-2">
										<Label htmlFor="max-size">Maximum Sync Size (GB)</Label>
										<Input
											id="max-size"
											type="number"
											{...register('sync.max_size_gb', { valueAsNumber: true })}
											placeholder="0 = unlimited"
											className="max-w-xs"
										/>
										<p className="text-sm text-muted-foreground">
											Set to 0 for unlimited. This limits the total size that can be synced.
										</p>
									</div>

									<div className="space-y-2">
										<Label>Exclude Patterns</Label>
										<p className="text-sm text-muted-foreground">
											Files matching these patterns will not be synced
										</p>
										<div className="flex flex-wrap gap-2">
											{watchSyncExcludePatterns?.map((pattern, index) => (
												<Badge key={index} variant="secondary">
													{pattern}
													<button
														type="button"
														onClick={() => {
															const patterns = watch('sync.exclude_patterns') || []
															setValue('sync.exclude_patterns', patterns.filter((_, i) => i !== index))
														}}
														className="ml-1"
													>
														<X className="h-3 w-3" />
													</button>
												</Badge>
											))}
										</div>
										<Input
											placeholder="Add pattern (e.g., *.tmp, .git)"
											onKeyDown={(e) => {
												if (e.key === 'Enter') {
													e.preventDefault()
													const input = e.currentTarget
													const value = input.value.trim()
													if (value) {
														const patterns = watch('sync.exclude_patterns') || []
														setValue('sync.exclude_patterns', [...patterns, value])
														input.value = ''
													}
												}
											}}
											className="max-w-xs"
										/>
									</div>
								</div>
							</>
						)}
					</Card>

					{!watchSyncEnabled && (
						<Alert>
							<Info className="h-4 w-4" />
							<AlertDescription>
								Enable NithronSync to allow devices to synchronize files from this share.
								This works alongside SMB/NFS access.
							</AlertDescription>
						</Alert>
					)}
				</TabsContent>

				<TabsContent value="permissions" className="space-y-6">
					<PermissionsEditor
						owners={watchOwners}
						readers={watchReaders}
						onOwnersChange={(owners: string[]) => setValue('owners', owners)}
						onReadersChange={(readers: string[]) => setValue('readers', readers)}
						guestEnabled={watchSmbGuest}
					/>
				</TabsContent>
			</Tabs>

			<div className="flex justify-end gap-3">
				<Button type="button" variant="outline" onClick={onCancel}>
					Cancel
				</Button>
				<Button type="submit" disabled={isSubmitting}>
					{isSubmitting ? 'Saving...' : mode === 'create' ? 'Create Share' : 'Save Changes'}
				</Button>
			</div>
		</form>
	)
}
