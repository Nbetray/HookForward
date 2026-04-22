import {
  createContext,
  useContext,
  useEffect,
  useMemo,
  useState,
  type PropsWithChildren,
} from "react";

import { getMe, login, type User } from "./api";

const TOKEN_STORAGE_KEY = "hookforward.accessToken";

type AuthContextValue = {
  token: string | null;
  user: User | null;
  loading: boolean;
  error: string | null;
  signIn: (email: string, password: string) => Promise<void>;
  signInWithToken: (token: string) => Promise<void>;
  signOut: () => void;
};

const AuthContext = createContext<AuthContextValue | null>(null);

export function AuthProvider({ children }: PropsWithChildren) {
  const [token, setToken] = useState<string | null>(() => localStorage.getItem(TOKEN_STORAGE_KEY));
  const [user, setUser] = useState<User | null>(null);
  const [loading, setLoading] = useState<boolean>(Boolean(token));
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!token) {
      setUser(null);
      setLoading(false);
      return;
    }

    let cancelled = false;
    setLoading(true);

    getMe(token)
      .then((result) => {
        if (cancelled) {
          return;
        }
        setUser(result.user);
        setError(null);
      })
      .catch((err: Error) => {
        if (cancelled) {
          return;
        }
        localStorage.removeItem(TOKEN_STORAGE_KEY);
        setToken(null);
        setUser(null);
        setError(err.message);
      })
      .finally(() => {
        if (!cancelled) {
          setLoading(false);
        }
      });

    return () => {
      cancelled = true;
    };
  }, [token]);

  const value = useMemo<AuthContextValue>(
    () => ({
      token,
      user,
      loading,
      error,
      async signIn(email: string, password: string) {
        const result = await login(email, password);
        localStorage.setItem(TOKEN_STORAGE_KEY, result.accessToken);
        setToken(result.accessToken);
        setUser(result.user);
        setError(null);
      },
      async signInWithToken(nextToken: string) {
        localStorage.setItem(TOKEN_STORAGE_KEY, nextToken);
        setToken(nextToken);
        try {
          const result = await getMe(nextToken);
          setUser(result.user);
          setError(null);
        } catch (err) {
          localStorage.removeItem(TOKEN_STORAGE_KEY);
          setToken(null);
          setUser(null);
          throw err;
        }
      },
      signOut() {
        localStorage.removeItem(TOKEN_STORAGE_KEY);
        setToken(null);
        setUser(null);
        setError(null);
      },
    }),
    [token, user, loading, error],
  );

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
}

export function useAuth() {
  const value = useContext(AuthContext);
  if (!value) {
    throw new Error("useAuth must be used inside AuthProvider");
  }
  return value;
}
