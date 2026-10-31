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
  Zap,
  CheckCircle2,
  AlertCircle,
} from 'lucide-react';
import { subscriptionsApi } from '@/api/subscriptions';
import { catalogApi } from '@/api/catalog';
import { customersApi } from '@/api/customers';
import {
  BillingRunResult,
  Customer,
  Discount,
  Plan,
  Product,
  QuotationTemplate,
  Subscription,
  SubscriptionStatus,
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
  const [discounts, setDiscounts] = React.useState<Discount[]>([]);
  const [search, setSearch] = React.useState('');
  const [loading, setLoading] = React.useState(false);

  // Detail Modal
  const [selectedSub, setSelectedSub] = React.useState<Subscription | null>(null);
  const [isDetailOpen, setIsDetailOpen] = React.useState(false);

  // Billing Run Modal
  const [isBillingModalOpen, setIsBillingModalOpen] = React.useState(false);
  const [billingRunning, setBillingRunning] = React.useState(false);
  const [billingResult, setBillingResult] = React.useState<BillingRunResult | null>(null);

  // New Quote / Subscription Builder Modal
  const [isBuilderOpen, setIsBuilderOpen] = React.useState(false);
  const [selectedCustomer, setSelectedCustomer] = React.useState('');
  const [selectedPlan, setSelectedPlan] = React.useState('');
  const [selectedDiscount, setSelectedDiscount] = React.useState('');
  const [subType, setSubType] = React.useState<'QUOTATION' | 'DRAFT'>('QUOTATION');
  const [startDate, setStartDate] = React.useState(new Date().toISOString().split('T')[0]);
  const [expiryDate, setExpiryDate] = React.useState('');
  const [paymentTermsDays, setPaymentTermsDays] = React.useState(30);
  const [subNotes, setSubNotes] = React.useState('');
  const [templateId, setTemplateId] = React.useState('');
  const [lines, setLines] = React.useState<
    {
      productId: string;
      variantId: string;
      quantity: number;
    }[]
  >([]);
  const [building, setBuilding] = React.useState(false);

  // New Template Modal
  const [isTemplateModalOpen, setIsTemplateModalOpen] = React.useState(false);
  const [tmplName, setTmplName] = React.useState('');
  const [tmplValidityDays, setTmplValidityDays] = React.useState(30);
  const [tmplPlanId, setTmplPlanId] = React.useState('');
  const [tmplQty, setTmplQty] = React.useState(1);

  const fetchData = React.useCallback(async () => {
    setLoading(true);
    try {
      const [subsRes, tmplRes, custRes, prodRes, planRes, discRes] = await Promise.all([
        subscriptionsApi.list({ search }),
        subscriptionsApi.listTemplates(),
        customersApi.list({ per_page: 100 }),
        catalogApi.listProducts(),
        catalogApi.listPlans(),
        catalogApi.listDiscounts(),
      ]);
      setSubscriptions(subsRes.data || []);
      setTemplates(tmplRes || []);
      setCustomers(custRes.data || []);
      setProducts(prodRes.data || []);
      setPlans(planRes.data || []);
      setDiscounts(discRes.data || []);

      if (custRes.data?.length && !selectedCustomer) {
        setSelectedCustomer(custRes.data[0].id);
      }
      if (planRes.data?.length && !selectedPlan) {
        setSelectedPlan(planRes.data[0].id);
      }
      if (planRes.data?.length && !tmplPlanId) {
        setTmplPlanId(planRes.data[0].id);
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
  }, [search, selectedCustomer, selectedPlan, tmplPlanId, toast]);

  React.useEffect(() => {
    fetchData();
  }, [fetchData]);

  // Load Template action in Builder
  const applyTemplate = (tId: string) => {
    setTemplateId(tId);
    if (!tId) return;
    const tmpl = templates.find((t) => t.id === tId);
    if (!tmpl) return;

    if (tmpl.lines && tmpl.lines.length > 0) {
      setSelectedPlan(tmpl.lines[0].plan_id);
    }
    // Auto populate lines from matching products
    if (products.length > 0) {
      setLines([
        {
          productId: products[0].id,
          variantId: products[0].variants?.[0]?.id || '',
          quantity: tmpl.lines?.[0]?.quantity || 1,
        },
      ]);
    }
    // Expiry date from validity
    const d = new Date();
    d.setDate(d.getDate() + tmpl.validity_days);
    setExpiryDate(d.toISOString().split('T')[0]);
  };

  const handleInstantiateTemplateDirectly = async (tmpl: QuotationTemplate) => {
    if (!customers.length) {
      toast({
        title: 'No Customers Found',
        description: 'Create a customer account first.',
        variant: 'destructive',
      });
      return;
    }
    try {
      const targetCustomer = customers[0];
      const newQuote = await subscriptionsApi.instantiateTemplate(tmpl.id, {
        customer_id: targetCustomer.id,
        customer_name: targetCustomer.name,
        quantity: tmpl.lines?.[0]?.quantity || 1,
        payment_terms_days: 30,
        notes: `Instantiated from template: ${tmpl.name}`,
      });
      toast({
        title: 'Quotation Created from Template',
        description: `Quotation ${newQuote.number || newQuote.subscription_number} generated for ${targetCustomer.name}.`,
        variant: 'success',
      });
      fetchData();
      setActiveTab('subscriptions');
    } catch (err: any) {
      toast({
        title: 'Instantiation Failed',
        description: err.message || 'Could not instantiate template',
        variant: 'destructive',
      });
    }
  };

  const addLineRow = () => {
    if (products.length === 0) return;
    const firstProd = products[0];
    setLines((prev) => [
      ...prev,
      {
        productId: firstProd.id,
        variantId: firstProd.variants?.[0]?.id || '',
        quantity: 1,
      },
    ]);
  };

  const removeLineRow = (index: number) => {
    setLines((prev) => prev.filter((_, i) => i !== index));
  };

  const updateLineRow = (
    index: number,
    field: 'productId' | 'variantId' | 'quantity',
    value: any
  ) => {
    setLines((prev) =>
      prev.map((l, i) => {
        if (i !== index) return l;
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
          description: variant ? `${prod?.name} (${variant.name})` : prod?.name || 'Product',
          quantity: l.quantity,
          unit_price_cents: unitPrice,
          tax_id: planObj?.tax_id || null,
        };
      });

      const newSub = await subscriptionsApi.create({
        customer_id: selectedCustomer,
        customer_name: custObj?.name,
        plan_id: selectedPlan,
        plan_name: planObj?.name,
        status: subType,
        start_date: new Date(startDate).toISOString(),
        expiry_date: expiryDate ? new Date(expiryDate).toISOString() : null,
        payment_terms_days: paymentTermsDays,
        quantity: lines.reduce((acc, curr) => acc + curr.quantity, 0),
        discount_id: selectedDiscount || null,
        quote_expires_at: expiryDate ? new Date(expiryDate).toISOString() : null,
        notes: subNotes || undefined,
        lines: computedLines,
      });

      toast({
        title: subType === 'QUOTATION' ? 'Quotation Prepared' : 'Draft Subscription Created',
        description: `${newSub.number || newSub.subscription_number} is ready.`,
        variant: 'success',
      });

      setIsBuilderOpen(false);
      setSubNotes('');
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

  const handleCreateTemplate = async (e: React.FormEvent) => {
    e.preventDefault();
    try {
      await subscriptionsApi.createTemplate({
        name: tmplName,
        validity_days: tmplValidityDays,
        lines: [{ plan_id: tmplPlanId, quantity: tmplQty }],
      });
      toast({
        title: 'Template Created',
        description: `Template "${tmplName}" is now available for sales reps.`,
        variant: 'success',
      });
      setIsTemplateModalOpen(false);
      setTmplName('');
      fetchData();
    } catch (err: any) {
      toast({
        title: 'Failed to Create Template',
        description: err.message || 'Validation error',
        variant: 'destructive',
      });
    }
  };

  // State transitions using explicit backend actions
  const handleConfirm = async () => {
    if (!selectedSub) return;
    try {
      const updated = await subscriptionsApi.confirm(selectedSub.id);
      setSelectedSub(updated);
      toast({
        title: 'Quotation Confirmed',
        description: `Subscription ${updated.number || updated.subscription_number} is now CONFIRMED.`,
        variant: 'success',
      });
      fetchData();
    } catch (err: any) {
      toast({
        title: 'Confirmation Failed',
        description: err.message || 'Cannot confirm subscription',
        variant: 'destructive',
      });
    }
  };

  const handleActivate = async () => {
    if (!selectedSub) return;
    try {
      const updated = await subscriptionsApi.activate(selectedSub.id);
      setSelectedSub(updated);
      toast({
        title: 'Subscription Activated',
        description: `Billing cadence started for ${updated.number || updated.subscription_number}.`,
        variant: 'success',
      });
      fetchData();
    } catch (err: any) {
      toast({
        title: 'Activation Failed',
        description: err.message || 'Cannot activate subscription',
        variant: 'destructive',
      });
    }
  };

  const handlePause = async () => {
    if (!selectedSub) return;
    try {
      const updated = await subscriptionsApi.pause(selectedSub.id);
      setSelectedSub(updated);
      toast({
        title: 'Subscription Paused',
        description: `Recurring billing paused.`,
        variant: 'success',
      });
      fetchData();
    } catch (err: any) {
      toast({
        title: 'Pause Failed',
        description: err.message || 'Plan does not allow pause or state invalid',
        variant: 'destructive',
      });
    }
  };

  const handleResume = async () => {
    if (!selectedSub) return;
    try {
      const updated = await subscriptionsApi.resume(selectedSub.id);
      setSelectedSub(updated);
      toast({
        title: 'Subscription Resumed',
        description: `Subscription is active and next billing date computed.`,
        variant: 'success',
      });
      fetchData();
    } catch (err: any) {
      toast({
        title: 'Resume Failed',
        description: err.message || 'Cannot resume subscription',
        variant: 'destructive',
      });
    }
  };

  const handleClose = async () => {
    if (!selectedSub) return;
    try {
      const updated = await subscriptionsApi.close(selectedSub.id);
      setSelectedSub(updated);
      toast({
        title: 'Subscription Closed',
        description: `Subscription has reached CLOSED lifecycle state.`,
        variant: 'success',
      });
      fetchData();
    } catch (err: any) {
      toast({
        title: 'Closure Failed',
        description: err.message || 'Cannot close subscription',
        variant: 'destructive',
      });
    }
  };

  const handleTriggerBillingRun = async () => {
    setBillingRunning(true);
    try {
      const result = await subscriptionsApi.runBilling();
      setBillingResult(result);
      toast({
        title: 'Billing Run Complete',
        description: `Processed: ${result.billed} billed, ${result.skipped} skipped, ${result.closed} closed.`,
        variant: 'success',
      });
      fetchData();
    } catch (err: any) {
      toast({
        title: 'Billing Run Failed',
        description: err.message || 'Billing cycle execution error',
        variant: 'destructive',
      });
    } finally {
      setBillingRunning(false);
    }
  };

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
        return <Badge variant="outline" className="border-amber-500/50 text-amber-400">PAUSED</Badge>;
      case 'CLOSED':
        return <Badge variant="destructive">CLOSED</Badge>;
      default:
        return <Badge>{status}</Badge>;
    }
  };

  return (
    <div className="space-y-6">
      <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-4">
        <div>
          <h1 className="text-2xl font-bold tracking-tight">Subscriptions & Quotations</h1>
          <p className="text-sm text-muted-foreground">
            Manage contract lifecycle (Draft &rarr; Quotation &rarr; Confirmed &rarr; Active &rarr; Closed), quotations templates, and billing runs.
          </p>
        </div>

        <div className="flex flex-wrap items-center gap-2">
          <PermissionGate permission="subscriptions.write">
            <Button
              onClick={() => {
                setBillingResult(null);
                setIsBillingModalOpen(true);
              }}
              variant="outline"
              size="sm"
              className="cursor-pointer border-emerald-500/30 text-emerald-400 hover:bg-emerald-500/10"
            >
              <Zap className="h-4 w-4 mr-1.5" />
              Run Billing Cycle
            </Button>
          </PermissionGate>

          <PermissionGate permission="subscriptions.write">
            <Button
              onClick={() => setIsTemplateModalOpen(true)}
              variant="outline"
              size="sm"
              className="cursor-pointer"
            >
              <Layers className="h-4 w-4 mr-1.5" />
              New Template
            </Button>
          </PermissionGate>

          <PermissionGate permission="subscriptions.write">
            <Button
              onClick={() => {
                addLineRow();
                setIsBuilderOpen(true);
              }}
              size="sm"
              className="cursor-pointer"
            >
              <Plus className="h-4 w-4 mr-1.5" />
              Prepare Quotation
            </Button>
          </PermissionGate>
        </div>
      </div>

      <Tabs value={activeTab} onValueChange={setActiveTab}>
        <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-4">
          <TabsList>
            <TabsTrigger value="subscriptions" className="flex items-center gap-2">
              <FileText className="h-4 w-4" />
              Subscriptions ({subscriptions.length})
            </TabsTrigger>
            <TabsTrigger value="templates" className="flex items-center gap-2">
              <Layers className="h-4 w-4" />
              Quotation Templates ({templates.length})
            </TabsTrigger>
          </TabsList>

          {activeTab === 'subscriptions' && (
            <div className="relative w-full sm:w-64">
              <Search className="absolute left-3 top-2.5 h-4 w-4 text-muted-foreground" />
              <Input
                placeholder="Search number or customer..."
                value={search}
                onChange={(e) => setSearch(e.target.value)}
                className="pl-9 h-9"
              />
            </div>
          )}
        </div>

        {/* SUBSCRIPTIONS TAB */}
        <TabsContent value="subscriptions" className="mt-4">
          <Card>
            <CardContent className="p-0">
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>Subscription #</TableHead>
                    <TableHead>Customer</TableHead>
                    <TableHead>Plan</TableHead>
                    <TableHead>Status</TableHead>
                    <TableHead>Terms / Next Billing</TableHead>
                    <TableHead>Total Value</TableHead>
                    <TableHead className="text-right">Actions</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {loading && subscriptions.length === 0 ? (
                    <TableRow>
                      <TableCell colSpan={7} className="text-center py-8 text-muted-foreground">
                        Loading subscriptions...
                      </TableCell>
                    </TableRow>
                  ) : subscriptions.length === 0 ? (
                    <TableRow>
                      <TableCell colSpan={7} className="text-center py-8 text-muted-foreground">
                        No subscriptions found. Create a quotation to get started.
                      </TableCell>
                    </TableRow>
                  ) : (
                    subscriptions.map((s) => (
                      <TableRow key={s.id} className="cursor-pointer hover:bg-muted/40">
                        <TableCell>
                          <div className="font-mono font-semibold text-sm">
                            {s.number || s.subscription_number}
                          </div>
                          <div className="text-xs text-muted-foreground">
                            Started {formatDate(s.start_date)}
                          </div>
                        </TableCell>
                        <TableCell>
                          <div className="font-medium text-sm">
                            {s.customer_name || customers.find((c) => c.id === s.customer_id)?.name || 'Customer'}
                          </div>
                        </TableCell>
                        <TableCell>
                          <div className="text-sm font-medium">
                            {s.plan_name || plans.find((p) => p.id === s.plan_id)?.name || 'Standard Plan'}
                          </div>
                          <div className="text-xs text-muted-foreground">Qty: {s.quantity}</div>
                        </TableCell>
                        <TableCell>{getStatusBadge(s.status)}</TableCell>
                        <TableCell className="text-xs">
                          <div>Terms: NET {s.payment_terms_days} days</div>
                          {s.next_billing_date ? (
                            <div className="text-emerald-400 font-mono mt-0.5">
                              Next: {formatDate(s.next_billing_date)}
                            </div>
                          ) : (
                            <div className="text-muted-foreground">No pending date</div>
                          )}
                        </TableCell>
                        <TableCell className="font-mono font-bold text-emerald-400 text-sm">
                          {formatCents(s.total_cents || s.subtotal_cents || 0)}
                        </TableCell>
                        <TableCell className="text-right">
                          <Button
                            size="sm"
                            variant="outline"
                            onClick={() => {
                              setSelectedSub(s);
                              setIsDetailOpen(true);
                            }}
                            className="text-xs h-8 cursor-pointer"
                          >
                            Lifecycle & Details
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

        {/* TEMPLATES TAB */}
        <TabsContent value="templates" className="mt-4">
          <div className="grid sm:grid-cols-2 lg:grid-cols-3 gap-4">
            {templates.map((tmpl) => (
              <Card key={tmpl.id} className="border hover:border-primary/40 transition-colors">
                <CardContent className="p-5 space-y-4">
                  <div className="flex items-start justify-between">
                    <div>
                      <h3 className="font-semibold text-base">{tmpl.name}</h3>
                      <p className="text-xs text-muted-foreground mt-0.5">
                        Valid for {tmpl.validity_days} days after quotation issuance
                      </p>
                    </div>
                    <Badge variant="outline" className="font-mono text-xs">
                      {tmpl.lines?.length || 1} line(s)
                    </Badge>
                  </div>

                  {tmpl.notes && (
                    <p className="text-xs text-muted-foreground bg-muted/40 p-2 rounded">
                      {tmpl.notes}
                    </p>
                  )}

                  <div className="pt-2 border-t flex items-center justify-between">
                    <span className="text-xs font-medium text-muted-foreground">Reusable Line Set</span>
                    <Button
                      size="sm"
                      onClick={() => handleInstantiateTemplateDirectly(tmpl)}
                      className="cursor-pointer text-xs"
                    >
                      <Copy className="h-3.5 w-3.5 mr-1.5" />
                      Instantiate Quote
                    </Button>
                  </div>
                </CardContent>
              </Card>
            ))}
          </div>
        </TabsContent>
      </Tabs>

      {/* SUBSCRIPTION DETAIL & LIFECYCLE MODAL */}
      <Dialog open={isDetailOpen} onOpenChange={setIsDetailOpen}>
        <DialogHeader>
          <div className="flex items-center justify-between pr-6">
            <DialogTitle className="font-mono text-xl">
              {selectedSub?.number || selectedSub?.subscription_number}
            </DialogTitle>
            {selectedSub && getStatusBadge(selectedSub.status)}
          </div>
          <DialogDescription>
            Audit subscription details, product line breakdowns, and apply validated state transitions.
          </DialogDescription>
        </DialogHeader>

        {selectedSub && (
          <div className="space-y-6">
            {/* Top metadata grid */}
            <div className="grid grid-cols-2 sm:grid-cols-4 gap-3 p-4 rounded-xl border bg-muted/30 text-xs">
              <div>
                <span className="text-muted-foreground">Customer:</span>
                <div className="font-semibold text-sm mt-0.5">
                  {selectedSub.customer_name || 'Customer Account'}
                </div>
              </div>
              <div>
                <span className="text-muted-foreground">Payment Terms:</span>
                <div className="font-semibold text-sm mt-0.5">NET {selectedSub.payment_terms_days} days</div>
              </div>
              <div>
                <span className="text-muted-foreground">Next Billing Date:</span>
                <div className="font-semibold text-sm mt-0.5 font-mono text-emerald-400">
                  {selectedSub.next_billing_date ? formatDate(selectedSub.next_billing_date) : 'None'}
                </div>
              </div>
              <div>
                <span className="text-muted-foreground">Total Value:</span>
                <div className="font-bold text-sm mt-0.5 font-mono text-emerald-400">
                  {formatCents(selectedSub.total_cents || selectedSub.subtotal_cents || 0)}
                </div>
              </div>
            </div>

            {/* Product Lines Breakdown */}
            <div className="space-y-2">
              <h4 className="text-xs font-semibold uppercase tracking-wider text-muted-foreground">
                Line Items
              </h4>
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>Item / Variant</TableHead>
                    <TableHead className="text-center">Qty</TableHead>
                    <TableHead className="text-right">Unit Price</TableHead>
                    <TableHead className="text-right">Tax</TableHead>
                    <TableHead className="text-right">Total</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {selectedSub.lines && selectedSub.lines.length > 0 ? (
                    selectedSub.lines.map((l) => (
                      <TableRow key={l.id}>
                        <TableCell>
                          <div className="font-semibold text-sm">{l.product_name || l.description}</div>
                          {l.variant_name && (
                            <div className="text-xs text-muted-foreground font-mono">{l.variant_name}</div>
                          )}
                        </TableCell>
                        <TableCell className="text-center font-mono text-xs">{l.quantity}</TableCell>
                        <TableCell className="text-right font-mono text-xs">
                          {formatCents(l.unit_price_cents)}
                        </TableCell>
                        <TableCell className="text-right font-mono text-xs text-muted-foreground">
                          {formatCents(l.tax_cents || 0)}
                        </TableCell>
                        <TableCell className="text-right font-mono font-semibold text-sm text-emerald-400">
                          {formatCents(l.total_cents || l.quantity * l.unit_price_cents)}
                        </TableCell>
                      </TableRow>
                    ))
                  ) : (
                    <TableRow>
                      <TableCell colSpan={5} className="text-center py-4 text-muted-foreground">
                        No line items recorded.
                      </TableCell>
                    </TableRow>
                  )}
                </TableBody>
              </Table>
            </div>

            {/* State Transition Actions */}
            <div className="p-4 rounded-xl border bg-muted/20 space-y-3">
              <h4 className="text-xs font-semibold uppercase tracking-wider text-muted-foreground flex items-center gap-1.5">
                <ArrowRight className="h-4 w-4" />
                Validated Lifecycle Transitions
              </h4>

              <div className="flex flex-wrap items-center gap-2">
                {/* DRAFT or QUOTATION -> CONFIRMED */}
                {(selectedSub.status === 'DRAFT' || selectedSub.status === 'QUOTATION') && (
                  <Button
                    size="sm"
                    onClick={handleConfirm}
                    className="cursor-pointer bg-blue-600 hover:bg-blue-700 text-white"
                  >
                    <CheckCircle className="h-4 w-4 mr-1.5" />
                    Confirm Quote &rarr; CONFIRMED
                  </Button>
                )}

                {/* CONFIRMED -> ACTIVE */}
                {selectedSub.status === 'CONFIRMED' && (
                  <Button
                    size="sm"
                    onClick={handleActivate}
                    className="cursor-pointer bg-emerald-600 hover:bg-emerald-700 text-white"
                  >
                    <PlayCircle className="h-4 w-4 mr-1.5" />
                    Activate Subscription &rarr; ACTIVE
                  </Button>
                )}

                {/* ACTIVE -> PAUSED */}
                {selectedSub.status === 'ACTIVE' && (
                  <Button
                    size="sm"
                    variant="outline"
                    onClick={handlePause}
                    className="cursor-pointer border-amber-500/40 text-amber-400 hover:bg-amber-500/10"
                  >
                    <PauseCircle className="h-4 w-4 mr-1.5" />
                    Pause Billing &rarr; PAUSED
                  </Button>
                )}

                {/* PAUSED -> ACTIVE */}
                {selectedSub.status === 'PAUSED' && (
                  <Button
                    size="sm"
                    onClick={handleResume}
                    className="cursor-pointer bg-emerald-600 hover:bg-emerald-700 text-white"
                  >
                    <RotateCw className="h-4 w-4 mr-1.5" />
                    Resume Billing &rarr; ACTIVE
                  </Button>
                )}

                {/* ACTIVE or PAUSED -> CLOSED */}
                {(selectedSub.status === 'ACTIVE' || selectedSub.status === 'PAUSED') && (
                  <Button
                    size="sm"
                    variant="destructive"
                    onClick={handleClose}
                    className="cursor-pointer"
                  >
                    <XCircle className="h-4 w-4 mr-1.5" />
                    Close Subscription &rarr; CLOSED
                  </Button>
                )}
              </div>
            </div>
          </div>
        )}

        <DialogFooter>
          <Button variant="outline" onClick={() => setIsDetailOpen(false)}>
            Close
          </Button>
        </DialogFooter>
      </Dialog>

      {/* BILLING RUN MODAL */}
      <Dialog open={isBillingModalOpen} onOpenChange={setIsBillingModalOpen}>
        <DialogHeader>
          <DialogTitle className="flex items-center gap-2">
            <Zap className="h-5 w-5 text-emerald-400" />
            Automated Billing Day Cycle
          </DialogTitle>
          <DialogDescription>
            Trigger the backend billing worker (<code className="font-mono text-xs">POST /billing/run</code>). Evaluates every active subscription due for renewal, generates confirmed invoices, calculates taxes & discounts, and advances billing periods idempotently.
          </DialogDescription>
        </DialogHeader>

        <div className="space-y-4 py-2">
          {billingResult ? (
            <div className="space-y-4 p-4 rounded-xl border bg-card">
              <div className="flex items-center gap-2 text-emerald-400 font-semibold text-sm">
                <CheckCircle2 className="h-5 w-5" />
                Billing Cycle Processed Successfully
              </div>

              <div className="grid grid-cols-4 gap-2 text-center text-xs">
                <div className="p-3 rounded-lg bg-muted/40 border">
                  <div className="text-2xl font-bold font-mono text-emerald-400">
                    {billingResult.billed}
                  </div>
                  <div className="text-muted-foreground mt-1">Invoices Generated</div>
                </div>
                <div className="p-3 rounded-lg bg-muted/40 border">
                  <div className="text-2xl font-bold font-mono text-blue-400">
                    {billingResult.skipped}
                  </div>
                  <div className="text-muted-foreground mt-1">Skipped / Up-to-date</div>
                </div>
                <div className="p-3 rounded-lg bg-muted/40 border">
                  <div className="text-2xl font-bold font-mono text-amber-400">
                    {billingResult.closed}
                  </div>
                  <div className="text-muted-foreground mt-1">Auto-Closed</div>
                </div>
                <div className="p-3 rounded-lg bg-muted/40 border">
                  <div className="text-2xl font-bold font-mono text-muted-foreground">
                    {billingResult.failed}
                  </div>
                  <div className="text-muted-foreground mt-1">Failed</div>
                </div>
              </div>

              <div className="text-xs text-muted-foreground">
                Processed at: {new Date(billingResult.as_of).toLocaleString()}
              </div>
            </div>
          ) : (
            <div className="p-4 rounded-xl border bg-muted/20 space-y-2 text-xs">
              <p>
                <strong>Zero Duplication Guarantee:</strong> The billing engine runs inside database transactions with composite unique constraints on <code className="font-mono">billing_runs(subscription_id, period_start)</code>.
              </p>
              <p className="text-muted-foreground">
                Invoices generated will immediately appear in the Invoices & Billing tab and in customer portals.
              </p>
            </div>
          )}
        </div>

        <DialogFooter>
          <Button variant="outline" onClick={() => setIsBillingModalOpen(false)}>
            Close
          </Button>
          <Button
            onClick={handleTriggerBillingRun}
            disabled={billingRunning}
            className="cursor-pointer bg-emerald-600 hover:bg-emerald-700 text-white"
          >
            {billingRunning ? 'Processing Billing...' : 'Execute Billing Run Now'}
          </Button>
        </DialogFooter>
      </Dialog>

      {/* QUOTATION BUILDER MODAL */}
      <Dialog open={isBuilderOpen} onOpenChange={setIsBuilderOpen}>
        <DialogHeader>
          <DialogTitle>Prepare Quotation / Subscription</DialogTitle>
          <DialogDescription>
            Assemble a custom proposal with plans, product lines, and additive brand variants.
          </DialogDescription>
        </DialogHeader>

        <form onSubmit={handleCreateSubscription} className="space-y-4">
          <div className="grid grid-cols-2 gap-4">
            <div className="space-y-2">
              <label className="text-xs font-semibold text-muted-foreground">Customer</label>
              <select
                className="w-full h-10 px-3 rounded-lg border bg-background text-sm"
                value={selectedCustomer}
                onChange={(e) => setSelectedCustomer(e.target.value)}
              >
                {customers.map((c) => (
                  <option key={c.id} value={c.id}>
                    {c.name}
                  </option>
                ))}
              </select>
            </div>

            <div className="space-y-2">
              <label className="text-xs font-semibold text-muted-foreground">Initial Status</label>
              <select
                className="w-full h-10 px-3 rounded-lg border bg-background text-sm"
                value={subType}
                onChange={(e) => setSubType(e.target.value as 'QUOTATION' | 'DRAFT')}
              >
                <option value="QUOTATION">QUOTATION (Offer with validity window)</option>
                <option value="DRAFT">DRAFT (Internal rough draft)</option>
              </select>
            </div>

            <div className="space-y-2">
              <label className="text-xs font-semibold text-muted-foreground">Billing Plan</label>
              <select
                className="w-full h-10 px-3 rounded-lg border bg-background text-sm"
                value={selectedPlan}
                onChange={(e) => setSelectedPlan(e.target.value)}
              >
                {plans.map((p) => (
                  <option key={p.id} value={p.id}>
                    {p.name} ({formatCents(p.price_cents)} / {p.interval || p.billing_interval})
                  </option>
                ))}
              </select>
            </div>

            <div className="space-y-2">
              <label className="text-xs font-semibold text-muted-foreground">Apply Discount Rule</label>
              <select
                className="w-full h-10 px-3 rounded-lg border bg-background text-sm"
                value={selectedDiscount}
                onChange={(e) => setSelectedDiscount(e.target.value)}
              >
                <option value="">No Discount</option>
                {discounts.map((d) => (
                  <option key={d.id} value={d.id}>
                    {d.name} ({d.kind === 'PERCENT' ? `${d.value_bp / 100}%` : formatCents(d.value_cents)})
                  </option>
                ))}
              </select>
            </div>

            <div className="space-y-2">
              <label className="text-xs font-semibold text-muted-foreground">Start Date</label>
              <Input
                type="date"
                required
                value={startDate}
                onChange={(e) => setStartDate(e.target.value)}
              />
            </div>

            <div className="space-y-2">
              <label className="text-xs font-semibold text-muted-foreground">Quote Expiry Date</label>
              <Input
                type="date"
                value={expiryDate}
                onChange={(e) => setExpiryDate(e.target.value)}
              />
            </div>

            <div className="space-y-2">
              <label className="text-xs font-semibold text-muted-foreground">Payment Terms (Days)</label>
              <Input
                type="number"
                min="0"
                value={paymentTermsDays}
                onChange={(e) => setPaymentTermsDays(parseInt(e.target.value, 10) || 30)}
              />
            </div>

            <div className="space-y-2">
              <label className="text-xs font-semibold text-muted-foreground">Template Preset</label>
              <select
                className="w-full h-10 px-3 rounded-lg border bg-background text-sm"
                value={templateId}
                onChange={(e) => applyTemplate(e.target.value)}
              >
                <option value="">-- Manual Configuration --</option>
                {templates.map((t) => (
                  <option key={t.id} value={t.id}>
                    {t.name} ({t.validity_days} days)
                  </option>
                ))}
              </select>
            </div>
          </div>

          {/* Line items section */}
          <div className="space-y-3 pt-3 border-t">
            <div className="flex items-center justify-between">
              <div>
                <h4 className="text-sm font-semibold">Product Lines</h4>
                <p className="text-xs text-muted-foreground">
                  Include software seats, brand variants (+560.00 for Odoo brand), and add-ons.
                </p>
              </div>
              <Button
                type="button"
                variant="outline"
                size="sm"
                onClick={addLineRow}
                className="cursor-pointer"
              >
                <Plus className="h-3.5 w-3.5 mr-1" />
                Add Line
              </Button>
            </div>

            <div className="space-y-2 max-h-48 overflow-y-auto pr-1">
              {lines.map((l, idx) => {
                const prod = products.find((p) => p.id === l.productId);
                return (
                  <div key={idx} className="flex items-center gap-2">
                    <select
                      className="flex-1 h-9 px-2.5 rounded-lg border bg-background text-xs"
                      value={l.productId}
                      onChange={(e) => updateLineRow(idx, 'productId', e.target.value)}
                    >
                      {products.map((p) => (
                        <option key={p.id} value={p.id}>
                          {p.name} ({formatCents(p.sales_price_cents)})
                        </option>
                      ))}
                    </select>

                    <select
                      className="w-48 h-9 px-2.5 rounded-lg border bg-background text-xs"
                      value={l.variantId}
                      onChange={(e) => updateLineRow(idx, 'variantId', e.target.value)}
                    >
                      <option value="">Standard Edition (+$0.00)</option>
                      {prod?.variants?.map((v) => (
                        <option key={v.id} value={v.id}>
                          {v.name} (+{formatCents(v.price_delta_cents)})
                        </option>
                      ))}
                    </select>

                    <Input
                      type="number"
                      min="1"
                      className="w-16 h-9 font-mono text-center text-xs"
                      value={l.quantity}
                      onChange={(e) =>
                        updateLineRow(idx, 'quantity', parseInt(e.target.value, 10) || 1)
                      }
                    />

                    <Button
                      type="button"
                      variant="ghost"
                      size="sm"
                      onClick={() => removeLineRow(idx)}
                      className="text-muted-foreground hover:text-destructive h-9 w-9 p-0 cursor-pointer"
                    >
                      <XCircle className="h-4 w-4" />
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
            <Button type="submit" disabled={building}>
              {building ? 'Saving...' : 'Create Offer'}
            </Button>
          </DialogFooter>
        </form>
      </Dialog>

      {/* CREATE TEMPLATE MODAL */}
      <Dialog open={isTemplateModalOpen} onOpenChange={setIsTemplateModalOpen}>
        <DialogHeader>
          <DialogTitle>Create Quotation Template</DialogTitle>
          <DialogDescription>
            Save reusable line sets and validity windows so sales reps can quote in seconds.
          </DialogDescription>
        </DialogHeader>

        <form onSubmit={handleCreateTemplate} className="space-y-4">
          <div className="space-y-2">
            <label className="text-xs font-semibold text-muted-foreground">Template Name</label>
            <Input
              required
              placeholder="e.g. Mid-Market Starter Bundle"
              value={tmplName}
              onChange={(e) => setTmplName(e.target.value)}
            />
          </div>

          <div className="grid grid-cols-2 gap-4">
            <div className="space-y-2">
              <label className="text-xs font-semibold text-muted-foreground">Validity Days</label>
              <Input
                type="number"
                min="1"
                required
                value={tmplValidityDays}
                onChange={(e) => setTmplValidityDays(parseInt(e.target.value, 10) || 30)}
              />
            </div>

            <div className="space-y-2">
              <label className="text-xs font-semibold text-muted-foreground">Default Plan</label>
              <select
                className="w-full h-10 px-3 rounded-lg border bg-background text-sm"
                value={tmplPlanId}
                onChange={(e) => setTmplPlanId(e.target.value)}
              >
                {plans.map((p) => (
                  <option key={p.id} value={p.id}>
                    {p.name}
                  </option>
                ))}
              </select>
            </div>
          </div>

          <DialogFooter>
            <Button type="button" variant="outline" onClick={() => setIsTemplateModalOpen(false)}>
              Cancel
            </Button>
            <Button type="submit">Create Template</Button>
          </DialogFooter>
        </form>
      </Dialog>
    </div>
  );
}
