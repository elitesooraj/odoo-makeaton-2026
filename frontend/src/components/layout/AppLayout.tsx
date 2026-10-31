import * as React from 'react';
import {
  Users,
  Package,
  Layers,
  FileText,
  DollarSign,
  ShieldAlert,
  LogOut,
  KeyRound,
  Building2,
  Receipt,
  BarChart3,
  Percent,
} from 'lucide-react';
import { useAuth } from '@/context/AuthContext';
import { EmailVerificationBanner } from '@/components/auth/EmailVerificationBanner';
import { ChangePasswordModal } from '@/components/auth/ChangePasswordModal';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';

export type ActiveNav =
  | 'customers'
  | 'products'
  | 'plans'
  | 'subscriptions'
  | 'quotations'
  | 'invoices'
  | 'taxes'
  | 'discounts'
  | 'reports'
  | 'users';

interface Props {
  activeNav: ActiveNav;
  onNavigate: (nav: ActiveNav) => void;
  children: React.ReactNode;
}

export function AppLayout({ activeNav, onNavigate, children }: Props) {
  const { user, logout, can } = useAuth();
  const [changePasswordOpen, setChangePasswordOpen] = React.useState(false);

  const navItems = [
    {
      id: 'customers' as ActiveNav,
      label: 'Customers',
      icon: Users,
      permission: 'customers.read',
    },
    {
      id: 'products' as ActiveNav,
      label: 'Products & Variants',
      icon: Package,
      permission: 'products.read',
    },
    {
      id: 'plans' as ActiveNav,
      label: 'Subscription Plans',
      icon: Layers,
      permission: 'plans.read',
    },
    {
      id: 'subscriptions' as ActiveNav,
      label: 'Subscriptions & Quotes',
      icon: FileText,
      permission: 'subscriptions.read',
    },
    {
      id: 'invoices' as ActiveNav,
      label: 'Invoices & Billing',
      icon: Receipt,
      permission: 'invoices.read',
      subtext: 'Finance Scope',
    },
    {
      id: 'taxes' as ActiveNav,
      label: 'Taxes & Rules',
      icon: DollarSign,
      permission: 'taxes.read',
      subtext: 'Finance Scope',
    },
    {
      id: 'discounts' as ActiveNav,
      label: 'Discounts',
      icon: Percent,
      permission: 'discounts.read',
      subtext: 'Finance Scope',
    },
    {
      id: 'reports' as ActiveNav,
      label: 'Revenue Reports',
      icon: BarChart3,
      permission: 'reports.read',
      subtext: 'Finance Scope',
    },
    {
      id: 'users' as ActiveNav,
      label: 'Staff & RBAC',
      icon: ShieldAlert,
      permission: 'users.read',
    },
  ];

  return (
    <div className="min-h-screen flex flex-col bg-background text-foreground">
      {/* Top soft verification alert */}
      <EmailVerificationBanner />

      {/* Forced or voluntary password change */}
      <ChangePasswordModal
        open={user?.must_change_password || changePasswordOpen}
        onOpenChange={setChangePasswordOpen}
        forced={user?.must_change_password}
      />

      <div className="flex-1 flex overflow-hidden">
        {/* Sidebar */}
        <aside className="w-64 border-r bg-card/60 backdrop-blur-sm flex flex-col shrink-0">
          {/* Brand */}
          <div className="h-16 flex items-center px-6 border-b gap-3">
            <div className="h-9 w-9 rounded-lg bg-primary/10 border border-primary/20 flex items-center justify-center text-primary font-bold text-lg">
              BL
            </div>
            <div>
              <div className="font-bold tracking-tight text-sm leading-none">Brightloop</div>
              <div className="text-[11px] text-muted-foreground font-mono mt-0.5">Subscription System</div>
            </div>
          </div>

          {/* Nav Items */}
          <nav className="flex-1 p-3 space-y-1 overflow-y-auto">
            <div className="px-3 py-1.5 text-[10px] font-semibold text-muted-foreground uppercase tracking-wider">
              Core Operations
            </div>
            {navItems.map((item) => {
              if (item.permission && !can(item.permission)) return null;
              const Icon = item.icon;
              const isActive = activeNav === item.id;

              return (
                <button
                  key={item.id}
                  onClick={() => onNavigate(item.id)}
                  className={`w-full flex items-center gap-3 px-3 py-2 rounded-lg text-sm font-medium transition-all text-left cursor-pointer ${
                    isActive
                      ? 'bg-primary text-primary-foreground shadow-sm'
                      : 'text-muted-foreground hover:bg-muted/70 hover:text-foreground'
                  }`}
                >
                  <Icon className={`h-4 w-4 shrink-0 ${isActive ? 'text-primary-foreground' : 'text-muted-foreground'}`} />
                  <span className="flex-1 truncate">{item.label}</span>
                  {item.subtext && (
                    <span className={`text-[10px] px-1.5 py-0.5 rounded ${isActive ? 'bg-primary-foreground/20 text-primary-foreground' : 'bg-muted text-muted-foreground'}`}>
                      M2
                    </span>
                  )}
                </button>
              );
            })}
          </nav>

          {/* User Profile Footer */}
          <div className="p-4 border-t bg-muted/20">
            <div className="flex items-center gap-3 mb-3">
              <div className="h-8 w-8 rounded-full bg-primary/20 text-primary font-semibold text-xs flex items-center justify-center">
                {user?.full_name ? user.full_name.charAt(0).toUpperCase() : user?.email.charAt(0).toUpperCase()}
              </div>
              <div className="flex-1 min-w-0">
                <div className="text-xs font-medium truncate">{user?.full_name || user?.email}</div>
                <div className="flex items-center gap-1.5 mt-0.5">
                  <Badge variant={user?.role === 'ADMIN' ? 'default' : 'secondary'} className="text-[10px] py-0 px-1.5">
                    {user?.role}
                  </Badge>
                  {user?.customer_id && (
                    <span className="text-[10px] text-muted-foreground font-mono flex items-center gap-1">
                      <Building2 className="h-3 w-3" /> Client
                    </span>
                  )}
                </div>
              </div>
            </div>

            <div className="flex items-center gap-2">
              <Button
                variant="outline"
                size="sm"
                className="flex-1 text-xs h-7"
                onClick={() => setChangePasswordOpen(true)}
              >
                <KeyRound className="h-3 w-3 mr-1" /> Password
              </Button>
              <Button
                variant="ghost"
                size="sm"
                className="text-xs h-7 text-muted-foreground hover:text-destructive"
                onClick={logout}
              >
                <LogOut className="h-3.5 w-3.5" />
              </Button>
            </div>
          </div>
        </aside>

        {/* Main Content Area */}
        <main className="flex-1 overflow-y-auto bg-background/50 p-8">
          <div className="max-w-7xl mx-auto space-y-6">{children}</div>
        </main>
      </div>
    </div>
  );
}

