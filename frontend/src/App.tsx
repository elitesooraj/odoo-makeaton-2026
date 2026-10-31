import * as React from 'react';
import { AuthProvider, useAuth } from '@/context/AuthContext';
import { ToastProvider } from '@/components/ui/toast';
import { AuthPage } from '@/features/auth/AuthPage';
import { AppLayout, ActiveNav } from '@/components/layout/AppLayout';
import { PortalLayout, PortalTab } from '@/components/layout/PortalLayout';
import { CustomersPage } from '@/features/customers/CustomersPage';
import { CatalogPage } from '@/features/catalog/CatalogPage';
import { SubscriptionsPage } from '@/features/subscriptions/SubscriptionsPage';
import { CustomerPortalPage } from '@/features/portal/CustomerPortalPage';
import { UsersPage } from '@/features/users/UsersPage';
import { FinancePage } from '@/features/finance/FinancePage';

function AppContent() {
  const { user, loading, isAuthenticated, isCustomer } = useAuth();

  // Internal Back-office navigation state
  const [activeNav, setActiveNav] = React.useState<ActiveNav>('customers');

  // Customer Portal navigation state
  const [portalTab, setPortalTab] = React.useState<PortalTab>('subscriptions');

  if (loading) {
    return (
      <div className="min-h-screen flex flex-col items-center justify-center bg-background text-foreground space-y-4">
        <div className="h-10 w-10 rounded-xl bg-primary animate-pulse flex items-center justify-center text-primary-foreground font-black text-lg">
          BL
        </div>
        <div className="text-sm font-medium text-muted-foreground animate-pulse">
          Connecting to Brightloop API...
        </div>
      </div>
    );
  }

  // Unauthenticated screen
  if (!isAuthenticated) {
    return <AuthPage />;
  }

  // Customer Portal shell (Exclusive to role: CUSTOMER)
  if (isCustomer) {
    return (
      <PortalLayout activeTab={portalTab} onTabChange={setPortalTab}>
        <CustomerPortalPage activeTab={portalTab} onChangeTab={setPortalTab} />
      </PortalLayout>
    );
  }

  // Operations Shell (ADMIN and STAFF)
  return (
    <AppLayout activeNav={activeNav} onNavigate={setActiveNav}>
      {activeNav === 'customers' && <CustomersPage />}
      {(activeNav === 'products' ||
        activeNav === 'plans' ||
        activeNav === 'taxes' ||
        activeNav === 'discounts') && <CatalogPage />}
      {(activeNav === 'subscriptions' || activeNav === 'quotations') && <SubscriptionsPage />}
      {activeNav === 'users' && <UsersPage />}
      {(activeNav === 'invoices' || activeNav === 'reports') && <FinancePage />}
    </AppLayout>
  );
}

export function App() {
  return (
    <ToastProvider>
      <AuthProvider>
        <AppContent />
      </AuthProvider>
    </ToastProvider>
  );
}

export default App;
