import * as React from 'react';
import { User } from '@/api/types';
import { authApi } from '@/api/auth';
import { ApiException } from '@/api/client';

interface AuthContextType {
  user: User | null;
  loading: boolean;
  error: string | null;
  isAuthenticated: boolean;
  isCustomer: boolean;
  isAdmin: boolean;
  isStaff: boolean;
  can: (permission: string) => boolean;
  login: (email: string, pass: string) => Promise<User>;
  register: (data: { email: string; password: string; full_name: string; company_name: string }) => Promise<User>;
  logout: () => Promise<void>;
  refresh: () => Promise<void>;
  clearError: () => void;
}

export const AuthContext = React.createContext<AuthContextType | undefined>(undefined);

export function AuthProvider({ children }: { children: React.ReactNode }) {
  const [user, setUser] = React.useState<User | null>(null);
  const [loading, setLoading] = React.useState(true);
  const [error, setError] = React.useState<string | null>(null);

  const refresh = React.useCallback(async () => {
    try {
      const currentUser = await authApi.getMe();
      setUser(currentUser);
      setError(null);
    } catch (err: any) {
      if (err instanceof ApiException) {
        if (err.status === 401) {
          setUser(null);
        } else {
          setError(err.message);
        }
      } else {
        setError('Server Error: Failed to connect to Brightloop backend API');
      }
    } finally {
      setLoading(false);
    }
  }, []);

  React.useEffect(() => {
    refresh();
  }, [refresh]);

  const login = async (email: string, pass: string) => {
    setLoading(true);
    try {
      const loggedIn = await authApi.login(email, pass);
      setUser(loggedIn);
      setError(null);
      return loggedIn;
    } catch (err: any) {
      const msg = err instanceof ApiException ? err.message : 'Server Error: Login failed';
      setError(msg);
      throw err;
    } finally {
      setLoading(false);
    }
  };

  const register = async (data: { email: string; password: string; full_name: string; company_name: string }) => {
    setLoading(true);
    try {
      const registered = await authApi.register(data);
      setUser(registered);
      setError(null);
      return registered;
    } catch (err: any) {
      const msg = err instanceof ApiException ? err.message : 'Server Error: Registration failed';
      setError(msg);
      throw err;
    } finally {
      setLoading(false);
    }
  };

  const logout = async () => {
    try {
      await authApi.logout();
    } catch (err) {
      console.warn('Logout error', err);
    } finally {
      setUser(null);
    }
  };

  const can = React.useCallback(
    (permission: string): boolean => {
      if (!user) return false;
      if (user.role === 'ADMIN') return true;
      if (!user.permissions) return false;
      return user.permissions.includes(permission);
    },
    [user]
  );

  const value: AuthContextType = {
    user,
    loading,
    error,
    isAuthenticated: !!user,
    isCustomer: user?.role === 'CUSTOMER',
    isAdmin: user?.role === 'ADMIN',
    isStaff: user?.role === 'STAFF',
    can,
    login,
    register,
    logout,
    refresh,
    clearError: () => setError(null),
  };

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
}

export function useAuth() {
  const context = React.useContext(AuthContext);
  if (!context) {
    throw new Error('useAuth must be used within an AuthProvider');
  }
  return context;
}
