import * as React from 'react';
import {
  Receipt,
  DollarSign,
  BarChart3,
  Search,
  Plus,
  Send,
  CheckCircle,
  XCircle,
  CreditCard,
  AlertTriangle,
  Clock,
  ArrowUpRight,
  TrendingUp,
  Download,
  Calendar,
} from 'lucide-react';
import { invoicesApi } from '@/api/invoices';
import { paymentsApi } from '@/api/payments';
import { reportsApi } from '@/api/reports';
import { customersApi } from '@/api/customers';
import {
  Customer,
  Invoice,
  InvoiceStatus,
  Payment,
  PaymentMethod,
  ReportSummary,
  RevenuePoint,
} from '@/api/types';
import { formatCents, dollarsToCents } from '@/lib/money';
import { formatDate } from '@/lib/date';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Badge } from '@/components/ui/badge';
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from '@/components/ui/card';
import { Dialog, DialogHeader, DialogTitle, DialogDescription, DialogFooter } from '@/components/ui/dialog';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';
import { Tabs, TabsList, TabsTrigger, TabsContent } from '@/components/ui/tabs';
import { useToast } from '@/components/ui/toast';
import { PermissionGate } from '@/components/auth/PermissionGate';

export function FinancePage() {
  const { toast } = useToast();

  const [activeTab, setActiveTab] = React.useState('invoices');
  const [invoices, setInvoices] = React.useState<Invoice[]>([]);
  const [payments, setPayments] = React.useState<Payment[]>([]);
  const [customers, setCustomers] = React.useState<Customer[]>([]);
  const [reportSummary, setReportSummary] = React.useState<ReportSummary | null>(null);
  const [revenuePoints, setRevenuePoints] = React.useState<RevenuePoint[]>([]);
  const [overdueInvoices, setOverdueInvoices] = React.useState<Invoice[]>([]);
  const [search, setSearch] = React.useState('');
  const [loading, setLoading] = React.useState(false);

  // Selected Invoice Modal
  const [selectedInvoice, setSelectedInvoice] = React.useState<Invoice | null>(null);
  const [isInvoiceModalOpen, setIsInvoiceModalOpen] = React.useState(false);

  // Record Payment Modal
  const [isPaymentModalOpen, setIsPaymentModalOpen] = React.useState(false);
  const [payInvoiceId, setPayInvoiceId] = React.useState('');
  const [payAmountDollars, setPayAmountDollars] = React.useState('');
  const [payMethod, setPayMethod] = React.useState<PaymentMethod>('CARD');
  const [payNote, setPayNote] = React.useState('');
  const [recordingPayment, setRecordingPayment] = React.useState(false);

  // Create Invoice Modal
  const [isCreateInvoiceOpen, setIsCreateInvoiceOpen] = React.useState(false);
  const [newInvCustomerId, setNewInvCustomerId] = React.useState('');
  const [newInvDescription, setNewInvDescription] = React.useState('Implementation Setup Fee');
  const [newInvAmountDollars, setNewInvAmountDollars] = React.useState('500.00');
  const [newInvQty, setNewInvQty] = React.useState('1');
  const [newInvNotes, setNewInvNotes] = React.useState('');
  const [creatingInvoice, setCreatingInvoice] = React.useState(false);

  const fetchData = React.useCallback(async () => {
    setLoading(true);
    try {
      const [invRes, payRes, custRes, summary, rev, overdue] = await Promise.all([
        invoicesApi.list({ search }),
        paymentsApi.list(),
        customersApi.list({ per_page: 100 }),
        reportsApi.getSummary(),
        reportsApi.getRevenue(12),
        reportsApi.getOverdue(),
      ]);
      setInvoices(invRes.data || []);
      setPayments(payRes.data || []);
      setCustomers(custRes.data || []);
      setReportSummary(summary);
      setRevenuePoints(rev || []);
      setOverdueInvoices(overdue || []);

      if (custRes.data?.length && !newInvCustomerId) {
        setNewInvCustomerId(custRes.data[0].id);
      }
    } catch {
      toast({
        title: 'Error',
        description: 'Failed to load financial records',
        variant: 'destructive',
      });
    } finally {
      setLoading(false);
    }
  }, [search, newInvCustomerId, toast]);

  React.useEffect(() => {
    fetchData();
  }, [fetchData]);

  // Invoice Actions
  const handleConfirmInvoice = async (invoiceId: string) => {
    try {
      const updated = await invoicesApi.confirm(invoiceId);
      toast({
        title: 'Invoice Confirmed',
        description: `Invoice ${updated.number} confirmed and posted to customer balance.`,
        variant: 'success',
      });
      setSelectedInvoice(updated);
      fetchData();
    } catch (err: any) {
      toast({
        title: 'Confirmation Failed',
        description: err.message || 'Cannot confirm invoice',
        variant: 'destructive',
      });
    }
  };

  const handleCancelInvoice = async (invoiceId: string) => {
    try {
      const updated = await invoicesApi.cancel(invoiceId);
      toast({
        title: 'Invoice Cancelled',
        description: `Invoice ${updated.number} marked as cancelled.`,
        variant: 'success',
      });
      setSelectedInvoice(updated);
      fetchData();
    } catch (err: any) {
      toast({
        title: 'Cancellation Failed',
        description: err.message || 'Only unpaid invoices may be cancelled',
        variant: 'destructive',
      });
    }
  };

  const handleSendInvoice = async (invoiceId: string) => {
    try {
      await invoicesApi.send(invoiceId);
      toast({
        title: 'Invoice Sent',
        description: 'Customer notified with billing receipt & invoice PDF.',
        variant: 'success',
      });
      fetchData();
    } catch (err: any) {
      toast({
        title: 'Send Failed',
        description: err.message || 'Failed to dispatch invoice notification',
        variant: 'destructive',
      });
    }
  };

  const handleCreateInvoice = async (e: React.FormEvent) => {
    e.preventDefault();
    setCreatingInvoice(true);
    try {
      const cust = customers.find((c) => c.id === newInvCustomerId);
      const unitCents = dollarsToCents(newInvAmountDollars);
      const qty = parseInt(newInvQty, 10) || 1;

      await invoicesApi.create({
        customer_id: newInvCustomerId,
        customer_name: cust?.name,
        notes: newInvNotes || undefined,
        lines: [
          {
            description: newInvDescription,
            quantity: qty,
            unit_price_cents: unitCents,
          },
        ],
      });

      toast({
        title: 'Draft Invoice Created',
        description: 'New invoice draft prepared.',
        variant: 'success',
      });
      setIsCreateInvoiceOpen(false);
      setNewInvNotes('');
      fetchData();
    } catch (err: any) {
      toast({
        title: 'Invoice Creation Failed',
        description: err.message || 'Validation failed',
        variant: 'destructive',
      });
    } finally {
      setCreatingInvoice(false);
    }
  };

  // Payment Recording
  const handleOpenPaymentForInvoice = (inv: Invoice) => {
    setPayInvoiceId(inv.id);
    setPayAmountDollars((inv.balance_cents / 100).toFixed(2));
    setPayNote(`Settlement for ${inv.number}`);
    setIsPaymentModalOpen(true);
  };

  const handleRecordPayment = async (e: React.FormEvent) => {
    e.preventDefault();
    setRecordingPayment(true);
    try {
      const inv = invoices.find((i) => i.id === payInvoiceId);
      const amountCents = dollarsToCents(payAmountDollars);

      await paymentsApi.record({
        invoice_id: payInvoiceId,
        method: payMethod,
        amount_cents: amountCents,
        note: payNote,
      });

      toast({
        title: 'Payment Recorded',
        description: `Collected ${formatCents(amountCents)} against ${inv?.number || 'invoice'}.`,
        variant: 'success',
      });
      setIsPaymentModalOpen(false);
      fetchData();
    } catch (err: any) {
      toast({
        title: 'Payment Recording Failed',
        description: err.message || 'Only confirmed invoices accept payments',
        variant: 'destructive',
      });
    } finally {
      setRecordingPayment(false);
    }
  };

  const getStatusBadge = (status: InvoiceStatus) => {
    switch (status) {
      case 'DRAFT':
        return <Badge variant="secondary">DRAFT</Badge>;
      case 'CONFIRMED':
        return <Badge variant="warning">CONFIRMED (UNPAID)</Badge>;
      case 'PAID':
        return <Badge variant="success">PAID</Badge>;
      case 'CANCELLED':
        return <Badge variant="destructive">CANCELLED</Badge>;
      default:
        return <Badge>{status}</Badge>;
    }
  };

  return (
    <div className="space-y-6">
      <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-4">
        <div>
          <h1 className="text-2xl font-bold tracking-tight">Finance, Invoicing & Reporting</h1>
          <p className="text-sm text-muted-foreground">
            Track automated billing runs, allocate cash/card payments, resolve overdue balances, and monitor MRR.
          </p>
        </div>

        <div className="flex flex-wrap items-center gap-2">
          <PermissionGate permission="payments.write">
            <Button
              onClick={() => {
                if (invoices.filter((i) => i.status === 'CONFIRMED').length > 0) {
                  const firstUnpaid = invoices.find((i) => i.status === 'CONFIRMED');
                  if (firstUnpaid) handleOpenPaymentForInvoice(firstUnpaid);
                } else {
                  setPayInvoiceId('');
                  setPayAmountDollars('100.00');
                  setIsPaymentModalOpen(true);
                }
              }}
              variant="outline"
              size="sm"
              className="cursor-pointer"
            >
              <CreditCard className="h-4 w-4 mr-1.5" />
              Record Payment
            </Button>
          </PermissionGate>

          <PermissionGate permission="invoices.write">
            <Button
              onClick={() => setIsCreateInvoiceOpen(true)}
              size="sm"
              className="cursor-pointer"
            >
              <Plus className="h-4 w-4 mr-1.5" />
              New Invoice
            </Button>
          </PermissionGate>
        </div>
      </div>

      {/* KPI CARDS */}
      {reportSummary && (
        <div className="grid grid-cols-2 lg:grid-cols-4 gap-4">
          <Card className="border shadow-sm">
            <CardHeader className="pb-2">
              <CardDescription className="text-xs font-semibold uppercase tracking-wider text-muted-foreground">
                Monthly Recurring Revenue (MRR)
              </CardDescription>
              <CardTitle className="text-2xl font-bold font-mono text-emerald-400">
                {formatCents(reportSummary.mrr_cents)}
              </CardTitle>
            </CardHeader>
            <CardContent className="text-xs text-muted-foreground">
              {reportSummary.active_subscriptions} active &bull; {reportSummary.paused_subscriptions} paused
            </CardContent>
          </Card>

          <Card className="border shadow-sm">
            <CardHeader className="pb-2">
              <CardDescription className="text-xs font-semibold uppercase tracking-wider text-muted-foreground">
                Total Outstanding Balance
              </CardDescription>
              <CardTitle className="text-2xl font-bold font-mono text-foreground">
                {formatCents(reportSummary.outstanding_cents)}
              </CardTitle>
            </CardHeader>
            <CardContent className="text-xs text-muted-foreground">
              Awaiting settlement across confirmed invoices
            </CardContent>
          </Card>

          <Card className="border shadow-sm">
            <CardHeader className="pb-2">
              <CardDescription className="text-xs font-semibold uppercase tracking-wider text-muted-foreground">
                Overdue Debt
              </CardDescription>
              <CardTitle className="text-2xl font-bold font-mono text-red-400">
                {formatCents(reportSummary.overdue_cents)}
              </CardTitle>
            </CardHeader>
            <CardContent className="text-xs text-red-400/80 font-medium">
              {reportSummary.overdue_invoices} invoice(s) past due date
            </CardContent>
          </Card>

          <Card className="border shadow-sm">
            <CardHeader className="pb-2">
              <CardDescription className="text-xs font-semibold uppercase tracking-wider text-muted-foreground">
                Collected This Month
              </CardDescription>
              <CardTitle className="text-2xl font-bold font-mono text-blue-400">
                {formatCents(reportSummary.collected_this_month_cents)}
              </CardTitle>
            </CardHeader>
            <CardContent className="text-xs text-muted-foreground">
              Recorded cash, card & bank payments
            </CardContent>
          </Card>
        </div>
      )}

      <Tabs value={activeTab} onValueChange={setActiveTab}>
        <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-4">
          <TabsList>
            <TabsTrigger value="invoices" className="flex items-center gap-2">
              <Receipt className="h-4 w-4" />
              Invoices ({invoices.length})
            </TabsTrigger>
            <TabsTrigger value="payments" className="flex items-center gap-2">
              <DollarSign className="h-4 w-4" />
              Payments ({payments.length})
            </TabsTrigger>
            <TabsTrigger value="reports" className="flex items-center gap-2">
              <BarChart3 className="h-4 w-4" />
              Financial Reports
            </TabsTrigger>
          </TabsList>

          {activeTab === 'invoices' && (
            <div className="relative w-full sm:w-64">
              <Search className="absolute left-3 top-2.5 h-4 w-4 text-muted-foreground" />
              <Input
                placeholder="Search invoice number..."
                value={search}
                onChange={(e) => setSearch(e.target.value)}
                className="pl-9 h-9"
              />
            </div>
          )}
        </div>

        {/* INVOICES TAB */}
        <TabsContent value="invoices" className="mt-4">
          <Card>
            <CardContent className="p-0">
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>Invoice #</TableHead>
                    <TableHead>Customer</TableHead>
                    <TableHead>Issue / Due Date</TableHead>
                    <TableHead>Status</TableHead>
                    <TableHead className="text-right">Total Amount</TableHead>
                    <TableHead className="text-right">Balance Due</TableHead>
                    <TableHead className="text-right">Actions</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {loading && invoices.length === 0 ? (
                    <TableRow>
                      <TableCell colSpan={7} className="text-center py-8 text-muted-foreground">
                        Loading invoices...
                      </TableCell>
                    </TableRow>
                  ) : invoices.length === 0 ? (
                    <TableRow>
                      <TableCell colSpan={7} className="text-center py-8 text-muted-foreground">
                        No invoices found. Trigger a billing run or create an invoice.
                      </TableCell>
                    </TableRow>
                  ) : (
                    invoices.map((inv) => (
                      <TableRow key={inv.id} className="cursor-pointer hover:bg-muted/40">
                        <TableCell>
                          <div className="font-mono font-semibold text-sm">{inv.number}</div>
                          {inv.subscription_number && (
                            <div className="text-xs text-muted-foreground font-mono">
                              Sub: {inv.subscription_number}
                            </div>
                          )}
                        </TableCell>
                        <TableCell>
                          <div className="font-medium text-sm">
                            {inv.customer_name || customers.find((c) => c.id === inv.customer_id)?.name || 'Customer'}
                          </div>
                        </TableCell>
                        <TableCell className="text-xs">
                          <div>Issued: {formatDate(inv.issue_date)}</div>
                          <div className="text-muted-foreground">Due: {formatDate(inv.due_date)}</div>
                        </TableCell>
                        <TableCell>{getStatusBadge(inv.status)}</TableCell>
                        <TableCell className="text-right font-mono font-medium text-sm">
                          {formatCents(inv.total_cents)}
                        </TableCell>
                        <TableCell className="text-right font-mono font-bold text-sm">
                          {inv.balance_cents > 0 ? (
                            <span className="text-red-400">{formatCents(inv.balance_cents)}</span>
                          ) : (
                            <span className="text-emerald-400">$0.00</span>
                          )}
                        </TableCell>
                        <TableCell className="text-right space-x-1">
                          {inv.status === 'CONFIRMED' && inv.balance_cents > 0 && (
                            <Button
                              size="sm"
                              variant="outline"
                              onClick={() => handleOpenPaymentForInvoice(inv)}
                              className="text-xs h-8 cursor-pointer border-emerald-500/30 text-emerald-400 hover:bg-emerald-500/10"
                            >
                              Pay
                            </Button>
                          )}
                          <Button
                            size="sm"
                            variant="outline"
                            onClick={() => {
                              setSelectedInvoice(inv);
                              setIsInvoiceModalOpen(true);
                            }}
                            className="text-xs h-8 cursor-pointer"
                          >
                            View
                          </Button>
                        </TableCell>
                      </TableRow>
                    ))
                  )}
                </TableBody>
              </Table>
            </CardContent>
          </Card>
        </TabsContent>

        {/* PAYMENTS TAB */}
        <TabsContent value="payments" className="mt-4">
          <Card>
            <CardContent className="p-0">
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>Payment Reference</TableHead>
                    <TableHead>Invoice #</TableHead>
                    <TableHead>Method</TableHead>
                    <TableHead>Paid At</TableHead>
                    <TableHead>Note / Memo</TableHead>
                    <TableHead className="text-right">Amount Collected</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {payments.length === 0 ? (
                    <TableRow>
                      <TableCell colSpan={6} className="text-center py-8 text-muted-foreground">
                        No payments recorded yet.
                      </TableCell>
                    </TableRow>
                  ) : (
                    payments.map((p) => (
                      <TableRow key={p.id}>
                        <TableCell className="font-mono text-xs">{p.id.substring(0, 12)}...</TableCell>
                        <TableCell className="font-mono font-semibold text-sm">
                          {p.invoice_number || 'INV-2026-XXXX'}
                        </TableCell>
                        <TableCell>
                          <Badge variant="outline" className="text-xs">
                            {p.method}
                          </Badge>
                        </TableCell>
                        <TableCell className="text-xs text-muted-foreground">
                          {new Date(p.paid_at).toLocaleString()}
                        </TableCell>
                        <TableCell className="text-xs">{p.note || 'Regular settlement'}</TableCell>
                        <TableCell className="text-right font-mono font-bold text-emerald-400 text-sm">
                          {formatCents(p.amount_cents)}
                        </TableCell>
                      </TableRow>
                    ))
                  )}
                </TableBody>
              </Table>
            </CardContent>
          </Card>
        </TabsContent>

        {/* REPORTS TAB */}
        <TabsContent value="reports" className="mt-4 space-y-6">
          <div className="grid lg:grid-cols-2 gap-6">
            {/* Revenue Points table */}
            <Card>
              <CardHeader>
                <CardTitle className="text-base font-semibold flex items-center gap-2">
                  <TrendingUp className="h-4 w-4 text-emerald-400" />
                  Monthly Invoiced vs Collected Revenue
                </CardTitle>
                <CardDescription className="text-xs">
                  Historical revenue trajectory aggregated by backend index
                </CardDescription>
              </CardHeader>
              <CardContent className="p-0">
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead>Billing Month</TableHead>
                      <TableHead className="text-right">Invoiced</TableHead>
                      <TableHead className="text-right">Cash Collected</TableHead>
                      <TableHead className="text-right">Realization Rate</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {revenuePoints.map((pt) => {
                      const rate = pt.invoiced_cents > 0 ? (pt.paid_cents / pt.invoiced_cents) * 100 : 100;
                      return (
                        <TableRow key={pt.month}>
                          <TableCell className="font-semibold text-sm">{pt.month}</TableCell>
                          <TableCell className="text-right font-mono text-sm">
                            {formatCents(pt.invoiced_cents)}
                          </TableCell>
                          <TableCell className="text-right font-mono font-medium text-emerald-400 text-sm">
                            {formatCents(pt.paid_cents)}
                          </TableCell>
                          <TableCell className="text-right font-mono text-xs">
                            {rate.toFixed(1)}%
                          </TableCell>
                        </TableRow>
                      );
                    })}
                  </TableBody>
                </Table>
              </CardContent>
            </Card>

            {/* Overdue Debt Aging & Chase Table */}
            <Card>
              <CardHeader>
                <CardTitle className="text-base font-semibold flex items-center gap-2 text-red-400">
                  <AlertTriangle className="h-4 w-4" />
                  Overdue Invoices Requiring Follow-up
                </CardTitle>
                <CardDescription className="text-xs">
                  Invoices past due date with remaining debt &bull; respond in &lt; 2 seconds
                </CardDescription>
              </CardHeader>
              <CardContent className="p-0">
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead>Invoice #</TableHead>
                      <TableHead>Customer</TableHead>
                      <TableHead>Due Date</TableHead>
                      <TableHead className="text-right">Overdue Balance</TableHead>
                      <TableHead className="text-right">Action</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {overdueInvoices.length === 0 ? (
                      <TableRow>
                        <TableCell colSpan={5} className="text-center py-8 text-muted-foreground">
                          No overdue invoices outstanding. Outstanding collection healthy!
                        </TableCell>
                      </TableRow>
                    ) : (
                      overdueInvoices.map((ov) => (
                        <TableRow key={ov.id}>
                          <TableCell className="font-mono font-semibold text-sm text-red-400">
                            {ov.number}
                          </TableCell>
                          <TableCell className="text-sm font-medium">
                            {ov.customer_name || 'Apex Retailers Ltd'}
                          </TableCell>
                          <TableCell className="text-xs text-muted-foreground">
                            {formatDate(ov.due_date)}
                          </TableCell>
                          <TableCell className="text-right font-mono font-bold text-red-400 text-sm">
                            {formatCents(ov.balance_cents)}
                          </TableCell>
                          <TableCell className="text-right">
                            <Button
                              size="sm"
                              variant="outline"
                              onClick={() => handleSendInvoice(ov.id)}
                              className="text-xs h-7 cursor-pointer"
                            >
                              <Send className="h-3 w-3 mr-1" />
                              Chase
                            </Button>
                          </TableCell>
                        </TableRow>
                      ))
                    )}
                  </TableBody>
                </Table>
              </CardContent>
            </Card>
          </div>
        </TabsContent>
      </Tabs>

      {/* INVOICE DETAIL MODAL */}
      <Dialog open={isInvoiceModalOpen} onOpenChange={setIsInvoiceModalOpen}>
        <DialogHeader>
          <div className="flex items-center justify-between pr-6">
            <DialogTitle className="font-mono text-xl">{selectedInvoice?.number}</DialogTitle>
            {selectedInvoice && getStatusBadge(selectedInvoice.status)}
          </div>
          <DialogDescription>
            Billing snapshot with lines, automatic tax allocation, discounts, and payment balance.
          </DialogDescription>
        </DialogHeader>

        {selectedInvoice && (
          <div className="space-y-6">
            <div className="grid grid-cols-2 sm:grid-cols-4 gap-3 p-4 rounded-xl border bg-muted/30 text-xs">
              <div>
                <span className="text-muted-foreground">Customer:</span>
                <div className="font-semibold text-sm mt-0.5">
                  {selectedInvoice.customer_name || 'Customer Profile'}
                </div>
              </div>
              <div>
                <span className="text-muted-foreground">Issue Date:</span>
                <div className="font-semibold text-sm mt-0.5">{formatDate(selectedInvoice.issue_date)}</div>
              </div>
              <div>
                <span className="text-muted-foreground">Due Date:</span>
                <div className="font-semibold text-sm mt-0.5">{formatDate(selectedInvoice.due_date)}</div>
              </div>
              <div>
                <span className="text-muted-foreground">Balance Remaining:</span>
                <div className="font-bold text-sm mt-0.5 font-mono text-red-400">
                  {formatCents(selectedInvoice.balance_cents)}
                </div>
              </div>
            </div>

            {/* Line items table */}
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Item / Description</TableHead>
                  <TableHead className="text-center">Qty</TableHead>
                  <TableHead className="text-right">Unit Price</TableHead>
                  <TableHead className="text-right">Tax</TableHead>
                  <TableHead className="text-right">Discount</TableHead>
                  <TableHead className="text-right">Line Total</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {selectedInvoice.lines && selectedInvoice.lines.length > 0 ? (
                  selectedInvoice.lines.map((l) => (
                    <TableRow key={l.id}>
                      <TableCell className="font-medium text-sm">{l.description}</TableCell>
                      <TableCell className="text-center font-mono text-xs">{l.quantity}</TableCell>
                      <TableCell className="text-right font-mono text-xs">
                        {formatCents(l.unit_price_cents)}
                      </TableCell>
                      <TableCell className="text-right font-mono text-xs text-muted-foreground">
                        {formatCents(l.tax_cents)}
                      </TableCell>
                      <TableCell className="text-right font-mono text-xs text-emerald-400">
                        {l.discount_cents > 0 ? `-${formatCents(l.discount_cents)}` : '$0.00'}
                      </TableCell>
                      <TableCell className="text-right font-mono font-semibold text-sm">
                        {formatCents(l.line_total_cents)}
                      </TableCell>
                    </TableRow>
                  ))
                ) : (
                  <TableRow>
                    <TableCell colSpan={6} className="text-center py-4 text-muted-foreground">
                      No item lines recorded.
                    </TableCell>
                  </TableRow>
                )}
              </TableBody>
            </Table>

            {/* Totals Summary */}
            <div className="flex justify-end">
              <div className="w-64 space-y-1.5 text-xs">
                <div className="flex justify-between text-muted-foreground">
                  <span>Subtotal:</span>
                  <span className="font-mono">{formatCents(selectedInvoice.subtotal_cents)}</span>
                </div>
                {selectedInvoice.discount_cents > 0 && (
                  <div className="flex justify-between text-emerald-400">
                    <span>Discount Applied:</span>
                    <span className="font-mono">-{formatCents(selectedInvoice.discount_cents)}</span>
                  </div>
                )}
                <div className="flex justify-between text-muted-foreground">
                  <span>Calculated Taxes:</span>
                  <span className="font-mono">{formatCents(selectedInvoice.tax_cents)}</span>
                </div>
                <div className="flex justify-between font-bold text-sm pt-2 border-t text-foreground">
                  <span>Grand Total:</span>
                  <span className="font-mono text-emerald-400">{formatCents(selectedInvoice.total_cents)}</span>
                </div>
              </div>
            </div>

            {/* Actions */}
            <div className="p-4 rounded-xl border bg-muted/20 flex flex-wrap items-center justify-between gap-2">
              <div className="flex items-center gap-2">
                {selectedInvoice.status === 'DRAFT' && (
                  <Button
                    size="sm"
                    onClick={() => handleConfirmInvoice(selectedInvoice.id)}
                    className="cursor-pointer bg-blue-600 hover:bg-blue-700 text-white"
                  >
                    <CheckCircle className="h-4 w-4 mr-1.5" />
                    Confirm Invoice &rarr; CONFIRMED
                  </Button>
                )}

                {selectedInvoice.status === 'CONFIRMED' && (
                  <Button
                    size="sm"
                    variant="outline"
                    onClick={() => handleSendInvoice(selectedInvoice.id)}
                    className="cursor-pointer"
                  >
                    <Send className="h-4 w-4 mr-1.5" />
                    Send to Customer Email
                  </Button>
                )}

                {selectedInvoice.status === 'CONFIRMED' && selectedInvoice.balance_cents === selectedInvoice.total_cents && (
                  <Button
                    size="sm"
                    variant="destructive"
                    onClick={() => handleCancelInvoice(selectedInvoice.id)}
                    className="cursor-pointer"
                  >
                    <XCircle className="h-4 w-4 mr-1.5" />
                    Cancel Invoice
                  </Button>
                )}
              </div>

              {selectedInvoice.status === 'CONFIRMED' && selectedInvoice.balance_cents > 0 && (
                <Button
                  size="sm"
                  onClick={() => {
                    setIsInvoiceModalOpen(false);
                    handleOpenPaymentForInvoice(selectedInvoice);
                  }}
                  className="cursor-pointer bg-emerald-600 hover:bg-emerald-700 text-white"
                >
                  <CreditCard className="h-4 w-4 mr-1.5" />
                  Record Payment
                </Button>
              )}
            </div>
          </div>
        )}

        <DialogFooter>
          <Button variant="outline" onClick={() => setIsInvoiceModalOpen(false)}>
            Close
          </Button>
        </DialogFooter>
      </Dialog>

      {/* RECORD PAYMENT MODAL */}
      <Dialog open={isPaymentModalOpen} onOpenChange={setIsPaymentModalOpen}>
        <DialogHeader>
          <DialogTitle>Record Payment</DialogTitle>
          <DialogDescription>
            Allocate cash, card, or bank settlement against a confirmed invoice balance.
          </DialogDescription>
        </DialogHeader>

        <form onSubmit={handleRecordPayment} className="space-y-4">
          <div className="space-y-2">
            <label className="text-xs font-semibold text-muted-foreground">Select Confirmed Invoice</label>
            <select
              required
              className="w-full h-10 px-3 rounded-lg border bg-background text-sm font-mono"
              value={payInvoiceId}
              onChange={(e) => {
                setPayInvoiceId(e.target.value);
                const inv = invoices.find((i) => i.id === e.target.value);
                if (inv) setPayAmountDollars((inv.balance_cents / 100).toFixed(2));
              }}
            >
              <option value="">-- Choose Invoice --</option>
              {invoices
                .filter((i) => i.status === 'CONFIRMED')
                .map((i) => (
                  <option key={i.id} value={i.id}>
                    {i.number} &bull; {i.customer_name} &bull; Balance: {formatCents(i.balance_cents)}
                  </option>
                ))}
            </select>
          </div>

          <div className="grid grid-cols-2 gap-4">
            <div className="space-y-2">
              <label className="text-xs font-semibold text-muted-foreground">Payment Method</label>
              <select
                className="w-full h-10 px-3 rounded-lg border bg-background text-sm"
                value={payMethod}
                onChange={(e) => setPayMethod(e.target.value as PaymentMethod)}
              >
                <option value="CARD">Credit / Debit Card</option>
                <option value="BANK_TRANSFER">Bank Wire / ACH</option>
                <option value="CASH">Cash</option>
                <option value="MOBILE_MONEY">Mobile Money / UPI</option>
                <option value="OTHER">Other Method</option>
              </select>
            </div>

            <div className="space-y-2">
              <label className="text-xs font-semibold text-muted-foreground">Amount Paid ($)</label>
              <Input
                required
                type="number"
                step="0.01"
                value={payAmountDollars}
                onChange={(e) => setPayAmountDollars(e.target.value)}
              />
            </div>
          </div>

          <div className="space-y-2">
            <label className="text-xs font-semibold text-muted-foreground">Reference / Memo</label>
            <Input
              placeholder="e.g. Stripe Charge ID or Bank Reference #"
              value={payNote}
              onChange={(e) => setPayNote(e.target.value)}
            />
          </div>

          <DialogFooter>
            <Button type="button" variant="outline" onClick={() => setIsPaymentModalOpen(false)}>
              Cancel
            </Button>
            <Button type="submit" disabled={recordingPayment || !payInvoiceId}>
              {recordingPayment ? 'Recording...' : 'Post Payment'}
            </Button>
          </DialogFooter>
        </form>
      </Dialog>

      {/* CREATE AD-HOC INVOICE MODAL */}
      <Dialog open={isCreateInvoiceOpen} onOpenChange={setIsCreateInvoiceOpen}>
        <DialogHeader>
          <DialogTitle>Create Ad-Hoc Invoice</DialogTitle>
          <DialogDescription>
            Issue a manual invoice for one-off onboarding, custom support, or professional services.
          </DialogDescription>
        </DialogHeader>

        <form onSubmit={handleCreateInvoice} className="space-y-4">
          <div className="space-y-2">
            <label className="text-xs font-semibold text-muted-foreground">Customer</label>
            <select
              className="w-full h-10 px-3 rounded-lg border bg-background text-sm"
              value={newInvCustomerId}
              onChange={(e) => setNewInvCustomerId(e.target.value)}
            >
              {customers.map((c) => (
                <option key={c.id} value={c.id}>
                  {c.name}
                </option>
              ))}
            </select>
          </div>

          <div className="space-y-2">
            <label className="text-xs font-semibold text-muted-foreground">Line Description</label>
            <Input
              required
              value={newInvDescription}
              onChange={(e) => setNewInvDescription(e.target.value)}
            />
          </div>

          <div className="grid grid-cols-2 gap-4">
            <div className="space-y-2">
              <label className="text-xs font-semibold text-muted-foreground">Unit Price ($)</label>
              <Input
                required
                type="number"
                step="0.01"
                value={newInvAmountDollars}
                onChange={(e) => setNewInvAmountDollars(e.target.value)}
              />
            </div>

            <div className="space-y-2">
              <label className="text-xs font-semibold text-muted-foreground">Quantity</label>
              <Input
                type="number"
                min="1"
                value={newInvQty}
                onChange={(e) => setNewInvQty(e.target.value)}
              />
            </div>
          </div>

          <div className="space-y-2">
            <label className="text-xs font-semibold text-muted-foreground">Invoice Notes</label>
            <Input
              placeholder="Payment instructions, PO reference, terms..."
              value={newInvNotes}
              onChange={(e) => setNewInvNotes(e.target.value)}
            />
          </div>

          <DialogFooter>
            <Button type="button" variant="outline" onClick={() => setIsCreateInvoiceOpen(false)}>
              Cancel
            </Button>
            <Button type="submit" disabled={creatingInvoice}>
              {creatingInvoice ? 'Creating...' : 'Create Draft'}
            </Button>
          </DialogFooter>
        </form>
      </Dialog>
    </div>
  );
}

