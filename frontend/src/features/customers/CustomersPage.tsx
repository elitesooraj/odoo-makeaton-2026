import * as React from 'react';
import {
  Users,
  Search,
  Plus,
  Building,
  Mail,
  Phone,
  MapPin,
  FileEdit,
  Trash2,
  CheckCircle2,
  XCircle,
  MoreVertical,
  ExternalLink,
} from 'lucide-react';
import { customersApi, CustomerListParams } from '@/api/customers';
import { Customer, CustomerCreateInput, CustomerUpdateInput } from '@/api/types';
import { formatDate } from '@/lib/date';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Badge } from '@/components/ui/badge';
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from '@/components/ui/card';
import { Dialog, DialogHeader, DialogTitle, DialogDescription, DialogFooter } from '@/components/ui/dialog';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';
import { useToast } from '@/components/ui/toast';
import { PermissionGate } from '@/components/auth/PermissionGate';

export function CustomersPage() {
  const { toast } = useToast();

  const [customers, setCustomers] = React.useState<Customer[]>([]);
  const [total, setTotal] = React.useState(0);
  const [page, setPage] = React.useState(1);
  const [perPage] = React.useState(10);
  const [search, setSearch] = React.useState('');
  const [statusFilter, setStatusFilter] = React.useState<string>('');
  const [loading, setLoading] = React.useState(false);

  // Modals state
  const [selectedCustomer, setSelectedCustomer] = React.useState<Customer | null>(null);
  const [isCreateOpen, setIsCreateOpen] = React.useState(false);
  const [isEditOpen, setIsEditOpen] = React.useState(false);
  const [isDetailOpen, setIsDetailOpen] = React.useState(false);
  const [isDeactivateOpen, setIsDeactivateOpen] = React.useState(false);

  // Form states
  const [formName, setFormName] = React.useState('');
  const [formEmail, setFormEmail] = React.useState('');
  const [formPhone, setFormPhone] = React.useState('');
  const [formTaxId, setFormTaxId] = React.useState('');
  const [formStreet, setFormStreet] = React.useState('');
  const [formCity, setFormCity] = React.useState('');
  const [formState, setFormState] = React.useState('');
  const [formPostal, setFormPostal] = React.useState('');
  const [formCountry, setFormCountry] = React.useState('');
  const [formNotes, setFormNotes] = React.useState('');
  const [formStatus, setFormStatus] = React.useState<'ACTIVE' | 'INACTIVE'>('ACTIVE');
  const [saving, setSaving] = React.useState(false);

  const fetchCustomers = React.useCallback(async () => {
    setLoading(true);
    try {
      const res = await customersApi.list({
        search,
        status: statusFilter,
        page,
        per_page: perPage,
      });
      setCustomers(res.data || []);
      setTotal(res.meta?.total || 0);
    } catch {
      toast({
        title: 'Error',
        description: 'Failed to fetch customers list',
        variant: 'destructive',
      });
    } finally {
      setLoading(false);
    }
  }, [search, statusFilter, page, perPage, toast]);

  React.useEffect(() => {
    fetchCustomers();
  }, [fetchCustomers]);

  const openCreate = () => {
    setFormName('');
    setFormEmail('');
    setFormPhone('');
    setFormTaxId('');
    setFormStreet('');
    setFormCity('');
    setFormState('');
    setFormPostal('');
    setFormCountry('');
    setFormNotes('');
    setFormStatus('ACTIVE');
    setIsCreateOpen(true);
  };

  const openEdit = (c: Customer) => {
    setSelectedCustomer(c);
    setFormName(c.name);
    setFormEmail(c.email || '');
    setFormPhone(c.phone || '');
    setFormTaxId(c.tax_id || '');
    setFormStreet(c.billing_address?.street || '');
    setFormCity(c.billing_address?.city || '');
    setFormState(c.billing_address?.state || '');
    setFormPostal(c.billing_address?.postal_code || '');
    setFormCountry(c.billing_address?.country || '');
    setFormNotes(c.notes || '');
    setFormStatus(c.status);
    setIsEditOpen(true);
  };

  const handleCreate = async (e: React.FormEvent) => {
    e.preventDefault();
    setSaving(true);
    try {
      const payload: CustomerCreateInput = {
        name: formName,
        email: formEmail || undefined,
        phone: formPhone || undefined,
        tax_id: formTaxId || undefined,
        notes: formNotes || undefined,
        billing_address: {
          street: formStreet,
          city: formCity,
          state: formState,
          postal_code: formPostal,
          country: formCountry,
        },
      };
      await customersApi.create(payload);
      toast({
        title: 'Customer Created',
        description: `${formName} successfully recorded.`,
        variant: 'success',
      });
      setIsCreateOpen(false);
      fetchCustomers();
    } catch (err: any) {
      toast({
        title: 'Validation Failed',
        description: err.message || 'Please verify form fields',
        variant: 'destructive',
      });
    } finally {
      setSaving(false);
    }
  };

  const handleUpdate = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!selectedCustomer) return;
    setSaving(true);
    try {
      const payload: CustomerUpdateInput = {
        name: formName,
        email: formEmail || undefined,
        phone: formPhone || undefined,
        tax_id: formTaxId || undefined,
        notes: formNotes || undefined,
        status: formStatus,
        billing_address: {
          street: formStreet,
          city: formCity,
          state: formState,
          postal_code: formPostal,
          country: formCountry,
        },
      };
      await customersApi.update(selectedCustomer.id, payload);
      toast({
        title: 'Customer Updated',
        description: 'Changes recorded successfully.',
        variant: 'success',
      });
      setIsEditOpen(false);
      fetchCustomers();
    } catch (err: any) {
      toast({
        title: 'Update Failed',
        description: err.message || 'Error updating customer record',
        variant: 'destructive',
      });
    } finally {
      setSaving(false);
    }
  };

  const handleDeactivate = async () => {
    if (!selectedCustomer) return;
    setSaving(true);
    try {
      await customersApi.deactivate(selectedCustomer.id);
      toast({
        title: 'Customer Deactivated',
        description: `${selectedCustomer.name} has been marked inactive.`,
        variant: 'info',
      });
      setIsDeactivateOpen(false);
      fetchCustomers();
    } catch (err: any) {
      toast({
        title: 'Deactivation Failed',
        description: err.message || 'Could not deactivate customer',
        variant: 'destructive',
      });
    } finally {
      setSaving(false);
    }
  };

  return (
    <div className="space-y-6">
      {/* Top Header */}
      <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-4">
        <div>
          <h1 className="text-2xl font-bold tracking-tight">Customer Accounts</h1>
          <p className="text-sm text-muted-foreground">
            Manage Brightloop client profiles, tax identifiers, and billing contacts.
          </p>
        </div>

        <PermissionGate permission="customers.write">
          <Button onClick={openCreate} className="cursor-pointer">
            <Plus className="h-4 w-4 mr-2" />
            Add Customer
          </Button>
        </PermissionGate>
      </div>

      {/* Filter and Search Bar */}
      <Card>
        <CardContent className="p-4 flex flex-col md:flex-row gap-4 items-center justify-between">
          <div className="relative w-full md:w-96">
            <Search className="absolute left-3 top-2.5 h-4 w-4 text-muted-foreground" />
            <Input
              placeholder="Search by name or email..."
              value={search}
              onChange={(e) => setSearch(e.target.value)}
              className="pl-9 h-9"
            />
          </div>

          {/* Status filter tabs */}
          <div className="flex items-center gap-1.5 self-start md:self-auto">
            <button
              onClick={() => {
                setStatusFilter('');
                setPage(1);
              }}
              className={`px-3 py-1 text-xs rounded-md font-medium transition-colors cursor-pointer ${
                statusFilter === '' ? 'bg-primary text-primary-foreground' : 'bg-muted text-muted-foreground hover:text-foreground'
              }`}
            >
              All Statuses
            </button>
            <button
              onClick={() => {
                setStatusFilter('ACTIVE');
                setPage(1);
              }}
              className={`px-3 py-1 text-xs rounded-md font-medium transition-colors cursor-pointer ${
                statusFilter === 'ACTIVE' ? 'bg-emerald-500/20 text-emerald-400 border border-emerald-500/30' : 'bg-muted text-muted-foreground hover:text-foreground'
              }`}
            >
              Active
            </button>
            <button
              onClick={() => {
                setStatusFilter('INACTIVE');
                setPage(1);
              }}
              className={`px-3 py-1 text-xs rounded-md font-medium transition-colors cursor-pointer ${
                statusFilter === 'INACTIVE' ? 'bg-rose-500/20 text-rose-400 border border-rose-500/30' : 'bg-muted text-muted-foreground hover:text-foreground'
              }`}
            >
              Inactive
            </button>
          </div>
        </CardContent>
      </Card>

      {/* Customers Table */}
      <Card>
        <CardContent className="p-0">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Customer / Company</TableHead>
                <TableHead>Contact Email</TableHead>
                <TableHead>Phone</TableHead>
                <TableHead>Tax ID</TableHead>
                <TableHead>Status</TableHead>
                <TableHead>Registered</TableHead>
                <TableHead className="text-right">Actions</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {loading ? (
                <TableRow>
                  <TableCell colSpan={7} className="h-32 text-center text-muted-foreground">
                    Loading customers...
                  </TableCell>
                </TableRow>
              ) : customers.length === 0 ? (
                <TableRow>
                  <TableCell colSpan={7} className="h-32 text-center text-muted-foreground">
                    No customers found matching the criteria.
                  </TableCell>
                </TableRow>
              ) : (
                customers.map((c) => (
                  <TableRow key={c.id}>
                    <TableCell>
                      <button
                        onClick={() => {
                          setSelectedCustomer(c);
                          setIsDetailOpen(true);
                        }}
                        className="font-medium hover:underline text-left cursor-pointer flex items-center gap-2"
                      >
                        <Building className="h-4 w-4 text-primary shrink-0" />
                        <span>{c.name}</span>
                      </button>
                    </TableCell>
                    <TableCell className="text-muted-foreground font-mono text-xs">
                      {c.email || '—'}
                    </TableCell>
                    <TableCell className="text-muted-foreground text-xs">{c.phone || '—'}</TableCell>
                    <TableCell className="text-muted-foreground font-mono text-xs">{c.tax_id || '—'}</TableCell>
                    <TableCell>
                      <Badge variant={c.status === 'ACTIVE' ? 'success' : 'secondary'}>
                        {c.status}
                      </Badge>
                    </TableCell>
                    <TableCell className="text-muted-foreground text-xs">{formatDate(c.created_at)}</TableCell>
                    <TableCell className="text-right space-x-1">
                      <Button
                        variant="ghost"
                        size="sm"
                        className="h-8 text-xs cursor-pointer"
                        onClick={() => {
                          setSelectedCustomer(c);
                          setIsDetailOpen(true);
                        }}
                      >
                        View
                      </Button>
                      <PermissionGate permission="customers.write">
                        <Button
                          variant="ghost"
                          size="sm"
                          className="h-8 text-xs cursor-pointer"
                          onClick={() => openEdit(c)}
                        >
                          <FileEdit className="h-3.5 w-3.5" />
                        </Button>
                        {c.status === 'ACTIVE' && (
                          <Button
                            variant="ghost"
                            size="sm"
                            className="h-8 text-xs text-rose-400 hover:text-rose-300 cursor-pointer"
                            onClick={() => {
                              setSelectedCustomer(c);
                              setIsDeactivateOpen(true);
                            }}
                          >
                            <Trash2 className="h-3.5 w-3.5" />
                          </Button>
                        )}
                      </PermissionGate>
                    </TableCell>
                  </TableRow>
                ))
              )}
            </TableBody>
          </Table>

          {/* Pagination bar */}
          <div className="p-4 border-t flex items-center justify-between text-xs text-muted-foreground">
            <div>
              Total: <span className="font-semibold text-foreground">{total}</span> customers
            </div>
            <div className="flex items-center gap-2">
              <Button
                variant="outline"
                size="sm"
                disabled={page <= 1 || loading}
                onClick={() => setPage((p) => Math.max(1, p - 1))}
              >
                Previous
              </Button>
              <span className="px-2">Page {page}</span>
              <Button
                variant="outline"
                size="sm"
                disabled={page * perPage >= total || loading}
                onClick={() => setPage((p) => p + 1)}
              >
                Next
              </Button>
            </div>
          </div>
        </CardContent>
      </Card>

      {/* Customer Detail Drawer / Modal */}
      <Dialog open={isDetailOpen} onOpenChange={setIsDetailOpen}>
        {selectedCustomer && (
          <div className="space-y-4">
            <DialogHeader>
              <div className="flex items-center gap-2 text-primary">
                <Building className="h-5 w-5" />
                <DialogTitle>{selectedCustomer.name}</DialogTitle>
              </div>
              <DialogDescription>
                Customer ID: <code className="text-xs">{selectedCustomer.id}</code>
              </DialogDescription>
            </DialogHeader>

            <div className="grid grid-cols-2 gap-4 text-xs">
              <div className="space-y-1 p-3 rounded-lg bg-muted/40 border">
                <div className="text-muted-foreground flex items-center gap-1.5">
                  <Mail className="h-3.5 w-3.5" /> Contact Email
                </div>
                <div className="font-medium font-mono">{selectedCustomer.email || 'None provided'}</div>
              </div>

              <div className="space-y-1 p-3 rounded-lg bg-muted/40 border">
                <div className="text-muted-foreground flex items-center gap-1.5">
                  <Phone className="h-3.5 w-3.5" /> Phone Number
                </div>
                <div className="font-medium">{selectedCustomer.phone || 'None provided'}</div>
              </div>

              <div className="space-y-1 p-3 rounded-lg bg-muted/40 border">
                <div className="text-muted-foreground">Tax Identifier (VAT/GST)</div>
                <div className="font-medium font-mono">{selectedCustomer.tax_id || 'None'}</div>
              </div>

              <div className="space-y-1 p-3 rounded-lg bg-muted/40 border">
                <div className="text-muted-foreground">Account Status</div>
                <Badge variant={selectedCustomer.status === 'ACTIVE' ? 'success' : 'secondary'}>
                  {selectedCustomer.status}
                </Badge>
              </div>

              <div className="col-span-2 space-y-1.5 p-3 rounded-lg bg-muted/40 border">
                <div className="text-muted-foreground flex items-center gap-1.5">
                  <MapPin className="h-3.5 w-3.5" /> Billing Address
                </div>
                <div className="font-medium leading-relaxed">
                  {selectedCustomer.billing_address?.street || 'No street address'},{' '}
                  {selectedCustomer.billing_address?.city || ''}{' '}
                  {selectedCustomer.billing_address?.state || ''}{' '}
                  {selectedCustomer.billing_address?.postal_code || ''},{' '}
                  {selectedCustomer.billing_address?.country || 'Not specified'}
                </div>
              </div>

              {selectedCustomer.notes && (
                <div className="col-span-2 space-y-1 p-3 rounded-lg bg-muted/40 border">
                  <div className="text-muted-foreground">Internal Notes</div>
                  <div className="italic text-muted-foreground">{selectedCustomer.notes}</div>
                </div>
              )}
            </div>

            <DialogFooter>
              <Button variant="outline" onClick={() => setIsDetailOpen(false)}>
                Close
              </Button>
            </DialogFooter>
          </div>
        )}
      </Dialog>

      {/* Create / Edit Customer Modal */}
      <Dialog
        open={isCreateOpen || isEditOpen}
        onOpenChange={(open) => {
          if (!open) {
            setIsCreateOpen(false);
            setIsEditOpen(false);
          }
        }}
      >
        <form onSubmit={isCreateOpen ? handleCreate : handleUpdate} className="space-y-4">
          <DialogHeader>
            <DialogTitle>{isCreateOpen ? 'Create Customer' : 'Edit Customer'}</DialogTitle>
            <DialogDescription>
              {isCreateOpen
                ? 'Register a new business account for subscriptions and billing.'
                : 'Modify customer billing information and address.'}
            </DialogDescription>
          </DialogHeader>

          <div className="space-y-3 text-xs">
            <div className="space-y-1">
              <label className="font-medium text-muted-foreground">Company / Customer Name *</label>
              <Input
                required
                value={formName}
                onChange={(e) => setFormName(e.target.value)}
                placeholder="Acme Global Inc"
              />
            </div>

            <div className="grid grid-cols-2 gap-3">
              <div className="space-y-1">
                <label className="font-medium text-muted-foreground">Billing Email</label>
                <Input
                  type="email"
                  value={formEmail}
                  onChange={(e) => setFormEmail(e.target.value)}
                  placeholder="billing@acme.com"
                />
              </div>

              <div className="space-y-1">
                <label className="font-medium text-muted-foreground">Phone Number</label>
                <Input
                  value={formPhone}
                  onChange={(e) => setFormPhone(e.target.value)}
                  placeholder="+1 (555) 019-2834"
                />
              </div>
            </div>

            <div className="grid grid-cols-2 gap-3">
              <div className="space-y-1">
                <label className="font-medium text-muted-foreground">Tax Identifier (VAT / EIN)</label>
                <Input
                  value={formTaxId}
                  onChange={(e) => setFormTaxId(e.target.value)}
                  placeholder="US-94827104"
                />
              </div>

              {isEditOpen && (
                <div className="space-y-1">
                  <label className="font-medium text-muted-foreground">Status</label>
                  <select
                    value={formStatus}
                    onChange={(e) => setFormStatus(e.target.value as any)}
                    className="flex h-9 w-full rounded-md border border-input bg-background px-3 py-1 text-sm shadow-sm"
                  >
                    <option value="ACTIVE">ACTIVE</option>
                    <option value="INACTIVE">INACTIVE</option>
                  </select>
                </div>
              )}
            </div>

            {/* Address group */}
            <div className="pt-2 border-t space-y-2">
              <span className="font-semibold text-muted-foreground">Billing Address</span>
              <div className="space-y-1">
                <Input
                  placeholder="Street Address"
                  value={formStreet}
                  onChange={(e) => setFormStreet(e.target.value)}
                />
              </div>
              <div className="grid grid-cols-2 gap-2">
                <Input
                  placeholder="City"
                  value={formCity}
                  onChange={(e) => setFormCity(e.target.value)}
                />
                <Input
                  placeholder="State / Province"
                  value={formState}
                  onChange={(e) => setFormState(e.target.value)}
                />
              </div>
              <div className="grid grid-cols-2 gap-2">
                <Input
                  placeholder="Postal Code"
                  value={formPostal}
                  onChange={(e) => setFormPostal(e.target.value)}
                />
                <Input
                  placeholder="Country (ISO or Name)"
                  value={formCountry}
                  onChange={(e) => setFormCountry(e.target.value)}
                />
              </div>
            </div>

            <div className="space-y-1">
              <label className="font-medium text-muted-foreground">Internal Notes</label>
              <Input
                placeholder="Optional billing reminders or contract notes"
                value={formNotes}
                onChange={(e) => setFormNotes(e.target.value)}
              />
            </div>
          </div>

          <DialogFooter>
            <Button
              type="button"
              variant="outline"
              onClick={() => {
                setIsCreateOpen(false);
                setIsEditOpen(false);
              }}
            >
              Cancel
            </Button>
            <Button type="submit" disabled={saving || !formName.trim()}>
              {saving ? 'Saving...' : isCreateOpen ? 'Create Customer' : 'Save Changes'}
            </Button>
          </DialogFooter>
        </form>
      </Dialog>

      {/* Deactivate Confirmation Dialog */}
      <Dialog open={isDeactivateOpen} onOpenChange={setIsDeactivateOpen}>
        {selectedCustomer && (
          <div className="space-y-4">
            <DialogHeader>
              <DialogTitle className="text-rose-400">Deactivate Customer</DialogTitle>
              <DialogDescription>
                Are you sure you want to deactivate <strong>{selectedCustomer.name}</strong>?
                This soft-deactivates the account. Existing invoices and subscription historical records will be preserved.
              </DialogDescription>
            </DialogHeader>

            <DialogFooter>
              <Button variant="outline" onClick={() => setIsDeactivateOpen(false)}>
                Cancel
              </Button>
              <Button variant="destructive" onClick={handleDeactivate} disabled={saving}>
                {saving ? 'Deactivating...' : 'Confirm Deactivation'}
              </Button>
            </DialogFooter>
          </div>
        )}
      </Dialog>
    </div>
  );
}

