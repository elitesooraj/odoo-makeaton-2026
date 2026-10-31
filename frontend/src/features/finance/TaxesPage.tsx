import * as React from 'react';
import {
  Percent, Plus, Search, ToggleLeft, ToggleRight, RefreshCw,
  CheckCircle2, XCircle, Info, Tag, Globe,
} from 'lucide-react';
import { Card, CardContent } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';
import { Dialog, DialogHeader, DialogTitle, DialogDescription, DialogFooter } from '@/components/ui/dialog';
import { useToast } from '@/components/ui/toast';
import { formatBasisPoints } from '@/lib/money';
import { taxesApi, Tax, TaxCreateInput, TaxUpdateInput } from '@/api/finance';

export function TaxesPage() {
  const { addToast } = useToast();
  const [taxes, setTaxes] = React.useState<Tax[]>([]);
  const [loading, setLoading] = React.useState(true);
  const [search, setSearch] = React.useState('');
  const [activeOnly, setActiveOnly] = React.useState(false);

  const [isCreateOpen, setIsCreateOpen] = React.useState(false);
  const [isEditOpen, setIsEditOpen] = React.useState(false);
  const [selected, setSelected] = React.useState<Tax | null>(null);
  const [saving, setSaving] = React.useState(false);

  // Form fields
  const [fName, setFName] = React.useState('');
  const [fRateBP, setFRateBP] = React.useState('');
  const [fInclusive, setFInclusive] = React.useState(false);
  const [fCountry, setFCountry] = React.useState('');
  const [fActive, setFActive] = React.useState(true);

  function resetForm() { setFName(''); setFRateBP(''); setFInclusive(false); setFCountry(''); setFActive(true); }

  function openEdit(t: Tax) {
    setSelected(t);
    setFName(t.name);
    setFRateBP(String(t.rate_bp));
    setFInclusive(t.is_inclusive);
    setFCountry(t.country_code);
    setFActive(t.is_active);
    setIsEditOpen(true);
  }

  const load = React.useCallback(async () => {
    setLoading(true);
    try {
      const res = await taxesApi.list({ search: search || undefined, active: activeOnly || undefined });
      setTaxes(res.data ?? []);
    } catch {
      addToast({ title: 'Failed to load tax rates', variant: 'destructive' });
    } finally {
      setLoading(false);
    }
  }, [search, activeOnly]);

  React.useEffect(() => { load(); }, [load]);

  async function handleCreate(e: React.FormEvent) {
    e.preventDefault();
    setSaving(true);
    try {
      const input: TaxCreateInput = {
        name: fName.trim(),
        rate_bp: parseInt(fRateBP, 10),
        is_inclusive: fInclusive,
        country_code: fCountry.trim().toUpperCase(),
      };
      const created = await taxesApi.create(input);
      setTaxes((prev) => [created, ...prev]);
      setIsCreateOpen(false);
      resetForm();
      addToast({ title: `Tax rule "${created.name}" created`, variant: 'success' });
    } catch (e: any) {
      addToast({ title: e?.message || 'Create failed', variant: 'destructive' });
    } finally {
      setSaving(false);
    }
  }

  async function handleUpdate(e: React.FormEvent) {
    e.preventDefault();
    if (!selected) return;
    setSaving(true);
    try {
      const input: TaxUpdateInput = {
        name: fName.trim(),
        rate_bp: parseInt(fRateBP, 10),
        is_inclusive: fInclusive,
        country_code: fCountry.trim().toUpperCase(),
        is_active: fActive,
      };
      const updated = await taxesApi.update(selected.id, input);
      setTaxes((prev) => prev.map((t) => (t.id === updated.id ? updated : t)));
      setIsEditOpen(false);
      addToast({ title: 'Tax rule updated', variant: 'success' });
    } catch (e: any) {
      addToast({ title: e?.message || 'Update failed', variant: 'destructive' });
    } finally {
      setSaving(false);
    }
  }

  async function handleDeactivate(t: Tax) {
    try {
      const updated = await taxesApi.deactivate(t.id);
      setTaxes((prev) => prev.map((x) => (x.id === updated.id ? updated : x)));
      addToast({ title: `"${t.name}" deactivated`, variant: 'success' });
    } catch (e: any) {
      addToast({ title: e?.message || 'Deactivate failed', variant: 'destructive' });
    }
  }

  const activeTaxes = taxes.filter((t) => t.is_active).length;

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold tracking-tight flex items-center gap-2">
            <Percent className="h-6 w-6 text-primary" />
            Tax Rules &amp; Configuration
          </h1>
          <p className="text-sm text-muted-foreground mt-0.5">
            Country / regional rates in basis points. 100 bp = 1%.
          </p>
        </div>
        <div className="flex items-center gap-2">
          <Button size="sm" variant="outline" className="gap-1.5" onClick={load}>
            <RefreshCw className="h-3.5 w-3.5" /> Refresh
          </Button>
          <Button size="sm" className="gap-1.5" onClick={() => { resetForm(); setIsCreateOpen(true); }}>
            <Plus className="h-3.5 w-3.5" /> New Tax Rule
          </Button>
        </div>
      </div>

      {/* Summary Cards */}
      <div className="grid grid-cols-3 gap-4">
        <Card className="border-0 bg-gradient-to-br from-violet-500/10 to-violet-600/5">
          <CardContent className="p-4">
            <div className="text-xs text-muted-foreground mb-1">Total Rules</div>
            <div className="text-2xl font-bold">{taxes.length}</div>
          </CardContent>
        </Card>
        <Card className="border-0 bg-gradient-to-br from-emerald-500/10 to-emerald-600/5">
          <CardContent className="p-4">
            <div className="text-xs text-muted-foreground mb-1">Active Rules</div>
            <div className="text-2xl font-bold text-emerald-500">{activeTaxes}</div>
          </CardContent>
        </Card>
        <Card className="border-0 bg-gradient-to-br from-blue-500/10 to-blue-600/5">
          <CardContent className="p-4">
            <div className="text-xs text-muted-foreground mb-1">Countries Covered</div>
            <div className="text-2xl font-bold">{new Set(taxes.filter((t) => t.country_code).map((t) => t.country_code)).size}</div>
          </CardContent>
        </Card>
      </div>

      {/* Filters */}
      <Card>
        <CardContent className="p-4">
          <div className="flex items-center gap-3">
            <div className="relative flex-1">
              <Search className="absolute left-3 top-1/2 -translate-y-1/2 h-3.5 w-3.5 text-muted-foreground" />
              <Input
                id="tax-search"
                className="pl-9 h-8 text-xs"
                placeholder="Search by name or country code…"
                value={search}
                onChange={(e) => setSearch(e.target.value)}
              />
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
                <TableHead className="text-xs">Rate (bp)</TableHead>
                <TableHead className="text-xs">Rate %</TableHead>
                <TableHead className="text-xs">Country</TableHead>
                <TableHead className="text-xs">Type</TableHead>
                <TableHead className="text-xs">Status</TableHead>
                <TableHead className="text-xs w-20" />
              </TableRow>
            </TableHeader>
            <TableBody>
              {loading ? (
                <TableRow><TableCell colSpan={7} className="text-center py-12 text-xs text-muted-foreground">Loading…</TableCell></TableRow>
              ) : taxes.length === 0 ? (
                <TableRow><TableCell colSpan={7} className="text-center py-12 text-xs text-muted-foreground">No tax rules found.</TableCell></TableRow>
              ) : (
                taxes.map((t) => (
                  <TableRow key={t.id} className={`text-xs ${!t.is_active ? 'opacity-50' : ''}`}>
                    <TableCell className="font-medium">{t.name}</TableCell>
                    <TableCell className="font-mono">{t.rate_bp.toLocaleString()}</TableCell>
                    <TableCell className="font-mono font-semibold text-primary">{formatBasisPoints(t.rate_bp)}</TableCell>
                    <TableCell>
                      {t.country_code ? (
                        <span className="flex items-center gap-1">
                          <Globe className="h-3 w-3 text-muted-foreground" /> {t.country_code}
                        </span>
                      ) : '—'}
                    </TableCell>
                    <TableCell>
                      <Badge variant={t.is_inclusive ? 'secondary' : 'outline'} className="text-[10px]">
                        {t.is_inclusive ? 'Inclusive' : 'Exclusive'}
                      </Badge>
                    </TableCell>
                    <TableCell>
                      <Badge variant={t.is_active ? 'success' : 'secondary'} className="gap-1 text-[10px]">
                        {t.is_active ? <CheckCircle2 className="h-3 w-3" /> : <XCircle className="h-3 w-3" />}
                        {t.is_active ? 'Active' : 'Inactive'}
                      </Badge>
                    </TableCell>
                    <TableCell className="flex gap-1">
                      <Button variant="ghost" size="sm" className="h-6 px-2 text-[10px]" onClick={() => openEdit(t)}>Edit</Button>
                      {t.is_active && (
                        <Button variant="ghost" size="sm" className="h-6 px-2 text-[10px] text-rose-500 hover:text-rose-600" onClick={() => handleDeactivate(t)}>
                          Deactivate
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

      {/* Create / Edit Dialog */}
      <Dialog
        open={isCreateOpen || isEditOpen}
        onOpenChange={(open) => { if (!open) { setIsCreateOpen(false); setIsEditOpen(false); } }}
      >
        <form onSubmit={isCreateOpen ? handleCreate : handleUpdate} className="space-y-4">
          <DialogHeader>
            <DialogTitle>{isCreateOpen ? 'New Tax Rule' : 'Edit Tax Rule'}</DialogTitle>
            <DialogDescription>
              Rates are in basis points: 100 bp = 1%. Max: 100,000 bp (100%).
            </DialogDescription>
          </DialogHeader>

          <div className="space-y-3 text-xs">
            <div className="space-y-1">
              <label className="font-medium text-muted-foreground">Rule Name *</label>
              <Input id="tax-name" required value={fName} onChange={(e) => setFName(e.target.value)} placeholder="e.g. Standard GST (India)" />
            </div>
            <div className="grid grid-cols-2 gap-3">
              <div className="space-y-1">
                <label className="font-medium text-muted-foreground">Rate (basis points) *</label>
                <Input id="tax-rate-bp" required type="number" min="0" max="100000" value={fRateBP} onChange={(e) => setFRateBP(e.target.value)} placeholder="1800" />
                {fRateBP && !isNaN(parseInt(fRateBP)) && (
                  <div className="text-[10px] text-primary font-mono">= {formatBasisPoints(parseInt(fRateBP))}</div>
                )}
              </div>
              <div className="space-y-1">
                <label className="font-medium text-muted-foreground">Country Code (ISO 2)</label>
                <Input id="tax-country" maxLength={2} value={fCountry} onChange={(e) => setFCountry(e.target.value.toUpperCase())} placeholder="IN" />
              </div>
            </div>
            <div className="flex items-center gap-3">
              <label className="flex items-center gap-2 cursor-pointer select-none text-muted-foreground">
                <input type="checkbox" checked={fInclusive} onChange={(e) => setFInclusive(e.target.checked)} className="rounded" />
                Tax is <strong>inclusive</strong> (carved out of price, not added on top)
              </label>
            </div>
            {isEditOpen && (
              <div className="flex items-center gap-3">
                <label className="flex items-center gap-2 cursor-pointer select-none text-muted-foreground">
                  <input type="checkbox" checked={fActive} onChange={(e) => setFActive(e.target.checked)} className="rounded" />
                  Active
                </label>
              </div>
            )}
          </div>

          <DialogFooter>
            <Button type="button" variant="outline" onClick={() => { setIsCreateOpen(false); setIsEditOpen(false); }}>Cancel</Button>
            <Button type="submit" disabled={saving || !fName.trim() || !fRateBP}>
              {saving ? 'Saving…' : isCreateOpen ? 'Create Rule' : 'Save Changes'}
            </Button>
          </DialogFooter>
        </form>
      </Dialog>
    </div>
  );
}
