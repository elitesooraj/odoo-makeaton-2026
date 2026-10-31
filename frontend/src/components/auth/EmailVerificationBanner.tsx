import * as React from 'react';
import { Mail, Send, Check } from 'lucide-react';
import { useAuth } from '@/context/AuthContext';
import { authApi } from '@/api/auth';
import { useToast } from '@/components/ui/toast';

export function EmailVerificationBanner() {
  const { user } = useAuth();
  const { toast } = useToast();
  const [sending, setSending] = React.useState(false);
  const [sent, setSent] = React.useState(false);

  if (!user || user.email_verified) return null;

  const handleResend = async () => {
    setSending(true);
    try {
      await authApi.resendVerification();
      setSent(true);
      toast({
        title: 'Verification Link Sent',
        description: `Check your inbox at ${user.email} (or Mailpit on :8025 in development).`,
        variant: 'success',
      });
    } catch {
      toast({
        title: 'Error',
        description: 'Failed to send verification email. Try again later.',
        variant: 'destructive',
      });
    } finally {
      setSending(false);
    }
  };

  return (
    <div className="bg-amber-500/10 border-b border-amber-500/20 px-4 py-2.5 text-xs text-amber-200 flex items-center justify-between gap-4">
      <div className="flex items-center gap-2">
        <Mail className="h-4 w-4 text-amber-400 shrink-0" />
        <span>
          Your email <strong className="font-semibold text-amber-300">{user.email}</strong> is not verified.
          Some administrative features may be restricted.
        </span>
      </div>
      <button
        onClick={handleResend}
        disabled={sending || sent}
        className="flex items-center gap-1.5 px-3 py-1 bg-amber-500/20 hover:bg-amber-500/30 text-amber-300 font-medium rounded transition-colors disabled:opacity-50 cursor-pointer"
      >
        {sent ? (
          <>
            <Check className="h-3.5 w-3.5 text-emerald-400" />
            <span>Email Sent</span>
          </>
        ) : (
          <>
            <Send className="h-3.5 w-3.5" />
            <span>{sending ? 'Sending...' : 'Resend Link'}</span>
          </>
        )}
      </button>
    </div>
  );
}

