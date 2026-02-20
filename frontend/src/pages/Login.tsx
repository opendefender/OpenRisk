import { useState } from 'react';
import { motion, AnimatePresence } from 'framer-motion';
import { Zap, Lock, ArrowRight, Github, Mail, Shield, Key } from 'lucide-react';
import { useAuthStore } from '../hooks/useAuthStore';
import { Input } from '../components/ui/Input';
import { Button } from '../components/ui/Button';
import { toast } from 'sonner';
import { useNavigate, Link, useSearchParams } from 'react-router-dom';

export const Login = () => {
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [isLoading, setIsLoading] = useState(false);
  const [showSSOOptions, setShowSSOOptions] = useState(false);
  const [searchParams] = useSearchParams();
  const login = useAuthStore((state) => state.login);
  const navigate = useNavigate();

  // Check for OAuth callback
  const code = searchParams.get('code');
  const provider = searchParams.get('provider');

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setIsLoading(true);
    try {
      await login(email, password);
      toast.success("Welcome back to OpenRisk");
      navigate('/');
    } catch (err) {
      toast.error("Incorrect email or password. Please check and try again.");
    } finally {
      setIsLoading(false);
    }
  };

  const handleOAuth2Login = (providerName: string) => {
    // In a real app, this would initiate OAuth2 flow
    const redirectUri = `${window.location.origin}/auth/oauth2/callback/${providerName}`;
    const state = Math.random().toString(36).substring(7);
    localStorage.setItem(`oauth_state_${providerName}`, state);

    const params = new URLSearchParams({
      redirect_uri: redirectUri,
      state,
      provider: providerName,
    });

    window.location.href = `/api/v1/auth/oauth2/login/${providerName}?${params.toString()}`;
  };

  const handleSAML2Login = () => {
    // Initiate SAML2 flow
    window.location.href = '/api/v1/auth/saml2/login';
  };

  const ssoProviders = [
    {
      id: 'google',
      name: 'Google',
      icon: Mail,
      color: 'hover:bg-red-900/20 hover:border-red-700',
    },
    {
      id: 'github',
      name: 'GitHub',
      icon: Github,
      color: 'hover:bg-gray-700/20 hover:border-gray-600',
    },
    {
      id: 'azure',
      name: 'Azure AD',
      icon: Shield,
      color: 'hover:bg-blue-900/20 hover:border-blue-700',
    },
    {
      id: 'saml',
      name: 'SAML 2.0',
      icon: Key,
      color: 'hover:bg-purple-900/20 hover:border-purple-700',
    },
  ];

  return (
    <div className="min-h-screen bg-background flex items-center justify-center p-4 relative overflow-hidden">
        {/* Background Effects */}
        <div className="absolute top-[-20%] left-[-10%] w-[500px] h-[500px] bg-blue-600/20 rounded-full blur-[120px]" />
        <div className="absolute bottom-[-20%] right-[-10%] w-[500px] h-[500px] bg-purple-600/20 rounded-full blur-[120px]" />

        <motion.div 
            initial={{ opacity: 0, y: 20 }}
            animate={{ opacity: 1, y: 0 }}
            className="w-full max-w-md bg-surface/50 backdrop-blur-xl border border-white/10 p-8 rounded-2xl shadow-2xl relative z-10"
        >
            <div className="flex justify-center mb-8">
                <div className="w-12 h-12 rounded-xl bg-gradient-to-br from-blue-500 to-purple-600 flex items-center justify-center shadow-glow">
                    <Zap className="text-white" fill="currentColor" />
                </div>
            </div>

            <h1 className="text-2xl font-bold text-center text-white mb-2">Welcome back</h1>
            <p className="text-zinc-400 text-center mb-8 text-sm">Enter your credentials to access the secure vault.</p>

            <form onSubmit={handleSubmit} className="space-y-4">
                <Input 
                    label="Email" 
                    type="email" 
                    placeholder="name@company.com" 
                    value={email}
                    onChange={(e) => setEmail(e.target.value)}
                    autoFocus
                />
                <Input 
                    label="Password" 
                    type="password" 
                    placeholder="••••••••" 
                    value={password}
                    onChange={(e) => setPassword(e.target.value)}
                />

                <Button className="w-full mt-4 group" isLoading={isLoading}>
                    Sign In <ArrowRight size={16} className="ml-2 group-hover:translate-x-1 transition-transform" />
                </Button>
            </form>

            {/* SSO Divider */}
            <div className="mt-6 flex items-center gap-3">
                <div className="flex-1 h-px bg-zinc-700" />
                <span className="text-xs text-zinc-500 px-2">OR</span>
                <div className="flex-1 h-px bg-zinc-700" />
            </div>

            {/* SSO Button */}
            <motion.button
                onClick={() => setShowSSOOptions(!showSSOOptions)}
                className="w-full mt-4 px-4 py-2 border border-zinc-700 rounded-lg text-sm text-zinc-300 hover:bg-zinc-800/50 hover:border-zinc-600 transition-all"
                whileHover={{ scale: 1.02 }}
                whileTap={{ scale: 0.98 }}
            >
                Continue with SSO
            </motion.button>

            {/* SSO Providers */}
            <AnimatePresence>
                {showSSOOptions && (
                    <motion.div
                        initial={{ opacity: 0, height: 0 }}
                        animate={{ opacity: 1, height: 'auto' }}
                        exit={{ opacity: 0, height: 0 }}
                        className="mt-4 grid grid-cols-2 gap-2"
                    >
                        {ssoProviders.map((provider) => {
                            const Icon = provider.icon;
                            return (
                                <motion.button
                                    key={provider.id}
                                    onClick={() =>
                                        provider.id === 'saml'
                                            ? handleSAML2Login()
                                            : handleOAuth2Login(provider.id)
                                    }
                                    className={`flex items-center justify-center gap-2 px-3 py-2 border border-zinc-700 rounded-lg text-xs font-medium text-zinc-300 transition-all ${provider.color}`}
                                    whileHover={{ scale: 1.05 }}
                                    whileTap={{ scale: 0.95 }}
                                >
                                    <Icon size={16} />
                                    <span>{provider.name}</span>
                                </motion.button>
                            );
                        })}
                    </motion.div>
                )}
            </AnimatePresence>

            <div className="mt-6 text-center text-sm">
              <span className="text-zinc-400">Don't have an account? </span>
              <Link to="/register" className="text-primary hover:text-blue-400 font-medium transition-colors">
                Create one
              </Link>
            </div>

            <div className="mt-6 flex items-center justify-center gap-2 text-xs text-zinc-500">
                <Lock size={12} />
                <span>End-to-end encrypted connection</span>
            </div>
        </motion.div>
    </div>
  );
};