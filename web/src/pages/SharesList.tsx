import { useState } from 'react'
import { Link, useNavigate } from 'react-router-dom'
import { AddIcon, DeleteIcon, EditIcon, FolderIcon, MoreIcon, NetworkIcon, ServerIcon, ShareIcon } from '@/components/icons'
import { useShares, useShareMutations, type Share } from '@/api/shares'
import { getErrorMessage } from '@/lib/nos-client'
import { toast } from '@/components/ui/toast'
import { PageHeader } from '@/components/layouts/PageHeader'
import { Card } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { DataTable } from '@/components/ui/data-table'
import { EmptyState } from '@/components/ui/empty-state'
import { StatusMark } from '@/components/ui/pixel'
import {
	DropdownMenu,
	DropdownMenuContent,
	DropdownMenuItem,
	DropdownMenuSeparator,
	DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import {
	AlertDialog,
	AlertDialogAction,
	AlertDialogCancel,
	AlertDialogContent,
	AlertDialogDescription,
	AlertDialogFooter,
	AlertDialogHeader,
	AlertDialogTitle,
} from '@/components/ui/alert-dialog'

export function SharesList() {
	const navigate = useNavigate()
	const { data: shares = [], isLoading } = useShares()
	const { remove: deleteShare } = useShareMutations()
	const [deleteTarget, setDeleteTarget] = useState<string | null>(null)

	const handleCreateShare = () => {
		navigate('/shares/new')
	}

	const handleEditShare = (name: string) => {
		navigate(`/shares/${name}`)
	}

	const handleDeleteShare = async () => {
		if (!deleteTarget) return
		try {
			await deleteShare.mutateAsync(deleteTarget)
			toast.success(`Share "${deleteTarget}" deleted; its files stay in /srv/shares/${deleteTarget}`)
		} catch (err) {
			toast.error(getErrorMessage(err))
		}
		setDeleteTarget(null)
	}

	const columns = [
		{
			header: 'Name',
			accessorKey: 'name',
			cell: ({ row }: any) => {
				const share = row.original as Share
				return (
					<div>
						<Link
							to={`/shares/${encodeURIComponent(share.name)}`}
							className="font-[560] underline decoration-ink-2 underline-offset-4 hover:decoration-ink"
							onClick={(e) => e.stopPropagation()}
						>
							{share.name}
						</Link>
						<div className="font-mono text-xs text-ink-2">/srv/shares/{share.name}</div>
					</div>
				)
			},
		},
		{
			header: 'Protocols',
			accessorKey: 'protocols',
			cell: ({ row }: any) => {
				const share = row.original as Share
				return (
					<div className="flex items-center gap-2">
						{share.smb?.enabled && (
							<div className="flex items-center gap-1.5">
								<ServerIcon className="h-4 w-4 text-muted-foreground" />
								<span className="text-sm">SMB</span>
							</div>
						)}
						{share.nfs?.enabled && (
							<div className="flex items-center gap-1.5">
								<NetworkIcon className="h-4 w-4 text-muted-foreground" />
								<span className="text-sm">NFS</span>
							</div>
						)}
						{!share.smb?.enabled && !share.nfs?.enabled && (
							<span className="text-sm text-muted-foreground">None</span>
						)}
					</div>
				)
			},
		},
		{
			header: 'Features',
			accessorKey: 'features',
			cell: ({ row }: any) => {
				const share = row.original as Share
				return (
					<div className="flex items-center gap-2">
						{/* Guest access is a state worth a warning; the rest are words. */}
						{share.smb?.guest && <StatusMark state="warning" className="text-sm">Guest access</StatusMark>}
						{share.smb?.time_machine && <span className="text-sm">Time Machine</span>}
						{share.smb?.recycle?.enabled && <span className="text-sm">Recycle bin</span>}
					</div>
				)
			},
		},
		{
			header: 'Permissions',
			accessorKey: 'permissions',
			cell: ({ row }: any) => {
				const share = row.original as Share
				const ownerCount = share.owners?.length || 0
				const readerCount = share.readers?.length || 0
				
				return (
					<div className="flex items-center gap-3 text-sm">
						{ownerCount > 0 && (
							<div className="flex items-center gap-1.5">
								<span className="font-mono tabular-nums">{ownerCount}</span>
								<span>owner{ownerCount !== 1 ? 's' : ''}</span>
							</div>
						)}
						{readerCount > 0 && (
							<div className="flex items-center gap-1.5">
								<span className="font-mono tabular-nums">{readerCount}</span>
								<span>reader{readerCount !== 1 ? 's' : ''}</span>
							</div>
						)}
						{ownerCount === 0 && readerCount === 0 && (
							<span className="text-muted-foreground">No permissions set</span>
						)}
					</div>
				)
			},
		},
		{
			header: '',
			id: 'actions',
			cell: ({ row }: any) => {
				const share = row.original as Share
				
				return (
					<DropdownMenu>
						<DropdownMenuTrigger asChild>
							<Button variant="ghost" size="icon">
								<MoreIcon className="h-4 w-4" />
							</Button>
						</DropdownMenuTrigger>
						<DropdownMenuContent align="end">
							<DropdownMenuItem onClick={() => handleEditShare(share.name)}>
								<EditIcon className="h-4 w-4 mr-2" />
								Edit share
							</DropdownMenuItem>
							<DropdownMenuItem>
								<FolderIcon className="h-4 w-4 mr-2" />
								Open in files
							</DropdownMenuItem>
							<DropdownMenuSeparator />
							<DropdownMenuItem 
								onClick={() => setDeleteTarget(share.name)}
								className="text-destructive"
							>
								<DeleteIcon className="h-4 w-4 mr-2" />
								Delete share
							</DropdownMenuItem>
						</DropdownMenuContent>
					</DropdownMenu>
				)
			},
		},
	]

	if (!isLoading && shares.length === 0) {
		return (
			<div className="flex flex-col h-full">
				<PageHeader
					title="Shares"
					description="Network file shares for SMB and NFS access"
				/>
				<div className="flex-1 flex items-center justify-center p-8">
					<EmptyState
						icon={ShareIcon}
						title="No shares configured"
						description="Create your first network share to enable file access over SMB or NFS"
						action={
							<Button onClick={handleCreateShare}>
								<AddIcon className="h-4 w-4 mr-2" />
								Create share
							</Button>
						}
					/>
				</div>
			</div>
		)
	}

	return (
		<div className="flex flex-col h-full">
			<PageHeader
				title="Shares"
				description="Network file shares for SMB and NFS access"
				action={
					<Button onClick={handleCreateShare}>
						<AddIcon className="h-4 w-4 mr-2" />
						Create share
					</Button>
				}
			/>
			
			<div className="flex-1 p-6 space-y-6">
				<Card className="p-6">
					<DataTable
						columns={columns}
						data={shares}
						loading={isLoading}
						searchPlaceholder="Search shares..."
						searchKey="name"
					/>
				</Card>
			</div>

			<AlertDialog open={!!deleteTarget} onOpenChange={() => setDeleteTarget(null)}>
				<AlertDialogContent>
					<AlertDialogHeader>
						<AlertDialogTitle>Delete share</AlertDialogTitle>
						<AlertDialogDescription>
							Are you sure you want to delete the share "{deleteTarget}"? 
							This will remove all share configurations but will not delete the actual files.
						</AlertDialogDescription>
					</AlertDialogHeader>
					<AlertDialogFooter>
						<AlertDialogCancel>Cancel</AlertDialogCancel>
						<AlertDialogAction 
							onClick={handleDeleteShare}
							className="bg-destructive text-destructive-foreground"
						>
							Delete share
						</AlertDialogAction>
					</AlertDialogFooter>
				</AlertDialogContent>
			</AlertDialog>
		</div>
	)
}
