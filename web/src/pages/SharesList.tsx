import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { AddIcon, DeleteIcon, EditIcon, FolderIcon, MoreIcon, NetworkIcon, PermissionsIcon, ServerIcon, ShareIcon, TimeIcon, UsersIcon } from '@/components/icons'
import { useShares, useShareMutations, type Share } from '@/api/shares'
import { getErrorMessage } from '@/lib/nos-client'
import { toast } from '@/components/ui/toast'
import { PageHeader } from '@/components/layouts/PageHeader'
import { Card } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { DataTable } from '@/components/ui/data-table'
import { EmptyState } from '@/components/ui/empty-state'
import { StatusPill } from '@/components/ui/status-pill'
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
					<div className="flex items-center gap-3">
						<div className="p-2 bg-blue-500/10 rounded-lg">
							<FolderIcon className="h-4 w-4 text-blue-500" />
						</div>
						<div>
							<div className="font-medium">{share.name}</div>
							<div className="text-xs text-muted-foreground">
								/srv/shares/{share.name}
							</div>
						</div>
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
						{share.smb?.guest && (
							<StatusPill status="warning" size="sm">
								<UsersIcon className="h-3 w-3" />
								Guest
							</StatusPill>
						)}
						{share.smb?.time_machine && (
							<StatusPill status="success" size="sm">
								<TimeIcon className="h-3 w-3" />
								Time Machine
							</StatusPill>
						)}
						{share.smb?.recycle?.enabled && (
							<StatusPill status="info" size="sm">
								<DeleteIcon className="h-3 w-3" />
								Recycle
							</StatusPill>
						)}
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
								<PermissionsIcon className="h-3.5 w-3.5 text-green-500" />
								<span>{ownerCount} owner{ownerCount !== 1 ? 's' : ''}</span>
							</div>
						)}
						{readerCount > 0 && (
							<div className="flex items-center gap-1.5">
								<PermissionsIcon className="h-3.5 w-3.5 text-blue-500" />
								<span>{readerCount} reader{readerCount !== 1 ? 's' : ''}</span>
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
								Edit Share
							</DropdownMenuItem>
							<DropdownMenuItem>
								<FolderIcon className="h-4 w-4 mr-2" />
								Open in Files
							</DropdownMenuItem>
							<DropdownMenuSeparator />
							<DropdownMenuItem 
								onClick={() => setDeleteTarget(share.name)}
								className="text-destructive"
							>
								<DeleteIcon className="h-4 w-4 mr-2" />
								Delete Share
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
								Create Share
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
						Create Share
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
						<AlertDialogTitle>Delete Share</AlertDialogTitle>
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
							Delete Share
						</AlertDialogAction>
					</AlertDialogFooter>
				</AlertDialogContent>
			</AlertDialog>
		</div>
	)
}
