import * as React from 'react';
import {
  Tag, Plus, Search, RefreshCw, CheckCircle2, XCircle,
  Percent, DollarSign, Clock, AlertTriangle, Flame,
} from 'lucide-react';
import { Card, CardContent } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';
import { Dialog, DialogHeader, DialogTitle, DialogDescription, DialogFooter } from '@/components/ui/dialog';
import { useToast } from '@/components/ui/toast';
import { formatCents, formatBasisPoints, dollarsToCents } from '@/lib/money';
import { discountsApi, Discount, DiscountCreateInput, DiscountUpdateInput, DiscountKind, DiscountScope } from '@/api/finance';

function discountValueLabel(d: Discount): string {
  return d.kind === 'PERCENT' ? formatBasisPoints(d.value_bp) : formatCents(d.value_cents);
}

function isExpired(d: Discount): boolean {
  if (!d.ends_at) return false;
  return new Date(d.ends_at) < new Date();
}

function isMaxedOut(d: Discount): boolean {
  return d.max_uses !== null && d.used_count >= d.max_uses;
}

export function DiscountsPage() {
  const { addToast } = useToast();
  const [discounts, setDiscounts] = React.useState<Discount[]>([]);
  const [loading, setLoading] = React.useState(true);
  const [search, setSearch] = React.useState('');
  const [activeOnly, setActiveOnly] = React.useState(false);

  const [isCreateOpen, setIsCreateOpen] = React.useState(false);
  const [isEditOpen, setIsEditOpen] = React.useState(false);
  const [selected, setSelected] = React.useState<Discount | null>(null);
  const [saving, setSaving] = React.useState(false);

  // Form fields
  const [fName, setFName] = React.useState('');
  const [fKind, setFKind] = React.useState<DiscountKind>('PERCENT');
  const [fValuePercent, setFValuePercent] = React.useState('');
  const [fValueDollar, setFValueDollar] = React.useState('');
  const [fScope, setFScope] = React.useState<DiscountScope>('SUBSCRIPTION');
  const [fProductId, setFProductId] = React.useState('');
  const [fMinPurchase, setFMinPurchase] = React.useState('');
  const [fMinQty, setFMinQty] = React.useState('');
  const [fStartsAt, setFStartsAt] = React.useState('');
  const [fEndsAt, setFEndsAt] = React.useState('');
  const [fMaxUses, setFMaxUses] = React.useState('');
  const [fActive, setFActive] = React.useState(true);

  function resetForm() {
    setFName(''); setFKind('PERCENT'); setFValuePercent(''); setFValueDollar('');
    setFScope('SUBSCRIPTION'); setFProductId(''); setFMinPurchase('');
    setFMinQty(''); setFStartsAt(''); setFEndsAt(''); setFMaxUses(''); setFActive(true);
  }

  function openEdit(d: Discount) {
    setSelected(d);
    setFName(d.name); setFKind(d.kind);
    setFValuePercent(d.kind === 'PERCENT' ? String(d.value_bp / 100) : '');
    setFValueDollar(d.kind === 'FIXED' ? String(d.value_cents / 100) : '');
    setFScope(d.scope); setFProductId(d.product_id ?? '');
    setFMinPurchase(d.min_purchase_cents ? String(d.min_purchase_cents / 100) : '');
    setFMinQty(d.min_quantity ? String(d.min_quantity) : '');
    setFStartsAt(d.starts_at ? d.starts_at.slice(0, 10) : '');
    setFEndsAt(d.ends_at ? d.ends_at.slice(0, 10) : '');
    setFMaxUses(d.max_uses ? String(d.max_uses) : '');
    setFActive(d.is_active);
    setIsEditOpen(true);
  }

  const load = React.useCallback(async () => {
    setLoading(true);
    try {
      const res = await discountsApi.list({ search: search || undefined, active: activeOnly || undefined });
      setDiscounts(res.data ?? []);
    } catch {
      addToast({ title: 'Failed to load discounts', variant: 'destructive' });
    } finally {
      setLoading(false);
    }
  }, [search, activeOnly]);

  React.useEffect(() => { load(); }, [load]);

  function buildInput(isActiveOverride?: boolean): DiscountCreateInput & { is_active?: boolean } {
    const percentBP = fKind === 'PERCENT' ? Math.round(parseFloat(fValuePercent) * 100) : 0;
    const fixedCents = fKind === 'FIXED' ? dollarsToCents(fValueDollar) : 0;
    return {
      name: fName.trim(), kind: fKind,
      value_bp: percentBP, value_cents: fixedCents,
      scope: fScope,
      product_id: fProductId.trim() || undefined,
      min_purchase_cents: fMinPurchase ? dollarsToCents(fMinPurchase) : 0,
      min_quantity: fMinQty ? parseInt(fMinQty) : 0,
      starts_at: fStartsAt ? new Date(fStartsAt).toISOString() : undefined,
      ends_at: fEndsAt ? new Date(fEndsAt).toISOString() : undefined,
      max_uses: fMaxUses ? parseInt(fMaxUses) : undefined,
      ...(isActiveOverride !== undefined ? { is_active: isActiveOverride } : {}),
    };
  }

  async function handleCreate(e: React.FormEvent) {
    e.preventDefault();
    setSaving(true);
    try {
      const created = await discountsApi.create(buildInput());
      setDiscounts((prev) => [created, ...prev]);
      setIsCreateOpen(false);
      resetForm();
      addToast({ title: `Discount "${created.name}" created`, variant: 'success' });
    } catch (e: any) {
      addToast({ title: e?.message || 'Create failed', variant: 'destructive' });
    } finally { setSaving(false); }
  }

  async function handleUpdate(e: React.FormEvent) {
    e.preventDefault();
    if (!selected) return;
    setSaving(true);
    try {
      const input: DiscountUpdateInput = { ...buildInput(fActive) };
      const updated = await discountsApi.update(selected.id, input);
      setDiscounts((prev) => prev.map((d) => (d.id === updated.id ? updated : d)));
      setIsEditOpen(false);
      addToast({ title: 'Discount updated', variant: 'success' });
    } catch (e: any) {
      addToast({ title: e?.message || 'Update failed', variant: 'destructive' });
    } finally { setSaving(false); }
  }

  async function handleDeactivate(d: Discount) {
    try {
      const updated = await discountsApi.deactivate(d.id);
      setDiscounts((prev) => prev.map((x) => (x.id === updated.id ? updated : x)));
      addToast({ title: `"${d.name}" deactivated`, variant: 'success' });
    } catch (e: any) {
      addToast({ title: e?.message || 'Deactivate failed', variant: 'destructive' });
    }
  }

  const activeCount = discounts.filter((d) => d.is_active).length;
  const totalRedemptions = discounts.reduce((a, d) => a + d.used_count, 0);

  const formBlock = (
    <div className="space-y-3 text-xs">
      <div className="space-y-1">
        <label className="font-medium text-muted-foreground">Name *</label>
        <Input id="disc-name" required value={fName} onChange={(e) => setFName(e.target.value)} placeholder="e.g. Early Bird 15%" />
      </div>
      <div className="grid grid-cols-2 gap-3">
        <div className="space-y-1">
          <label className="font-medium text-muted-foreground">Kind</label>
          <select id="disc-kind" value={fKind} onChange={(e) => setFKind(e.target.value as DiscountKind)}
            className="flex h-9 w-full rounded-md border border-input bg-background px-3 py-1 text-sm shadow-sm">
            <option value="PERCENT">Percent (%)</option>
            <option value="FIXED">Fixed Amount ($)</option>
          </select>
        </div>
        <div className="space-y-1">
          <label className="font-medium text-muted-foreground">{fKind === 'PERCENT' ? 'Discount % *' : 'Discount Amount ($) *'}</label>
          {fKind === 'PERCENT' ? (
            <Input id="disc-value-pct" required type="number" step="0.01" min="0.01" max="100" value={fValuePercent} onChange={(e) => setFValuePercent(e.target.value)} placeholder="15.00" />
          ) : (
            <Input id="disc-value-usd" required type="number" step="0.01" min="0.01" value={fValueDollar} onChange={(e) => setFValueDollar(e.target.value)} placeholder="500.00" />
          )}
        </div>
      </div>
      <div className="grid grid-cols-2 gap-3">
        <div className="space-y-1">
          <label className="font-medium text-muted-foreground">Scope</label>
          <select id="disc-scope" value={fScope} onChange={(e) => setFScope(e.target.value as DiscountScope)}
            className="flex h-9 w-full rounded-md border border-input bg-background px-3 py-1 text-sm shadow-sm">
            <option value="SUBSCRIPTION">Subscription (cart-level)</option>
            <option value="PRODUCT">Product-specific</option>
          </select>
        </div>
        {fScope === 'PRODUCT' && (
          <div className="space-y-1">
            <label className="font-medium text-muted-foreground">Product ID *</label>
            <Input id="disc-product-id" required={fScope === 'PRODUCT'} value={fProductId} onChange={(e) => setFProductId(e.target.value)} placeholder="UUID" />
          </div>
        )}
      </div>
      <div className="grid grid-cols-2 gap-3">
        <div className="space-y-1">
          <label className="font-medium text-muted-foreground">Min Purchase ($)</label>
          <Input id="disc-min-purchase" type="number" step="0.01" min="0" value={fMinPurchase} onChange={(e) => setFMinPurchase(e.target.value)} placeholder="0.00" />
        </div>
        <div className="space-y-1">
          <label className="font-medium text-muted-foreground">Min Qty</label>
          <Input id="disc-min-qty" type="number" min="0" value={fMinQty} onChange={(e) => setFMinQty(e.target.value)} placeholder="0" />
        </div>
      </div>
      <div className="grid grid-cols-2 gap-3">
        <div className="space-y-1">
          <label className="font-medium text-muted-foreground">Starts At</label>
          <Input id="disc-starts-at" type="date" value={fStartsAt} onChange={(e) => setFStartsAt(e.target.value)} />
        </div>
        <div className="space-y-1">
          <label className="font-medium text-muted-foreground">Ends At</label>
          <Input id="disc-ends-at" type="date" value={fEndsAt} onChange={(e) => setFEndsAt(e.target.value)} />
        </div>
      </div>
      <div className="space-y-1">
        <label className="font-medium text-muted-foreground">Max Uses (leave blank for unlimited)</label>
        <Input id="disc-max-uses" type="number" min="1" value={fMaxUses} onChange={(e) => setFMaxUses(e.target.value)} placeholder="Unlimited" />
      </div>
      {isEditOpen && (
        <label className="flex items-center gap-2 cursor-pointer select-none text-muted-foreground">
          <input type="checkbox" checked={fActive} onChange={(e) => setFActive(e.target.checked)} className="rounded" /> Active
        </label>
      )}
    </div>
  );

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold tracking-tight flex items-center gap-2">
            <Tag className="h-6 w-6 text-primary" />
            Discount Rules &amp; Coupons
          </h1>
          <p className="text-sm text-muted-foreground mt-0.5">
            Fixed or percentage discounts with usage quotas and redemption tracking.
          </p>
        </div>
        <div className="flex items-center gap-2">
          <Button size="sm" variant="outline" className="gap-1.5" onClick={load}><RefreshCw className="h-3.5 w-3.5" /> Refresh</Button>
          <Button size="sm" className="gap-1.5" onClick={() => { resetForm(); setIsCreateOpen(true); }}><Plus className="h-3.5 w-3.5" /> New Discount</Button>
        </div>
      </div>

      {/* Summary Cards */}
      <div className="grid grid-cols-3 gap-4">
        <Card className="border-0 bg-gradient-to-br from-violet-500/10 to-violet-600/5">
          <CardContent className="p-4">
            <div className="text-xs text-muted-foreground mb-1">Total Rules</div>
            <div className="text-2xl font-bold">{discounts.length}</div>
          </CardContent>
        </Card>
        <Card className="border-0 bg-gradient-to-br from-emerald-500/10 to-emerald-600/5">
          <CardContent className="p-4">
            <div className="text-xs text-muted-foreground mb-1">Active</div>
            <div className="text-2xl font-bold text-emerald-500">{activeCount}</div>
          </CardContent>
        </Card>
        <Card className="border-0 bg-gradient-to-br from-orange-500/10 to-orange-600/5">
          <CardContent className="p-4">
            <div className="text-xs text-muted-foreground mb-1">Total Redemptions</div>
            <div className="text-2xl font-bold text-orange-500">{totalRedemptions}</div>
          </CardContent>
        </Card>
      </div>

      {/* Filters */}
      <Card>
        <CardContent className="p-4">
          <div className="flex items-center gap-3">
            <div className="relative flex-1">
              <Search className="absolute left-3 top-1/2 -translate-y-1/2 h-3.5 w-3.5 text-muted-foreground" />
              <Input id="disc-search" className="pl-9 h-8 text-xs" placeholder="Search by name…" value={search} onChange={(e) => setSearch(e.target.value)} />
            </div>
            <label className="flex items-center gap-2 text-xs text-muted-foreground cursor-pointer select-none">
              <input type="checkbox" checked={activeOnly} onChange={(e) => setActiveOnly(e.target.checked)} className="rounded" />
              Active only
            </label>
          </div>
        </CardContent>
      </Card>

      {/* Table */}
      <Card>
        <CardContent className="p-0">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead className="text-xs">Name</TableHead>
                <TableHead className="text-xs">Kind</TableHead>
                <TableHead className="text-xs">Value</TableHead>
                <TableHead className="text-xs">Scope</TableHead>
                <TableHead className="text-xs">Period</TableHead>
                <TableHead className="text-xs">Usage</TableHead>
                <TableHead className="text-xs">Status</TableHead>
                <TableHead className="text-xs w-24" />
              </TableRow>
            </TableHeader>
            <TableBody>
              {loading ? (
                <TableRow><TableCell colSpan={8} className="text-center py-12 text-xs text-muted-foreground">Loading…</TableCell></TableRow>
              ) : discounts.length === 0 ? (
                <TableRow><TableCell colSpan={8} className="text-center py-12 text-xs text-muted-foreground">No discounts found.</TableCell></TableRow>
              ) : (
                discounts.map((d) => {
                  const expired = isExpired(d);
                  const maxed = isMaxedOut(d);
                  const effective = d.is_active && !expired && !maxed;
                  return (
                    <TableRow key={d.id} className={`text-xs ${!effective ? 'opacity-60' : ''}`}>
                      <TableCell className="font-medium">
                        {d.name}
                        {expired && <span className="ml-1.5 text-[10px] text-rose-400">(expired)</span>}
                        {maxed && <span className="ml-1.5 text-[10px] text-amber-400">(maxed)</span>}
                      </TableCell>
                      <TableCell>
                        <Badge variant="outline" className="text-[10px] gap-1">
                          {d.kind === 'PERCENT' ? <Percent className="h-2.5 w-2.5" /> : <DollarSign className="h-2.5 w-2.5" />}
                          {d.kind}
                        </Badge>
                      </TableCell>
                      <TableCell className="font-mono font-semibold text-primary">{discountValueLabel(d)}</TableCell>
                      <TableCell>
                        <Badge variant={d.scope === 'PRODUCT' ? 'secondary' : 'outline'} className="text-[10px]">{d.scope}</Badge>
                      </TableCell>
                      <TableCell className="text-muted-foreground">
                        {d.starts_at || d.ends_at
                          ? `${d.starts_at ? d.starts_at.slice(0, 10) : '∞'} → ${d.ends_at ? d.ends_at.slice(0, 10) : '∞'}`
                          : 'Always'}
                      </TableCell>
                      <TableCell>
                        <div className="flex items-center gap-1.5">
                          <span className="font-mono">{d.used_count}</span>
                          {d.max_uses !== null && (
                            <>
                              <span className="text-muted-foreground">/ {d.max_uses}</span>
                              <div className="h-1 w-12 bg-muted rounded-full overflow-hidden">
                                <div className="h-full bg-primary rounded-full" style={{ width: `${Math.min(100, (d.used_count / d.max_uses) * 100)}%` }} />
                              </div>
                            </>
                          )}
                        </div>
                      </TableCell>
                      <TableCell>
                        <Badge variant={effective ? 'success' : 'secondary'} className="gap-1 text-[10px]">
                          {effective ? <CheckCircle2 className="h-3 w-3" /> : <XCircle className="h-3 w-3" />}
                          {effective ? 'Active' : 'Inactive'}
                        </Badge>
                      </TableCell>
                      <TableCell className="flex gap-1">
                        <Button variant="ghost" size="sm" className="h-6 px-2 text-[10px]" onClick={() => openEdit(d)}>Edit</Button>
                        {d.is_active && (
                          <Button variant="ghost" size="sm" className="h-6 px-2 text-[10px] text-rose-500 hover:text-rose-600" onClick={() => handleDeactivate(d)}>
                            Deactivate
                          </Button>
                        )}
                      </TableCell>
                    </TableRow>
                  );
                })
              )}
            </TableBody>
          </Table>
        </CardContent>
      </Card>

      {/* Create / Edit Dialog */}
      <Dialog
        open={isCreateOpen || isEditOpen}
        onOpenChange={(open) => { if (!open) { setIsCreateOpen(false); setIsEditOpen(false); } }}
      >
        <form onSubmit={isCreateOpen ? handleCreate : handleUpdate} className="space-y-4">
          <DialogHeader>
            <DialogTitle>{isCreateOpen ? 'New Discount Rule' : 'Edit Discount Rule'}</DialogTitle>
            <DialogDescription>Configure discount rules, scope, validity window, and usage caps.</DialogDescription>
          </DialogHeader>
          {formBlock}
          <DialogFooter>
            <Button type="button" variant="outline" onClick={() => { setIsCreateOpen(false); setIsEditOpen(false); }}>Cancel</Button>
            <Button type="submit" disabled={saving || !fName.trim()}>
              {saving ? 'Saving…' : isCreateOpen ? 'Create Discount' : 'Save Changes'}
            </Button>
          </DialogFooter>
        </form>
      </Dialog>
    </div>
  );
}
