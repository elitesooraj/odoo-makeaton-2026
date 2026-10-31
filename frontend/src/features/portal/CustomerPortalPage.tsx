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
} from 'lucide-react';
import { useAuth } from '@/context/AuthContext';
import { subscriptionsApi } from '@/api/subscriptions';
import { Subscription } from '@/api/types';
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

  const [subscriptions, setSubscriptions] = React.useState<Subscription[]>([]);
  const [loading, setLoading] = React.useState(false);
  const [selectedSub, setSelectedSub] = React.useState<Subscription | null>(null);
  const [isDetailOpen, setIsDetailOpen] = React.useState(false);

  // Mock client invoices
  const [invoices] = React.useState([
    {
      id: 'inv-001',
      number: 'INV-2026-0042',
      period: 'Mar 1, 2026 – Mar 31, 2026',
      amount_cents: 189200,
      status: 'PAID',
      due_date: '2026-03-31T00:00:00Z',
    },
    {
      id: 'inv-002',
      number: 'INV-2026-0089',
      period: 'Apr 1, 2026 – Apr 30, 2026',
      amount_cents: 189200,
      status: 'CONFIRMED',
      due_date: '2026-04-30T00:00:00Z',
    },
  ]);

  const fetchSubscriptions = React.useCallback(async () => {
    setLoading(true);
    try {
      // In customer portal, the query is scoped to this user's customer_id
      const res = await subscriptionsApi.list({
        customer_id: user?.customer_id || undefined,
      });
      setSubscriptions(res.data || []);
    } catch {
      toast({
        title: 'Error',
        description: 'Failed to retrieve your subscriptions',
        variant: 'destructive',
      });
    } finally {
      setLoading(false);
    }
  }, [user?.customer_id, toast]);

  React.useEffect(() => {
    fetchSubscriptions();
  }, [fetchSubscriptions]);

  const handleSelfServicePause = async (sub: Subscription) => {
    try {
      const nextStatus = sub.status === 'PAUSED' ? 'ACTIVE' : 'PAUSED';
      await subscriptionsApi.transition(sub.id, nextStatus);
      toast({
        title: nextStatus === 'ACTIVE' ? 'Subscription Resumed' : 'Subscription Paused',
        description: `Your subscription billing has been updated.`,
        variant: 'success',
      });
      fetchSubscriptions();
    } catch {
      toast({
        title: 'Action Failed',
        description: 'Could not change subscription status',
        variant: 'destructive',
      });
    }
  };

  const handlePayInvoice = (invNumber: string) => {
    toast({
      title: 'Payment Simulated',
      description: `Invoice ${invNumber} was processed through mock payment gateway.`,
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
            Welcome, <strong>{user?.full_name || user?.email}</strong>. View and manage your Brightloop subscriptions and invoices.
          </p>
        </div>
        <div className="flex items-center gap-2">
          <Badge variant="outline" className="font-mono text-xs px-3 py-1 bg-muted/50">
            Account ID: {user?.customer_id ? user.customer_id.substring(0, 8) + '...' : 'Self-Registered'}
          </Badge>
        </div>
      </div>

      {/* Subscriptions Tab */}
      {activeTab === 'subscriptions' && (
        <div className="space-y-4">
          <div className="flex items-center justify-between">
            <h2 className="text-lg font-bold tracking-tight">My Active Subscriptions</h2>
            <span className="text-xs text-muted-foreground">{subscriptions.length} active services</span>
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
                          {sub.subscription_number}
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
                          {formatCents(sub.total_cents)}
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
                        {sub.lines.map((l) => (
                          <li key={l.id} className="flex justify-between">
                            <span>
                              {l.quantity} &times; {l.product_name}
                            </span>
                            <span className="font-mono text-foreground">{formatCents(l.total_cents)}</span>
                          </li>
                        ))}
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
                        Details
                      </Button>

                      {/* Self-service pause/resume toggle */}
                      {sub.status === 'ACTIVE' && (
                        <Button
                          size="sm"
                          variant="ghost"
                          className="text-xs h-8 text-purple-400 hover:text-purple-300 cursor-pointer"
                          onClick={() => handleSelfServicePause(sub)}
                        >
                          <PauseCircle className="h-3.5 w-3.5 mr-1" />
                          Pause Subscription
                        </Button>
                      )}

                      {sub.status === 'PAUSED' && (
                        <Button
                          size="sm"
                          variant="ghost"
                          className="text-xs h-8 text-emerald-400 hover:text-emerald-300 cursor-pointer"
                          onClick={() => handleSelfServicePause(sub)}
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
        <Card>
          <CardHeader>
            <CardTitle>Invoices & Recurring Statements</CardTitle>
            <CardDescription>
              Review your monthly statements and process open balances.
            </CardDescription>
          </CardHeader>
          <CardContent className="p-0">
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Invoice #</TableHead>
                  <TableHead>Billing Cycle</TableHead>
                  <TableHead>Due Date</TableHead>
                  <TableHead>Amount Due</TableHead>
                  <TableHead>Status</TableHead>
                  <TableHead className="text-right">Action</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {invoices.map((inv) => (
                  <TableRow key={inv.id}>
                    <TableCell className="font-mono font-medium">{inv.number}</TableCell>
                    <TableCell className="text-xs text-muted-foreground">{inv.period}</TableCell>
                    <TableCell className="text-xs text-muted-foreground">{formatDate(inv.due_date)}</TableCell>
                    <TableCell className="font-mono font-semibold text-emerald-400">
                      {formatCents(inv.amount_cents)}
                    </TableCell>
                    <TableCell>
                      <Badge variant={inv.status === 'PAID' ? 'success' : 'warning'}>
                        {inv.status}
                      </Badge>
                    </TableCell>
                    <TableCell className="text-right">
                      {inv.status !== 'PAID' ? (
                        <Button
                          size="sm"
                          className="h-8 text-xs cursor-pointer"
                          onClick={() => handlePayInvoice(inv.number)}
                        >
                          <CreditCard className="h-3.5 w-3.5 mr-1" /> Pay Now
                        </Button>
                      ) : (
                        <span className="text-xs text-emerald-400 font-medium">Settled</span>
                      )}
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </CardContent>
        </Card>
      )}

      {/* Security Tab */}
      {activeTab === 'security' && (
        <div className="grid md:grid-cols-2 gap-4">
          <Card>
            <CardHeader>
              <CardTitle className="text-base">Registered Company Profile</CardTitle>
              <CardDescription>Client account details and linked contact info</CardDescription>
            </CardHeader>
            <CardContent className="space-y-3 text-xs">
              <div className="p-3 rounded-lg bg-muted/40 border space-y-1">
                <span className="text-muted-foreground">Contact Email</span>
                <div className="font-medium font-mono">{user?.email}</div>
              </div>
              <div className="p-3 rounded-lg bg-muted/40 border space-y-1">
                <span className="text-muted-foreground">Full Name</span>
                <div className="font-medium">{user?.full_name || 'Not configured'}</div>
              </div>
              <div className="p-3 rounded-lg bg-muted/40 border space-y-1">
                <span className="text-muted-foreground">Account Role</span>
                <Badge variant="outline" className="font-mono text-[10px]">
                  {user?.role} (Customer Portal Access)
                </Badge>
              </div>
            </CardContent>
          </Card>

          <Card>
            <CardHeader>
              <CardTitle className="text-base">Password & Credentials</CardTitle>
              <CardDescription>Update your portal access credentials</CardDescription>
            </CardHeader>
            <CardContent className="space-y-4 text-xs">
              <p className="text-muted-foreground leading-relaxed">
                Brightloop requires passwords with at least 9 characters, uppercase, lowercase, and special characters.
              </p>
              <Button
                variant="outline"
                size="sm"
                className="cursor-pointer"
                onClick={() => {
                  toast({
                    title: 'Use Top Header',
                    description: 'Click the Password button in the top navigation bar to change your password.',
                    variant: 'info',
                  });
                }}
              >
                <KeyRound className="h-3.5 w-3.5 mr-1" /> Change Password
              </Button>
            </CardContent>
          </Card>
        </div>
      )}

      {/* Subscription Breakdown Modal */}
      <Dialog open={isDetailOpen} onOpenChange={setIsDetailOpen}>
        {selectedSub && (
          <div className="space-y-4">
            <DialogHeader>
              <DialogTitle>{selectedSub.plan_name}</DialogTitle>
              <DialogDescription>
                Reference: <code className="text-xs">{selectedSub.subscription_number}</code>
              </DialogDescription>
            </DialogHeader>

            <div className="space-y-3 text-xs">
              <div className="border rounded-lg overflow-hidden">
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead>Service / Feature</TableHead>
                      <TableHead className="text-center">Qty</TableHead>
                      <TableHead className="text-right">Total</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {selectedSub.lines.map((l) => (
                      <TableRow key={l.id}>
                        <TableCell>
                          <div className="font-semibold">{l.product_name}</div>
                          {l.variant_name && <div className="text-primary">{l.variant_name}</div>}
                        </TableCell>
                        <TableCell className="text-center font-mono">{l.quantity}</TableCell>
                        <TableCell className="text-right font-mono text-emerald-400">
                          {formatCents(l.total_cents)}
                        </TableCell>
                      </TableRow>
                    ))}
                  </TableBody>
                </Table>
              </div>

              <div className="p-3 rounded-lg bg-muted/40 border flex justify-between items-center">
                <span className="font-semibold text-muted-foreground">Total Recurring Charge:</span>
                <span className="font-mono font-bold text-emerald-400 text-sm">
                  {formatCents(selectedSub.total_cents)}
                </span>
              </div>
            </div>

            <DialogFooter>
              <Button variant="outline" onClick={() => setIsDetailOpen(false)}>
                Close
              </Button>
            </DialogFooter>
          </div>
        )}
      </Dialog>
    </div>
  );
}

