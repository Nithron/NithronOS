import { useState } from 'react'


import { 
  Users,

  Edit,
  Trash2,
  Shield,
  Key,
  Mail,

  CheckCircle,
  XCircle,
  AlertCircle,

  UserPlus,
  UserCheck,
  UserX,
  Lock,
  Unlock,
  RefreshCw,
  Download,
  Upload,
  Search,
} from 'lucide-react'
import { PageHeader } from '@/components/ui/page-header'
import { Card } from '@/components/ui/card-enhanced'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter } from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Switch } from '@/components/ui/switch'

import { DataTable } from '@/components/ui/data-table'
import { ColumnDef } from '@tanstack/react-table'

import { toast } from '@/components/ui/toast'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { api } from '@/lib/nos-client'
import { formatDistanceToNow } from 'date-fns'

// User type definition
interface User {
  id: string
  email: string
  name: string
  role: 'admin' | 'user' | 'viewer'
  enabled: boolean
  totpEnabled: boolean
  createdAt: string
  lastLogin?: string
  storageQuota?: number
  storageUsed?: number
}

// Create/Edit User Dialog
function UserDialog({ 
  user, 
  open, 
  onClose 
}: { 
  user?: User | null
  open: boolean
  onClose: (saved?: boolean) => void 
}) {
  const [formData, setFormData] = useState({
    email: user?.email || '',
    name: user?.name || '',
    role: user?.role || 'user',
    password: '',
    confirmPassword: '',
    enabled: user?.enabled ?? true,
    storageQuota: user?.storageQuota || 0,
    sendWelcomeEmail: false,
  })
  const [errors, setErrors] = useState<Record<string, string>>({})
  const queryClient = useQueryClient()

  const createMutation = useMutation({
    mutationFn: (data: any) => api.post('/v1/users', data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['users'] })
      toast.success('User created successfully')
      onClose(true)
    },
    onError: (error: any) => {
      toast.error(error.response?.data?.message || 'Failed to create user')
    }
  })

  const updateMutation = useMutation({
    mutationFn: (data: any) => api.put(`/v1/users/${user?.id}`, data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['users'] })
      toast.success('User updated successfully')
      onClose(true)
    },
    onError: (error: any) => {
      toast.error(error.response?.data?.message || 'Failed to update user')
    }
  })

  const handleSubmit = () => {
    const newErrors: Record<string, string> = {}
    
    if (!formData.email) {
      newErrors.email = 'Email is required'
    } else if (!/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(formData.email)) {
      newErrors.email = 'Invalid email address'
    }
    
    if (!formData.name) {
      newErrors.name = 'Name is required'
    }
    
    if (!user && !formData.password) {
      newErrors.password = 'Password is required for new users'
    }
    
    if (formData.password && formData.password.length < 8) {
      newErrors.password = 'Password must be at least 8 characters'
    }
    
    if (formData.password && formData.password !== formData.confirmPassword) {
      newErrors.confirmPassword = 'Passwords do not match'
    }
    
    if (Object.keys(newErrors).length > 0) {
      setErrors(newErrors)
      return
    }
    
    const data = {
      email: formData.email,
      name: formData.name,
      role: formData.role,
      enabled: formData.enabled,
      storageQuota: formData.storageQuota,
    }
    
    if (formData.password) {
      (data as any).password = formData.password
    }
    
    if (!user && formData.sendWelcomeEmail) {
      (data as any).sendWelcomeEmail = true
    }
    
    if (user) {
      updateMutation.mutate(data)
    } else {
      createMutation.mutate(data)
    }
  }

  return (
    <Dialog open={open} onOpenChange={() => onClose()}>
      <DialogContent className="sm:max-w-[500px]">
        <DialogHeader>
          <DialogTitle>{user ? 'Edit User' : 'Create New User'}</DialogTitle>
        </DialogHeader>
        
        <div className="space-y-4 py-4">
          <div className="space-y-2">
            <Label htmlFor="email">Email Address</Label>
            <Input
              id="email"
              type="email"
              value={formData.email}
              onChange={(e) => {
                setFormData({ ...formData, email: e.target.value })
                setErrors({ ...errors, email: '' })
              }}
              disabled={!!user}
              className={errors.email ? 'border-red-500' : ''}
            />
            {errors.email && (
              <p className="text-xs text-red-500">{errors.email}</p>
            )}
          </div>
          
          <div className="space-y-2">
            <Label htmlFor="name">Full Name</Label>
            <Input
              id="name"
              value={formData.name}
              onChange={(e) => {
                setFormData({ ...formData, name: e.target.value })
                setErrors({ ...errors, name: '' })
              }}
              className={errors.name ? 'border-red-500' : ''}
            />
            {errors.name && (
              <p className="text-xs text-red-500">{errors.name}</p>
            )}
          </div>
          
          <div className="space-y-2">
            <Label htmlFor="role">Role</Label>
            <Select
              value={formData.role}
              onValueChange={(value) => setFormData({ ...formData, role: value as any })}
            >
              <SelectTrigger>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="admin">Administrator</SelectItem>
                <SelectItem value="user">User</SelectItem>
                <SelectItem value="viewer">Viewer</SelectItem>
              </SelectContent>
            </Select>
            <p className="text-xs text-muted-foreground">
              Administrators have full system access
            </p>
          </div>
          
          {(!user || formData.password) && (
            <>
              <div className="space-y-2">
                <Label htmlFor="password">
                  {user ? 'New Password (leave blank to keep current)' : 'Password'}
                </Label>
                <Input
                  id="password"
                  type="password"
                  value={formData.password}
                  onChange={(e) => {
                    setFormData({ ...formData, password: e.target.value })
                    setErrors({ ...errors, password: '' })
                  }}
                  className={errors.password ? 'border-red-500' : ''}
                />
                {errors.password && (
                  <p className="text-xs text-red-500">{errors.password}</p>
                )}
              </div>
              
              <div className="space-y-2">
                <Label htmlFor="confirmPassword">Confirm Password</Label>
                <Input
                  id="confirmPassword"
                  type="password"
                  value={formData.confirmPassword}
                  onChange={(e) => {
                    setFormData({ ...formData, confirmPassword: e.target.value })
                    setErrors({ ...errors, confirmPassword: '' })
                  }}
                  className={errors.confirmPassword ? 'border-red-500' : ''}
                />
                {errors.confirmPassword && (
                  <p className="text-xs text-red-500">{errors.confirmPassword}</p>
                )}
              </div>
            </>
          )}
          
          <div className="space-y-2">
            <Label htmlFor="storageQuota">Storage Quota (GB)</Label>
            <Input
              id="storageQuota"
              type="number"
              value={formData.storageQuota}
              onChange={(e) => setFormData({ ...formData, storageQuota: parseInt(e.target.value) || 0 })}
              min="0"
            />
            <p className="text-xs text-muted-foreground">
              Set to 0 for unlimited storage
            </p>
          </div>
          
          <div className="flex items-center justify-between">
            <Label htmlFor="enabled">Account Enabled</Label>
            <Switch
              id="enabled"
              checked={formData.enabled}
              onCheckedChange={(checked) => setFormData({ ...formData, enabled: checked })}
            />
          </div>
          
          {!user && (
            <div className="flex items-center justify-between">
              <Label htmlFor="sendWelcomeEmail">Send Welcome Email</Label>
              <Switch
                id="sendWelcomeEmail"
                checked={formData.sendWelcomeEmail}
                onCheckedChange={(checked) => setFormData({ ...formData, sendWelcomeEmail: checked })}
              />
            </div>
          )}
        </div>
        
        <DialogFooter>
          <Button variant="outline" onClick={() => onClose()}>
            Cancel
          </Button>
          <Button 
            onClick={handleSubmit}
            disabled={createMutation.isPending || updateMutation.isPending}
          >
            {createMutation.isPending || updateMutation.isPending ? (
              <>
                <RefreshCw className="mr-2 h-4 w-4 animate-spin" />
                Saving...
              </>
            ) : (
              user ? 'Update User' : 'Create User'
            )}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

// Reset Password Dialog
function ResetPasswordDialog({ 
  user, 
  open, 
  onClose 
}: { 
  user: User | null
  open: boolean
  onClose: () => void 
}) {
  const [password, setPassword] = useState('')
  const [confirmPassword, setConfirmPassword] = useState('')
  const [error, setError] = useState('')
  
  const mutation = useMutation({
    mutationFn: (data: any) => api.post(`/v1/users/${user?.id}/reset-password`, data),
    onSuccess: () => {
      toast.success('Password reset successfully')
      onClose()
    },
    onError: (error: any) => {
      toast.error(error.response?.data?.message || 'Failed to reset password')
    }
  })

  const handleSubmit = () => {
    if (password.length < 8) {
      setError('Password must be at least 8 characters')
      return
    }
    
    if (password !== confirmPassword) {
      setError('Passwords do not match')
      return
    }
    
    mutation.mutate({ password })
  }

  return (
    <Dialog open={open} onOpenChange={onClose}>
      <DialogContent className="sm:max-w-[400px]">
        <DialogHeader>
          <DialogTitle>Reset Password</DialogTitle>
        </DialogHeader>
        
        <div className="space-y-4 py-4">
          <Alert>
            <AlertCircle className="h-4 w-4" />
            <AlertDescription>
              Resetting password for: <strong>{user?.email}</strong>
            </AlertDescription>
          </Alert>
          
          <div className="space-y-2">
            <Label htmlFor="new-password">New Password</Label>
            <Input
              id="new-password"
              type="password"
              value={password}
              onChange={(e) => {
                setPassword(e.target.value)
                setError('')
              }}
            />
          </div>
          
          <div className="space-y-2">
            <Label htmlFor="confirm-new-password">Confirm Password</Label>
            <Input
              id="confirm-new-password"
              type="password"
              value={confirmPassword}
              onChange={(e) => {
                setConfirmPassword(e.target.value)
                setError('')
              }}
            />
          </div>
          
          {error && (
            <p className="text-sm text-red-500">{error}</p>
          )}
        </div>
        
        <DialogFooter>
          <Button variant="outline" onClick={onClose}>
            Cancel
          </Button>
          <Button 
            onClick={handleSubmit}
            disabled={mutation.isPending}
          >
            {mutation.isPending ? (
              <>
                <RefreshCw className="mr-2 h-4 w-4 animate-spin" />
                Resetting...
              </>
            ) : (
              'Reset Password'
            )}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

export function SettingsUsers() {

  const [searchQuery, setSearchQuery] = useState('')
  const [selectedUser, setSelectedUser] = useState<User | null>(null)
  const [showUserDialog, setShowUserDialog] = useState(false)
  const [showResetDialog, setShowResetDialog] = useState(false)
  const [showDeleteDialog, setShowDeleteDialog] = useState(false)
  const queryClient = useQueryClient()

  // Fetch users
  const { data: users = [], isLoading } = useQuery({
    queryKey: ['users'],
    queryFn: async () => {
      const response = await api.get<{ data: User[] }>('/v1/users')
      return response.data
    },
    refetchInterval: 30000, // Refresh every 30 seconds
  })

  // Delete user mutation
  const deleteMutation = useMutation({
    mutationFn: (userId: string) => api.delete(`/v1/users/${userId}`),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['users'] })
      toast.success('User deleted successfully')
      setShowDeleteDialog(false)
      setSelectedUser(null)
    },
    onError: (error: any) => {
      toast.error(error.response?.data?.message || 'Failed to delete user')
    }
  })

  // Toggle user enabled status
  const toggleEnabledMutation = useMutation({
    mutationFn: ({ userId, enabled }: { userId: string, enabled: boolean }) => 
      api.put(`/v1/users/${userId}`, { enabled }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['users'] })
      toast.success('User status updated')
    },
    onError: (error: any) => {
      toast.error(error.response?.data?.message || 'Failed to update user status')
    }
  })

  // Filter users based on search
  const filteredUsers = users.filter((user: User) => 
    user.name.toLowerCase().includes(searchQuery.toLowerCase()) ||
    user.email.toLowerCase().includes(searchQuery.toLowerCase())
  )

  // Table columns
  const columns: ColumnDef<User>[] = [
    {
      accessorKey: 'name',
      header: 'Name',
      cell: ({ row }) => (
        <div className="flex items-center gap-3">
          <div className="h-8 w-8 rounded-full bg-primary/10 flex items-center justify-center">
            <span className="text-sm font-medium">
              {row.original.name.charAt(0).toUpperCase()}
            </span>
          </div>
          <div>
            <div className="font-medium">{row.original.name}</div>
            <div className="text-xs text-muted-foreground">{row.original.email}</div>
          </div>
        </div>
      ),
    },
    {
      accessorKey: 'role',
      header: 'Role',
      cell: ({ row }) => (
        <Badge variant={row.original.role === 'admin' ? 'default' : 'secondary'}>
          {row.original.role}
        </Badge>
      ),
    },
    {
      accessorKey: 'enabled',
      header: 'Status',
      cell: ({ row }) => (
        <div className="flex items-center gap-2">
          {row.original.enabled ? (
            <CheckCircle className="h-4 w-4 text-green-500" />
          ) : (
            <XCircle className="h-4 w-4 text-red-500" />
          )}
          <span className={row.original.enabled ? 'text-green-600' : 'text-red-600'}>
            {row.original.enabled ? 'Active' : 'Disabled'}
          </span>
        </div>
      ),
    },
    {
      accessorKey: 'totpEnabled',
      header: '2FA',
      cell: ({ row }) => (
        row.original.totpEnabled ? (
          <Badge variant="outline" className="gap-1">
            <Shield className="h-3 w-3" />
            Enabled
          </Badge>
        ) : (
          <span className="text-muted-foreground">-</span>
        )
      ),
    },
    {
      accessorKey: 'lastLogin',
      header: 'Last Login',
      cell: ({ row }) => (
        row.original.lastLogin ? (
          <span className="text-sm">
            {formatDistanceToNow(new Date(row.original.lastLogin), { addSuffix: true })}
          </span>
        ) : (
          <span className="text-muted-foreground">Never</span>
        )
      ),
    },
    {
      id: 'actions',
      header: 'Actions',
      cell: ({ row }) => (
        <div className="flex items-center gap-2">
          <Button
            variant="ghost"
            size="sm"
            onClick={() => {
              setSelectedUser(row.original)
              setShowUserDialog(true)
            }}
          >
            <Edit className="h-4 w-4" />
          </Button>
          <Button
            variant="ghost"
            size="sm"
            onClick={() => {
              setSelectedUser(row.original)
              setShowResetDialog(true)
            }}
          >
            <Key className="h-4 w-4" />
          </Button>
          <Button
            variant="ghost"
            size="sm"
            onClick={() => {
              toggleEnabledMutation.mutate({ 
                userId: row.original.id, 
                enabled: !row.original.enabled 
              })
            }}
          >
            {row.original.enabled ? (
              <Lock className="h-4 w-4" />
            ) : (
              <Unlock className="h-4 w-4" />
            )}
          </Button>
          <Button
            variant="ghost"
            size="sm"
            onClick={() => {
              setSelectedUser(row.original)
              setShowDeleteDialog(true)
            }}
          >
            <Trash2 className="h-4 w-4 text-red-500" />
          </Button>
        </div>
      ),
    },
  ]

  return (
    <div className="container mx-auto py-6 space-y-6">
      <PageHeader
        title="User Management"
        description="Manage user accounts and permissions"
        icon={Users}
      />

      {/* Quick Stats */}
      <div className="grid grid-cols-1 md:grid-cols-4 gap-4">
        <Card className="p-4">
          <div className="flex items-center justify-between">
            <div>
              <p className="text-sm text-muted-foreground">Total Users</p>
              <p className="text-2xl font-bold">{users.length}</p>
            </div>
            <Users className="h-8 w-8 text-muted-foreground" />
          </div>
        </Card>
        
        <Card className="p-4">
          <div className="flex items-center justify-between">
            <div>
              <p className="text-sm text-muted-foreground">Active Users</p>
              <p className="text-2xl font-bold">
                {users.filter((u: User) => u.enabled).length}
              </p>
            </div>
            <UserCheck className="h-8 w-8 text-green-500" />
          </div>
        </Card>
        
        <Card className="p-4">
          <div className="flex items-center justify-between">
            <div>
              <p className="text-sm text-muted-foreground">Administrators</p>
              <p className="text-2xl font-bold">
                {users.filter((u: User) => u.role === 'admin').length}
              </p>
            </div>
            <Shield className="h-8 w-8 text-blue-500" />
          </div>
        </Card>
        
        <Card className="p-4">
          <div className="flex items-center justify-between">
            <div>
              <p className="text-sm text-muted-foreground">2FA Enabled</p>
              <p className="text-2xl font-bold">
                {users.filter((u: User) => u.totpEnabled).length}
              </p>
            </div>
            <Lock className="h-8 w-8 text-purple-500" />
          </div>
        </Card>
      </div>

      {/* Users Table */}
      <Card>
        <div className="p-6">
          <div className="flex items-center justify-between mb-6">
            <h2 className="text-lg font-semibold">All Users</h2>
            <div className="flex items-center gap-4">
              <div className="relative">
                <Search className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-muted-foreground" />
                <Input
                  placeholder="Search users..."
                  value={searchQuery}
                  onChange={(e) => setSearchQuery(e.target.value)}
                  className="pl-9 w-64"
                />
              </div>
              <Button
                onClick={() => {
                  setSelectedUser(null)
                  setShowUserDialog(true)
                }}
              >
                <UserPlus className="mr-2 h-4 w-4" />
                Add User
              </Button>
            </div>
          </div>
          
          {isLoading ? (
            <div className="flex items-center justify-center py-8">
              <RefreshCw className="h-6 w-6 animate-spin text-muted-foreground" />
            </div>
          ) : filteredUsers.length === 0 ? (
            <div className="text-center py-8">
              <UserX className="h-12 w-12 text-muted-foreground mx-auto mb-4" />
              <p className="text-muted-foreground">No users found</p>
            </div>
          ) : (
            <DataTable columns={columns} data={filteredUsers} />
          )}
        </div>
      </Card>

      {/* Bulk Actions */}
      <Card>
        <div className="p-6">
          <h3 className="text-lg font-semibold mb-4">Bulk Actions</h3>
          <div className="flex flex-wrap gap-4">
            <Button variant="outline">
              <Download className="mr-2 h-4 w-4" />
              Export Users
            </Button>
            <Button variant="outline">
              <Upload className="mr-2 h-4 w-4" />
              Import Users
            </Button>
            <Button variant="outline">
              <Mail className="mr-2 h-4 w-4" />
              Send Bulk Email
            </Button>
          </div>
        </div>
      </Card>

      {/* Dialogs */}
      <UserDialog
        user={selectedUser}
        open={showUserDialog}
        onClose={(saved) => {
          setShowUserDialog(false)
          if (!saved) setSelectedUser(null)
        }}
      />
      
      <ResetPasswordDialog
        user={selectedUser}
        open={showResetDialog}
        onClose={() => {
          setShowResetDialog(false)
          setSelectedUser(null)
        }}
      />
      
      {/* Delete Confirmation Dialog */}
      <Dialog open={showDeleteDialog} onOpenChange={setShowDeleteDialog}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Delete User</DialogTitle>
          </DialogHeader>
          <Alert variant="destructive">
            <AlertCircle className="h-4 w-4" />
            <AlertDescription>
              Are you sure you want to delete <strong>{selectedUser?.email}</strong>? 
              This action cannot be undone.
            </AlertDescription>
          </Alert>
          <DialogFooter>
            <Button 
              variant="outline" 
              onClick={() => {
                setShowDeleteDialog(false)
                setSelectedUser(null)
              }}
            >
              Cancel
            </Button>
            <Button 
              variant="destructive"
              onClick={() => selectedUser && deleteMutation.mutate(selectedUser.id)}
              disabled={deleteMutation.isPending}
            >
              {deleteMutation.isPending ? (
                <>
                  <RefreshCw className="mr-2 h-4 w-4 animate-spin" />
                  Deleting...
                </>
              ) : (
                'Delete User'
              )}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}
