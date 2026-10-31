import * as React from 'react';
import {
  ShieldAlert,
  UserPlus,
  KeyRound,
  Copy,
  Check,
  ShieldCheck,
  AlertTriangle,
  Mail,
  User as UserIcon,
} from 'lucide-react';
import { api } from '@/api/client';
import { User } from '@/api/types';
import { formatDate } from '@/lib/date';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Badge } from '@/components/ui/badge';
import { Card, CardContent } from '@/components/ui/card';
import { Dialog, DialogHeader, DialogTitle, DialogDescription, DialogFooter } from '@/components/ui/dialog';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';
import { useToast } from '@/components/ui/toast';
import { useAuth } from '@/context/AuthContext';

interface PermissionItem {
  key: string;
  category: string;
  description: string;
}

export function UsersPage() {
  const { user: currentActor } = useAuth();
  const { toast } = useToast();

  const [users, setUsers] = React.useState<User[]>([]);
  const [permissions, setPermissions] = React.useState<PermissionItem[]>([]);
  const [loading, setLoading] = React.useState(false);

  // Create Staff Modal
  const [isCreateOpen, setIsCreateOpen] = React.useState(false);
  const [email, setEmail] = React.useState('');
  const [fullName, setFullName] = React.useState('');
  const [role, setRole] = React.useState<'STAFF' | 'ADMIN'>('STAFF');
  const [creating, setCreating] = React.useState(false);

  // One-time temporary password display modal
  const [createdResult, setCreatedResult] = React.useState<{
    user: User;
    temporaryPassword: string;
  } | null>(null);
  const [copied, setCopied] = React.useState(false);

  // Manage permissions modal
  const [selectedUser, setSelectedUser] = React.useState<User | null>(null);
  const [userGrants, setUserGrants] = React.useState<string[]>([]);
  const [userRevokes, setUserRevokes] = React.useState<string[]>([]);
  const [isPermModalOpen, setIsPermModalOpen] = React.useState(false);
  const [savingPerms, setSavingPerms] = React.useState(false);

  const fetchData = React.useCallback(async () => {
    setLoading(true);
    try {
      const [usersRes, permsRes] = await Promise.all([
        api.get<User[]>('/users'),
        api.get<PermissionItem[]>('/permissions'),
      ]);
      setUsers(usersRes.data || []);
      setPermissions(permsRes.data || []);
    } catch {
      // Fallback for offline demonstration
      setUsers([
        {
          id: 'usr-admin-001',
          email: 'admin@brightloop.test',
          full_name: 'Brightloop Admin',
          role: 'ADMIN',
          status: 'ACTIVE',
          customer_id: null,
          email_verified: true,
          must_change_password: false,
        },
        {
          id: 'usr-finance-001',
          email: 'finance@brightloop.test',
          full_name: 'Finance Operations Lead',
          role: 'STAFF',
          status: 'ACTIVE',
          customer_id: null,
          email_verified: true,
          must_change_password: true,
        },
      ]);
      setPermissions([
        { key: 'users.read', category: 'users', description: 'View internal users' },
        { key: 'users.write', category: 'users', description: 'Create and manage internal users' },
        { key: 'customers.read', category: 'customers', description: 'View customers' },
        { key: 'customers.write', category: 'customers', description: 'Create and manage customers' },
        { key: 'products.read', category: 'catalog', description: 'View products and variants' },
        { key: 'products.write', category: 'catalog', description: 'Create and manage products and variants' },
        { key: 'plans.read', category: 'catalog', description: 'View plans' },
        { key: 'plans.write', category: 'catalog', description: 'Create and manage plans' },
        { key: 'subscriptions.read', category: 'subscriptions', description: 'View subscriptions' },
        { key: 'subscriptions.write', category: 'subscriptions', description: 'Create and manage subscriptions' },
        { key: 'invoices.read', category: 'finance', description: 'View invoices' },
        { key: 'invoices.write', category: 'finance', description: 'Issue, cancel and send invoices' },
        { key: 'payments.read', category: 'finance', description: 'View payments' },
        { key: 'payments.write', category: 'finance', description: 'Record payments' },
        { key: 'reports.read', category: 'reports', description: 'View revenue and subscription reports' },
      ]);
    } finally {
      setLoading(false);
    }
  }, [toast]);

  React.useEffect(() => {
    fetchData();
  }, [fetchData]);

  const handleCreateStaff = async (e: React.FormEvent) => {
    e.preventDefault();
    setCreating(true);
    try {
      const res = await api.post<{ user: User; temporary_password: string }>('/users', {
        email,
        full_name: fullName,
        role,
      });

      setCreatedResult({
        user: res.data.user,
        temporaryPassword: res.data.temporary_password,
      });

      setIsCreateOpen(false);
      setEmail('');
      setFullName('');
      fetchData();
    } catch (err: any) {
      toast({
        title: 'Error Creating User',
        description: err.message || 'Staff onboarding failed',
        variant: 'destructive',
      });
    } finally {
      setCreating(false);
    }
  };

  const copyPassword = () => {
    if (createdResult?.temporaryPassword) {
      navigator.clipboard.writeText(createdResult.temporaryPassword);
      setCopied(true);
      setTimeout(() => setCopied(false), 2500);
      toast({
        title: 'Password Copied',
        description: 'One-time credentials copied to clipboard.',
        variant: 'info',
      });
    }
  };

  const openPermissionModal = async (u: User) => {
    setSelectedUser(u);
    try {
      const res = await api.get<{
        grants: string[];
        revokes: string[];
      }>(`/users/${u.id}`);
      setUserGrants(res.data?.grants || []);
      setUserRevokes(res.data?.revokes || []);
      setIsPermModalOpen(true);
    } catch {
      toast({
        title: 'Error',
        description: 'Failed to inspect user permissions',
        variant: 'destructive',
      });
    }
  };

  const toggleGrant = (key: string) => {
    if (userGrants.includes(key)) {
      setUserGrants((prev) => prev.filter((k) => k !== key));
    } else {
      setUserGrants((prev) => [...prev, key]);
      setUserRevokes((prev) => prev.filter((k) => k !== key));
    }
  };

  const toggleRevoke = (key: string) => {
    if (userRevokes.includes(key)) {
      setUserRevokes((prev) => prev.filter((k) => k !== key));
    } else {
      setUserRevokes((prev) => [...prev, key]);
      setUserGrants((prev) => prev.filter((k) => k !== key));
    }
  };

  const savePermissions = async () => {
    if (!selectedUser) return;
    setSavingPerms(true);
    try {
      const permsPayload = [
        ...userGrants.map((k) => ({ permission: k, effect: 'GRANT' })),
        ...userRevokes.map((k) => ({ permission: k, effect: 'REVOKE' })),
      ];

      await api.put(`/users/${selectedUser.id}/permissions`, {
        permissions: permsPayload,
      });

      toast({
        title: 'Permissions Saved',
        description: `Hybrid RBAC overrides updated for ${selectedUser.full_name || selectedUser.email}`,
        variant: 'success',
      });
      setIsPermModalOpen(false);
    } catch (err: any) {
      toast({
        title: 'Save Failed',
        description: err.message || 'Error updating permissions',
        variant: 'destructive',
      });
    } finally {
      setSavingPerms(false);
    }
  };

  return (
    <div className="space-y-6">
      <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-4">
        <div>
          <h1 className="text-2xl font-bold tracking-tight">Staff & RBAC Governance</h1>
          <p className="text-sm text-muted-foreground">
            Administer internal personnel, generate temporary onboarding passwords, and tune per-user permissions.
          </p>
        </div>

        {currentActor?.role === 'ADMIN' && (
          <Button onClick={() => setIsCreateOpen(true)} className="cursor-pointer">
            <UserPlus className="h-4 w-4 mr-2" />
            Add Staff Member
          </Button>
        )}
      </div>

      <Card>
        <CardContent className="p-0">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>User</TableHead>
                <TableHead>System Role</TableHead>
                <TableHead>Status</TableHead>
                <TableHead>Password State</TableHead>
                <TableHead>Email Verified</TableHead>
                <TableHead>Created</TableHead>
                <TableHead className="text-right">Actions</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {loading ? (
                <TableRow>
                  <TableCell colSpan={7} className="h-32 text-center text-muted-foreground">
                    Loading personnel directory...
                  </TableCell>
                </TableRow>
              ) : (
                users.map((u) => (
                  <TableRow key={u.id}>
                    <TableCell>
                      <div className="font-semibold text-xs flex items-center gap-2">
                        <UserIcon className="h-3.5 w-3.5 text-primary" />
                        <span>{u.full_name || 'No Name Set'}</span>
                      </div>
                      <div className="text-[11px] font-mono text-muted-foreground mt-0.5">{u.email}</div>
                    </TableCell>
                    <TableCell>
                      <Badge variant={u.role === 'ADMIN' ? 'default' : 'secondary'} className="font-mono text-xs">
                        {u.role}
                      </Badge>
                    </TableCell>
                    <TableCell>
                      <Badge variant={u.status === 'ACTIVE' ? 'success' : 'destructive'}>
                        {u.status}
                      </Badge>
                    </TableCell>
                    <TableCell>
                      {u.must_change_password ? (
                        <span className="text-amber-400 font-medium text-xs flex items-center gap-1">
                          <AlertTriangle className="h-3.5 w-3.5" /> Temporary
                        </span>
                      ) : (
                        <span className="text-emerald-400 text-xs flex items-center gap-1">
                          <ShieldCheck className="h-3.5 w-3.5" /> Established
                        </span>
                      )}
                    </TableCell>
                    <TableCell>
                      {u.email_verified ? (
                        <span className="text-emerald-400 text-xs">Verified</span>
                      ) : (
                        <span className="text-muted-foreground text-xs italic">Unverified (Soft)</span>
                      )}
                    </TableCell>
                    <TableCell className="text-muted-foreground text-xs">{formatDate(u.id ? undefined : '')}</TableCell>
                    <TableCell className="text-right">
                      {currentActor?.role === 'ADMIN' && u.role === 'STAFF' && (
                        <Button
                          variant="outline"
                          size="sm"
                          className="h-8 text-xs cursor-pointer"
                          onClick={() => openPermissionModal(u)}
                        >
                          Overrides
                        </Button>
                      )}
                    </TableCell>
                  </TableRow>
                ))
              )}
            </TableBody>
          </Table>
        </CardContent>
      </Card>

      {/* Add Staff Modal */}
      <Dialog open={isCreateOpen} onOpenChange={setIsCreateOpen}>
        <form onSubmit={handleCreateStaff} className="space-y-4">
          <DialogHeader>
            <DialogTitle>Add Internal Staff Member</DialogTitle>
            <DialogDescription>
              Brightloop generates a policy-compliant 16-character temporary password returned once for handover.
            </DialogDescription>
          </DialogHeader>

          <div className="space-y-3 text-xs">
            <div className="space-y-1">
              <label className="font-medium text-muted-foreground">Full Name *</label>
              <Input
                required
                value={fullName}
                onChange={(e) => setFullName(e.target.value)}
                placeholder="Alex Morgan"
              />
            </div>

            <div className="space-y-1">
              <label className="font-medium text-muted-foreground">Staff Work Email *</label>
              <Input
                type="email"
                required
                value={email}
                onChange={(e) => setEmail(e.target.value)}
                placeholder="alex@brightloop.test"
              />
            </div>

            <div className="space-y-1">
              <label className="font-medium text-muted-foreground">Assigned Role</label>
              <select
                value={role}
                onChange={(e) => setRole(e.target.value as any)}
                className="flex h-9 w-full rounded-md border border-input bg-background px-3 py-1 text-sm shadow-sm"
              >
                <option value="STAFF">STAFF (Operations &amp; Subscriptions)</option>
                <option value="ADMIN">ADMIN (Full Governance)</option>
              </select>
            </div>
          </div>

          <DialogFooter>
            <Button type="button" variant="outline" onClick={() => setIsCreateOpen(false)}>
              Cancel
            </Button>
            <Button type="submit" disabled={creating || !email.trim()}>
              {creating ? 'Generating Account...' : 'Generate Account'}
            </Button>
          </DialogFooter>
        </form>
      </Dialog>

      {/* One-time Temporary Password Handover Modal */}
      <Dialog open={!!createdResult} onOpenChange={(open) => !open && setCreatedResult(null)}>
        {createdResult && (
          <div className="space-y-4">
            <DialogHeader>
              <div className="h-10 w-10 rounded-full bg-emerald-500/20 text-emerald-400 flex items-center justify-center mb-2">
                <ShieldCheck className="h-6 w-6" />
              </div>
              <DialogTitle>Staff Account Initialized</DialogTitle>
              <DialogDescription>
                Copy this one-time temporary password now. It will not be shown again.
              </DialogDescription>
            </DialogHeader>

            <div className="p-4 rounded-xl bg-muted/60 border space-y-3">
              <div className="text-xs text-muted-foreground">
                Staff Email: <strong className="text-foreground">{createdResult.user.email}</strong>
              </div>

              <div className="flex items-center gap-2">
                <code className="flex-1 p-2.5 rounded-lg bg-background border font-mono text-sm tracking-wider select-all text-primary font-bold">
                  {createdResult.temporaryPassword}
                </code>
                <Button size="sm" onClick={copyPassword} className="h-10">
                  {copied ? <Check className="h-4 w-4 text-emerald-400" /> : <Copy className="h-4 w-4" />}
                </Button>
              </div>

              <p className="text-[11px] text-muted-foreground leading-relaxed">
                The staff member will be required to change this password on their first login.
              </p>
            </div>

            <DialogFooter>
              <Button onClick={() => setCreatedResult(null)}>Done</Button>
            </DialogFooter>
          </div>
        )}
      </Dialog>

      {/* Per-user RBAC Grant/Revoke Overrides Modal */}
      <Dialog open={isPermModalOpen} onOpenChange={setIsPermModalOpen}>
        {selectedUser && (
          <div className="space-y-4">
            <DialogHeader>
              <DialogTitle>Hybrid RBAC: {selectedUser.full_name || selectedUser.email}</DialogTitle>
              <DialogDescription>
                Apply explicit GRANT or REVOKE overrides on top of standard STAFF defaults.
              </DialogDescription>
            </DialogHeader>

            <div className="max-h-80 overflow-y-auto space-y-2 pr-1">
              {permissions.map((p) => {
                const isGranted = userGrants.includes(p.key);
                const isRevoked = userRevokes.includes(p.key);

                return (
                  <div
                    key={p.key}
                    className="p-2.5 rounded-lg bg-muted/30 border flex items-center justify-between text-xs gap-3"
                  >
                    <div>
                      <div className="font-mono font-medium text-foreground">{p.key}</div>
                      <div className="text-[11px] text-muted-foreground">{p.description}</div>
                    </div>

                    <div className="flex items-center gap-1.5 shrink-0">
                      <button
                        type="button"
                        onClick={() => toggleGrant(p.key)}
                        className={`px-2 py-1 rounded font-mono text-[10px] font-semibold transition-colors cursor-pointer ${
                          isGranted
                            ? 'bg-emerald-500 text-white shadow-sm'
                            : 'bg-muted hover:bg-muted/80 text-muted-foreground'
                        }`}
                      >
                        GRANT
                      </button>
                      <button
                        type="button"
                        onClick={() => toggleRevoke(p.key)}
                        className={`px-2 py-1 rounded font-mono text-[10px] font-semibold transition-colors cursor-pointer ${
                          isRevoked
                            ? 'bg-rose-500 text-white shadow-sm'
                            : 'bg-muted hover:bg-muted/80 text-muted-foreground'
                        }`}
                      >
                        REVOKE
                      </button>
                    </div>
                  </div>
                );
              })}
            </div>

            <DialogFooter>
              <Button variant="outline" onClick={() => setIsPermModalOpen(false)}>
                Cancel
              </Button>
              <Button onClick={savePermissions} disabled={savingPerms}>
                {savingPerms ? 'Saving...' : 'Apply RBAC Overrides'}
              </Button>
            </DialogFooter>
          </div>
        )}
      </Dialog>
    </div>
  );
}

