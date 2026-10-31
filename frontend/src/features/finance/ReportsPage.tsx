import * as React from 'react';
import {
  BarChart3, TrendingUp, TrendingDown, Users, AlertTriangle,
  CheckCircle2, Clock, Pause, RefreshCw, Calendar,
  DollarSign, Activity, ArrowUpRight, ArrowDownRight,
  Building2, FileText,
} from 'lucide-react';
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';
import { useToast } from '@/components/ui/toast';
import { formatCents } from '@/lib/money';
import { formatDate } from '@/lib/date';
import {
  reportsApi, ReportSummary, RevenuePoint, StatusCount, OverdueInvoice,
} from '@/api/finance';

// ─── Mini bar chart (pure SVG — no external chart lib needed) ─────────────────

interface BarChartProps {
  data: { label: string; value: number; value2?: number }[];
  color?: string;
  color2?: string;
  maxValue?: number;
  height?: number;
}

function MiniBarChart({ data, color = '#3b82f6', color2 = '#10b981', height = 120 }: BarChartProps) {
  if (!data.length) return null;
  const max = Math.max(...data.flatMap((d) => [d.value, d.value2 ?? 0]), 1);
  const barWidth = 100 / (data.length * (data[0].value2 !== undefined ? 2.5 : 1.5));
  const gap = barWidth * 0.5;

  return (
    <svg
      viewBox={`0 0 100 ${height}`}
      preserveAspectRatio="none"
      className="w-full"
      style={{ height }}
      aria-label="Revenue bar chart"
    >
      {data.map((d, i) => {
        const hasPair = d.value2 !== undefined;
        const groupWidth = hasPair ? barWidth * 2 + gap / 2 : barWidth;
        const x = i * (groupWidth + gap);
        const h1 = (d.value / max) * (height - 24);
        const h2 = hasPair ? ((d.value2 ?? 0) / max) * (height - 24) : 0;

        return (
          <g key={i}>
            {/* Bar 1 */}
            <rect
              x={`${x}%`}
              y={height - 20 - h1}
              width={`${barWidth}%`}
              height={h1}
              rx="2"
              fill={color}
              fillOpacity={0.85}
            />
            {/* Bar 2 (if pair) */}
            {hasPair && (
              <rect
                x={`${x + parseFloat(String(barWidth)) + gap / 4}%`}
                y={height - 20 - h2}
                width={`${barWidth}%`}
                height={h2}
                rx="2"
                fill={color2}
                fillOpacity={0.7}
              />
            )}
            {/* Label */}
            <text
              x={`${x + (hasPair ? barWidth : barWidth / 2)}%`}
              y={height - 4}
              textAnchor="middle"
              fontSize="4"
              fill="currentColor"
              className="text-muted-foreground"
            >
              {d.label}
            </text>
          </g>
        );
      })}
    </svg>
  );
}

// ─── Donut chart ──────────────────────────────────────────────────────────────

const STATUS_COLORS: Record<string, string> = {
  ACTIVE: '#10b981',
  PAUSED: '#f59e0b',
  QUOTATION: '#3b82f6',
  CONFIRMED: '#8b5cf6',
  DRAFT: '#94a3b8',
  CLOSED: '#475569',
};

interface DonutProps { data: StatusCount[] }

function DonutChart({ data }: DonutProps) {
  const total = data.reduce((a, d) => a + d.count, 0);
  if (!total) return null;

  let cumulative = 0;
  const slices = data.map((d) => {
    const pct = d.count / total;
    const start = cumulative;
    cumulative += pct;
    return { ...d, pct, start };
  });

  const R = 38;
  const cx = 50;
  const cy = 50;

  function polarToXY(pct: number) {
    const angle = pct * 2 * Math.PI - Math.PI / 2;
    return { x: cx + R * Math.cos(angle), y: cy + R * Math.sin(angle) };
  }

  return (
    <svg viewBox="0 0 100 100" className="w-40 h-40 shrink-0" aria-label="Subscription status donut">
      {slices.map((s, i) => {
        const start = polarToXY(s.start);
        const end = polarToXY(s.start + s.pct);
        const large = s.pct > 0.5 ? 1 : 0;
        const path = `M ${cx} ${cy} L ${start.x} ${start.y} A ${R} ${R} 0 ${large} 1 ${end.x} ${end.y} Z`;
        return (
          <path key={i} d={path} fill={STATUS_COLORS[s.status] ?? '#64748b'} fillOpacity={0.9} />
        );
      })}
      {/* Donut hole */}
      <circle cx={cx} cy={cy} r={22} fill="var(--card-bg, white)" className="fill-card" />
      <text x={cx} y={cy - 4} textAnchor="middle" fontSize="8" fontWeight="bold" className="fill-foreground">{total}</text>
      <text x={cx} y={cy + 6} textAnchor="middle" fontSize="4" className="fill-muted-foreground">TOTAL</text>
    </svg>
  );
}

