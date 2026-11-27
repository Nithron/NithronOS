import { useParams, useNavigate } from 'react-router-dom'
import { BackIcon } from '@/components/icons'
import { Working } from '@/components/ui/pixel'
import { PageHeader } from '@/components/layouts/PageHeader'
import { Button } from '@/components/ui/button'
import { ShareForm } from '@/components/shares/ShareForm'
import { useShare, useShareMutations, type CreateShareRequest, type UpdateShareRequest } from '@/api/shares'
import { getErrorMessage } from '@/lib/nos-client'
import { toast } from '@/components/ui/toast'

export function ShareDetails() {
	const { name } = useParams<{ name: string }>()
	const navigate = useNavigate()
	const isNew = !name || name === 'new'
	
	const { data: share, isLoading } = useShare(isNew ? undefined : name)
	const { create: createShare, update: updateShare } = useShareMutations()

	const handleSubmit = async (data: CreateShareRequest | UpdateShareRequest) => {
		try {
			if (isNew) {
				const created = await createShare.mutateAsync(data as CreateShareRequest)
				toast.success(`Share "${created.name}" created`)
				navigate('/shares')
			} else if (name) {
				await updateShare.mutateAsync({ name, data: data as UpdateShareRequest })
				toast.success(`Share "${name}" saved`)
				navigate('/shares')
			}
		} catch (err) {
			toast.error(getErrorMessage(err))
		}
	}

	const handleCancel = () => {
		navigate('/shares')
	}

	if (isLoading && !isNew) {
		return (
			<div className="flex items-center justify-center h-full">
				<Working>Loading the share</Working>
			</div>
		)
	}

	if (!isNew && !share && !isLoading) {
		return (
			<div className="flex flex-col h-full">
				<PageHeader
					title="Share not found"
					description="The requested share could not be found"
				/>
				<div className="flex-1 flex items-center justify-center">
					<div className="text-center space-y-4">
						<p className="text-muted-foreground">
							The share "{name}" does not exist or has been deleted.
						</p>
						<Button onClick={() => navigate('/shares')}>
							<BackIcon className="h-4 w-4 mr-2" />
							Back to shares
						</Button>
					</div>
				</div>
			</div>
		)
	}

	return (
		<div className="flex flex-col h-full">
			<PageHeader
				title={isNew ? 'Create share' : `Edit share: ${name}`}
				breadcrumbs={[
					{ label: 'Shares', href: '/shares' },
					{ label: isNew ? 'New' : name || '' }
				]}
			/>
			
			<div className="w-full max-w-4xl flex-1 p-6">
				<ShareForm
					share={share || undefined}
					onSubmit={handleSubmit}
					onCancel={handleCancel}
					mode={isNew ? 'create' : 'edit'}
				/>
			</div>
		</div>
	)
}
