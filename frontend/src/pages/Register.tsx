// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

import { useState } from 'react';
import { motion } from 'framer-motion';
import { Zap, Lock, ArrowRight, AlertCircle } from 'lucide-react';
import { Button, Field, Input } from '../shared/ds';
import { toast } from 'sonner';
import { useNavigate, Link } from 'react-router';
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
    } else if (username.length < 3) {
      newErrors.username = 'Username must be at least 3 characters';
    }

    if (!email.trim()) {
      newErrors.email = 'Email is required';
    } else if (!/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(email)) {
      newErrors.email = 'Please enter a valid email';
    }

    if (!password) {
      newErrors.password = 'Password is required';
    } else if (password.length < 8) {
      newErrors.password = 'Password must be at least 8 characters';
    }

    if (password !== confirmPassword) {
      newErrors.confirmPassword = 'Passwords do not match';
    }

    setErrors(newErrors);
    return Object.keys(newErrors).length === 0;
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
      setTimeout(() => navigate('/login'), 1500);
    } catch (err: any) {
      const status = err?.response?.status;
      const message = err?.response?.data?.error || 'Registration failed';
      
      if (status === 409) {
        setErrors({ email: 'Email or username already in use' });
      } else if (status === 400) {
        setErrors({ form: message });
      } else {
        toast.error(message);
      }
    } finally {
      setIsLoading(false);
    }
  };

  return (
    <div className="min-h-screen bg-background flex items-center justify-center p-4 relative overflow-hidden">
      {/* Background Effects */}
      <div className="absolute top-[-20%] left-[-10%] w-[500px] h-[500px] bg-accent-soft rounded-full blur-[120px]" />
      <div className="absolute bottom-[-20%] right-[-10%] w-[500px] h-[500px] bg-purple-600/20 rounded-full blur-[120px]" />

      <motion.div 
        initial={{ opacity: 0, y: 20 }}
        animate={{ opacity: 1, y: 0 }}
        className="w-full max-w-md bg-surface/50 backdrop-blur-xl border border-border-strong/10 p-8 rounded-2xl shadow-2xl relative z-10"
      >
        <div className="flex justify-center mb-8">
          <div className="w-12 h-12 rounded-xl bg-linear-to-br from-blue-500 to-purple-600 flex items-center justify-center shadow-glow">
            <Zap className="text-text-primary" fill="currentColor" />
          </div>
        </div>

        <h1 className="text-2xl font-bold text-center text-text-primary mb-2">Create Account</h1>
        <p className="text-text-secondary text-center mb-8 text-sm">Join OpenRisk to manage risks and mitigations</p>

        {errors.form && (
          <div className="mb-6 p-3 bg-danger/10 border border-danger/20 rounded-lg flex items-start gap-3">
            <AlertCircle size={16} className="text-danger-text shrink-0 mt-0.5" />
            <p className="text-sm text-danger-text">{errors.form}</p>
          </div>
        )}

        <form onSubmit={handleSubmit} className="space-y-4">
          <div>
            <Field label="Full Name" message={errors.fullName} status={errors.fullName ? 'invalid' : 'default'}>
              <Input
                type="text"
                placeholder="John Doe"
                value={fullName}
                onChange={(e) => {
                  setFullName(e.target.value);
                  if (errors.fullName) setErrors({ ...errors, fullName: '' });
                }}
              />
            </Field>
          </div>

          <div>
            <Field label="Username" message={errors.username} status={errors.username ? 'invalid' : 'default'}>
              <Input
                type="text"
                placeholder="johndoe"
                value={username}
                onChange={(e) => {
                  setUsername(e.target.value);
                  if (errors.username) setErrors({ ...errors, username: '' });
                }}
              />
            </Field>
          </div>

          <div>
            <Field label="Email" message={errors.email} status={errors.email ? 'invalid' : 'default'}>
              <Input
                type="email"
                placeholder="name@company.com"
                value={email}
                onChange={(e) => {
                  setEmail(e.target.value);
                  if (errors.email) setErrors({ ...errors, email: '' });
                }}
              />
            </Field>
          </div>

          <div>
            <Field label="Password" message={errors.password} status={errors.password ? 'invalid' : 'default'}>
              <Input
                type="password"
                placeholder="••••••••"
                value={password}
                onChange={(e) => {
                  setPassword(e.target.value);
                  if (errors.password) setErrors({ ...errors, password: '' });
                }}
              />
            </Field>
            <p className="text-xs text-text-muted mt-1">Minimum 8 characters</p>
          </div>

          <div>
            <Field label="Confirm Password" message={errors.confirmPassword} status={errors.confirmPassword ? 'invalid' : 'default'}>
              <Input
                type="password"
                placeholder="••••••••"
                value={confirmPassword}
                onChange={(e) => {
                  setConfirmPassword(e.target.value);
                  if (errors.confirmPassword) setErrors({ ...errors, confirmPassword: '' });
                }}
              />
            </Field>
          </div>

          <Button variant="primary" type="submit" className="w-full mt-6 group" loading={isLoading}>
            Create Account <ArrowRight size={16} className="ml-2 group-hover:translate-x-1 transition-transform" />
          </Button>
        </form>

        <div className="mt-6 text-center text-sm">
          <span className="text-text-secondary">Already have an account? </span>
          <Link to="/login" className="text-primary hover:text-info-text font-medium transition-colors">
            Sign In
          </Link>
        </div>

        <div className="mt-6 flex items-center justify-center gap-2 text-xs text-text-muted">
          <Lock size={12} />
          <span>End-to-end encrypted connection</span>
        </div>
      </motion.div>
    </div>
  );
};
