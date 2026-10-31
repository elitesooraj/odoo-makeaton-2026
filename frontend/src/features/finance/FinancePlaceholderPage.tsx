import * as React from 'react';
import { Receipt, DollarSign, Percent, BarChart3, AlertCircle } from 'lucide-react';
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import { ActiveNav } from '@/components/layout/AppLayout';

interface Props {
  domain: ActiveNav;
}

export function FinancePlaceholderPage({ domain }: Props) {
  const getDomainInfo = () => {
    switch (domain) {
      case 'invoices':
        return {
          title: 'Invoices & Billing',
          icon: Receipt,
          desc: 'Automated invoice generation, payment allocation, and overdue tracking.',
          migrations: '0300-0399',
          owner: 'Frontend Member 2 & Experienced Member 2',
        };
      case 'taxes':
        return {
          title: 'Tax Rules & Configuration',
          icon: DollarSign,
          desc: 'Country/regional rates in basis points, inclusive vs. exclusive tax calculation.',
          migrations: '0100-0199',
          owner: 'Frontend Member 2 & Experienced Member 2',
        };
      case 'discounts':
        return {
          title: 'Discount Rules & Coupons',
          icon: Percent,
          desc: 'Fixed or percentage discounts with usage quotas and redemption tracking.',
          migrations: '0100-0199',
          owner: 'Frontend Member 2 & Experienced Member 2',
        };
      case 'reports':
        return {
          title: 'Financial & Revenue Reporting',
          icon: BarChart3,
          desc: 'MRR, ARR, cash collection, and overdue debt analytics (< 2s latency budget).',
          migrations: '0300+ / Indexing',
          owner: 'Frontend Member 2 & Experienced Member 2',
        };
      default:
        return {
          title: 'Finance Module',
          icon: Receipt,
          desc: 'Brightloop financial subsystem',
          migrations: '0100-0399',
          owner: 'Team 2',
        };
    }
  };

  const info = getDomainInfo();
  const Icon = info.icon;

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-bold tracking-tight">{info.title}</h1>
        <p className="text-sm text-muted-foreground">{info.desc}</p>
      </div>

      <Card className="border-dashed border-2">
        <CardHeader>
          <div className="flex items-center gap-3">
            <div className="h-10 w-10 rounded-xl bg-primary/10 border border-primary/20 text-primary flex items-center justify-center">
              <Icon className="h-5 w-5" />
            </div>
            <div>
              <CardTitle className="text-base">Assigned Scope: {info.owner}</CardTitle>
              <CardDescription>
                Database migrations reserved in range: <code className="font-mono text-xs">{info.migrations}</code>
              </CardDescription>
            </div>
          </div>
        </CardHeader>
        <CardContent className="space-y-4 text-xs text-muted-foreground">
          <p className="leading-relaxed">
            Frontend Member 1 has wired the route navigation and RBAC permission guards for this view.
            Frontend Member 2 can drop their invoice, payment, tax, discount, or reporting dashboard components directly here.
          </p>

          <div className="p-4 rounded-xl bg-muted/40 border space-y-2 text-foreground font-mono text-xs">
            <div className="font-semibold text-primary">Integration Ready:</div>
            <div>&bull; Route: <span className="text-muted-foreground">/app/{domain}</span></div>
            <div>&bull; Envelopes: <span className="text-muted-foreground">&#123; data, meta &#125; standard adhered</span></div>
            <div>&bull; Money format: <span className="text-muted-foreground">BIGINT cents (formatCents utility in @/lib/money)</span></div>
          </div>
        </CardContent>
      </Card>
    </div>
  );
}

