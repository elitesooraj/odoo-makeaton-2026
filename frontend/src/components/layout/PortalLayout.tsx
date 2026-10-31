import * as React from 'react';
import { FileText, Receipt, Shield, LogOut, KeyRound, DollarSign, UserCheck } from 'lucide-react';
import { useAuth } from '@/context/AuthContext';
import { EmailVerificationBanner } from '@/components/auth/EmailVerificationBanner';
import { ChangePasswordModal } from '@/components/auth/ChangePasswordModal';
import { Button } from '@/components/ui/button';

export type PortalTab = 'subscriptions' | 'invoices' | 'payments' | 'security' | 'profile';

interface Props {
  activeTab: PortalTab;
  onTabChange: (tab: PortalTab) => void;
  children: React.ReactNode;
}

export function PortalLayout({ activeTab, onTabChange, children }: Props) {
  const { user, logout } = useAuth();
  const [changePasswordOpen, setChangePasswordOpen] = React.useState(false);

  return (
    <div className="min-h-screen flex flex-col bg-background text-foreground">
      <EmailVerificationBanner />

      <ChangePasswordModal
        open={user?.must_change_password || changePasswordOpen}
        onOpenChange={setChangePasswordOpen}
        forced={user?.must_change_password}
      />

      {/* Top Portal Navbar */}
      <header className="border-b bg-card/60 backdrop-blur-md sticky top-0 z-30">
        <div className="max-w-6xl mx-auto px-6 h-16 flex items-center justify-between">
          <div className="flex items-center gap-6">
            <div className="flex items-center gap-3">
              <div className="h-9 w-9 rounded-lg bg-primary text-primary-foreground font-bold flex items-center justify-center text-sm shadow-sm">
                BL
              </div>
              <div>
                <span className="font-bold tracking-tight text-sm">Brightloop</span>
                <span className="ml-2 text-xs text-muted-foreground font-medium px-2 py-0.5 rounded-full bg-muted">
                  Client Portal
                </span>
              </div>
            </div>

            {/* Navigation Pills */}
            <nav className="flex items-center gap-1 ml-4">
              <button
                onClick={() => onTabChange('subscriptions')}
                className={`flex items-center gap-2 px-3.5 py-1.5 rounded-lg text-xs font-medium transition-colors cursor-pointer ${
                  activeTab === 'subscriptions'
                    ? 'bg-primary text-primary-foreground shadow-sm'
                    : 'text-muted-foreground hover:bg-muted hover:text-foreground'
                }`}
              >
                <FileText className="h-3.5 w-3.5" />
                My Subscriptions
              </button>

              <button
                onClick={() => onTabChange('invoices')}
                className={`flex items-center gap-2 px-3.5 py-1.5 rounded-lg text-xs font-medium transition-colors cursor-pointer ${
                  activeTab === 'invoices'
                    ? 'bg-primary text-primary-foreground shadow-sm'
                    : 'text-muted-foreground hover:bg-muted hover:text-foreground'
                }`}
              >
                <Receipt className="h-3.5 w-3.5" />
                Invoices
              </button>

              <button
                onClick={() => onTabChange('payments')}
                className={`flex items-center gap-2 px-3.5 py-1.5 rounded-lg text-xs font-medium transition-colors cursor-pointer ${
                  activeTab === 'payments'
                    ? 'bg-primary text-primary-foreground shadow-sm'
                    : 'text-muted-foreground hover:bg-muted hover:text-foreground'
                }`}
              >
                <DollarSign className="h-3.5 w-3.5" />
                Payments
              </button>

              <button
                onClick={() => onTabChange('profile')}
                className={`flex items-center gap-2 px-3.5 py-1.5 rounded-lg text-xs font-medium transition-colors cursor-pointer ${
                  activeTab === 'profile'
                    ? 'bg-primary text-primary-foreground shadow-sm'
                    : 'text-muted-foreground hover:bg-muted hover:text-foreground'
                }`}
              >
                <UserCheck className="h-3.5 w-3.5" />
                Account Profile
              </button>
            </nav>
          </div>

          <div className="flex items-center gap-3">
            <span className="text-xs text-muted-foreground hidden sm:inline">
              {user?.full_name || user?.email}
            </span>

            <Button
              variant="ghost"
              size="sm"
              onClick={() => setChangePasswordOpen(true)}
              className="text-xs cursor-pointer h-8"
              title="Change Password"
            >
              <KeyRound className="h-3.5 w-3.5 mr-1" />
              Password
            </Button>

            <Button
              variant="outline"
              size="sm"
              onClick={logout}
              className="text-xs cursor-pointer h-8"
            >
              <LogOut className="h-3.5 w-3.5 mr-1" />
              Logout
            </Button>
          </div>
        </div>
      </header>

      {/* Main Content Area */}
      <main className="flex-1 max-w-6xl w-full mx-auto p-6 md:p-8">
        {children}
      </main>

      <footer className="border-t py-6 text-center text-xs text-muted-foreground">
        Brightloop Software &bull; Subscription & Recurring Billing Platform
      </footer>
    </div>
  );
}
