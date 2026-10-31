import * as React from 'react';
import {
  FileText,
  Plus,
  Send,
  CheckCircle,
  PlayCircle,
  PauseCircle,
  RotateCw,
  XCircle,
  Copy,
  Receipt,
  Search,
  Calendar,
  Layers,
  Building,
  ArrowRight,
  ShieldAlert,
} from 'lucide-react';
import { subscriptionsApi } from '@/api/subscriptions';
import { catalogApi } from '@/api/catalog';
import { customersApi } from '@/api/customers';
import {
  Subscription,
  SubscriptionStatus,
  QuotationTemplate,
  Product,
  Plan,
  Customer,
} from '@/api/types';
import { formatCents, dollarsToCents } from '@/lib/money';
import { formatDate } from '@/lib/date';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Badge } from '@/components/ui/badge';
import { Card, CardContent } from '@/components/ui/card';
import { Dialog, DialogHeader, DialogTitle, DialogDescription, DialogFooter } from '@/components/ui/dialog';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';
import { Tabs, TabsList, TabsTrigger, TabsContent } from '@/components/ui/tabs';
import { useToast } from '@/components/ui/toast';
import { PermissionGate } from '@/components/auth/PermissionGate';

export function SubscriptionsPage() {
  const { toast } = useToast();

  const [activeTab, setActiveTab] = React.useState('subscriptions');
  const [subscriptions, setSubscriptions] = React.useState<Subscription[]>([]);
  const [templates, setTemplates] = React.useState<QuotationTemplate[]>([]);
  const [customers, setCustomers] = React.useState<Customer[]>([]);
  const [products, setProducts] = React.useState<Product[]>([]);
  const [plans, setPlans] = React.useState<Plan[]>([]);
  const [search, setSearch] = React.useState('');
  const [loading, setLoading] = React.useState(false);

  // Detail Modal
  const [selectedSub, setSelectedSub] = React.useState<Subscription | null>(null);
  const [isDetailOpen, setIsDetailOpen] = React.useState(false);

  // New Quote / Subscription Builder Modal
  const [isBuilderOpen, setIsBuilderOpen] = React.useState(false);
  const [selectedCustomer, setSelectedCustomer] = React.useState('');
  const [selectedPlan, setSelectedPlan] = React.useState('');
  const [startDate, setStartDate] = React.useState(new Date().toISOString().split('T')[0]);
  const [expiryDate, setExpiryDate] = React.useState('');
  const [paymentTerms, setPaymentTerms] = React.useState('NET_30');
  const [templateId, setTemplateId] = React.useState('');
  const [lines, setLines] = React.useState<
    {
      productId: string;
      variantId: string;
      quantity: number;
    }[]
  >([]);
  const [building, setBuilding] = React.useState(false);

  const fetchData = React.useCallback(async () => {
    setLoading(true);
    try {
      const [subsRes, tmplRes, custRes, prodRes, planRes] = await Promise.all([
        subscriptionsApi.list({ search }),
        subscriptionsApi.listTemplates(),
        customersApi.list({ per_page: 100 }),
        catalogApi.listProducts(),
        catalogApi.listPlans(),
      ]);
      setSubscriptions(subsRes.data || []);
      setTemplates(tmplRes || []);
      setCustomers(custRes.data || []);
      setProducts(prodRes.data || []);
      setPlans(planRes.data || []);

      if (custRes.data?.length && !selectedCustomer) {
        setSelectedCustomer(custRes.data[0].id);
      }
      if (planRes.data?.length && !selectedPlan) {
        setSelectedPlan(planRes.data[0].id);
      }
    } catch {
      toast({
        title: 'Error',
        description: 'Failed to load subscription data',
        variant: 'destructive',
      });
    } finally {
      setLoading(false);
    }
  }, [search, selectedCustomer, selectedPlan, toast]);

  React.useEffect(() => {
    fetchData();
  }, [fetchData]);

  // Load Template action
  const applyTemplate = (tmplId: string) => {
    setTemplateId(tmplId);
    if (!tmplId) return;
    const tmpl = templates.find((t) => t.id === tmplId);
    if (!tmpl) return;

    setSelectedPlan(tmpl.plan_id);
    // Set expiry based on validity days
    const exp = new Date(Date.now() + tmpl.validity_days * 86400000).toISOString().split('T')[0];
    setExpiryDate(exp);

    // Populate lines
    setLines(
      tmpl.lines.map((l) => ({
        productId: l.product_id,
        variantId: l.variant_id || '',
        quantity: l.quantity,
      }))
    );
  };

  const addLine = () => {
    if (!products.length) return;
    setLines((prev) => [
      ...prev,
      {
        productId: products[0].id,
        variantId: products[0].variants?.[0]?.id || '',
        quantity: 1,
      },
    ]);
  };

  const removeLine = (idx: number) => {
    setLines((prev) => prev.filter((_, i) => i !== idx));
  };

  const updateLine = (idx: number, field: string, value: any) => {
    setLines((prev) =>
      prev.map((l, i) => {
        if (i !== idx) return l;
        if (field === 'productId') {
          const newProd = products.find((p) => p.id === value);
          return {
            ...l,
            productId: value,
            variantId: newProd?.variants?.[0]?.id || '',
          };
        }
        return { ...l, [field]: value };
      })
    );
  };

  const handleCreateSubscription = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!selectedCustomer || !selectedPlan || lines.length === 0) {
      toast({
        title: 'Incomplete Quote',
        description: 'Please select a customer, plan, and at least one product line.',
        variant: 'destructive',
      });
      return;
    }

    setBuilding(true);
    try {
      const custObj = customers.find((c) => c.id === selectedCustomer);
      const planObj = plans.find((p) => p.id === selectedPlan);

      const computedLines = lines.map((l) => {
        const prod = products.find((p) => p.id === l.productId);
        const variant = prod?.variants.find((v) => v.id === l.variantId);
        const basePrice = prod?.sales_price_cents || 0;
        const delta = variant?.price_delta_cents || 0;
        const unitPrice = basePrice + delta;

        return {
          product_id: l.productId,
          product_name: prod?.name || 'Product',
          variant_id: l.variantId || null,
          variant_name: variant ? `${variant.name} (+${formatCents(delta)})` : null,
          quantity: l.quantity,
          unit_price_cents: unitPrice,
          tax_rate_bp: 750, // 7.5% default tax
        };
      });

      const newSub = await subscriptionsApi.create({
        customer_id: selectedCustomer,
        customer_name: custObj?.name,
        plan_id: selectedPlan,
        plan_name: planObj?.name,
        start_date: new Date(startDate).toISOString(),
        expiry_date: expiryDate ? new Date(expiryDate).toISOString() : null,
        payment_terms: paymentTerms,
        lines: computedLines,
      });

      toast({
        title: 'Quotation Generated',
        description: `Draft subscription ${newSub.subscription_number} created successfully.`,
        variant: 'success',
      });

      setIsBuilderOpen(false);
      fetchData();
    } catch (err: any) {
      toast({
        title: 'Quote Generation Failed',
        description: err.message || 'Validation error',
        variant: 'destructive',
      });
    } finally {
      setBuilding(false);
    }
  };

  const handleTransition = async (targetStatus: SubscriptionStatus) => {
    if (!selectedSub) return;
    try {
      const updated = await subscriptionsApi.transition(selectedSub.id, targetStatus);
      setSelectedSub(updated);
      toast({
        title: 'Lifecycle State Updated',
        description: `Subscription ${updated.subscription_number} is now ${updated.status}.`,
        variant: 'success',
      });
      fetchData();
    } catch (err: any) {
      toast({
        title: 'Transition Failed',
        description: err.message || 'Backend validation rejected this state change',
        variant: 'destructive',
      });
    }
  };

  // Helper status badge variant
  const getStatusBadge = (status: SubscriptionStatus) => {
    switch (status) {
      case 'DRAFT':
        return <Badge variant="secondary">DRAFT</Badge>;
      case 'QUOTATION':
        return <Badge variant="warning">QUOTATION</Badge>;
      case 'CONFIRMED':
        return <Badge variant="info">CONFIRMED</Badge>;
      case 'ACTIVE':
        return <Badge variant="success">ACTIVE</Badge>;
      case 'PAUSED':
        return <Badge variant="outline" className="text-purple-400 border-purple-500/40">PAUSED</Badge>;
      case 'CLOSED':
        return <Badge variant="destructive">CLOSED</Badge>;
      default:
        return <Badge>{status}</Badge>;
    }
  };

  return (
    <div className="space-y-6">
      {/* Top Header */}
      <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-4">
        <div>
          <h1 className="text-2xl font-bold tracking-tight">Subscriptions & Quotations</h1>
          <p className="text-sm text-muted-foreground">
            Manage quotation templates, customer orders, and validate lifecycle state transitions.
          </p>
        </div>

        <PermissionGate permission="subscriptions.write">
          <Button
            onClick={() => {
              setLines([
                {
                  productId: products[0]?.id || '',
                  variantId: products[0]?.variants?.[0]?.id || '',
                  quantity: 1,
                },
              ]);
              setIsBuilderOpen(true);
            }}
            className="cursor-pointer"
          >
            <Plus className="h-4 w-4 mr-2" />
            New Quote / Subscription
          </Button>
        </PermissionGate>
      </div>

      {/* Tabs */}
      <Tabs value={activeTab} onValueChange={setActiveTab}>
        <TabsList>
          <TabsTrigger value="subscriptions" className="flex items-center gap-2">
            <FileText className="h-4 w-4" />
            Subscriptions ({subscriptions.length})
          </TabsTrigger>
          <TabsTrigger value="templates" className="flex items-center gap-2">
            <Copy className="h-4 w-4" />
            Quotation Templates ({templates.length})
          </TabsTrigger>
        </TabsList>

        {/* Subscriptions Table */}
        <TabsContent value="subscriptions">
          <Card>
            <CardContent className="p-0">
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>Subscription #</TableHead>
                    <TableHead>Customer</TableHead>
                    <TableHead>Plan</TableHead>
                    <TableHead>Status</TableHead>
                    <TableHead>Recurring Amount</TableHead>
                    <TableHead>Start Date</TableHead>
                    <TableHead className="text-right">Actions</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {subscriptions.length === 0 ? (
                    <TableRow>
                      <TableCell colSpan={7} className="h-32 text-center text-muted-foreground">
                        No subscriptions or quotes recorded.
                      </TableCell>
                    </TableRow>
                  ) : (
                    subscriptions.map((s) => (
                      <TableRow key={s.id}>
                        <TableCell>
                          <button
                            onClick={() => {
                              setSelectedSub(s);
                              setIsDetailOpen(true);
                            }}
                            className="font-mono font-semibold text-primary hover:underline cursor-pointer"
                          >
                            {s.subscription_number}
                          </button>
                        </TableCell>
                        <TableCell className="font-medium">{s.customer_name || 'Client'}</TableCell>
                        <TableCell className="text-xs text-muted-foreground">{s.plan_name}</TableCell>
                        <TableCell>{getStatusBadge(s.status)}</TableCell>
                        <TableCell className="font-mono font-semibold text-emerald-400">
                          {formatCents(s.total_cents)}
                        </TableCell>
                        <TableCell className="text-xs text-muted-foreground">{formatDate(s.start_date)}</TableCell>
                        <TableCell className="text-right">
                          <Button
                            variant="ghost"
                            size="sm"
                            className="h-8 text-xs cursor-pointer"
                            onClick={() => {
                              setSelectedSub(s);
                              setIsDetailOpen(true);
                            }}
                          >
                            Manage
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

        {/* Templates Tab */}
        <TabsContent value="templates">
          <div className="grid md:grid-cols-2 gap-4">
            {templates.map((tmpl) => (
              <Card key={tmpl.id} className="border">
                <CardContent className="p-6 space-y-4">
                  <div className="flex items-start justify-between">
                    <div>
                      <div className="font-bold text-base">{tmpl.name}</div>
                      <div className="text-xs text-muted-foreground mt-0.5">
                        Plan: <span className="text-foreground font-medium">{tmpl.plan_name}</span>
                      </div>
                    </div>
                    <Badge variant="outline" className="font-mono text-xs">
                      {tmpl.validity_days} days validity
                    </Badge>
                  </div>

                  <div className="space-y-1.5 p-3 rounded-lg bg-muted/40 border text-xs">
                    <span className="font-semibold text-muted-foreground">Pre-configured Lines:</span>
                    <ul className="list-disc list-inside space-y-1 text-muted-foreground">
                      {tmpl.lines.map((l, i) => (
                        <li key={i}>
                          Quantity {l.quantity} &times; Product ID: <code>{l.product_id}</code>
                        </li>
                      ))}
                    </ul>
                  </div>

                  <PermissionGate permission="subscriptions.write">
                    <Button
                      size="sm"
                      variant="secondary"
                      className="w-full text-xs cursor-pointer"
                      onClick={() => {
                        applyTemplate(tmpl.id);
                        setIsBuilderOpen(true);
                      }}
                    >
                      <Plus className="h-3.5 w-3.5 mr-1" />
                      Create Quote from this Template
                    </Button>
                  </PermissionGate>
                </CardContent>
              </Card>
            ))}
          </div>
        </TabsContent>
      </Tabs>

      {/* Subscription Detail & Lifecycle Stepper Modal */}
      <Dialog open={isDetailOpen} onOpenChange={setIsDetailOpen}>
        {selectedSub && (
          <div className="space-y-6">
            <DialogHeader>
              <div className="flex items-center justify-between">
                <div>
                  <DialogTitle className="font-mono text-xl">{selectedSub.subscription_number}</DialogTitle>
                  <DialogDescription>
                    Client: <strong className="text-foreground">{selectedSub.customer_name}</strong> &bull; Plan: {selectedSub.plan_name}
                  </DialogDescription>
                </div>
                <div>{getStatusBadge(selectedSub.status)}</div>
              </div>
            </DialogHeader>

            {/* Visual Lifecycle Stepper */}
            <div className="p-4 rounded-xl bg-muted/30 border space-y-3">
              <span className="text-xs font-semibold text-muted-foreground uppercase tracking-wider">
                Subscription Lifecycle Stage
              </span>
              <div className="flex items-center justify-between text-xs font-medium">
                {(['DRAFT', 'QUOTATION', 'CONFIRMED', 'ACTIVE', 'CLOSED'] as SubscriptionStatus[]).map((st, idx) => {
                  const isCurrent = selectedSub.status === st;
                  const isPast =
                    ['DRAFT', 'QUOTATION', 'CONFIRMED', 'ACTIVE', 'CLOSED'].indexOf(selectedSub.status) >= idx;

                  return (
                    <div key={st} className="flex items-center gap-2">
                      <div
                        className={`h-7 w-7 rounded-full flex items-center justify-center font-bold text-xs ${
                          isCurrent
                            ? 'bg-primary text-primary-foreground ring-4 ring-primary/20'
                            : isPast
                            ? 'bg-emerald-500/20 text-emerald-400'
                            : 'bg-muted text-muted-foreground'
                        }`}
                      >
                        {idx + 1}
                      </div>
                      <span className={isCurrent ? 'font-bold text-foreground' : 'text-muted-foreground'}>
                        {st}
                      </span>
                      {idx < 4 && <ArrowRight className="h-3 w-3 text-muted-foreground/40 hidden sm:block" />}
                    </div>
                  );
                })}
              </div>
            </div>

            {/* Contextual Action Toolbar based on current state */}
            <div className="p-3 rounded-lg bg-card border flex flex-wrap gap-2 items-center justify-between">
              <span className="text-xs text-muted-foreground font-medium">State Actions:</span>
              <div className="flex flex-wrap gap-2">
                {selectedSub.status === 'DRAFT' && (
                  <Button
                    size="sm"
                    className="text-xs cursor-pointer"
                    onClick={() => handleTransition('QUOTATION')}
                  >
                    <Send className="h-3.5 w-3.5 mr-1" /> Send Quotation
                  </Button>
                )}
                {selectedSub.status === 'QUOTATION' && (
                  <Button
                    size="sm"
                    className="text-xs bg-emerald-600 hover:bg-emerald-500 text-white cursor-pointer"
                    onClick={() => handleTransition('CONFIRMED')}
                  >
                    <CheckCircle className="h-3.5 w-3.5 mr-1" /> Confirm Quote
                  </Button>
                )}
                {selectedSub.status === 'CONFIRMED' && (
                  <Button
                    size="sm"
                    className="text-xs bg-emerald-600 hover:bg-emerald-500 text-white cursor-pointer"
                    onClick={() => handleTransition('ACTIVE')}
                  >
                    <PlayCircle className="h-3.5 w-3.5 mr-1" /> Activate Subscription
                  </Button>
                )}
                {selectedSub.status === 'ACTIVE' && (
                  <>
                    <Button
                      size="sm"
                      variant="outline"
                      className="text-xs cursor-pointer text-purple-400"
                      onClick={() => handleTransition('PAUSED')}
                    >
                      <PauseCircle className="h-3.5 w-3.5 mr-1" /> Pause Billing
                    </Button>
                    <Button
                      size="sm"
                      variant="outline"
                      className="text-xs cursor-pointer text-emerald-400"
                      onClick={() => handleTransition('ACTIVE')}
                    >
                      <RotateCw className="h-3.5 w-3.5 mr-1" /> Renew Term
                    </Button>
                    <Button
                      size="sm"
                      variant="outline"
                      className="text-xs cursor-pointer text-rose-400"
                      onClick={() => handleTransition('CLOSED')}
                    >
                      <XCircle className="h-3.5 w-3.5 mr-1" /> Close Subscription
                    </Button>
                  </>
                )}
                {selectedSub.status === 'PAUSED' && (
                  <Button
                    size="sm"
                    className="text-xs bg-emerald-600 hover:bg-emerald-500 text-white cursor-pointer"
                    onClick={() => handleTransition('ACTIVE')}
                  >
                    <PlayCircle className="h-3.5 w-3.5 mr-1" /> Resume Billing
                  </Button>
                )}
              </div>
            </div>

            {/* Line Items Details */}
            <div className="space-y-2">
              <span className="text-xs font-semibold text-muted-foreground uppercase tracking-wider">
                Subscription Line Items
              </span>
              <div className="border rounded-lg overflow-hidden">
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead>Product & Variant</TableHead>
                      <TableHead className="text-right">Unit Price</TableHead>
                      <TableHead className="text-center">Qty</TableHead>
                      <TableHead className="text-right">Tax (7.5%)</TableHead>
                      <TableHead className="text-right">Total</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {selectedSub.lines.map((l) => (
                      <TableRow key={l.id}>
                        <TableCell>
                          <div className="font-semibold text-xs">{l.product_name}</div>
                          {l.variant_name && (
                            <div className="text-[11px] text-primary">{l.variant_name}</div>
                          )}
                        </TableCell>
                        <TableCell className="font-mono text-xs text-right">
                          {formatCents(l.unit_price_cents)}
                        </TableCell>
                        <TableCell className="font-mono text-xs text-center">{l.quantity}</TableCell>
                        <TableCell className="font-mono text-xs text-right text-muted-foreground">
                          {formatCents(l.tax_cents)}
                        </TableCell>
                        <TableCell className="font-mono text-xs text-right font-medium text-emerald-400">
                          {formatCents(l.total_cents)}
                        </TableCell>
                      </TableRow>
                    ))}
                  </TableBody>
                </Table>
              </div>

              {/* Financial Totals */}
              <div className="flex justify-end p-3 rounded-lg bg-muted/40 border">
                <div className="w-64 space-y-1.5 text-xs">
                  <div className="flex justify-between text-muted-foreground">
                    <span>Subtotal:</span>
                    <span className="font-mono">{formatCents(selectedSub.subtotal_cents)}</span>
                  </div>
                  <div className="flex justify-between text-muted-foreground">
                    <span>Applicable Tax:</span>
                    <span className="font-mono">{formatCents(selectedSub.tax_cents)}</span>
                  </div>
                  <div className="flex justify-between font-bold text-sm pt-2 border-t">
                    <span>Total Recurring:</span>
                    <span className="font-mono text-emerald-400">{formatCents(selectedSub.total_cents)}</span>
                  </div>
                </div>
              </div>
            </div>

            <DialogFooter>
              <Button variant="outline" onClick={() => setIsDetailOpen(false)}>
                Done
              </Button>
            </DialogFooter>
          </div>
        )}
      </Dialog>

      {/* Quote / Subscription Builder Modal */}
      <Dialog open={isBuilderOpen} onOpenChange={setIsBuilderOpen}>
        <form onSubmit={handleCreateSubscription} className="space-y-4">
          <DialogHeader>
            <DialogTitle>Quotation & Subscription Builder</DialogTitle>
            <DialogDescription>
              Build an offer from a quotation template or customize product lines and variant options.
            </DialogDescription>
          </DialogHeader>

          <div className="space-y-4 text-xs">
            {/* Optional Template selector */}
            <div className="p-3 rounded-lg bg-primary/10 border border-primary/20 space-y-1.5">
              <label className="font-semibold text-primary flex items-center gap-1.5">
                <Copy className="h-3.5 w-3.5" /> Start from Quotation Template (Optional)
              </label>
              <select
                value={templateId}
                onChange={(e) => applyTemplate(e.target.value)}
                className="flex h-9 w-full rounded-md border border-input bg-background px-3 py-1 text-sm shadow-sm"
              >
                <option value="">-- Custom Quote (No Template) --</option>
                {templates.map((t) => (
                  <option key={t.id} value={t.id}>
                    {t.name} ({t.validity_days} days validity)
                  </option>
                ))}
              </select>
            </div>

            <div className="grid grid-cols-2 gap-3">
              <div className="space-y-1">
                <label className="font-medium text-muted-foreground">Customer *</label>
                <select
                  value={selectedCustomer}
                  onChange={(e) => setSelectedCustomer(e.target.value)}
                  className="flex h-9 w-full rounded-md border border-input bg-background px-3 py-1 text-sm shadow-sm"
                >
                  {customers.map((c) => (
                    <option key={c.id} value={c.id}>
                      {c.name}
                    </option>
                  ))}
                </select>
              </div>

              <div className="space-y-1">
                <label className="font-medium text-muted-foreground">Subscription Plan *</label>
                <select
                  value={selectedPlan}
                  onChange={(e) => setSelectedPlan(e.target.value)}
                  className="flex h-9 w-full rounded-md border border-input bg-background px-3 py-1 text-sm shadow-sm"
                >
                  {plans.map((p) => (
                    <option key={p.id} value={p.id}>
                      {p.name} ({formatCents(p.price_cents)} / {p.billing_interval})
                    </option>
                  ))}
                </select>
              </div>
            </div>

            <div className="grid grid-cols-3 gap-3">
              <div className="space-y-1">
                <label className="font-medium text-muted-foreground">Start Date</label>
                <Input
                  type="date"
                  value={startDate}
                  onChange={(e) => setStartDate(e.target.value)}
                />
              </div>

              <div className="space-y-1">
                <label className="font-medium text-muted-foreground">Expiry / Validity Date</label>
                <Input
                  type="date"
                  value={expiryDate}
                  onChange={(e) => setExpiryDate(e.target.value)}
                />
              </div>

              <div className="space-y-1">
                <label className="font-medium text-muted-foreground">Payment Terms</label>
                <select
                  value={paymentTerms}
                  onChange={(e) => setPaymentTerms(e.target.value)}
                  className="flex h-9 w-full rounded-md border border-input bg-background px-3 py-1 text-sm shadow-sm"
                >
                  <option value="IMMEDIATE">Immediate</option>
                  <option value="NET_15">Net 15 Days</option>
                  <option value="NET_30">Net 30 Days</option>
                  <option value="NET_60">Net 60 Days</option>
                </select>
              </div>
            </div>

            {/* Dynamic Product Lines Builder */}
            <div className="pt-2 border-t space-y-2">
              <div className="flex items-center justify-between">
                <div>
                  <span className="font-semibold text-muted-foreground">Product & Variant Lines</span>
                  <p className="text-[11px] text-muted-foreground">
                    Unit price is computed automatically from catalog base + variant delta.
                  </p>
                </div>
                <Button
                  type="button"
                  variant="outline"
                  size="sm"
                  onClick={addLine}
                  className="h-7 text-xs"
                >
                  <Plus className="h-3 w-3 mr-1" /> Add Line
                </Button>
              </div>

              {lines.map((l, idx) => {
                const prod = products.find((p) => p.id === l.productId);
                const variant = prod?.variants.find((v) => v.id === l.variantId);
                const unitPrice = (prod?.sales_price_cents || 0) + (variant?.price_delta_cents || 0);

                return (
                  <div key={idx} className="flex items-center gap-2 p-2.5 rounded-lg bg-muted/40 border">
                    <select
                      value={l.productId}
                      onChange={(e) => updateLine(idx, 'productId', e.target.value)}
                      className="flex h-9 w-44 rounded-md border border-input bg-background px-2 text-xs"
                    >
                      {products.map((p) => (
                        <option key={p.id} value={p.id}>
                          {p.name}
                        </option>
                      ))}
                    </select>

                    <select
                      value={l.variantId}
                      onChange={(e) => updateLine(idx, 'variantId', e.target.value)}
                      className="flex h-9 flex-1 rounded-md border border-input bg-background px-2 text-xs"
                    >
                      <option value="">Base / Standard</option>
                      {prod?.variants.map((v) => (
                        <option key={v.id} value={v.id}>
                          {v.name} (+{formatCents(v.price_delta_cents)})
                        </option>
                      ))}
                    </select>

                    <div className="w-16">
                      <Input
                        type="number"
                        min="1"
                        value={l.quantity}
                        onChange={(e) => updateLine(idx, 'quantity', parseInt(e.target.value, 10) || 1)}
                        className="h-9 font-mono text-center text-xs"
                      />
                    </div>

                    <div className="w-24 text-right font-mono text-xs text-emerald-400 font-semibold">
                      {formatCents(unitPrice * l.quantity)}
                    </div>

                    <Button
                      type="button"
                      variant="ghost"
                      size="sm"
                      onClick={() => removeLine(idx)}
                      className="h-8 w-8 p-0 text-rose-400"
                    >
                      &times;
                    </Button>
                  </div>
                );
              })}
            </div>
          </div>

          <DialogFooter>
            <Button type="button" variant="outline" onClick={() => setIsBuilderOpen(false)}>
              Cancel
            </Button>
            <Button type="submit" disabled={building || lines.length === 0}>
              {building ? 'Generating...' : 'Save Draft Quotation'}
            </Button>
          </DialogFooter>
        </form>
      </Dialog>
    </div>
  );
}

