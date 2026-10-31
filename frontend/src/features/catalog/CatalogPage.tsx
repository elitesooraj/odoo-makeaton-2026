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
  Percent,
  DollarSign,
  ShieldCheck,
  CheckCircle2,
} from 'lucide-react';
import { catalogApi } from '@/api/catalog';
import { BillingInterval, Discount, Plan, Product, Tax } from '@/api/types';
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
  const [taxes, setTaxes] = React.useState<Tax[]>([]);
  const [discounts, setDiscounts] = React.useState<Discount[]>([]);
  const [loading, setLoading] = React.useState(false);

  // Modals
  const [isProductModalOpen, setIsProductModalOpen] = React.useState(false);
  const [isPlanModalOpen, setIsPlanModalOpen] = React.useState(false);
  const [isTaxModalOpen, setIsTaxModalOpen] = React.useState(false);
  const [isDiscountModalOpen, setIsDiscountModalOpen] = React.useState(false);

  // Product Form state
  const [prodName, setProdName] = React.useState('');
  const [prodType, setProdType] = React.useState<'SERVICE' | 'GOODS'>('SERVICE');
  const [prodSalesPrice, setProdSalesPrice] = React.useState('1200.00');
  const [prodCostPrice, setProdCostPrice] = React.useState('350.00');
  const [prodNotes, setProdNotes] = React.useState('');
  const [variants, setVariants] = React.useState<{ name: string; delta: string }[]>([
    { name: 'Odoo Connected Brand', delta: '560.00' },
  ]);

  // Plan Form state
  const [planProductId, setPlanProductId] = React.useState('');
  const [planTaxId, setPlanTaxId] = React.useState('');
  const [planName, setPlanName] = React.useState('');
  const [planInterval, setPlanInterval] = React.useState<BillingInterval>('MONTHLY');
  const [planPrice, setPlanPrice] = React.useState('1200.00');
  const [planMinQty, setPlanMinQty] = React.useState('1');
  const [planAvailableFrom, setPlanAvailableFrom] = React.useState('');
  const [planAvailableTo, setPlanAvailableTo] = React.useState('');
  const [planAutoClose, setPlanAutoClose] = React.useState(false);
  const [planAllowEarlyClose, setPlanAllowEarlyClose] = React.useState(true);
  const [planAllowPause, setPlanAllowPause] = React.useState(true);
  const [planAutoRenew, setPlanAutoRenew] = React.useState(true);

  // Tax Form state
  const [taxName, setTaxName] = React.useState('');
  const [taxRatePercent, setTaxRatePercent] = React.useState('18.00'); // 18% -> 1800 bp
  const [taxInclusive, setTaxInclusive] = React.useState(false);
  const [taxCountryCode, setTaxCountryCode] = React.useState('US');

  // Discount Form state
  const [discName, setDiscName] = React.useState('');
  const [discKind, setDiscKind] = React.useState<'PERCENT' | 'FIXED'>('PERCENT');
  const [discValue, setDiscValue] = React.useState('10'); // 10% or $10
  const [discScope, setDiscScope] = React.useState<'ORDER' | 'PRODUCT'>('ORDER');
  const [discProductId, setDiscProductId] = React.useState('');
  const [discMinPurchase, setDiscMinPurchase] = React.useState('500.00');
  const [discMinQty, setDiscMinQty] = React.useState('1');
  const [discMaxUses, setDiscMaxUses] = React.useState('100');

  const [saving, setSaving] = React.useState(false);

  const fetchData = React.useCallback(async () => {
    setLoading(true);
    try {
      const [prodRes, planRes, taxRes, discRes] = await Promise.all([
        catalogApi.listProducts(),
        catalogApi.listPlans(),
        catalogApi.listTaxes(),
        catalogApi.listDiscounts(),
      ]);
      setProducts(prodRes.data || []);
      setPlans(planRes.data || []);
      setTaxes(taxRes.data || []);
      setDiscounts(discRes.data || []);
      if (prodRes.data?.length && !planProductId) {
        setPlanProductId(prodRes.data[0].id);
      }
      if (taxRes.data?.length && !planTaxId) {
        setPlanTaxId(taxRes.data[0].id);
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
  }, [planProductId, planTaxId, toast]);

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
      setProdName('');
      setProdNotes('');
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
        tax_id: planTaxId || null,
        name: planName,
        interval: planInterval,
        price_cents: dollarsToCents(planPrice),
        minimum_quantity: parseInt(planMinQty, 10) || 1,
        available_from: planAvailableFrom || null,
        available_to: planAvailableTo || null,
        auto_close: planAutoClose,
        auto_renew: planAutoRenew,
        allow_pause: planAllowPause,
        allow_early_close: planAllowEarlyClose,
      });

      toast({
        title: 'Plan Configured',
        description: `${planName} successfully registered.`,
        variant: 'success',
      });
      setIsPlanModalOpen(false);
      setPlanName('');
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

  const handleCreateTax = async (e: React.FormEvent) => {
    e.preventDefault();
    setSaving(true);
    try {
      const rateBp = Math.round(parseFloat(taxRatePercent) * 100);
      await catalogApi.createTax({
        name: taxName,
        rate_bp: rateBp,
        is_inclusive: taxInclusive,
        country_code: taxCountryCode,
      });

      toast({
        title: 'Tax Rule Created',
        description: `${taxName} (${taxRatePercent}%) active.`,
        variant: 'success',
      });
      setIsTaxModalOpen(false);
      setTaxName('');
      fetchData();
    } catch (err: any) {
      toast({
        title: 'Error Creating Tax Rule',
        description: err.message || 'Validation failed',
        variant: 'destructive',
      });
    } finally {
      setSaving(false);
    }
  };

  const handleCreateDiscount = async (e: React.FormEvent) => {
    e.preventDefault();
    setSaving(true);
    try {
      const isPercent = discKind === 'PERCENT';
      await catalogApi.createDiscount({
        name: discName,
        kind: discKind,
        value_bp: isPercent ? Math.round(parseFloat(discValue) * 100) : 0,
        value_cents: isPercent ? 0 : dollarsToCents(discValue),
        scope: discScope,
        product_id: discScope === 'PRODUCT' ? discProductId || null : null,
        min_purchase_cents: dollarsToCents(discMinPurchase),
        min_quantity: parseInt(discMinQty, 10) || 1,
        max_uses: discMaxUses ? parseInt(discMaxUses, 10) : null,
      });

      toast({
        title: 'Discount Rule Created',
        description: `${discName} active for orders.`,
        variant: 'success',
      });
      setIsDiscountModalOpen(false);
      setDiscName('');
      fetchData();
    } catch (err: any) {
      toast({
        title: 'Error Creating Discount',
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
          <h1 className="text-2xl font-bold tracking-tight">Catalog & Pricing Rules</h1>
          <p className="text-sm text-muted-foreground">
            Configure subscription offerings, additive variants (e.g. Odoo brand delta), taxes, and discount coupons.
          </p>
        </div>

        <div className="flex flex-wrap items-center gap-2">
          <PermissionGate permission="products.write">
            <Button
              onClick={() => setIsProductModalOpen(true)}
              variant="outline"
              size="sm"
              className="cursor-pointer"
            >
              <Package className="h-4 w-4 mr-1.5" />
              Add Product
            </Button>
          </PermissionGate>

          <PermissionGate permission="plans.write">
            <Button
              onClick={() => setIsPlanModalOpen(true)}
              variant="outline"
              size="sm"
              className="cursor-pointer"
            >
              <Layers className="h-4 w-4 mr-1.5" />
              New Plan
            </Button>
          </PermissionGate>

          <PermissionGate permission="taxes.write">
            <Button
              onClick={() => setIsTaxModalOpen(true)}
              variant="outline"
              size="sm"
              className="cursor-pointer"
            >
              <DollarSign className="h-4 w-4 mr-1.5" />
              Tax Rule
            </Button>
          </PermissionGate>

          <PermissionGate permission="discounts.write">
            <Button
              onClick={() => setIsDiscountModalOpen(true)}
              size="sm"
              className="cursor-pointer"
            >
              <Percent className="h-4 w-4 mr-1.5" />
              Discount Rule
            </Button>
          </PermissionGate>
        </div>
      </div>

      <Tabs value={activeTab} onValueChange={setActiveTab}>
        <TabsList className="grid grid-cols-2 sm:grid-cols-4 w-full sm:w-auto">
          <TabsTrigger value="products" className="flex items-center gap-2">
            <Package className="h-4 w-4" />
            Products ({products.length})
          </TabsTrigger>
          <TabsTrigger value="plans" className="flex items-center gap-2">
            <Layers className="h-4 w-4" />
            Plans ({plans.length})
          </TabsTrigger>
          <TabsTrigger value="taxes" className="flex items-center gap-2">
            <DollarSign className="h-4 w-4" />
            Taxes ({taxes.length})
          </TabsTrigger>
          <TabsTrigger value="discounts" className="flex items-center gap-2">
            <Percent className="h-4 w-4" />
            Discounts ({discounts.length})
          </TabsTrigger>
        </TabsList>

        {/* PRODUCTS TAB */}
        <TabsContent value="products" className="mt-4 space-y-4">
          <Card>
            <CardContent className="p-0">
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>Product Name</TableHead>
                    <TableHead>Type</TableHead>
                    <TableHead>Base Sales Price</TableHead>
                    <TableHead>Cost Price</TableHead>
                    <TableHead>Variants / Add-ons</TableHead>
                    <TableHead>Status</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {loading && products.length === 0 ? (
                    <TableRow>
                      <TableCell colSpan={6} className="text-center py-8 text-muted-foreground">
                        Loading products...
                      </TableCell>
                    </TableRow>
                  ) : products.length === 0 ? (
                    <TableRow>
                      <TableCell colSpan={6} className="text-center py-8 text-muted-foreground">
                        No products configured yet.
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
                          <Badge variant={p.type === 'GOODS' ? 'secondary' : 'outline'} className="text-xs">
                            {p.type}
                          </Badge>
                        </TableCell>
                        <TableCell className="font-mono font-medium">
                          {formatCents(p.sales_price_cents)}
                        </TableCell>
                        <TableCell className="font-mono text-muted-foreground">
                          {formatCents(p.cost_price_cents)}
                        </TableCell>
                        <TableCell>
                          {p.variants && p.variants.length > 0 ? (
                            <div className="flex flex-wrap gap-1.5">
                              {p.variants.map((v) => (
                                <Badge
                                  key={v.id}
                                  variant="secondary"
                                  className="text-[11px] font-mono py-0 px-2 bg-muted/60"
                                >
                                  {v.name}
                                  {v.price_delta_cents > 0
                                    ? ` (+${formatCents(v.price_delta_cents)})`
                                    : ''}
                                </Badge>
                              ))}
                            </div>
                          ) : (
                            <span className="text-xs text-muted-foreground">No variants</span>
                          )}
                        </TableCell>
                        <TableCell>
                          <Badge variant={p.is_active ? 'success' : 'secondary'}>
                            {p.is_active ? 'Active' : 'Inactive'}
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

        {/* PLANS TAB */}
        <TabsContent value="plans" className="mt-4 space-y-4">
          <Card>
            <CardContent className="p-0">
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>Plan Name</TableHead>
                    <TableHead>Associated Product</TableHead>
                    <TableHead>Billing Cadence</TableHead>
                    <TableHead>Price / Interval</TableHead>
                    <TableHead>Min Quantity</TableHead>
                    <TableHead>Lifecycle Rules</TableHead>
                    <TableHead>Status</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {loading && plans.length === 0 ? (
                    <TableRow>
                      <TableCell colSpan={7} className="text-center py-8 text-muted-foreground">
                        Loading plans...
                      </TableCell>
                    </TableRow>
                  ) : plans.length === 0 ? (
                    <TableRow>
                      <TableCell colSpan={7} className="text-center py-8 text-muted-foreground">
                        No subscription plans created yet.
                      </TableCell>
                    </TableRow>
                  ) : (
                    plans.map((pl) => (
                      <TableRow key={pl.id}>
                        <TableCell>
                          <div className="font-semibold">{pl.name}</div>
                          {pl.available_to && (
                            <div className="text-xs text-muted-foreground flex items-center gap-1 mt-0.5">
                              <Calendar className="h-3 w-3" />
                              Valid until {formatDate(pl.available_to)}
                            </div>
                          )}
                        </TableCell>
                        <TableCell className="text-sm">
                          {pl.product_name || products.find((pr) => pr.id === pl.product_id)?.name || 'Product'}
                        </TableCell>
                        <TableCell>
                          <Badge variant="outline" className="font-mono text-xs">
                            <Clock className="h-3 w-3 mr-1" />
                            {pl.interval || pl.billing_interval}
                          </Badge>
                        </TableCell>
                        <TableCell className="font-mono font-medium text-emerald-400">
                          {formatCents(pl.price_cents)}
                        </TableCell>
                        <TableCell className="font-mono text-sm">
                          {pl.minimum_quantity || pl.min_quantity || 1}
                        </TableCell>
                        <TableCell>
                          <div className="flex flex-wrap gap-1 text-[11px]">
                            {pl.auto_renew && (
                              <Badge variant="outline" className="bg-emerald-500/10 text-emerald-400 border-emerald-500/20">
                                Auto-Renew
                              </Badge>
                            )}
                            {pl.auto_close && (
                              <Badge variant="outline" className="bg-amber-500/10 text-amber-400 border-amber-500/20">
                                Auto-Close
                              </Badge>
                            )}
                            {pl.allow_pause && (
                              <Badge variant="outline" className="bg-blue-500/10 text-blue-400 border-blue-500/20">
                                Pauseable
                              </Badge>
                            )}
                            {pl.allow_early_close && (
                              <Badge variant="outline" className="bg-purple-500/10 text-purple-400 border-purple-500/20">
                                Early Close
                              </Badge>
                            )}
                          </div>
                        </TableCell>
                        <TableCell>
                          <Badge variant={pl.is_active ? 'success' : 'secondary'}>
                            {pl.is_active ? 'Active' : 'Archived'}
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

        {/* TAXES TAB */}
        <TabsContent value="taxes" className="mt-4 space-y-4">
          <Card>
            <CardContent className="p-0">
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>Tax Name</TableHead>
                    <TableHead>Rate (Basis Points)</TableHead>
                    <TableHead>Calculation</TableHead>
                    <TableHead>Country Code</TableHead>
                    <TableHead>Status</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {taxes.length === 0 ? (
                    <TableRow>
                      <TableCell colSpan={5} className="text-center py-8 text-muted-foreground">
                        No tax rules configured.
                      </TableCell>
                    </TableRow>
                  ) : (
                    taxes.map((t) => (
                      <TableRow key={t.id}>
                        <TableCell className="font-semibold">{t.name}</TableCell>
                        <TableCell className="font-mono text-sm">
                          {(t.rate_bp / 100).toFixed(2)}% ({t.rate_bp} bp)
                        </TableCell>
                        <TableCell>
                          <Badge variant={t.is_inclusive ? 'outline' : 'secondary'}>
                            {t.is_inclusive ? 'Inclusive (Carved Out)' : 'Exclusive (Added On Top)'}
                          </Badge>
                        </TableCell>
                        <TableCell className="font-mono text-xs">{t.country_code || 'Global'}</TableCell>
                        <TableCell>
                          <Badge variant={t.is_active ? 'success' : 'secondary'}>
                            {t.is_active ? 'Active' : 'Inactive'}
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

        {/* DISCOUNTS TAB */}
        <TabsContent value="discounts" className="mt-4 space-y-4">
          <Card>
            <CardContent className="p-0">
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>Discount Name</TableHead>
                    <TableHead>Kind & Value</TableHead>
                    <TableHead>Scope</TableHead>
                    <TableHead>Minimum Requirements</TableHead>
                    <TableHead>Quota / Uses</TableHead>
                    <TableHead>Status</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {discounts.length === 0 ? (
                    <TableRow>
                      <TableCell colSpan={6} className="text-center py-8 text-muted-foreground">
                        No discount rules configured.
                      </TableCell>
                    </TableRow>
                  ) : (
                    discounts.map((d) => (
                      <TableRow key={d.id}>
                        <TableCell className="font-semibold">{d.name}</TableCell>
                        <TableCell className="font-mono font-medium text-emerald-400">
                          {d.kind === 'PERCENT'
                            ? `${(d.value_bp / 100).toFixed(0)}% OFF`
                            : `${formatCents(d.value_cents)} FLAT`}
                        </TableCell>
                        <TableCell>
                          <Badge variant="outline" className="text-xs">
                            {d.scope === 'PRODUCT' ? 'Product Specific' : 'Order Level'}
                          </Badge>
                        </TableCell>
                        <TableCell className="text-xs text-muted-foreground">
                          Min ${d.min_purchase_cents / 100} &bull; Min {d.min_quantity} seat(s)
                        </TableCell>
                        <TableCell className="font-mono text-xs">
                          {d.used_count} used {d.max_uses ? `/ ${d.max_uses} limit` : '(unlimited)'}
                        </TableCell>
                        <TableCell>
                          <Badge variant={d.is_active ? 'success' : 'secondary'}>
                            {d.is_active ? 'Active' : 'Inactive'}
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

      {/* CREATE PRODUCT MODAL */}
      <Dialog open={isProductModalOpen} onOpenChange={setIsProductModalOpen}>
        <DialogHeader>
          <DialogTitle>Launch New Product</DialogTitle>
          <DialogDescription>
            Register a base business software product with additive brand or feature variants.
          </DialogDescription>
        </DialogHeader>

        <form onSubmit={handleCreateProduct} className="space-y-4">
          <div className="grid grid-cols-2 gap-4">
            <div className="space-y-2 col-span-2">
              <label className="text-xs font-semibold text-muted-foreground">Product Name</label>
              <Input
                required
                placeholder="e.g. BrightERP Core Platform"
                value={prodName}
                onChange={(e) => setProdName(e.target.value)}
              />
            </div>

            <div className="space-y-2">
              <label className="text-xs font-semibold text-muted-foreground">Type</label>
              <select
                className="w-full h-10 px-3 rounded-lg border bg-background text-sm"
                value={prodType}
                onChange={(e) => setProdType(e.target.value as 'SERVICE' | 'GOODS')}
              >
                <option value="SERVICE">SERVICE (Subscription Software / Add-on)</option>
                <option value="GOODS">GOODS (Physical Devices / Hardware)</option>
              </select>
            </div>

            <div className="space-y-2">
              <label className="text-xs font-semibold text-muted-foreground">Base Sales Price ($)</label>
              <Input
                required
                type="number"
                step="0.01"
                value={prodSalesPrice}
                onChange={(e) => setProdSalesPrice(e.target.value)}
              />
            </div>

            <div className="space-y-2">
              <label className="text-xs font-semibold text-muted-foreground">Cost Price ($)</label>
              <Input
                required
                type="number"
                step="0.01"
                value={prodCostPrice}
                onChange={(e) => setProdCostPrice(e.target.value)}
              />
            </div>

            <div className="space-y-2 col-span-2">
              <label className="text-xs font-semibold text-muted-foreground">Internal Notes</label>
              <Input
                placeholder="Operational notes, SLAs, and technical details"
                value={prodNotes}
                onChange={(e) => setProdNotes(e.target.value)}
              />
            </div>
          </div>

          {/* Variants builder */}
          <div className="space-y-3 pt-2 border-t">
            <div className="flex items-center justify-between">
              <div>
                <h4 className="text-sm font-semibold">Additive Variants (e.g. Odoo Brand +$560)</h4>
                <p className="text-xs text-muted-foreground">
                  Variants augment the base sales price when selected in subscription lines.
                </p>
              </div>
              <Button
                type="button"
                variant="outline"
                size="sm"
                onClick={addVariantRow}
                className="cursor-pointer"
              >
                <Plus className="h-3.5 w-3.5 mr-1" />
                Add Variant
              </Button>
            </div>

            <div className="space-y-2 max-h-48 overflow-y-auto pr-1">
              {variants.map((v, idx) => (
                <div key={idx} className="flex items-center gap-2">
                  <Input
                    placeholder="Variant name (e.g. Odoo Brand)"
                    value={v.name}
                    onChange={(e) => updateVariantRow(idx, 'name', e.target.value)}
                    className="flex-1 text-sm h-9"
                  />
                  <div className="relative w-36">
                    <span className="absolute left-2.5 top-2 text-xs text-muted-foreground">+$</span>
                    <Input
                      type="number"
                      step="0.01"
                      placeholder="560.00"
                      value={v.delta}
                      onChange={(e) => updateVariantRow(idx, 'delta', e.target.value)}
                      className="pl-7 text-sm h-9 font-mono"
                    />
                  </div>
                  <Button
                    type="button"
                    variant="ghost"
                    size="sm"
                    onClick={() => removeVariantRow(idx)}
                    className="text-muted-foreground hover:text-destructive h-9 w-9 p-0 cursor-pointer"
                  >
                    <Trash2 className="h-4 w-4" />
                  </Button>
                </div>
              ))}
            </div>
          </div>

          <DialogFooter>
            <Button
              type="button"
              variant="outline"
              onClick={() => setIsProductModalOpen(false)}
            >
              Cancel
            </Button>
            <Button type="submit" disabled={saving}>
              {saving ? 'Creating...' : 'Launch Product'}
            </Button>
          </DialogFooter>
        </form>
      </Dialog>

      {/* CREATE PLAN MODAL */}
      <Dialog open={isPlanModalOpen} onOpenChange={setIsPlanModalOpen}>
        <DialogHeader>
          <DialogTitle>Configure Subscription Plan</DialogTitle>
          <DialogDescription>
            Specify billing cadence, minimum seat requirements, tax profile, and automated lifecycle controls.
          </DialogDescription>
        </DialogHeader>

        <form onSubmit={handleCreatePlan} className="space-y-4">
          <div className="grid grid-cols-2 gap-4">
            <div className="space-y-2 col-span-2 sm:col-span-1">
              <label className="text-xs font-semibold text-muted-foreground">Associated Product</label>
              <select
                className="w-full h-10 px-3 rounded-lg border bg-background text-sm"
                value={planProductId}
                onChange={(e) => setPlanProductId(e.target.value)}
              >
                {products.map((p) => (
                  <option key={p.id} value={p.id}>
                    {p.name} ({formatCents(p.sales_price_cents)})
                  </option>
                ))}
              </select>
            </div>

            <div className="space-y-2 col-span-2 sm:col-span-1">
              <label className="text-xs font-semibold text-muted-foreground">Tax Profile</label>
              <select
                className="w-full h-10 px-3 rounded-lg border bg-background text-sm"
                value={planTaxId}
                onChange={(e) => setPlanTaxId(e.target.value)}
              >
                <option value="">No Tax</option>
                {taxes.map((t) => (
                  <option key={t.id} value={t.id}>
                    {t.name} ({(t.rate_bp / 100).toFixed(0)}%)
                  </option>
                ))}
              </select>
            </div>

            <div className="space-y-2 col-span-2">
              <label className="text-xs font-semibold text-muted-foreground">Plan Name</label>
              <Input
                required
                placeholder="e.g. Monthly Growth Plan"
                value={planName}
                onChange={(e) => setPlanName(e.target.value)}
              />
            </div>

            <div className="space-y-2">
              <label className="text-xs font-semibold text-muted-foreground">Billing Cadence</label>
              <select
                className="w-full h-10 px-3 rounded-lg border bg-background text-sm"
                value={planInterval}
                onChange={(e) => setPlanInterval(e.target.value as BillingInterval)}
              >
                <option value="DAILY">Daily</option>
                <option value="WEEKLY">Weekly</option>
                <option value="MONTHLY">Monthly</option>
                <option value="YEARLY">Yearly</option>
              </select>
            </div>

            <div className="space-y-2">
              <label className="text-xs font-semibold text-muted-foreground">Price per Interval ($)</label>
              <Input
                required
                type="number"
                step="0.01"
                value={planPrice}
                onChange={(e) => setPlanPrice(e.target.value)}
              />
            </div>

            <div className="space-y-2">
              <label className="text-xs font-semibold text-muted-foreground">Minimum Quantity</label>
              <Input
                type="number"
                min="1"
                value={planMinQty}
                onChange={(e) => setPlanMinQty(e.target.value)}
              />
            </div>

            <div className="space-y-2">
              <label className="text-xs font-semibold text-muted-foreground">Available Until (Optional)</label>
              <Input
                type="date"
                value={planAvailableTo}
                onChange={(e) => setPlanAvailableTo(e.target.value)}
              />
            </div>
          </div>

          {/* Lifecycle toggles */}
          <div className="p-4 rounded-xl border bg-muted/30 space-y-3">
            <h4 className="text-xs font-semibold uppercase tracking-wider text-muted-foreground">
              Automated Lifecycle Rules
            </h4>

            <div className="grid grid-cols-2 gap-3 text-xs">
              <label className="flex items-center gap-2 cursor-pointer">
                <input
                  type="checkbox"
                  checked={planAutoRenew}
                  onChange={(e) => setPlanAutoRenew(e.target.checked)}
                  className="rounded border-border text-primary focus:ring-primary"
                />
                <span>Auto-Renew at End of Cadence</span>
              </label>

              <label className="flex items-center gap-2 cursor-pointer">
                <input
                  type="checkbox"
                  checked={planAutoClose}
                  onChange={(e) => setPlanAutoClose(e.target.checked)}
                  className="rounded border-border text-primary focus:ring-primary"
                />
                <span>Auto-Close when Date Reached</span>
              </label>

              <label className="flex items-center gap-2 cursor-pointer">
                <input
                  type="checkbox"
                  checked={planAllowPause}
                  onChange={(e) => setPlanAllowPause(e.target.checked)}
                  className="rounded border-border text-primary focus:ring-primary"
                />
                <span>Allow Customer to Pause</span>
              </label>

              <label className="flex items-center gap-2 cursor-pointer">
                <input
                  type="checkbox"
                  checked={planAllowEarlyClose}
                  onChange={(e) => setPlanAllowEarlyClose(e.target.checked)}
                  className="rounded border-border text-primary focus:ring-primary"
                />
                <span>Allow Early Cancellation</span>
              </label>
            </div>
          </div>

          <DialogFooter>
            <Button
              type="button"
              variant="outline"
              onClick={() => setIsPlanModalOpen(false)}
            >
              Cancel
            </Button>
            <Button type="submit" disabled={saving}>
              {saving ? 'Creating...' : 'Register Plan'}
            </Button>
          </DialogFooter>
        </form>
      </Dialog>

      {/* CREATE TAX MODAL */}
      <Dialog open={isTaxModalOpen} onOpenChange={setIsTaxModalOpen}>
        <DialogHeader>
          <DialogTitle>Configure Tax Rule</DialogTitle>
          <DialogDescription>
            Admin defines basis points for regional tax calculations (1800 bp = 18%).
          </DialogDescription>
        </DialogHeader>

        <form onSubmit={handleCreateTax} className="space-y-4">
          <div className="space-y-2">
            <label className="text-xs font-semibold text-muted-foreground">Tax Name</label>
            <Input
              required
              placeholder="e.g. Standard VAT 18%"
              value={taxName}
              onChange={(e) => setTaxName(e.target.value)}
            />
          </div>

          <div className="grid grid-cols-2 gap-4">
            <div className="space-y-2">
              <label className="text-xs font-semibold text-muted-foreground">Rate Percentage (%)</label>
              <Input
                required
                type="number"
                step="0.01"
                placeholder="18.00"
                value={taxRatePercent}
                onChange={(e) => setTaxRatePercent(e.target.value)}
              />
            </div>

            <div className="space-y-2">
              <label className="text-xs font-semibold text-muted-foreground">Country Code</label>
              <Input
                placeholder="US"
                value={taxCountryCode}
                onChange={(e) => setTaxCountryCode(e.target.value.toUpperCase())}
              />
            </div>
          </div>

          <div className="p-3 rounded-lg border bg-muted/40">
            <label className="flex items-center gap-2 text-xs cursor-pointer">
              <input
                type="checkbox"
                checked={taxInclusive}
                onChange={(e) => setTaxInclusive(e.target.checked)}
                className="rounded border-border text-primary"
              />
              <span><strong>Tax Inclusive:</strong> Price already contains tax (carved out vs. added on top)</span>
            </label>
          </div>

          <DialogFooter>
            <Button type="button" variant="outline" onClick={() => setIsTaxModalOpen(false)}>
              Cancel
            </Button>
            <Button type="submit" disabled={saving}>
              {saving ? 'Saving...' : 'Save Tax Rule'}
            </Button>
          </DialogFooter>
        </form>
      </Dialog>

      {/* CREATE DISCOUNT MODAL */}
      <Dialog open={isDiscountModalOpen} onOpenChange={setIsDiscountModalOpen}>
        <DialogHeader>
          <DialogTitle>Create Discount Rule</DialogTitle>
          <DialogDescription>
            Admin configures percentage or fixed discount coupons with usage thresholds and seat quotas.
          </DialogDescription>
        </DialogHeader>

        <form onSubmit={handleCreateDiscount} className="space-y-4">
          <div className="space-y-2">
            <label className="text-xs font-semibold text-muted-foreground">Discount Rule Name</label>
            <Input
              required
              placeholder="e.g. Volume Seat Discount 15%"
              value={discName}
              onChange={(e) => setDiscName(e.target.value)}
            />
          </div>

          <div className="grid grid-cols-2 gap-4">
            <div className="space-y-2">
              <label className="text-xs font-semibold text-muted-foreground">Discount Kind</label>
              <select
                className="w-full h-10 px-3 rounded-lg border bg-background text-sm"
                value={discKind}
                onChange={(e) => setDiscKind(e.target.value as 'PERCENT' | 'FIXED')}
              >
                <option value="PERCENT">Percentage (%)</option>
                <option value="FIXED">Fixed Amount ($)</option>
              </select>
            </div>

            <div className="space-y-2">
              <label className="text-xs font-semibold text-muted-foreground">
                {discKind === 'PERCENT' ? 'Percentage Value (%)' : 'Amount ($)'}
              </label>
              <Input
                required
                type="number"
                step="0.01"
                value={discValue}
                onChange={(e) => setDiscValue(e.target.value)}
              />
            </div>

            <div className="space-y-2">
              <label className="text-xs font-semibold text-muted-foreground">Scope</label>
              <select
                className="w-full h-10 px-3 rounded-lg border bg-background text-sm"
                value={discScope}
                onChange={(e) => setDiscScope(e.target.value as 'ORDER' | 'PRODUCT')}
              >
                <option value="ORDER">Whole Order / Subscription</option>
                <option value="PRODUCT">Specific Product Line</option>
              </select>
            </div>

            <div className="space-y-2">
              <label className="text-xs font-semibold text-muted-foreground">Usage Quota (Max Uses)</label>
              <Input
                type="number"
                placeholder="100 (blank for unlimited)"
                value={discMaxUses}
                onChange={(e) => setDiscMaxUses(e.target.value)}
              />
            </div>

            <div className="space-y-2">
              <label className="text-xs font-semibold text-muted-foreground">Min Purchase ($)</label>
              <Input
                type="number"
                step="0.01"
                value={discMinPurchase}
                onChange={(e) => setDiscMinPurchase(e.target.value)}
              />
            </div>

            <div className="space-y-2">
              <label className="text-xs font-semibold text-muted-foreground">Min Seat Quantity</label>
              <Input
                type="number"
                min="1"
                value={discMinQty}
                onChange={(e) => setDiscMinQty(e.target.value)}
              />
            </div>
          </div>

          <DialogFooter>
            <Button type="button" variant="outline" onClick={() => setIsDiscountModalOpen(false)}>
              Cancel
            </Button>
            <Button type="submit" disabled={saving}>
              {saving ? 'Creating...' : 'Register Discount'}
            </Button>
          </DialogFooter>
        </form>
      </Dialog>
    </div>
  );
}
