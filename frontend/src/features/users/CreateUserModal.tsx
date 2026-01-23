import { useState } from 'react';
import { motion, AnimatePresence } from 'framer-motion';
import { X, Loader2 } from 'lucide-react';
import { Button } from '../../components/ui/Button';
import { Input } from '../../components/ui/Input';
import { toast } from 'sonner';
import { api } from '../../lib/api';

interface CreateUserModalProps {
  isOpen: boolean;
  onClose: () => void;
  onSuccess: () => void;
}

export const CreateUserModal = ({ isOpen, onClose, onSuccess }: CreateUserModalProps) => {
  const [isLoading, setIsLoading] = useState(false);
  const [formData, setFormData] = useState({
    email: '',
    full_name: '',
    username: '',
    password: '',
    confirmPassword: '',
    role: 'viewer',
    group: '',
  });

  const handleChange = (field: string, value: string) => {
    setFormData(prev => ({ ...prev, [field]: value }));
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    
    // Validation
    if (!formData.email || !formData.full_name || !formData.username || !formData.password) {
      toast.error('Please fill in all required fields');
      return;
    }

    if (formData.password !== formData.confirmPassword) {
      toast.error('Passwords do not match');
      return;
    }

    if (formData.password.length < 8) {
      toast.error('Password must be at least 8 characters');
      return;
    }

    setIsLoading(true);
    try {
      await api.post('/users', {
        email: formData.email,
        full_name: formData.full_name,
        username: formData.username,
        password: formData.password,
        role: formData.role,
        group: formData.group || undefined,
      });

      toast.success('User created successfully');
      setFormData({
        email: '',
        full_name: '',
        username: '',
        password: '',
        confirmPassword: '',
        role: 'viewer',
        group: '',
      });
      onSuccess();
      onClose();
    } catch (error: any) {
      toast.error(error.response?.data?.message || 'We couldn't add the new user. Please verify all information is correct and try again.");
    } finally {
      setIsLoading(false);
    }
  };

  return (
    <AnimatePresence>
      {isOpen && (
        <>
          {/* Backdrop */}
          <motion.div
            initial={{ opacity: 0 }}
            animate={{ opacity: 1 }}
            exit={{ opacity: 0 }}
            onClick={onClose}
            className="fixed inset-0 bg-black/50 backdrop-blur-sm z-50"
          />

          {/* Modal */}
          <motion.div
            initial={{ opacity: 0, scale: 0.95, y: 20 }}
            animate={{ opacity: 1, scale: 1, y: 0 }}
            exit={{ opacity: 0, scale: 0.95, y: 20 }}
            className="fixed left-1/2 top-1/2 -translate-x-1/2 -translate-y-1/2 w-full max-w-md z-50"
          >
            <div className="bg-surface border border-border rounded-xl shadow-xl overflow-hidden">
              {/* Header */}
              <div className="bg-gradient-to-r from-primary/20 to-primary/10 border-b border-border px-6 py-4 flex items-center justify-between">
                <h2 className="text-xl font-bold text-white">Create New User</h2>
                <button
                  onClick={onClose}
                  className="p-1 hover:bg-white/10 rounded-lg transition-colors"
                >
                  <X size={20} className="text-zinc-400" />
                </button>
              </div>

              {/* Content */}
              <form onSubmit={handleSubmit} className="p-6 space-y-4 max-h-[calc(100vh-200px)] overflow-y-auto">
                <div>
                  <label className="text-sm font-medium text-white block mb-2">Full Name *</label>
                  <Input
                    placeholder="John Doe"
                    value={formData.full_name}
                    onChange={(e) => handleChange('full_name', e.target.value)}
                    disabled={isLoading}
                  />
                </div>

                <div>
                  <label className="text-sm font-medium text-white block mb-2">Email *</label>
                  <Input
                    type="email"
                    placeholder="john@example.com"
                    value={formData.email}
                    onChange={(e) => handleChange('email', e.target.value)}
                    disabled={isLoading}
                  />
                </div>

                <div>
                  <label className="text-sm font-medium text-white block mb-2">Username *</label>
                  <Input
                    placeholder="johndoe"
                    value={formData.username}
                    onChange={(e) => handleChange('username', e.target.value)}
                    disabled={isLoading}
                  />
                </div>

                <div>
                  <label className="text-sm font-medium text-white block mb-2">Password *</label>
                  <Input
                    type="password"
                    placeholder="••••••••"
                    value={formData.password}
                    onChange={(e) => handleChange('password', e.target.value)}
                    disabled={isLoading}
                  />
                  <p className="text-xs text-zinc-500 mt-1">Minimum 8 characters</p>
                </div>

                <div>
                  <label className="text-sm font-medium text-white block mb-2">Confirm Password *</label>
                  <Input
                    type="password"
                    placeholder="••••••••"
                    value={formData.confirmPassword}
                    onChange={(e) => handleChange('confirmPassword', e.target.value)}
                    disabled={isLoading}
                  />
                </div>

                <div className="grid grid-cols-2 gap-3">
                  <div>
                    <label className="text-sm font-medium text-white block mb-2">Role *</label>
                    <select
                      value={formData.role}
                      onChange={(e) => handleChange('role', e.target.value)}
                      disabled={isLoading}
                      className="w-full bg-zinc-900 border border-border rounded-lg px-3 py-2 text-sm text-white focus:ring-2 focus:ring-primary/50 outline-none"
                    >
                      <option value="viewer">Viewer</option>
                      <option value="analyst">Analyst</option>
                      <option value="admin">Admin</option>
                    </select>
                  </div>

                  <div>
                    <label className="text-sm font-medium text-white block mb-2">Group</label>
                    <Input
                      placeholder="Security Team"
                      value={formData.group}
                      onChange={(e) => handleChange('group', e.target.value)}
                      disabled={isLoading}
                    />
                  </div>
                </div>

                <div className="bg-blue-500/10 border border-blue-500/20 rounded-lg p-3">
                  <p className="text-xs text-blue-300">
                    💡 The user will be sent an email to verify their account and set a custom password.
                  </p>
                </div>
              </form>

              {/* Footer */}
              <div className="bg-white/5 border-t border-border px-6 py-4 flex items-center justify-end gap-3">
                <Button
                  variant="ghost"
                  onClick={onClose}
                  disabled={isLoading}
                >
                  Cancel
                </Button>
                <Button
                  onClick={handleSubmit}
                  disabled={isLoading}
                >
                  {isLoading ? (
                    <>
                      <Loader2 size={16} className="mr-2 animate-spin" />
                      Creating...
                    </>
                  ) : (
                    'Create User'
                  )}
                </Button>
              </div>
            </div>
          </motion.div>
        </>
      )}
    </AnimatePresence>
  );
};