// ─── KPI Card ─────────────────────────────────────────────────────────────────

interface KpiCardProps {
  label: string;
  value: string;
  sub?: string;
  icon: React.ReactNode;
  trend?: 'up' | 'down' | 'neutral';
  gradient: string;
}

function KpiCard({ label, value, sub, icon, trend, gradient }: KpiCardProps) {
  return (
    <Card className={`border-0 ${gradient}`}>
      <CardContent className="p-5">
        <div className="flex items-start justify-between">
          <div className="space-y-1">
            <div className="text-xs text-muted-foreground">{label}</div>
            <div className="text-2xl font-bold tracking-tight">{value}</div>
            {sub && <div className="text-[11px] text-muted-foreground">{sub}</div>}
          </div>
          <div className="h-10 w-10 rounded-xl bg-background/40 flex items-center justify-center text-current">
            {icon}
          </div>
        </div>
        {trend && (
          <div className={`mt-3 flex items-center gap-1 text-[11px] font-medium ${trend === 'up' ? 'text-emerald-500' : trend === 'down' ? 'text-rose-500' : 'text-muted-foreground'}`}>
            {trend === 'up' && <ArrowUpRight className="h-3 w-3" />}
            {trend === 'down' && <ArrowDownRight className="h-3 w-3" />}
            {trend === 'up' ? 'On track' : trend === 'down' ? 'Needs attention' : 'Stable'}
          </div>
        )}
      </CardContent>
    </Card>
  );
}

// ─── Main Page ────────────────────────────────────────────────────────────────

