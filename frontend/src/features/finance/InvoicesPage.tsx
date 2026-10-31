import * as React from 'react';
import {
  Receipt, Plus, Search, Eye, CheckCircle2, XCircle, Send,
  Clock, AlertTriangle, CreditCard, Filter, ChevronDown,
  Download, RefreshCw, Calendar, Building2,
} from 'lucide-react';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';
import { Dialog, DialogHeader, DialogTitle, DialogDescription, DialogFooter } from '@/components/ui/dialog';
import { useToast } from '@/components/ui/toast';
import { formatCents, formatBasisPoints } from '@/lib/money';
import { formatDate } from '@/lib/date';
import { invoicesApi, paymentsApi, Invoice, Payment, PaymentMethod, PaymentRecordInput } from '@/api/finance';

const STATUS_VARIANTS = {
  DRAFT: 'secondary',
  CONFIRMED: 'warning',
  PAID: 'success',
  CANCELLED: 'destructive',
} as const;

const STATUS_ICONS: Record<string, React.ReactNode> = {
  DRAFT: <Clock className="h-3 w-3" />,
  CONFIRMED: <AlertTriangle className="h-3 w-3" />,
  PAID: <CheckCircle2 className="h-3 w-3" />,
  CANCELLED: <XCircle className="h-3 w-3" />,
};

function isOverdue(inv: Invoice): boolean {
  return inv.status === 'CONFIRMED' && inv.balance_cents > 0 && new Date(inv.due_date) < new Date();
}

