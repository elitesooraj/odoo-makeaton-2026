import * as React from 'react';
import { FileText, Receipt, Shield, LogOut, KeyRound } from 'lucide-react';
import { useAuth } from '@/context/AuthContext';
import { EmailVerificationBanner } from '@/components/auth/EmailVerificationBanner';
import { ChangePasswordModal } from '@/components/auth/ChangePasswordModal';
import { Button } from '@/components/ui/button';

export type PortalTab = 'subscriptions' | 'invoices' | 'security';

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
                Invoices & Billing
              </button>

              <button
                onClick={() => onTabChange('security')}
                className={`flex items-center gap-2 px-3.5 py-1.5 rounded-lg text-xs font-medium transition-colors cursor-pointer ${
                  activeTab === 'security'
                    ? 'bg-primary text-primary-foreground shadow-sm'
                    : 'text-muted-foreground hover:bg-muted hover:text-foreground'
                }`}
              >
                <Shield className="h-3.5 w-3.5" />
                Account Security
              </button>
            </nav>
          </div>

          {/* Right User Bar */}
          <div className="flex items-center gap-3">
            <div className="text-right hidden sm:block">
              <div className="text-xs font-medium">{user?.full_name || user?.email}</div>
              <div className="text-[10px] text-muted-foreground font-mono">{user?.email}</div>
            </div>

            <Button
              variant="outline"
              size="sm"
              className="text-xs h-8"
              onClick={() => setChangePasswordOpen(true)}
            >
              <KeyRound className="h-3.5 w-3.5 mr-1" /> Password
            </Button>

            <Button
              variant="ghost"
              size="sm"
              className="text-xs h-8 text-muted-foreground hover:text-destructive"
              onClick={logout}
            >
              <LogOut className="h-4 w-4" />
            </Button>
          </div>
        </div>
      </header>

      {/* Main Container */}
      <main className="flex-1 max-w-6xl w-full mx-auto p-6 md:p-8 space-y-6">
        {children}
      </main>
    </div>
  );
}

