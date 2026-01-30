import { useState } from 'react';
import { motion } from 'framer-motion';
import { Zap, Lock, ArrowRight } from 'lucide-react';
import { useAuthStore } from '../hooks/useAuthStore';
import { Input } from '../components/ui/Input';
import { Button } from '../components/ui/Button';
import { toast } from 'sonner';
import { useNavigate, Link } from 'react-router-dom';

export const Login = () => {
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [isLoading, setIsLoading] = useState(false);
  const login = useAuthStore((state) => state.login);
  const navigate = useNavigate();

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

  return (
    <div className="min-h-screen bg-background flex items-center justify-center p- relative overflow-hidden">
        {/ Background Effects /}
        <div className="absolute top-[-%] left-[-%] w-[px] h-[px] bg-blue-/ rounded-full blur-[px]" />
        <div className="absolute bottom-[-%] right-[-%] w-[px] h-[px] bg-purple-/ rounded-full blur-[px]" />

        <motion.div 
            initial={{ opacity: , y:  }}
            animate={{ opacity: , y:  }}
            className="w-full max-w-md bg-surface/ backdrop-blur-xl border border-white/ p- rounded-xl shadow-xl relative z-"
        >
            <div className="flex justify-center mb-">
                <div className="w- h- rounded-xl bg-gradient-to-br from-blue- to-purple- flex items-center justify-center shadow-glow">
                    <Zap className="text-white" fill="currentColor" />
                </div>
            </div>

            <h className="text-xl font-bold text-center text-white mb-">Welcome back</h>
            <p className="text-zinc- text-center mb- text-sm">Enter your credentials to access the secure vault.</p>

            <form onSubmit={handleSubmit} className="space-y-">
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

                <Button className="w-full mt- group" isLoading={isLoading}>
                    Sign In <ArrowRight size={} className="ml- group-hover:translate-x- transition-transform" />
                </Button>
            </form>

            <div className="mt- text-center text-sm">
              <span className="text-zinc-">Don't have an account? </span>
              <Link to="/register" className="text-primary hover:text-blue- font-medium transition-colors">
                Create one
              </Link>
            </div>

            <div className="mt- flex items-center justify-center gap- text-xs text-zinc-">
                <Lock size={} />
                <span>End-to-end encrypted connection</span>
            </div>
        </motion.div>
    </div>
  );
};