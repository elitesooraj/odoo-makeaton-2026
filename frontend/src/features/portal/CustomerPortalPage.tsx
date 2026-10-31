import * as React from 'react';
import {
  FileText,
  Receipt,
  Shield,
  CreditCard,
  PauseCircle,
  PlayCircle,
  XCircle,
  Clock,
  CheckCircle2,
  AlertCircle,
  Building2,
  Mail,
  KeyRound,
  Download,
} from 'lucide-react';
import { useAuth } from '@/context/AuthContext';
import { portalApi } from '@/api/portal';
import { subscriptionsApi } from '@/api/subscriptions';
import { Invoice, Payment, PortalMe, Subscription } from '@/api/types';
import { formatCents } from '@/lib/money';
import { formatDate } from '@/lib/date';
import { Button } from '@/components/ui/button';
import { Badge } from '@/components/ui/badge';
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from '@/components/ui/card';
import { Dialog, DialogHeader, DialogTitle, DialogDescription, DialogFooter } from '@/components/ui/dialog';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';
import { useToast } from '@/components/ui/toast';
import { PortalTab } from '@/components/layout/PortalLayout';

interface Props {
  activeTab: PortalTab;
  onChangeTab: (tab: PortalTab) => void;
}

export function CustomerPortalPage({ activeTab }: Props) {
  const { user } = useAuth();
  const { toast } = useToast();

  const [portalMe, setPortalMe] = React.useState<PortalMe | null>(null);
  const [subscriptions, setSubscriptions] = React.useState<Subscription[]>([]);
  const [invoices, setInvoices] = React.useState<Invoice[]>([]);
  const [payments, setPayments] = React.useState<Payment[]>([]);
  const [loading, setLoading] = React.useState(false);
  const [selectedSub, setSelectedSub] = React.useState<Subscription | null>(null);
  const [isDetailOpen, setIsDetailOpen] = React.useState(false);

  const fetchData = React.useCallback(async () => {
    setLoading(true);
    try {
      const [meRes, subsRes, invRes, payRes] = await Promise.all([
        portalApi.getMe(),
        portalApi.listSubscriptions(),
        portalApi.listInvoices(),
        portalApi.listPayments(),
      ]);
      setPortalMe(meRes);
      setSubscriptions(subsRes || []);
      setInvoices(invRes || []);
      setPayments(payRes || []);
    } catch {
      toast({
        title: 'Error',
        description: 'Failed to retrieve portal information',
        variant: 'destructive',
      });
    } finally {
      setLoading(false);
    }
  }, [toast]);

  React.useEffect(() => {
    fetchData();
  }, [fetchData]);

  const handleSelfServiceTogglePause = async (sub: Subscription) => {
    try {
      if (sub.status === 'PAUSED') {
        await subscriptionsApi.resume(sub.id);
        toast({
          title: 'Subscription Resumed',
          description: `Your subscription billing has been resumed.`,
          variant: 'success',
        });
      } else {
        await subscriptionsApi.pause(sub.id);
        toast({
          title: 'Subscription Paused',
          description: `Your subscription is now paused.`,
          variant: 'success',
        });
      }
      fetchData();
    } catch {
      toast({
        title: 'Action Failed',
        description: 'Plan terms do not permit customer pause, or backend returned an error.',
        variant: 'destructive',
      });
    }
  };

  const handlePayInvoice = (inv: Invoice) => {
    toast({
      title: 'Payment Simulated',
      description: `Payment for ${inv.number} (${formatCents(inv.balance_cents)}) processed through simulated gateway.`,
      variant: 'success',
    });
  };

  return (
    <div className="space-y-6">
      {/* Welcome Banner */}
      <div className="p-6 rounded-2xl bg-gradient-to-r from-card to-card/60 border shadow-sm flex flex-col md:flex-row md:items-center md:justify-between gap-4">
        <div>
          <h1 className="text-2xl font-bold tracking-tight">Customer Portal</h1>
          <p className="text-sm text-muted-foreground mt-1">
            Welcome, <strong>{portalMe?.full_name || user?.full_name || user?.email}</strong>. View and manage your Brightloop subscriptions and invoices.
          </p>
        </div>
        <div className="flex items-center gap-2">
          <Badge variant="outline" className="font-mono text-xs px-3 py-1 bg-muted/50">
            Account ID: {user?.customer_id ? user.customer_id.substring(0, 8) + '...' : 'Self-Registered'}
          </Badge>
        </div>
      </div>

      {/* KPI Highlights */}
      {portalMe && (
        <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
          <Card className="border shadow-sm">
            <CardHeader className="pb-2">
              <CardDescription className="text-xs">Active Subscriptions</CardDescription>
              <CardTitle className="text-2xl font-bold font-mono text-emerald-400">
                {portalMe.active_status} / {portalMe.subscriptions}
              </CardTitle>
            </CardHeader>
          </Card>

          <Card className="border shadow-sm">
            <CardHeader className="pb-2">
              <CardDescription className="text-xs">Pending Invoices</CardDescription>
              <CardTitle className="text-2xl font-bold font-mono text-amber-400">
                {portalMe.unpaid_invoices}
              </CardTitle>
            </CardHeader>
          </Card>

          <Card className="border shadow-sm">
            <CardHeader className="pb-2">
              <CardDescription className="text-xs">Total Invoices</CardDescription>
              <CardTitle className="text-2xl font-bold font-mono text-foreground">
                {portalMe.invoices}
              </CardTitle>
            </CardHeader>
          </Card>

          <Card className="border shadow-sm">
            <CardHeader className="pb-2">
              <CardDescription className="text-xs">Account Status</CardDescription>
              <CardTitle className="text-xl font-bold text-emerald-400 flex items-center gap-1.5">
                <CheckCircle2 className="h-5 w-5" />
                Good Standing
              </CardTitle>
            </CardHeader>
          </Card>
        </div>
      )}

      {/* Subscriptions Tab */}
      {activeTab === 'subscriptions' && (
        <div className="space-y-4">
          <div className="flex items-center justify-between">
            <h2 className="text-lg font-bold tracking-tight">My Subscriptions</h2>
            <span className="text-xs text-muted-foreground">{subscriptions.length} registered contract(s)</span>
          </div>

          <div className="grid md:grid-cols-2 gap-4">
            {subscriptions.length === 0 ? (
              <div className="col-span-2 text-center py-16 text-muted-foreground bg-card/40 rounded-xl border">
                No active subscriptions found for this account.
              </div>
            ) : (
              subscriptions.map((sub) => (
                <Card key={sub.id} className="border hover:border-primary/40 transition-colors">
                  <CardHeader className="pb-3">
                    <div className="flex items-start justify-between">
                      <div>
                        <CardTitle className="text-base font-semibold">{sub.plan_name}</CardTitle>
                        <CardDescription className="font-mono text-xs text-muted-foreground">
                          {sub.number || sub.subscription_number}
                        </CardDescription>
                      </div>
                      <Badge
                        variant={
                          sub.status === 'ACTIVE'
                            ? 'success'
                            : sub.status === 'PAUSED'
                            ? 'outline'
                            : 'secondary'
                        }
                      >
                        {sub.status}
                      </Badge>
                    </div>
                  </CardHeader>
                  <CardContent className="space-y-4 text-xs">
                    <div className="p-3 rounded-lg bg-muted/40 border grid grid-cols-2 gap-2">
                      <div>
                        <span className="text-muted-foreground">Recurring Charge:</span>
                        <div className="font-mono font-bold text-base text-emerald-400 mt-0.5">
                          {formatCents(sub.total_cents || sub.subtotal_cents || 0)}
                        </div>
                      </div>
                      <div>
                        <span className="text-muted-foreground">Start Date:</span>
                        <div className="font-medium mt-0.5">{formatDate(sub.start_date)}</div>
                      </div>
                    </div>

                    <div className="space-y-1">
                      <span className="font-semibold text-muted-foreground">Included Items:</span>
                      <ul className="space-y-1 text-muted-foreground">
                        {sub.lines && sub.lines.length > 0 ? (
                          sub.lines.map((l) => (
                            <li key={l.id} className="flex justify-between">
                              <span>
                                {l.quantity} &times; {l.product_name || l.description}
                              </span>
                              <span className="font-mono text-foreground">
                                {formatCents(l.total_cents || l.quantity * l.unit_price_cents)}
                              </span>
                            </li>
                          ))
                        ) : (
                          <li className="text-muted-foreground">Standard plan configuration</li>
                        )}
                      </ul>
                    </div>

                    <div className="pt-2 border-t flex items-center justify-between gap-2">
                      <Button
                        size="sm"
                        variant="outline"
                        className="text-xs h-8 cursor-pointer"
                        onClick={() => {
                          setSelectedSub(sub);
                          setIsDetailOpen(true);
                        }}
                      >
                        <FileText className="h-3.5 w-3.5 mr-1" />
                        Details
                      </Button>

                      {sub.status === 'ACTIVE' && (
                        <Button
                          size="sm"
                          variant="outline"
                          onClick={() => handleSelfServiceTogglePause(sub)}
                          className="text-xs h-8 cursor-pointer border-amber-500/40 text-amber-400 hover:bg-amber-500/10"
                        >
                          <PauseCircle className="h-3.5 w-3.5 mr-1" />
                          Pause Subscription
                        </Button>
                      )}

                      {sub.status === 'PAUSED' && (
                        <Button
                          size="sm"
                          onClick={() => handleSelfServiceTogglePause(sub)}
                          className="text-xs h-8 cursor-pointer bg-emerald-600 hover:bg-emerald-700 text-white"
                        >
                          <PlayCircle className="h-3.5 w-3.5 mr-1" />
                          Resume Subscription
                        </Button>
                      )}
                    </div>
                  </CardContent>
                </Card>
              ))
            )}
          </div>
        </div>
      )}

      {/* Invoices Tab */}
      {activeTab === 'invoices' && (
        <div className="space-y-4">
          <div className="flex items-center justify-between">
            <h2 className="text-lg font-bold tracking-tight">Billing History & Invoices</h2>
            <span className="text-xs text-muted-foreground">{invoices.length} billing document(s)</span>
          </div>

          <Card>
            <CardContent className="p-0">
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>Invoice #</TableHead>
                    <TableHead>Issue Date</TableHead>
                    <TableHead>Due Date</TableHead>
                    <TableHead className="text-right">Amount</TableHead>
                    <TableHead className="text-right">Balance Due</TableHead>
                    <TableHead>Status</TableHead>
                    <TableHead className="text-right">Payment</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {invoices.length === 0 ? (
                    <TableRow>
                      <TableCell colSpan={7} className="text-center py-8 text-muted-foreground">
                        No invoices issued to this account yet.
                      </TableCell>
                    </TableRow>
                  ) : (
                    invoices.map((inv) => (
                      <TableRow key={inv.id}>
                        <TableCell className="font-mono font-semibold text-sm">{inv.number}</TableCell>
                        <TableCell className="text-xs text-muted-foreground">
                          {formatDate(inv.issue_date)}
                        </TableCell>
                        <TableCell className="text-xs text-muted-foreground">
                          {formatDate(inv.due_date)}
                        </TableCell>
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
                        <TableCell>
                          <Badge variant={inv.status === 'PAID' ? 'success' : 'warning'}>
                            {inv.status}
                          </Badge>
                        </TableCell>
                        <TableCell className="text-right">
                          {inv.status === 'CONFIRMED' && inv.balance_cents > 0 ? (
                            <Button
                              size="sm"
                              onClick={() => handlePayInvoice(inv)}
                              className="text-xs h-7 cursor-pointer bg-emerald-600 hover:bg-emerald-700 text-white"
                            >
                              <CreditCard className="h-3.5 w-3.5 mr-1" />
                              Pay Now
                            </Button>
                          ) : (
                            <Button size="sm" variant="ghost" className="text-xs h-7" disabled>
                              <CheckCircle2 className="h-3.5 w-3.5 mr-1 text-emerald-400" />
                              Settled
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
        </div>
      )}

      {/* Payments History Tab */}
      {activeTab === 'payments' && (
        <div className="space-y-4">
          <div className="flex items-center justify-between">
            <h2 className="text-lg font-bold tracking-tight">Payment Receipts</h2>
            <span className="text-xs text-muted-foreground">{payments.length} settled receipt(s)</span>
          </div>

          <Card>
            <CardContent className="p-0">
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>Payment Reference</TableHead>
                    <TableHead>Invoice #</TableHead>
                    <TableHead>Method</TableHead>
                    <TableHead>Date Settled</TableHead>
                    <TableHead className="text-right">Amount Paid</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {payments.length === 0 ? (
                    <TableRow>
                      <TableCell colSpan={5} className="text-center py-8 text-muted-foreground">
                        No payments recorded for this account.
                      </TableCell>
                    </TableRow>
                  ) : (
                    payments.map((p) => (
                      <TableRow key={p.id}>
                        <TableCell className="font-mono text-xs">{p.id}</TableCell>
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
        </div>
      )}

      {/* Account Profile Tab */}
      {activeTab === 'profile' && (
        <Card className="border">
          <CardHeader>
            <CardTitle>Organization & Billing Profile</CardTitle>
            <CardDescription>
              Your contact details registered with Brightloop Software.
            </CardDescription>
          </CardHeader>
          <CardContent className="space-y-4 text-sm">
            <div className="grid md:grid-cols-2 gap-4">
              <div className="p-4 rounded-xl border bg-muted/30 space-y-2">
                <span className="text-xs font-semibold uppercase text-muted-foreground flex items-center gap-1.5">
                  <Building2 className="h-4 w-4" />
                  Account Identity
                </span>
                <div className="font-bold text-base">{portalMe?.full_name || user?.full_name}</div>
                <div className="text-xs text-muted-foreground flex items-center gap-1">
                  <Mail className="h-3.5 w-3.5" />
                  {portalMe?.email || user?.email}
                </div>
              </div>

              <div className="p-4 rounded-xl border bg-muted/30 space-y-2">
                <span className="text-xs font-semibold uppercase text-muted-foreground flex items-center gap-1.5">
                  <Shield className="h-4 w-4" />
                  Security & Access
                </span>
                <div className="text-xs text-muted-foreground">
                  Role: <Badge variant="secondary" className="ml-1">CUSTOMER</Badge>
                </div>
                <div className="text-xs text-muted-foreground">
                  Session Type: Secure SameSite=Lax Cookie
                </div>
              </div>
            </div>
          </CardContent>
        </Card>
      )}

      {/* Subscription Detail Dialog */}
      <Dialog open={isDetailOpen} onOpenChange={setIsDetailOpen}>
        <DialogHeader>
          <DialogTitle className="font-mono text-lg">
            {selectedSub?.number || selectedSub?.subscription_number}
          </DialogTitle>
          <DialogDescription>
            Active line items and current recurring schedule.
          </DialogDescription>
        </DialogHeader>

        {selectedSub && (
          <div className="space-y-4 text-xs">
            <div className="p-3 rounded-lg border bg-muted/30 grid grid-cols-2 gap-2">
              <div>
                <span className="text-muted-foreground">Plan:</span>
                <div className="font-semibold text-sm mt-0.5">{selectedSub.plan_name}</div>
              </div>
              <div>
                <span className="text-muted-foreground">Next Billing Date:</span>
                <div className="font-semibold text-sm mt-0.5 font-mono text-emerald-400">
                  {selectedSub.next_billing_date ? formatDate(selectedSub.next_billing_date) : 'Ongoing'}
                </div>
              </div>
            </div>

            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Line Description</TableHead>
                  <TableHead className="text-center">Qty</TableHead>
                  <TableHead className="text-right">Amount</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {selectedSub.lines && selectedSub.lines.length > 0 ? (
                  selectedSub.lines.map((l) => (
                    <TableRow key={l.id}>
                      <TableCell className="font-medium">{l.product_name || l.description}</TableCell>
                      <TableCell className="text-center font-mono">{l.quantity}</TableCell>
                      <TableCell className="text-right font-mono font-bold text-emerald-400">
                        {formatCents(l.total_cents || l.quantity * l.unit_price_cents)}
                      </TableCell>
                    </TableRow>
                  ))
                ) : (
                  <TableRow>
                    <TableCell colSpan={3} className="text-center py-2 text-muted-foreground">
                      Standard package lines
                    </TableCell>
                  </TableRow>
                )}
              </TableBody>
            </Table>
          </div>
        )}

        <DialogFooter>
          <Button variant="outline" onClick={() => setIsDetailOpen(false)}>
            Close
          </Button>
        </DialogFooter>
      </Dialog>
    </div>
  );
}
