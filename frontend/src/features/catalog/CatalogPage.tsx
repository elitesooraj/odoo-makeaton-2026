import * as React from 'react';
import {
  Package,
  Layers,
  Plus,
  Trash2,
  Calendar,
  AlertCircle,
  Tag,
  Clock,
  Sparkles,
} from 'lucide-react';
import { catalogApi } from '@/api/catalog';
import { Plan, Product, BillingInterval } from '@/api/types';
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

export function CatalogPage() {
  const { toast } = useToast();

  const [activeTab, setActiveTab] = React.useState('products');
  const [products, setProducts] = React.useState<Product[]>([]);
  const [plans, setPlans] = React.useState<Plan[]>([]);
  const [loading, setLoading] = React.useState(false);

  // Modals
  const [isProductModalOpen, setIsProductModalOpen] = React.useState(false);
  const [isPlanModalOpen, setIsPlanModalOpen] = React.useState(false);

  // Product Form state
  const [prodName, setProdName] = React.useState('');
  const [prodType, setProdType] = React.useState('SOFTWARE');
  const [prodSalesPrice, setProdSalesPrice] = React.useState('1200.00');
  const [prodCostPrice, setProdCostPrice] = React.useState('350.00');
  const [prodNotes, setProdNotes] = React.useState('');
  const [variants, setVariants] = React.useState<{ name: string; delta: string }[]>([
    { name: 'Odoo Connected Brand', delta: '560.00' },
  ]);

  // Plan Form state
  const [planProductId, setPlanProductId] = React.useState('');
  const [planName, setPlanName] = React.useState('');
  const [planInterval, setPlanInterval] = React.useState<BillingInterval>('MONTHLY');
  const [planPrice, setPlanPrice] = React.useState('1200.00');
  const [planMinQty, setPlanMinQty] = React.useState('1');
  const [planAvailableFrom, setPlanAvailableFrom] = React.useState('');
  const [planAvailableUntil, setPlanAvailableUntil] = React.useState('');
  const [planAutoClose, setPlanAutoClose] = React.useState(false);
  const [planAllowEarlyClose, setPlanAllowEarlyClose] = React.useState(true);
  const [planAllowPause, setPlanAllowPause] = React.useState(true);
  const [planAllowRenewal, setPlanAllowRenewal] = React.useState(true);

  const [saving, setSaving] = React.useState(false);

  const fetchData = React.useCallback(async () => {
    setLoading(true);
    try {
      const [prodRes, planRes] = await Promise.all([
        catalogApi.listProducts(),
        catalogApi.listPlans(),
      ]);
      setProducts(prodRes.data || []);
      setPlans(planRes.data || []);
      if (prodRes.data?.length && !planProductId) {
        setPlanProductId(prodRes.data[0].id);
      }
    } catch {
      toast({
        title: 'Catalog Error',
        description: 'Failed to retrieve catalog items',
        variant: 'destructive',
      });
    } finally {
      setLoading(false);
    }
  }, [planProductId, toast]);

  React.useEffect(() => {
    fetchData();
  }, [fetchData]);

  // Product actions
  const addVariantRow = () => {
    setVariants((prev) => [...prev, { name: '', delta: '0.00' }]);
  };

  const removeVariantRow = (index: number) => {
    setVariants((prev) => prev.filter((_, i) => i !== index));
  };

  const updateVariantRow = (index: number, field: 'name' | 'delta', value: string) => {
    setVariants((prev) =>
      prev.map((v, i) => (i === index ? { ...v, [field]: value } : v))
    );
  };

  const handleCreateProduct = async (e: React.FormEvent) => {
    e.preventDefault();
    setSaving(true);
    try {
      const formattedVariants = variants
        .filter((v) => v.name.trim() !== '')
        .map((v) => ({
          name: v.name.trim(),
          price_delta_cents: dollarsToCents(v.delta),
        }));

      await catalogApi.createProduct({
        name: prodName,
        type: prodType,
        sales_price_cents: dollarsToCents(prodSalesPrice),
        cost_price_cents: dollarsToCents(prodCostPrice),
        notes: prodNotes || undefined,
        variants: formattedVariants,
      });

      toast({
        title: 'Product Launched',
        description: `${prodName} with ${formattedVariants.length} variants added to catalog.`,
        variant: 'success',
      });
      setIsProductModalOpen(false);
      fetchData();
    } catch (err: any) {
      toast({
        title: 'Error Creating Product',
        description: err.message || 'Validation failed',
        variant: 'destructive',
      });
    } finally {
      setSaving(false);
    }
  };

  const handleCreatePlan = async (e: React.FormEvent) => {
    e.preventDefault();
    setSaving(true);
    try {
      await catalogApi.createPlan({
        product_id: planProductId,
        name: planName,
        billing_interval: planInterval,
        price_cents: dollarsToCents(planPrice),
        min_quantity: parseInt(planMinQty, 10) || 1,
        available_from: planAvailableFrom || null,
        available_until: planAvailableUntil || null,
        auto_close: planAutoClose,
        allow_early_close: planAllowEarlyClose,
        allow_pause: planAllowPause,
        allow_renewal: planAllowRenewal,
        status: 'ACTIVE',
      });

      toast({
        title: 'Plan Configured',
        description: `${planName} successfully registered.`,
        variant: 'success',
      });
      setIsPlanModalOpen(false);
      fetchData();
    } catch (err: any) {
      toast({
        title: 'Error Creating Plan',
        description: err.message || 'Validation failed',
        variant: 'destructive',
      });
    } finally {
      setSaving(false);
    }
  };

  return (
    <div className="space-y-6">
      <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-4">
        <div>
          <h1 className="text-2xl font-bold tracking-tight">Product & Plan Catalog</h1>
          <p className="text-sm text-muted-foreground">
            Configure subscription offerings, additive variants (e.g. Odoo brand delta), and recurrence rules.
          </p>
        </div>

        <div className="flex items-center gap-2">
          <PermissionGate permission="products.write">
            <Button
              onClick={() => setIsProductModalOpen(true)}
              variant="outline"
              className="cursor-pointer"
            >
              <Package className="h-4 w-4 mr-2" />
              Add Product
            </Button>
          </PermissionGate>

          <PermissionGate permission="plans.write">
            <Button onClick={() => setIsPlanModalOpen(true)} className="cursor-pointer">
              <Layers className="h-4 w-4 mr-2" />
              New Plan
            </Button>
          </PermissionGate>
        </div>
      </div>

      <Tabs value={activeTab} onValueChange={setActiveTab}>
        <TabsList>
          <TabsTrigger value="products" className="flex items-center gap-2">
            <Package className="h-4 w-4" />
            Products & Variants ({products.length})
          </TabsTrigger>
          <TabsTrigger value="plans" className="flex items-center gap-2">
            <Layers className="h-4 w-4" />
            Subscription Plans ({plans.length})
          </TabsTrigger>
        </TabsList>

        {/* Products Tab */}
        <TabsContent value="products">
          <Card>
            <CardContent className="p-0">
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>Product Name</TableHead>
                    <TableHead>Type</TableHead>
                    <TableHead>Base Sales Price</TableHead>
                    <TableHead>Cost Price</TableHead>
                    <TableHead>Configured Variants</TableHead>
                    <TableHead>Status</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {products.length === 0 ? (
                    <TableRow>
                      <TableCell colSpan={6} className="h-32 text-center text-muted-foreground">
                        No products available in catalog.
                      </TableCell>
                    </TableRow>
                  ) : (
                    products.map((p) => (
                      <TableRow key={p.id}>
                        <TableCell>
                          <div className="font-semibold">{p.name}</div>
                          {p.notes && <div className="text-xs text-muted-foreground mt-0.5">{p.notes}</div>}
                        </TableCell>
                        <TableCell>
                          <Badge variant="outline" className="font-mono text-[11px]">
                            {p.type}
                          </Badge>
                        </TableCell>
                        <TableCell className="font-mono font-medium text-emerald-400">
                          {formatCents(p.sales_price_cents)}
                        </TableCell>
                        <TableCell className="font-mono text-muted-foreground text-xs">
                          {formatCents(p.cost_price_cents)}
                        </TableCell>
                        <TableCell>
                          {p.variants && p.variants.length > 0 ? (
                            <div className="flex flex-wrap gap-1.5">
                              {p.variants.map((v) => (
                                <span
                                  key={v.id}
                                  className="inline-flex items-center gap-1 text-[11px] px-2 py-0.5 rounded bg-muted/60 border text-muted-foreground"
                                >
                                  <Tag className="h-3 w-3 text-primary" />
                                  <span>{v.name}</span>
                                  {v.price_delta_cents !== 0 && (
                                    <span className="font-mono text-primary font-semibold">
                                      +{formatCents(v.price_delta_cents)}
                                    </span>
                                  )}
                                </span>
                              ))}
                            </div>
                          ) : (
                            <span className="text-xs text-muted-foreground italic">Standard only</span>
                          )}
                        </TableCell>
                        <TableCell>
                          <Badge variant={p.is_active ? 'success' : 'secondary'}>
                            {p.is_active ? 'ACTIVE' : 'ARCHIVED'}
                          </Badge>
                        </TableCell>
                      </TableRow>
                    ))
                  )}
                </TableBody>
              </Table>
            </CardContent>
          </Card>
        </TabsContent>

        {/* Plans Tab */}
        <TabsContent value="plans">
          <Card>
            <CardContent className="p-0">
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>Plan Name</TableHead>
                    <TableHead>Parent Product</TableHead>
                    <TableHead>Recurrence Interval</TableHead>
                    <TableHead>Recurring Price</TableHead>
                    <TableHead>Min Qty</TableHead>
                    <TableHead>Behavioral Policies</TableHead>
                    <TableHead>Status</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {plans.length === 0 ? (
                    <TableRow>
                      <TableCell colSpan={7} className="h-32 text-center text-muted-foreground">
                        No subscription plans configured yet.
                      </TableCell>
                    </TableRow>
                  ) : (
                    plans.map((plan) => (
                      <TableRow key={plan.id}>
                        <TableCell className="font-semibold">{plan.name}</TableCell>
                        <TableCell className="text-xs text-muted-foreground">
                          {plan.product_name || 'Core Product'}
                        </TableCell>
                        <TableCell>
                          <Badge variant="outline" className="font-mono text-xs flex items-center w-fit gap-1">
                            <Clock className="h-3 w-3" />
                            {plan.billing_interval}
                          </Badge>
                        </TableCell>
                        <TableCell className="font-mono font-medium text-emerald-400">
                          {formatCents(plan.price_cents)}
                        </TableCell>
                        <TableCell className="text-xs font-mono">{plan.min_quantity}</TableCell>
                        <TableCell>
                          <div className="flex flex-wrap gap-1 text-[10px]">
                            {plan.auto_close && (
                              <span className="px-1.5 py-0.5 rounded bg-amber-500/15 text-amber-400 border border-amber-500/30">
                                Auto-close
                              </span>
                            )}
                            {plan.allow_early_close && (
                              <span className="px-1.5 py-0.5 rounded bg-sky-500/15 text-sky-400 border border-sky-500/30">
                                Early Close
                              </span>
                            )}
                            {plan.allow_pause && (
                              <span className="px-1.5 py-0.5 rounded bg-purple-500/15 text-purple-400 border border-purple-500/30">
                                Pause/Resume
                              </span>
                            )}
                            {plan.allow_renewal && (
                              <span className="px-1.5 py-0.5 rounded bg-emerald-500/15 text-emerald-400 border border-emerald-500/30">
                                Auto-renew
                              </span>
                            )}
                          </div>
                        </TableCell>
                        <TableCell>
                          <Badge variant={plan.status === 'ACTIVE' ? 'success' : 'secondary'}>
                            {plan.status}
                          </Badge>
                        </TableCell>
                      </TableRow>
                    ))
                  )}
                </TableBody>
              </Table>
            </CardContent>
          </Card>
        </TabsContent>
      </Tabs>

      {/* Create Product Modal with Dynamic Variant Matrix */}
      <Dialog open={isProductModalOpen} onOpenChange={setIsProductModalOpen}>
        <form onSubmit={handleCreateProduct} className="space-y-4">
          <DialogHeader>
            <DialogTitle>Launch New Product</DialogTitle>
            <DialogDescription>
              Add a core offering to Brightloop. Variants can have additive price deltas.
            </DialogDescription>
          </DialogHeader>

          <div className="space-y-3 text-xs">
            <div className="space-y-1">
              <label className="font-medium text-muted-foreground">Product Name *</label>
              <Input
                required
                value={prodName}
                onChange={(e) => setProdName(e.target.value)}
                placeholder="e.g. Brightloop ERP"
              />
            </div>

            <div className="grid grid-cols-3 gap-3">
              <div className="space-y-1">
                <label className="font-medium text-muted-foreground">Product Type</label>
                <select
                  value={prodType}
                  onChange={(e) => setProdType(e.target.value)}
                  className="flex h-9 w-full rounded-md border border-input bg-background px-3 py-1 text-sm shadow-sm"
                >
                  <option value="SOFTWARE">SOFTWARE</option>
                  <option value="SERVICE">SERVICE</option>
                  <option value="ADDON">ADDON</option>
                </select>
              </div>

              <div className="space-y-1">
                <label className="font-medium text-muted-foreground">Sales Price ($)</label>
                <Input
                  required
                  type="number"
                  step="0.01"
                  value={prodSalesPrice}
                  onChange={(e) => setProdSalesPrice(e.target.value)}
                />
              </div>

              <div className="space-y-1">
                <label className="font-medium text-muted-foreground">Cost Price ($)</label>
                <Input
                  required
                  type="number"
                  step="0.01"
                  value={prodCostPrice}
                  onChange={(e) => setProdCostPrice(e.target.value)}
                />
              </div>
            </div>

            <div className="space-y-1">
              <label className="font-medium text-muted-foreground">Notes / Description</label>
              <Input
                value={prodNotes}
                onChange={(e) => setProdNotes(e.target.value)}
                placeholder="Operational notes, SLA specifications"
              />
            </div>

            {/* Variants Builder */}
            <div className="pt-2 border-t space-y-2">
              <div className="flex items-center justify-between">
                <div>
                  <span className="font-semibold text-muted-foreground">Product Variants</span>
                  <p className="text-[11px] text-muted-foreground">
                    e.g. Brand delta: "Odoo" adds $560.00 to sales price
                  </p>
                </div>
                <Button
                  type="button"
                  variant="outline"
                  size="sm"
                  onClick={addVariantRow}
                  className="h-7 text-xs"
                >
                  <Plus className="h-3 w-3 mr-1" /> Add Variant
                </Button>
              </div>

              {variants.map((v, idx) => (
                <div key={idx} className="flex items-center gap-2">
                  <Input
                    placeholder="Variant name (e.g. Odoo brand)"
                    value={v.name}
                    onChange={(e) => updateVariantRow(idx, 'name', e.target.value)}
                    className="flex-1"
                  />
                  <div className="w-36 flex items-center gap-1">
                    <span className="text-muted-foreground font-mono">+ $</span>
                    <Input
                      type="number"
                      step="0.01"
                      placeholder="560.00"
                      value={v.delta}
                      onChange={(e) => updateVariantRow(idx, 'delta', e.target.value)}
                      className="font-mono"
                    />
                  </div>
                  <Button
                    type="button"
                    variant="ghost"
                    size="sm"
                    onClick={() => removeVariantRow(idx)}
                    className="h-9 w-9 p-0 text-rose-400"
                  >
                    <Trash2 className="h-3.5 w-3.5" />
                  </Button>
                </div>
              ))}
            </div>
          </div>

          <DialogFooter>
            <Button type="button" variant="outline" onClick={() => setIsProductModalOpen(false)}>
              Cancel
            </Button>
            <Button type="submit" disabled={saving || !prodName.trim()}>
              {saving ? 'Launching...' : 'Create Product'}
            </Button>
          </DialogFooter>
        </form>
      </Dialog>

      {/* Create Plan Modal */}
      <Dialog open={isPlanModalOpen} onOpenChange={setIsPlanModalOpen}>
        <form onSubmit={handleCreatePlan} className="space-y-4">
          <DialogHeader>
            <DialogTitle>Create Subscription Plan</DialogTitle>
            <DialogDescription>
              Define billing frequency, price, minimum quantities, and behavioral flags.
            </DialogDescription>
          </DialogHeader>

          <div className="space-y-3 text-xs">
            <div className="space-y-1">
              <label className="font-medium text-muted-foreground">Parent Product *</label>
              <select
                value={planProductId}
                onChange={(e) => setPlanProductId(e.target.value)}
                className="flex h-9 w-full rounded-md border border-input bg-background px-3 py-1 text-sm shadow-sm"
              >
                {products.map((p) => (
                  <option key={p.id} value={p.id}>
                    {p.name} ({formatCents(p.sales_price_cents)})
                  </option>
                ))}
              </select>
            </div>

            <div className="space-y-1">
              <label className="font-medium text-muted-foreground">Plan Name *</label>
              <Input
                required
                value={planName}
                onChange={(e) => setPlanName(e.target.value)}
                placeholder="e.g. Enterprise Monthly"
              />
            </div>

            <div className="grid grid-cols-3 gap-3">
              <div className="space-y-1">
                <label className="font-medium text-muted-foreground">Billing Interval</label>
                <select
                  value={planInterval}
                  onChange={(e) => setPlanInterval(e.target.value as any)}
                  className="flex h-9 w-full rounded-md border border-input bg-background px-3 py-1 text-sm shadow-sm"
                >
                  <option value="DAILY">DAILY</option>
                  <option value="WEEKLY">WEEKLY</option>
                  <option value="MONTHLY">MONTHLY</option>
                  <option value="YEARLY">YEARLY</option>
                </select>
              </div>

              <div className="space-y-1">
                <label className="font-medium text-muted-foreground">Price per Period ($)</label>
                <Input
                  required
                  type="number"
                  step="0.01"
                  value={planPrice}
                  onChange={(e) => setPlanPrice(e.target.value)}
                />
              </div>

              <div className="space-y-1">
                <label className="font-medium text-muted-foreground">Min Quantity</label>
                <Input
                  type="number"
                  min="1"
                  value={planMinQty}
                  onChange={(e) => setPlanMinQty(e.target.value)}
                />
              </div>
            </div>

            <div className="grid grid-cols-2 gap-3">
              <div className="space-y-1">
                <label className="font-medium text-muted-foreground">Available From</label>
                <Input
                  type="date"
                  value={planAvailableFrom}
                  onChange={(e) => setPlanAvailableFrom(e.target.value)}
                />
              </div>
              <div className="space-y-1">
                <label className="font-medium text-muted-foreground">Available Until</label>
                <Input
                  type="date"
                  value={planAvailableUntil}
                  onChange={(e) => setPlanAvailableUntil(e.target.value)}
                />
              </div>
            </div>

            {/* Behavioral flags checkboxes */}
            <div className="pt-2 border-t space-y-2">
              <span className="font-semibold text-muted-foreground">Plan Policies</span>
              <div className="grid grid-cols-2 gap-2 text-xs">
                <label className="flex items-center gap-2 cursor-pointer p-2 rounded bg-muted/40 border">
                  <input
                    type="checkbox"
                    checked={planAutoClose}
                    onChange={(e) => setPlanAutoClose(e.target.checked)}
                    className="rounded text-primary"
                  />
                  <span>Auto-close when expired</span>
                </label>

                <label className="flex items-center gap-2 cursor-pointer p-2 rounded bg-muted/40 border">
                  <input
                    type="checkbox"
                    checked={planAllowEarlyClose}
                    onChange={(e) => setPlanAllowEarlyClose(e.target.checked)}
                    className="rounded text-primary"
                  />
                  <span>Allow early termination</span>
                </label>

                <label className="flex items-center gap-2 cursor-pointer p-2 rounded bg-muted/40 border">
                  <input
                    type="checkbox"
                    checked={planAllowPause}
                    onChange={(e) => setPlanAllowPause(e.target.checked)}
                    className="rounded text-primary"
                  />
                  <span>Allow subscription pause</span>
                </label>

                <label className="flex items-center gap-2 cursor-pointer p-2 rounded bg-muted/40 border">
                  <input
                    type="checkbox"
                    checked={planAllowRenewal}
                    onChange={(e) => setPlanAllowRenewal(e.target.checked)}
                    className="rounded text-primary"
                  />
                  <span>Allow renewal</span>
                </label>
              </div>
            </div>
          </div>

          <DialogFooter>
            <Button type="button" variant="outline" onClick={() => setIsPlanModalOpen(false)}>
              Cancel
            </Button>
            <Button type="submit" disabled={saving || !planName.trim()}>
              {saving ? 'Configuring...' : 'Save Plan'}
            </Button>
          </DialogFooter>
        </form>
      </Dialog>
    </div>
  );
}

