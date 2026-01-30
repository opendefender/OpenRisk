import { useState } from 'react';
import { motion, AnimatePresence } from 'framer-motion';
import { X, Loader } from 'lucide-react';
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

    if (formData.password.length < ) {
      toast.error('Password must be at least  characters');
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
      toast.error(error.response?.data?.message || "We couldn't add the new user. Please verify all information is correct and try again.");
    } finally {
      setIsLoading(false);
    }
  };

  return (
    <AnimatePresence>
      {isOpen && (
        <>
          {/ Backdrop /}
          <motion.div
            initial={{ opacity:  }}
            animate={{ opacity:  }}
            exit={{ opacity:  }}
            onClick={onClose}
            className="fixed inset- bg-black/ backdrop-blur-sm z-"
          />

          {/ Modal /}
          <motion.div
            initial={{ opacity: , scale: ., y:  }}
            animate={{ opacity: , scale: , y:  }}
            exit={{ opacity: , scale: ., y:  }}
            className="fixed left-/ top-/ -translate-x-/ -translate-y-/ w-full max-w-md z-"
          >
            <div className="bg-surface border border-border rounded-xl shadow-xl overflow-hidden">
              {/ Header /}
              <div className="bg-gradient-to-r from-primary/ to-primary/ border-b border-border px- py- flex items-center justify-between">
                <h className="text-xl font-bold text-white">Create New User</h>
                <button
                  onClick={onClose}
                  className="p- hover:bg-white/ rounded-lg transition-colors"
                >
                  <X size={} className="text-zinc-" />
                </button>
              </div>

              {/ Content /}
              <form onSubmit={handleSubmit} className="p- space-y- max-h-[calc(vh-px)] overflow-y-auto">
                <div>
                  <label className="text-sm font-medium text-white block mb-">Full Name </label>
                  <Input
                    placeholder="John Doe"
                    value={formData.full_name}
                    onChange={(e) => handleChange('full_name', e.target.value)}
                    disabled={isLoading}
                  />
                </div>

                <div>
                  <label className="text-sm font-medium text-white block mb-">Email </label>
                  <Input
                    type="email"
                    placeholder="john@example.com"
                    value={formData.email}
                    onChange={(e) => handleChange('email', e.target.value)}
                    disabled={isLoading}
                  />
                </div>

                <div>
                  <label className="text-sm font-medium text-white block mb-">Username </label>
                  <Input
                    placeholder="johndoe"
                    value={formData.username}
                    onChange={(e) => handleChange('username', e.target.value)}
                    disabled={isLoading}
                  />
                </div>

                <div>
                  <label className="text-sm font-medium text-white block mb-">Password </label>
                  <Input
                    type="password"
                    placeholder="••••••••"
                    value={formData.password}
                    onChange={(e) => handleChange('password', e.target.value)}
                    disabled={isLoading}
                  />
                  <p className="text-xs text-zinc- mt-">Minimum  characters</p>
                </div>

                <div>
                  <label className="text-sm font-medium text-white block mb-">Confirm Password </label>
                  <Input
                    type="password"
                    placeholder="••••••••"
                    value={formData.confirmPassword}
                    onChange={(e) => handleChange('confirmPassword', e.target.value)}
                    disabled={isLoading}
                  />
                </div>

                <div className="grid grid-cols- gap-">
                  <div>
                    <label className="text-sm font-medium text-white block mb-">Role </label>
                    <select
                      value={formData.role}
                      onChange={(e) => handleChange('role', e.target.value)}
                      disabled={isLoading}
                      className="w-full bg-zinc- border border-border rounded-lg px- py- text-sm text-white focus:ring- focus:ring-primary/ outline-none"
                    >
                      <option value="viewer">Viewer</option>
                      <option value="analyst">Analyst</option>
                      <option value="admin">Admin</option>
                    </select>
                  </div>

                  <div>
                    <label className="text-sm font-medium text-white block mb-">Group</label>
                    <Input
                      placeholder="Security Team"
                      value={formData.group}
                      onChange={(e) => handleChange('group', e.target.value)}
                      disabled={isLoading}
                    />
                  </div>
                </div>

                <div className="bg-blue-/ border border-blue-/ rounded-lg p-">
                  <p className="text-xs text-blue-">
                     The user will be sent an email to verify their account and set a custom password.
                  </p>
                </div>
              </form>

              {/ Footer /}
              <div className="bg-white/ border-t border-border px- py- flex items-center justify-end gap-">
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
                      <Loader size={} className="mr- animate-spin" />
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
