import * as React from 'react';
import { ShieldCheck, ArrowRight, CheckCircle2, AlertCircle, Building2, UserPlus, LogIn, KeyRound } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { useAuth } from '@/context/AuthContext';
import { authApi } from '@/api/auth';
import { useToast } from '@/components/ui/toast';
import { ApiException } from '@/api/client';

export function AuthPage() {
  const { login, register, error: serverError } = useAuth();
  const { toast } = useToast();

  const [mode, setMode] = React.useState<'login' | 'register' | 'forgot'>('login');
  const [loading, setLoading] = React.useState(false);
  const [error, setError] = React.useState<string | null>(null);

  const displayError = error || serverError;

  // Form states
  const [email, setEmail] = React.useState('');
  const [password, setPassword] = React.useState('');
  const [fullName, setFullName] = React.useState('');
  const [companyName, setCompanyName] = React.useState('');
  const [forgotSent, setForgotSent] = React.useState(false);

  // Password rules validation
  const hasMinLength = password.length >= 9;
  const hasUpper = /[A-Z]/.test(password);
  const hasLower = /[a-z]/.test(password);
  const hasSpecial = /[^A-Za-z0-9]/.test(password);
  const isPasswordValid = hasMinLength && hasUpper && hasLower && hasSpecial;

  const handleLogin = async (e: React.FormEvent) => {
    e.preventDefault();
    setLoading(true);
    setError(null);
    try {
      await login(email, password);
      toast({
        title: 'Welcome Back',
        description: 'Successfully authenticated.',
        variant: 'success',
      });
    } catch (err: any) {
      if (err instanceof ApiException) {
        setError(err.message);
      } else {
        setError('Invalid email or password');
      }
    } finally {
      setLoading(false);
    }
  };

  const handleRegister = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!isPasswordValid) return;

    setLoading(true);
    setError(null);
    try {
      await register({
        email,
        password,
        full_name: fullName,
        company_name: companyName,
      });
      toast({
        title: 'Account Created',
        description: 'Welcome to Brightloop. Your session is active.',
        variant: 'success',
      });
    } catch (err: any) {
      if (err instanceof ApiException) {
        setError(err.message);
      } else {
        setError('Registration failed');
      }
    } finally {
      setLoading(false);
    }
  };

  const handleForgotPassword = async (e: React.FormEvent) => {
    e.preventDefault();
    setLoading(true);
    setError(null);
    try {
      await authApi.forgotPassword(email);
      setForgotSent(true);
      toast({
        title: 'Reset Link Sent',
        description: 'If an account exists, password reset instructions have been emailed.',
        variant: 'info',
      });
    } catch {
      setError('Unable to submit request. Please try again.');
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="min-h-screen grid lg:grid-cols-2 bg-background">
      {/* Left Marketing Panel */}
      <div className="hidden lg:flex flex-col justify-between p-12 bg-card border-r relative overflow-hidden">
        <div className="absolute -top-32 -left-32 w-96 h-96 bg-primary/10 rounded-full blur-3xl pointer-events-none" />
        <div className="absolute -bottom-32 -right-32 w-96 h-96 bg-sky-500/10 rounded-full blur-3xl pointer-events-none" />

        <div className="relative z-10 flex items-center gap-3">
          <div className="h-10 w-10 rounded-xl bg-primary text-primary-foreground font-black text-xl flex items-center justify-center shadow-lg shadow-primary/25">
            BL
          </div>
          <div>
            <div className="font-extrabold text-lg tracking-tight">Brightloop Software</div>
            <div className="text-xs text-muted-foreground font-mono">Subscription Platform</div>
          </div>
        </div>

        <div className="relative z-10 space-y-6 max-w-lg">
          <h1 className="text-4xl font-extrabold tracking-tight leading-tight">
            The digital backbone for recurring revenue businesses.
          </h1>
          <p className="text-muted-foreground leading-relaxed text-sm">
            Run your plans, subscriptions, and automated recurring billing with total precision.
            Deterministic integer cents, automated lifecycle workflows, and instant self-service client portals.
          </p>

          <div className="space-y-3 pt-2">
            <div className="flex items-center gap-3 text-xs text-muted-foreground">
              <CheckCircle2 className="h-4 w-4 text-emerald-400 shrink-0" />
              <span>Modular monolith architecture with server-side sessions</span>
            </div>
            <div className="flex items-center gap-3 text-xs text-muted-foreground">
              <CheckCircle2 className="h-4 w-4 text-emerald-400 shrink-0" />
              <span>Complete subscription lifecycle: Draft &rarr; Quotation &rarr; Active</span>
            </div>
            <div className="flex items-center gap-3 text-xs text-muted-foreground">
              <CheckCircle2 className="h-4 w-4 text-emerald-400 shrink-0" />
              <span>Hybrid RBAC with granular per-user overrides</span>
            </div>
          </div>
        </div>

        <div className="relative z-10 text-xs text-muted-foreground font-mono">
          Seed accounts: admin@brightloop.test &bull; finance@brightloop.test
        </div>
      </div>

      {/* Right Form Card */}
      <div className="flex items-center justify-center p-6 md:p-12">
        <div className="w-full max-w-md space-y-6">
          <Card className="border shadow-xl">
            <CardHeader className="space-y-1">
              <div className="flex items-center justify-between">
                <CardTitle className="text-2xl font-bold tracking-tight">
                  {mode === 'login' && 'Sign in to Brightloop'}
                  {mode === 'register' && 'Create customer account'}
                  {mode === 'forgot' && 'Reset your password'}
                </CardTitle>
              </div>
              <CardDescription>
                {mode === 'login' && 'Enter your account credentials to access the platform.'}
                {mode === 'register' && 'Setup your business account and access the customer portal.'}
                {mode === 'forgot' && 'Enter your email and we will dispatch a secure reset link.'}
              </CardDescription>
            </CardHeader>

            <CardContent>
              {displayError && (
                <div className="mb-4 flex items-start gap-2 p-3 text-xs rounded-lg bg-destructive/15 text-rose-300 border border-destructive/30">
                  <AlertCircle className="h-4 w-4 shrink-0 mt-0.5 text-red-400" />
                  <span className="leading-relaxed">{displayError}</span>
                </div>
              )}

              {/* Login Form */}
              {mode === 'login' && (
                <form onSubmit={handleLogin} className="space-y-4">
                  <div className="space-y-1.5">
                    <label className="text-xs font-medium text-muted-foreground">Email Address</label>
                    <Input
                      type="email"
                      required
                      value={email}
                      onChange={(e) => setEmail(e.target.value)}
                      placeholder="admin@brightloop.test"
                    />
                  </div>

                  <div className="space-y-1.5">
                    <div className="flex items-center justify-between">
                      <label className="text-xs font-medium text-muted-foreground">Password</label>
                      <button
                        type="button"
                        onClick={() => {
                          setMode('forgot');
                          setError(null);
                        }}
                        className="text-xs text-primary hover:underline cursor-pointer"
                      >
                        Forgot password?
                      </button>
                    </div>
                    <Input
                      type="password"
                      required
                      value={password}
                      onChange={(e) => setPassword(e.target.value)}
                      placeholder="••••••••••••"
                    />
                  </div>

                  <Button type="submit" className="w-full" disabled={loading}>
                    {loading ? 'Authenticating...' : 'Sign In'}
                    {!loading && <LogIn className="h-4 w-4 ml-2" />}
                  </Button>
                </form>
              )}

              {/* Register Form */}
              {mode === 'register' && (
                <form onSubmit={handleRegister} className="space-y-3.5">
                  <div className="space-y-1">
                    <label className="text-xs font-medium text-muted-foreground">Company Name</label>
                    <Input
                      required
                      value={companyName}
                      onChange={(e) => setCompanyName(e.target.value)}
                      placeholder="Acme Corporation"
                    />
                  </div>

                  <div className="space-y-1">
                    <label className="text-xs font-medium text-muted-foreground">Contact Full Name</label>
                    <Input
                      required
                      value={fullName}
                      onChange={(e) => setFullName(e.target.value)}
                      placeholder="Jane Doe"
                    />
                  </div>

                  <div className="space-y-1">
                    <label className="text-xs font-medium text-muted-foreground">Work Email</label>
                    <Input
                      type="email"
                      required
                      value={email}
                      onChange={(e) => setEmail(e.target.value)}
                      placeholder="jane@acme.test"
                    />
                  </div>

                  <div className="space-y-1">
                    <label className="text-xs font-medium text-muted-foreground">Password</label>
                    <Input
                      type="password"
                      required
                      value={password}
                      onChange={(e) => setPassword(e.target.value)}
                      placeholder="••••••••••••"
                    />
                  </div>

                  {/* Password validation indicators */}
                  <div className="p-2.5 rounded-lg bg-muted/40 border text-[11px] grid grid-cols-2 gap-1 text-muted-foreground">
                    <span className={hasMinLength ? 'text-emerald-400' : ''}>&bull; &gt; 8 characters</span>
                    <span className={hasUpper ? 'text-emerald-400' : ''}>&bull; Uppercase</span>
                    <span className={hasLower ? 'text-emerald-400' : ''}>&bull; Lowercase</span>
                    <span className={hasSpecial ? 'text-emerald-400' : ''}>&bull; Special symbol</span>
                  </div>

                  <Button type="submit" className="w-full" disabled={!isPasswordValid || loading}>
                    {loading ? 'Creating Account...' : 'Register Company'}
                    {!loading && <UserPlus className="h-4 w-4 ml-2" />}
                  </Button>
                </form>
              )}

              {/* Forgot Password Form */}
              {mode === 'forgot' && (
                <div className="space-y-4">
                  {forgotSent ? (
                    <div className="text-center py-4 space-y-3">
                      <div className="h-12 w-12 rounded-full bg-emerald-500/20 text-emerald-400 mx-auto flex items-center justify-center">
                        <CheckCircle2 className="h-6 w-6" />
                      </div>
                      <p className="text-sm font-medium">Password Reset Dispatched</p>
                      <p className="text-xs text-muted-foreground">
                        If an account matches <strong>{email}</strong>, check Mailpit inbox on port 8025 to complete the reset.
                      </p>
                      <Button
                        variant="outline"
                        className="w-full mt-2"
                        onClick={() => {
                          setMode('login');
                          setForgotSent(false);
                        }}
                      >
                        Return to Sign In
                      </Button>
                    </div>
                  ) : (
                    <form onSubmit={handleForgotPassword} className="space-y-4">
                      <div className="space-y-1.5">
                        <label className="text-xs font-medium text-muted-foreground">Registered Email</label>
                        <Input
                          type="email"
                          required
                          value={email}
                          onChange={(e) => setEmail(e.target.value)}
                          placeholder="your-email@company.com"
                        />
                      </div>
                      <Button type="submit" className="w-full" disabled={loading}>
                        {loading ? 'Sending link...' : 'Send Password Reset Link'}
                        {!loading && <KeyRound className="h-4 w-4 ml-2" />}
                      </Button>
                      <Button
                        type="button"
                        variant="ghost"
                        className="w-full text-xs"
                        onClick={() => setMode('login')}
                      >
                        Back to Login
                      </Button>
                    </form>
                  )}
                </div>
              )}

              {/* Switch between Login and Register */}
              {mode !== 'forgot' && (
                <div className="mt-6 pt-4 border-t text-center text-xs text-muted-foreground">
                  {mode === 'login' ? (
                    <span>
                      Don't have an account?{' '}
                      <button
                        type="button"
                        onClick={() => {
                          setMode('register');
                          setError(null);
                        }}
                        className="text-primary font-medium hover:underline cursor-pointer"
                      >
                        Register as customer
                      </button>
                    </span>
                  ) : (
                    <span>
                      Already registered?{' '}
                      <button
                        type="button"
                        onClick={() => {
                          setMode('login');
                          setError(null);
                        }}
                        className="text-primary font-medium hover:underline cursor-pointer"
                      >
                        Sign in
                      </button>
                    </span>
                  )}
                </div>
              )}
            </CardContent>
          </Card>
        </div>
      </div>
    </div>
  );
}