export function ReportsPage() {
  const { addToast } = useToast();

  const [summary, setSummary] = React.useState<ReportSummary | null>(null);
  const [revenue, setRevenue] = React.useState<RevenuePoint[]>([]);
  const [statusCounts, setStatusCounts] = React.useState<StatusCount[]>([]);
  const [overdue, setOverdue] = React.useState<OverdueInvoice[]>([]);
  const [loading, setLoading] = React.useState(true);
  const [revenueMonths, setRevenueMonths] = React.useState(6);

  const load = React.useCallback(async (months: number) => {
    setLoading(true);
    try {
      const [sum, rev, stats, ov] = await Promise.all([
        reportsApi.summary(),
        reportsApi.revenue(months),
        reportsApi.subscriptionsByStatus(),
        reportsApi.overdue(),
      ]);
      setSummary(sum);
      setRevenue(rev);
      setStatusCounts(stats);
      setOverdue(ov);
    } catch {
      addToast({ title: 'Failed to load report data', variant: 'destructive' });
    } finally {
      setLoading(false);
    }
  }, []);

  React.useEffect(() => { load(revenueMonths); }, [load, revenueMonths]);

  const arrCents = summary ? summary.mrr_cents * 12 : 0;
  const collectionRate = summary && summary.mrr_cents > 0
    ? Math.round((summary.collected_this_month_cents / summary.mrr_cents) * 100)
    : 0;

  const revenueChartData = revenue.map((r) => ({
    label: r.month.slice(5), // "MM"
    value: r.paid_cents,
    value2: r.invoiced_cents,
  }));

  return (
    <div className="space-y-8">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold tracking-tight flex items-center gap-2">
            <BarChart3 className="h-6 w-6 text-primary" />
            Financial &amp; Revenue Reporting
          </h1>
          <p className="text-sm text-muted-foreground mt-0.5">
            MRR, ARR, cash collection, and overdue debt analytics — live from the database.
          </p>
        </div>
        <Button size="sm" variant="outline" className="gap-1.5" onClick={() => load(revenueMonths)} disabled={loading}>
          <RefreshCw className={`h-3.5 w-3.5 ${loading ? 'animate-spin' : ''}`} /> Refresh
        </Button>
      </div>

      {loading && !summary ? (
        <div className="flex items-center justify-center h-48 text-sm text-muted-foreground animate-pulse">
          Loading report data…
        </div>
      ) : (
        <>
          {/* ─── KPI Row ──────────────────────────────────────────── */}
          <div className="grid grid-cols-2 lg:grid-cols-4 gap-4">
            <KpiCard
              label="Monthly Recurring Revenue"
              value={formatCents(summary?.mrr_cents ?? 0)}
              sub={`ARR: ${formatCents(arrCents)}`}
              icon={<TrendingUp className="h-5 w-5 text-emerald-500" />}
              gradient="bg-gradient-to-br from-emerald-500/10 to-emerald-600/5"
              trend="up"
            />
            <KpiCard
              label="Collected This Month"
              value={formatCents(summary?.collected_this_month_cents ?? 0)}
              sub={`${collectionRate}% of MRR`}
              icon={<DollarSign className="h-5 w-5 text-blue-500" />}
              gradient="bg-gradient-to-br from-blue-500/10 to-blue-600/5"
              trend={collectionRate >= 80 ? 'up' : 'down'}
            />
            <KpiCard
              label="Outstanding Balance"
              value={formatCents(summary?.outstanding_cents ?? 0)}
              sub={`${summary?.overdue_invoices ?? 0} overdue invoice${(summary?.overdue_invoices ?? 0) !== 1 ? 's' : ''}`}
              icon={<AlertTriangle className="h-5 w-5 text-amber-500" />}
              gradient="bg-gradient-to-br from-amber-500/10 to-amber-600/5"
              trend={summary && summary.overdue_invoices > 0 ? 'down' : 'neutral'}
            />
            <KpiCard
              label="Overdue Debt"
              value={formatCents(summary?.overdue_cents ?? 0)}
              sub="Unpaid past due date"
              icon={<Activity className="h-5 w-5 text-rose-500" />}
              gradient="bg-gradient-to-br from-rose-500/10 to-rose-600/5"
              trend={summary && summary.overdue_cents > 0 ? 'down' : 'up'}
            />
          </div>

          {/* ─── Subscription stats row ────────────────────────────── */}
          <div className="grid grid-cols-4 gap-3">
            {[
              { label: 'Active', value: summary?.active_subscriptions ?? 0, icon: <CheckCircle2 className="h-4 w-4 text-emerald-500" />, color: 'text-emerald-500' },
              { label: 'Paused', value: summary?.paused_subscriptions ?? 0, icon: <Pause className="h-4 w-4 text-amber-500" />, color: 'text-amber-500' },
              { label: 'Open Quotes', value: summary?.open_quotations ?? 0, icon: <FileText className="h-4 w-4 text-blue-500" />, color: 'text-blue-500' },
              { label: 'Overdue Inv.', value: summary?.overdue_invoices ?? 0, icon: <AlertTriangle className="h-4 w-4 text-rose-500" />, color: 'text-rose-500' },
            ].map(({ label, value, icon, color }) => (
              <Card key={label} className="border bg-card/60">
                <CardContent className="p-4 flex items-center gap-3">
                  <div className="h-9 w-9 rounded-lg bg-muted/50 flex items-center justify-center shrink-0">{icon}</div>
                  <div>
                    <div className={`text-xl font-bold ${color}`}>{value.toLocaleString()}</div>
                    <div className="text-[11px] text-muted-foreground">{label}</div>
                  </div>
                </CardContent>
              </Card>
            ))}
          </div>

          {/* ─── Revenue Chart + Subscription Donut ───────────────── */}
          <div className="grid grid-cols-5 gap-4">
            {/* Revenue chart (3/5) */}
            <Card className="col-span-3">
              <CardHeader className="pb-2">
                <div className="flex items-center justify-between">
                  <div>
                    <CardTitle className="text-sm">Revenue: Paid vs. Invoiced</CardTitle>
                    <CardDescription className="text-xs">Monthly cash collection vs. invoiced amounts</CardDescription>
                  </div>
                  <select
                    id="revenue-months"
                    value={revenueMonths}
                    onChange={(e) => setRevenueMonths(parseInt(e.target.value))}
                    className="h-7 text-xs rounded-md border border-input bg-background px-2"
                  >
                    {[3, 6, 12, 24].map((m) => <option key={m} value={m}>{m} months</option>)}
                  </select>
                </div>
                {/* Legend */}
                <div className="flex items-center gap-4 text-[11px] text-muted-foreground">
                  <span className="flex items-center gap-1.5"><span className="h-2 w-4 rounded-sm bg-blue-500/80 inline-block" /> Paid</span>
                  <span className="flex items-center gap-1.5"><span className="h-2 w-4 rounded-sm bg-emerald-500/70 inline-block" /> Invoiced</span>
                </div>
              </CardHeader>
              <CardContent className="pb-4">
                {revenue.length > 0 ? (
                  <MiniBarChart
                    data={revenueChartData}
                    color="#3b82f6"
                    color2="#10b981"
                    height={140}
                  />
                ) : (
                  <div className="h-36 flex items-center justify-center text-xs text-muted-foreground">No revenue data</div>
                )}

                {/* Table of values */}
                <div className="mt-4 space-y-1">
                  {revenue.slice(-4).map((r) => (
                    <div key={r.month} className="flex items-center justify-between text-xs">
                      <span className="text-muted-foreground font-mono w-16">{r.month}</span>
                      <div className="flex items-center gap-4">
                        <span className="font-mono text-blue-500">{formatCents(r.paid_cents)}</span>
                        <span className="font-mono text-emerald-500">{formatCents(r.invoiced_cents)}</span>
                        <span className={`text-[10px] ${r.paid_cents >= r.invoiced_cents ? 'text-emerald-500' : 'text-amber-500'}`}>
                          {r.invoiced_cents > 0
                            ? `${Math.round((r.paid_cents / r.invoiced_cents) * 100)}%`
                            : '—'}
                        </span>
                      </div>
                    </div>
                  ))}
                </div>
              </CardContent>
            </Card>

            {/* Subscription status breakdown (2/5) */}
            <Card className="col-span-2">
              <CardHeader className="pb-2">
                <CardTitle className="text-sm">Subscriptions by Status</CardTitle>
                <CardDescription className="text-xs">Current distribution across lifecycle states</CardDescription>
              </CardHeader>
              <CardContent>
                <div className="flex flex-col items-center gap-4">
                  <DonutChart data={statusCounts} />
                  <div className="w-full space-y-1.5">
                    {statusCounts.map((sc) => (
                      <div key={sc.status} className="flex items-center justify-between text-xs">
                        <span className="flex items-center gap-1.5">
                          <span className="h-2 w-2 rounded-full shrink-0" style={{ background: STATUS_COLORS[sc.status] ?? '#94a3b8' }} />
                          <span className="text-muted-foreground">{sc.status}</span>
                        </span>
                        <span className="font-mono font-semibold">{sc.count}</span>
                      </div>
                    ))}
                  </div>
                </div>
              </CardContent>
            </Card>
          </div>

          {/* ─── Overdue Invoices Table ────────────────────────────── */}
          <Card>
            <CardHeader className="pb-3">
              <div className="flex items-center gap-2">
                <AlertTriangle className="h-4 w-4 text-rose-500" />
                <CardTitle className="text-sm">Overdue Invoices</CardTitle>
                {overdue.length > 0 && (
                  <Badge variant="destructive" className="text-[10px]">{overdue.length}</Badge>
                )}
              </div>
              <CardDescription className="text-xs">
                Confirmed invoices with a non-zero balance past their due date, ordered by oldest first.
              </CardDescription>
            </CardHeader>
            <CardContent className="p-0">
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead className="text-xs">Invoice #</TableHead>
                    <TableHead className="text-xs">Customer</TableHead>
                    <TableHead className="text-xs">Due Date</TableHead>
                    <TableHead className="text-xs">Days Late</TableHead>
                    <TableHead className="text-xs text-right">Balance</TableHead>
                    <TableHead className="text-xs text-right">Total</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {overdue.length === 0 ? (
                    <TableRow>
                      <TableCell colSpan={6} className="text-center py-10 text-xs">
                        <div className="flex flex-col items-center gap-2 text-muted-foreground">
                          <CheckCircle2 className="h-8 w-8 text-emerald-500/60" />
                          <span>No overdue invoices. All collections are current.</span>
                        </div>
                      </TableCell>
                    </TableRow>
                  ) : (
                    overdue.map((oi) => (
                      <TableRow key={oi.id} className="text-xs">
                        <TableCell className="font-mono font-medium">{oi.number}</TableCell>
                        <TableCell className="flex items-center gap-1.5">
                          <Building2 className="h-3 w-3 text-muted-foreground shrink-0" />
                          {oi.customer_name}
                        </TableCell>
                        <TableCell className="text-rose-400 font-medium">{formatDate(oi.due_date)}</TableCell>
                        <TableCell>
                          <Badge
                            variant={oi.days_late > 30 ? 'destructive' : 'warning'}
                            className="text-[10px] font-mono"
                          >
                            {oi.days_late}d late
                          </Badge>
                        </TableCell>
                        <TableCell className="text-right font-mono font-bold text-rose-500">{formatCents(oi.balance_cents)}</TableCell>
                        <TableCell className="text-right font-mono text-muted-foreground">{formatCents(oi.total_cents)}</TableCell>
                      </TableRow>
                    ))
                  )}
                </TableBody>
              </Table>
              {overdue.length > 0 && (
                <div className="px-4 py-3 border-t flex items-center justify-between text-xs">
                  <span className="text-muted-foreground">Total overdue exposure</span>
                  <span className="font-mono font-bold text-rose-500">
                    {formatCents(overdue.reduce((a, o) => a + o.balance_cents, 0))}
                  </span>
                </div>
              )}
            </CardContent>
          </Card>
        </>
      )}
    </div>
  );
}
