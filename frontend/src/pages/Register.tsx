import { useState } from 'react';
import { motion } from 'framer-motion';
import { Zap, Lock, ArrowRight, AlertCircle } from 'lucide-react';
import { Input } from '../components/ui/Input';
import { Button } from '../components/ui/Button';
import { toast } from 'sonner';
import { useNavigate, Link } from 'react-router-dom';
import { api } from '../lib/api';

export const Register = () => {
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [confirmPassword, setConfirmPassword] = useState('');
  const [fullName, setFullName] = useState('');
  const [username, setUsername] = useState('');
  const [isLoading, setIsLoading] = useState(false);
  const [errors, setErrors] = useState<Record<string, string>>({});
  const navigate = useNavigate();

  const validateForm = (): boolean => {
    const newErrors: Record<string, string> = {};

    if (!fullName.trim()) {
      newErrors.fullName = 'Full name is required';
    }

    if (!username.trim()) {
      newErrors.username = 'Username is required';
    } else if (username.length < ) {
      newErrors.username = 'Username must be at least  characters';
    }

    if (!email.trim()) {
      newErrors.email = 'Email is required';
    } else if (!/[\s@]+@[\s@]+\.[\s@]+$/.test(email)) {
      newErrors.email = 'Please enter a valid email';
    }

    if (!password) {
      newErrors.password = 'Password is required';
    } else if (password.length < ) {
      newErrors.password = 'Password must be at least  characters';
    }

    if (password !== confirmPassword) {
      newErrors.confirmPassword = 'Passwords do not match';
    }

    setErrors(newErrors);
    return Object.keys(newErrors).length === ;
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    
    if (!validateForm()) {
      toast.error('Please fix the errors below');
      return;
    }

    setIsLoading(true);
    try {
      await api.post('/auth/register', {
        email,
        password,
        username,
        full_name: fullName,
      });

      toast.success('Account created successfully! Redirecting to login...');
      setTimeout(() => navigate('/login'), );
    } catch (err: any) {
      const status = err?.response?.status;
      const message = err?.response?.data?.error || 'Registration failed';
      
      if (status === ) {
        setErrors({ email: 'Email or username already in use' });
      } else if (status === ) {
        setErrors({ form: message });
      } else {
        toast.error(message);
      }
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

        <h className="text-xl font-bold text-center text-white mb-">Create Account</h>
        <p className="text-zinc- text-center mb- text-sm">Join OpenRisk to manage risks and mitigations</p>

        {errors.form && (
          <div className="mb- p- bg-red-/ border border-red-/ rounded-lg flex items-start gap-">
            <AlertCircle size={} className="text-red- flex-shrink- mt-." />
            <p className="text-sm text-red-">{errors.form}</p>
          </div>
        )}

        <form onSubmit={handleSubmit} className="space-y-">
          <div>
            <Input 
              label="Full Name" 
              type="text" 
              placeholder="John Doe" 
              value={fullName}
              onChange={(e) => {
                setFullName(e.target.value);
                if (errors.fullName) setErrors({ ...errors, fullName: '' });
              }}
              error={errors.fullName}
            />
          </div>

          <div>
            <Input 
              label="Username" 
              type="text" 
              placeholder="johndoe" 
              value={username}
              onChange={(e) => {
                setUsername(e.target.value);
                if (errors.username) setErrors({ ...errors, username: '' });
              }}
              error={errors.username}
            />
          </div>

          <div>
            <Input 
              label="Email" 
              type="email" 
              placeholder="name@company.com" 
              value={email}
              onChange={(e) => {
                setEmail(e.target.value);
                if (errors.email) setErrors({ ...errors, email: '' });
              }}
              error={errors.email}
            />
          </div>

          <div>
            <Input 
              label="Password" 
              type="password" 
              placeholder="••••••••" 
              value={password}
              onChange={(e) => {
                setPassword(e.target.value);
                if (errors.password) setErrors({ ...errors, password: '' });
              }}
              error={errors.password}
            />
            <p className="text-xs text-zinc- mt-">Minimum  characters</p>
          </div>

          <div>
            <Input 
              label="Confirm Password" 
              type="password" 
              placeholder="••••••••" 
              value={confirmPassword}
              onChange={(e) => {
                setConfirmPassword(e.target.value);
                if (errors.confirmPassword) setErrors({ ...errors, confirmPassword: '' });
              }}
              error={errors.confirmPassword}
            />
          </div>

          <Button className="w-full mt- group" isLoading={isLoading}>
            Create Account <ArrowRight size={} className="ml- group-hover:translate-x- transition-transform" />
          </Button>
        </form>

        <div className="mt- text-center text-sm">
          <span className="text-zinc-">Already have an account? </span>
          <Link to="/login" className="text-primary hover:text-blue- font-medium transition-colors">
            Sign In
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