export function InvoicesPage() {
  const { addToast } = useToast();
  const [invoices, setInvoices] = React.useState<Invoice[]>([]);
  const [loading, setLoading] = React.useState(true);
  const [search, setSearch] = React.useState('');
  const [statusFilter, setStatusFilter] = React.useState('');
  const [overdueFilter, setOverdueFilter] = React.useState(false);

  // Detail / Payment modals
  const [selectedInvoice, setSelectedInvoice] = React.useState<Invoice | null>(null);
  const [isDetailOpen, setIsDetailOpen] = React.useState(false);
  const [isPaymentOpen, setIsPaymentOpen] = React.useState(false);
  const [payments, setPayments] = React.useState<Payment[]>([]);
  const [loadingPayments, setLoadingPayments] = React.useState(false);

  // Payment form
  const [payMethod, setPayMethod] = React.useState<PaymentMethod>('BANK_TRANSFER');
  const [payAmount, setPayAmount] = React.useState('');
  const [payNote, setPayNote] = React.useState('');
  const [saving, setSaving] = React.useState(false);

  const load = React.useCallback(async () => {
    setLoading(true);
    try {
      const res = await invoicesApi.list({
        search: search || undefined,
        status: statusFilter || undefined,
        overdue: overdueFilter || undefined,
      });
      setInvoices(res.data ?? []);
    } catch {
      addToast({ title: 'Failed to load invoices', variant: 'destructive' });
    } finally {
      setLoading(false);
    }
  }, [search, statusFilter, overdueFilter]);

  React.useEffect(() => { load(); }, [load]);

  async function openDetail(inv: Invoice) {
    setSelectedInvoice(inv);
    setIsDetailOpen(true);
    setLoadingPayments(true);
    try {
      const res = await paymentsApi.list({ invoice_id: inv.id });
      setPayments(res.data ?? []);
    } catch { setPayments([]); }
    finally { setLoadingPayments(false); }
  }

  async function handleTransition(inv: Invoice, action: 'confirm' | 'cancel' | 'send') {
    setSaving(true);
    try {
      let updated: Invoice;
      if (action === 'confirm') updated = await invoicesApi.confirm(inv.id);
      else if (action === 'cancel') updated = await invoicesApi.cancel(inv.id);
      else updated = await invoicesApi.send(inv.id);
      setInvoices((prev) => prev.map((i) => (i.id === updated.id ? updated : i)));
      setSelectedInvoice(updated);
      addToast({ title: `Invoice ${action}ed successfully`, variant: 'success' });
    } catch (e: any) {
      addToast({ title: e?.message || 'Action failed', variant: 'destructive' });
    } finally {
      setSaving(false);
    }
  }

  async function handleRecordPayment(e: React.FormEvent) {
    e.preventDefault();
    if (!selectedInvoice) return;
    const amountCents = Math.round(parseFloat(payAmount) * 100);
    if (!amountCents || amountCents <= 0) {
      addToast({ title: 'Enter a valid positive amount', variant: 'destructive' });
      return;
    }
    setSaving(true);
    try {
      const input: PaymentRecordInput = {
        invoice_id: selectedInvoice.id,
        method: payMethod,
        amount_cents: amountCents,
        note: payNote || undefined,
      };
      const { invoice: updated } = await paymentsApi.record(input);
      setInvoices((prev) => prev.map((i) => (i.id === updated.id ? updated : i)));
      setSelectedInvoice(updated);
      const res = await paymentsApi.list({ invoice_id: selectedInvoice.id });
      setPayments(res.data ?? []);
      setIsPaymentOpen(false);
      setPayAmount('');
      setPayNote('');
      addToast({ title: 'Payment recorded', variant: 'success' });
    } catch (e: any) {
      addToast({ title: e?.message || 'Payment failed', variant: 'destructive' });
    } finally {
      setSaving(false);
    }
  }

  const totalOutstanding = invoices
    .filter((i) => i.status === 'CONFIRMED')
    .reduce((a, i) => a + i.balance_cents, 0);

  const overdueCount = invoices.filter(isOverdue).length;

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold tracking-tight flex items-center gap-2">
            <Receipt className="h-6 w-6 text-primary" />
            Invoices &amp; Billing
          </h1>
          <p className="text-sm text-muted-foreground mt-0.5">
            Manage customer invoices, confirm, send, and record payments.
          </p>
        </div>
        <Button size="sm" className="gap-1.5" onClick={load}>
          <RefreshCw className="h-3.5 w-3.5" /> Refresh
        </Button>
      </div>

      {/* Summary Cards */}
      <div className="grid grid-cols-3 gap-4">
        <Card className="border-0 bg-gradient-to-br from-blue-500/10 to-blue-600/5">
          <CardContent className="p-4">
            <div className="text-xs text-muted-foreground mb-1">Total Invoices</div>
            <div className="text-2xl font-bold">{invoices.length}</div>
          </CardContent>
        </Card>
        <Card className="border-0 bg-gradient-to-br from-amber-500/10 to-amber-600/5">
          <CardContent className="p-4">
            <div className="text-xs text-muted-foreground mb-1">Outstanding</div>
            <div className="text-2xl font-bold text-amber-500">{formatCents(totalOutstanding)}</div>
          </CardContent>
        </Card>
        <Card className="border-0 bg-gradient-to-br from-rose-500/10 to-rose-600/5">
          <CardContent className="p-4">
            <div className="text-xs text-muted-foreground mb-1">Overdue</div>
            <div className="text-2xl font-bold text-rose-500">{overdueCount} invoices</div>
          </CardContent>
        </Card>
      </div>

      {/* Filters */}
      <Card>
        <CardContent className="p-4">
          <div className="flex flex-wrap items-center gap-3">
            <div className="relative flex-1 min-w-48">
              <Search className="absolute left-3 top-1/2 -translate-y-1/2 h-3.5 w-3.5 text-muted-foreground" />
              <Input
                id="invoice-search"
                className="pl-9 h-8 text-xs"
                placeholder="Search by number or customer…"
                value={search}
                onChange={(e) => setSearch(e.target.value)}
              />
            </div>
            <select
              id="invoice-status-filter"
              value={statusFilter}
              onChange={(e) => setStatusFilter(e.target.value)}
              className="h-8 text-xs rounded-md border border-input bg-background px-3 min-w-32"
            >
              <option value="">All Statuses</option>
              <option value="DRAFT">Draft</option>
              <option value="CONFIRMED">Confirmed</option>
              <option value="PAID">Paid</option>
              <option value="CANCELLED">Cancelled</option>
            </select>
            <label className="flex items-center gap-2 text-xs text-muted-foreground cursor-pointer select-none">
              <input
                type="checkbox"
                checked={overdueFilter}
                onChange={(e) => setOverdueFilter(e.target.checked)}
                className="rounded"
              />
              <AlertTriangle className="h-3 w-3 text-rose-400" /> Overdue only
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
                <TableHead className="text-xs">Number</TableHead>
                <TableHead className="text-xs">Customer</TableHead>
                <TableHead className="text-xs">Status</TableHead>
                <TableHead className="text-xs">Issue Date</TableHead>
                <TableHead className="text-xs">Due Date</TableHead>
                <TableHead className="text-xs text-right">Total</TableHead>
                <TableHead className="text-xs text-right">Balance</TableHead>
                <TableHead className="text-xs w-12" />
              </TableRow>
            </TableHeader>
            <TableBody>
              {loading ? (
                <TableRow>
                  <TableCell colSpan={8} className="text-center py-12 text-xs text-muted-foreground">
                    Loading invoices…
                  </TableCell>
                </TableRow>
              ) : invoices.length === 0 ? (
                <TableRow>
                  <TableCell colSpan={8} className="text-center py-12 text-xs text-muted-foreground">
                    No invoices found.
                  </TableCell>
                </TableRow>
              ) : (
                invoices.map((inv) => (
                  <TableRow
                    key={inv.id}
                    className={`cursor-pointer hover:bg-muted/40 text-xs ${isOverdue(inv) ? 'bg-rose-500/5' : ''}`}
                    onClick={() => openDetail(inv)}
                  >
                    <TableCell className="font-mono font-medium">
                      {inv.number}
                      {isOverdue(inv) && (
                        <span className="ml-1.5 inline-flex items-center gap-0.5 text-rose-500 text-[10px]">
                          <AlertTriangle className="h-2.5 w-2.5" /> OVERDUE
                        </span>
                      )}
                    </TableCell>
                    <TableCell className="flex items-center gap-1.5">
                      <Building2 className="h-3 w-3 text-muted-foreground shrink-0" />
                      {inv.customer_name ?? inv.customer_id.slice(0, 8) + '…'}
                    </TableCell>
                    <TableCell>
                      <Badge variant={STATUS_VARIANTS[inv.status] as any} className="gap-1 text-[10px]">
                        {STATUS_ICONS[inv.status]} {inv.status}
                      </Badge>
                    </TableCell>
                    <TableCell className="text-muted-foreground">{formatDate(inv.issue_date)}</TableCell>
                    <TableCell className={`text-muted-foreground ${isOverdue(inv) ? 'text-rose-500 font-medium' : ''}`}>
                      {formatDate(inv.due_date)}
                    </TableCell>
                    <TableCell className="text-right font-mono">{formatCents(inv.total_cents)}</TableCell>
                    <TableCell className={`text-right font-mono font-medium ${inv.balance_cents > 0 ? 'text-amber-500' : 'text-emerald-500'}`}>
                      {formatCents(inv.balance_cents)}
                    </TableCell>
                    <TableCell>
                      <Button variant="ghost" size="sm" className="h-6 w-6 p-0" onClick={(e) => { e.stopPropagation(); openDetail(inv); }}>
                        <Eye className="h-3 w-3" />
                      </Button>
                    </TableCell>
                  </TableRow>
                ))
              )}
            </TableBody>
          </Table>
        </CardContent>
      </Card>

      {/* Invoice Detail Dialog */}
      <Dialog open={isDetailOpen} onOpenChange={setIsDetailOpen}>
        {selectedInvoice && (
          <div className="space-y-5 max-h-[80vh] overflow-y-auto pr-1">
            <DialogHeader>
              <div className="flex items-center justify-between">
                <DialogTitle className="font-mono text-base">{selectedInvoice.number}</DialogTitle>
                <Badge variant={STATUS_VARIANTS[selectedInvoice.status] as any} className="gap-1">
                  {STATUS_ICONS[selectedInvoice.status]} {selectedInvoice.status}
                </Badge>
              </div>
              <DialogDescription>
                {selectedInvoice.customer_name} · Due {formatDate(selectedInvoice.due_date)}
              </DialogDescription>
            </DialogHeader>

            {/* Info grid */}
            <div className="grid grid-cols-2 gap-3 text-xs">
              {[
                { label: 'Issue Date', value: formatDate(selectedInvoice.issue_date) },
                { label: 'Due Date', value: formatDate(selectedInvoice.due_date) },
                { label: 'Period', value: selectedInvoice.period_start ? `${formatDate(selectedInvoice.period_start)} → ${formatDate(selectedInvoice.period_end ?? '')}` : '—' },
                { label: 'Subscription', value: selectedInvoice.subscription_id ? selectedInvoice.subscription_id.slice(0, 12) + '…' : '—' },
              ].map(({ label, value }) => (
                <div key={label} className="p-3 rounded-lg bg-muted/40 border space-y-0.5">
                  <div className="text-muted-foreground">{label}</div>
                  <div className="font-medium font-mono">{value}</div>
                </div>
              ))}
            </div>

            {/* Financials */}
            <div className="rounded-lg border bg-muted/20 divide-y text-xs">
              {[
                { label: 'Subtotal', value: formatCents(selectedInvoice.subtotal_cents) },
                { label: 'Discount', value: `−${formatCents(selectedInvoice.discount_cents)}`, className: 'text-emerald-500' },
                { label: 'Tax', value: formatCents(selectedInvoice.tax_cents) },
                { label: 'Total', value: formatCents(selectedInvoice.total_cents), bold: true },
                { label: 'Balance Due', value: formatCents(selectedInvoice.balance_cents), bold: true, className: selectedInvoice.balance_cents > 0 ? 'text-amber-500' : 'text-emerald-500' },
              ].map(({ label, value, bold, className }) => (
                <div key={label} className="flex items-center justify-between px-4 py-2">
                  <span className={`${bold ? 'font-semibold' : 'text-muted-foreground'}`}>{label}</span>
                  <span className={`font-mono ${bold ? 'font-bold' : ''} ${className ?? ''}`}>{value}</span>
                </div>
              ))}
            </div>

            {/* Notes */}
            {selectedInvoice.notes && (
              <div className="p-3 rounded-lg bg-muted/40 border text-xs text-muted-foreground italic">
                {selectedInvoice.notes}
              </div>
            )}

            {/* Payments list */}
            <div className="space-y-2">
              <div className="text-xs font-semibold text-muted-foreground uppercase tracking-wider">Payments</div>
              {loadingPayments ? (
                <div className="text-xs text-muted-foreground">Loading…</div>
              ) : payments.length === 0 ? (
                <div className="text-xs text-muted-foreground italic">No payments recorded.</div>
              ) : (
                payments.map((p) => (
                  <div key={p.id} className="flex items-center justify-between text-xs p-2.5 rounded-lg bg-muted/30 border">
                    <div className="flex items-center gap-2">
                      <CreditCard className="h-3 w-3 text-muted-foreground" />
                      <span className="font-medium">{p.method.replace('_', ' ')}</span>
                      {p.note && <span className="text-muted-foreground">· {p.note}</span>}
                    </div>
                    <div className="font-mono font-semibold text-emerald-500">{formatCents(p.amount_cents)}</div>
                  </div>
                ))
              )}
            </div>

            {/* Actions */}
            <DialogFooter className="gap-2 pt-2">
              {selectedInvoice.status === 'DRAFT' && (
                <Button size="sm" className="gap-1.5" onClick={() => handleTransition(selectedInvoice, 'confirm')} disabled={saving}>
                  <CheckCircle2 className="h-3.5 w-3.5" /> Confirm
                </Button>
              )}
              {selectedInvoice.status === 'CONFIRMED' && (
                <>
                  <Button size="sm" variant="outline" className="gap-1.5" onClick={() => handleTransition(selectedInvoice, 'send')} disabled={saving}>
                    <Send className="h-3.5 w-3.5" /> Mark Sent
                  </Button>
                  {selectedInvoice.balance_cents > 0 && (
                    <Button size="sm" className="gap-1.5 bg-emerald-600 hover:bg-emerald-700 text-white"
                      onClick={() => { setIsPaymentOpen(true); setPayAmount((selectedInvoice.balance_cents / 100).toFixed(2)); }}
                      disabled={saving}>
                      <CreditCard className="h-3.5 w-3.5" /> Record Payment
                    </Button>
                  )}
                </>
              )}
              {(selectedInvoice.status === 'DRAFT' || selectedInvoice.status === 'CONFIRMED') && (
                <Button size="sm" variant="destructive" className="gap-1.5" onClick={() => handleTransition(selectedInvoice, 'cancel')} disabled={saving}>
                  <XCircle className="h-3.5 w-3.5" /> Cancel
                </Button>
              )}
              <Button size="sm" variant="outline" onClick={() => setIsDetailOpen(false)}>Close</Button>
            </DialogFooter>
          </div>
        )}
      </Dialog>

      {/* Record Payment Dialog */}
      <Dialog open={isPaymentOpen} onOpenChange={setIsPaymentOpen}>
        {selectedInvoice && (
          <form onSubmit={handleRecordPayment} className="space-y-4">
            <DialogHeader>
              <DialogTitle>Record Payment</DialogTitle>
              <DialogDescription>
                {selectedInvoice.number} · Balance: <strong>{formatCents(selectedInvoice.balance_cents)}</strong>
              </DialogDescription>
            </DialogHeader>
            <div className="space-y-3 text-xs">
              <div className="space-y-1">
                <label className="font-medium text-muted-foreground">Payment Method</label>
                <select
                  id="pay-method"
                  value={payMethod}
                  onChange={(e) => setPayMethod(e.target.value as PaymentMethod)}
                  className="flex h-9 w-full rounded-md border border-input bg-background px-3 py-1 text-sm shadow-sm"
                >
                  {(['CASH', 'BANK_TRANSFER', 'CARD', 'MOBILE_MONEY', 'OTHER'] as PaymentMethod[]).map((m) => (
                    <option key={m} value={m}>{m.replace('_', ' ')}</option>
                  ))}
                </select>
              </div>
              <div className="space-y-1">
                <label className="font-medium text-muted-foreground">Amount (USD)</label>
                <Input
                  id="pay-amount"
                  type="number"
                  step="0.01"
                  min="0.01"
                  required
                  value={payAmount}
                  onChange={(e) => setPayAmount(e.target.value)}
                  placeholder="0.00"
                />
              </div>
              <div className="space-y-1">
                <label className="font-medium text-muted-foreground">Note (optional)</label>
                <Input
                  id="pay-note"
                  value={payNote}
                  onChange={(e) => setPayNote(e.target.value)}
                  placeholder="Wire ref #TXN-12345"
                />
              </div>
            </div>
            <DialogFooter>
              <Button type="button" variant="outline" onClick={() => setIsPaymentOpen(false)}>Cancel</Button>
              <Button type="submit" disabled={saving} className="bg-emerald-600 hover:bg-emerald-700 text-white">
                {saving ? 'Recording…' : 'Record Payment'}
              </Button>
            </DialogFooter>
          </form>
        )}
      </Dialog>
    </div>
  );
}
