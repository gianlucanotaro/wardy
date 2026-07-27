import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { api, type Me } from "./api.ts";

function LoginPage({ onLogin }: { onLogin: () => void }) {
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState("");

  const login = useMutation({
    mutationFn: () => api.post("/api/auth/login", { email, password }),
    onSuccess: onLogin,
    onError: (e: Error) => setError(e.message),
  });

  return (
    <div className="min-h-screen flex items-center justify-center p-4">
      <div className="w-full max-w-sm">
        <h1 className="text-3xl font-bold text-center mb-8 tracking-tight">
          wardy
        </h1>
        <form
          onSubmit={(e) => {
            e.preventDefault();
            setError("");
            login.mutate();
          }}
          className="space-y-4"
        >
          <div>
            <label className="block text-sm font-medium text-gray-300 mb-1">
              Email
            </label>
            <input
              type="email"
              value={email}
              onChange={(e) => setEmail(e.target.value)}
              required
              className="w-full px-3 py-2 bg-gray-800 border border-gray-700 rounded-lg text-white focus:outline-none focus:border-indigo-500"
            />
          </div>
          <div>
            <label className="block text-sm font-medium text-gray-300 mb-1">
              Password
            </label>
            <input
              type="password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              required
              className="w-full px-3 py-2 bg-gray-800 border border-gray-700 rounded-lg text-white focus:outline-none focus:border-indigo-500"
            />
          </div>
          {error && (
            <p className="text-red-400 text-sm">{error}</p>
          )}
          <button
            type="submit"
            disabled={login.isPending}
            className="w-full py-2 px-4 bg-indigo-600 hover:bg-indigo-500 disabled:opacity-50 rounded-lg font-medium transition-colors"
          >
            {login.isPending ? "Signing in…" : "Sign in"}
          </button>
        </form>
      </div>
    </div>
  );
}

function Dashboard({ user }: { user: Me }) {
  const queryClient = useQueryClient();

  const logout = useMutation({
    mutationFn: () => api.post("/api/auth/logout"),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ["me"] }),
  });

  return (
    <div className="min-h-screen p-4">
      <header className="flex items-center justify-between mb-8">
        <h1 className="text-2xl font-bold tracking-tight">wardy</h1>
        <div className="flex items-center gap-4">
          <span className="text-sm text-gray-400">
            {user.display_name || user.email}
          </span>
          <button
            onClick={() => logout.mutate()}
            className="text-sm text-gray-400 hover:text-white transition-colors"
          >
            Sign out
          </button>
        </div>
      </header>

      <div className="text-center py-24 text-gray-500">
        <p className="text-lg">Your wardrobe is empty.</p>
        <p className="text-sm mt-2">Upload your first garment to get started.</p>
        <button className="mt-6 px-6 py-3 bg-indigo-600 hover:bg-indigo-500 rounded-lg font-medium transition-colors text-white">
          Add item
        </button>
      </div>
    </div>
  );
}

export default function App() {
  const { data: user, isLoading } = useQuery<Me>({
    queryKey: ["me"],
    queryFn: () => api.get("/api/auth/me"),
    retry: false,
  });

  const queryClient = useQueryClient();

  if (isLoading) {
    return (
      <div className="min-h-screen flex items-center justify-center">
        <div className="w-6 h-6 border-2 border-indigo-500 border-t-transparent rounded-full animate-spin" />
      </div>
    );
  }

  if (!user) {
    return (
      <LoginPage
        onLogin={() => queryClient.invalidateQueries({ queryKey: ["me"] })}
      />
    );
  }

  return <Dashboard user={user} />;
}
