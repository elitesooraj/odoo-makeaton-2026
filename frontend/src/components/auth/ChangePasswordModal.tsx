import * as React from 'react';
import { Lock, ShieldCheck, AlertCircle } from 'lucide-react';
import { Dialog, DialogHeader, DialogTitle, DialogDescription } from '@/components/ui/dialog';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { useAuth } from '@/context/AuthContext';
import { authApi } from '@/api/auth';
import { useToast } from '@/components/ui/toast';
import { ApiException } from '@/api/client';

interface Props {
  open: boolean;
  onOpenChange?: (open: boolean) => void;
  forced?: boolean;
}

export function ChangePasswordModal({ open, onOpenChange, forced = false }: Props) {
  const { refresh } = useAuth();
  const { toast } = useToast();
  const [currentPassword, setCurrentPassword] = React.useState('');
  const [newPassword, setNewPassword] = React.useState('');
  const [confirmPassword, setConfirmPassword] = React.useState('');
  const [loading, setLoading] = React.useState(false);
  const [error, setError] = React.useState<string | null>(null);

  // Validation rules matching backend:
  // > 8 chars, uppercase, lowercase, special char
  const hasMinLength = newPassword.length >= 9;
  const hasUpper = /[A-Z]/.test(newPassword);
  const hasLower = /[a-z]/.test(newPassword);
  const hasSpecial = /[^A-Za-z0-9]/.test(newPassword);
  const passwordsMatch = newPassword === confirmPassword && newPassword.length > 0;
  const isValid = hasMinLength && hasUpper && hasLower && hasSpecial && passwordsMatch;

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!isValid) return;

    setLoading(true);
    setError(null);
    try {
      await authApi.changePassword(currentPassword, newPassword);
      toast({
        title: 'Password Changed',
        description: 'Your new credentials are now active.',
        variant: 'success',
      });
      await refresh();
      onOpenChange?.(false);
      setCurrentPassword('');
      setNewPassword('');
      setConfirmPassword('');
    } catch (err) {
      if (err instanceof ApiException) {
        setError(err.message);
      } else {
        setError('Failed to update password');
      }
    } finally {
      setLoading(false);
    }
  };

  return (
    <Dialog
      open={open}
      onOpenChange={(val) => {
        if (!forced) {
          onOpenChange?.(val);
        }
      }}
    >
      <div className="space-y-4">
        <DialogHeader>
          <div className="flex items-center gap-2 text-primary">
            <Lock className="h-5 w-5" />
            <DialogTitle>
              {forced ? 'Mandatory Password Reset' : 'Change Password'}
            </DialogTitle>
          </div>
          <DialogDescription>
            {forced
              ? 'Your account was initialized with a temporary password. You must set a permanent secure password to continue.'
              : 'Choose a strong password satisfying the Brightloop security policy.'}
          </DialogDescription>
        </DialogHeader>

        {error && (
          <div className="flex items-center gap-2 p-3 text-xs rounded-md bg-destructive/15 text-rose-300 border border-destructive/30">
            <AlertCircle className="h-4 w-4 shrink-0" />
            <span>{error}</span>
          </div>
        )}

        <form onSubmit={handleSubmit} className="space-y-4">
          <div className="space-y-1">
            <label className="text-xs font-medium text-muted-foreground">Current / Temporary Password</label>
            <Input
              type="password"
              required
              value={currentPassword}
              onChange={(e) => setCurrentPassword(e.target.value)}
              placeholder="••••••••••••"
            />
          </div>

          <div className="space-y-1">
            <label className="text-xs font-medium text-muted-foreground">New Password</label>
            <Input
              type="password"
              required
              value={newPassword}
              onChange={(e) => setNewPassword(e.target.value)}
              placeholder="••••••••••••"
            />
          </div>

          <div className="space-y-1">
            <label className="text-xs font-medium text-muted-foreground">Confirm New Password</label>
            <Input
              type="password"
              required
              value={confirmPassword}
              onChange={(e) => setConfirmPassword(e.target.value)}
              placeholder="••••••••••••"
            />
          </div>

          {/* Policy checks checklist */}
          <div className="p-3 rounded-lg bg-muted/40 border text-xs space-y-1.5">
            <span className="font-semibold text-muted-foreground">Password Requirements:</span>
            <div className="grid grid-cols-2 gap-1.5">
              <span className={`flex items-center gap-1.5 ${hasMinLength ? 'text-emerald-400' : 'text-muted-foreground'}`}>
                <ShieldCheck className="h-3.5 w-3.5" /> More than 8 characters
              </span>
              <span className={`flex items-center gap-1.5 ${hasUpper ? 'text-emerald-400' : 'text-muted-foreground'}`}>
                <ShieldCheck className="h-3.5 w-3.5" /> Uppercase letter
              </span>
              <span className={`flex items-center gap-1.5 ${hasLower ? 'text-emerald-400' : 'text-muted-foreground'}`}>
                <ShieldCheck className="h-3.5 w-3.5" /> Lowercase letter
              </span>
              <span className={`flex items-center gap-1.5 ${hasSpecial ? 'text-emerald-400' : 'text-muted-foreground'}`}>
                <ShieldCheck className="h-3.5 w-3.5" /> Special character
              </span>
              <span className={`flex items-center gap-1.5 col-span-2 ${passwordsMatch ? 'text-emerald-400' : 'text-muted-foreground'}`}>
                <ShieldCheck className="h-3.5 w-3.5" /> Passwords match
              </span>
            </div>
          </div>

          <div className="flex justify-end gap-2 pt-2">
            {!forced && (
              <Button type="button" variant="outline" onClick={() => onOpenChange?.(false)}>
                Cancel
              </Button>
            )}
            <Button type="submit" disabled={!isValid || loading}>
              {loading ? 'Updating...' : 'Set New Password'}
            </Button>
          </div>
        </form>
      </div>
    </Dialog>
  );
}

