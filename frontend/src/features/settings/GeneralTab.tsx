// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

import { useState } from 'react';
import { useAuthStore } from '../../hooks/useAuthStore';
import { Button, Field, Input } from '../../shared/ds';
import { UserLevelCard } from '../gamification/UserLevelCard';
import { toast } from 'sonner';
import { Camera, Save } from 'lucide-react';

export const GeneralTab = () => {
  const { user } = useAuthStore();
  const [isEditing, setIsEditing] = useState(false);
  const [isSaving, setIsSaving] = useState(false);
  const [formData, setFormData] = useState({
    full_name: user?.full_name || '',
    email: user?.email || '',
    bio: user?.bio || '',
    phone: user?.phone || '',
    department: user?.department || '',
    timezone: user?.timezone || 'UTC',
  });

  const handleSave = async () => {
    setIsSaving(true);
    try {
      // TODO: Call API to update user profile
      toast.success('Profile updated successfully');
      setIsEditing(false);
    } catch (error) {
      toast.error("We couldn't save your profile changes. Please verify your information and try again.");
    } finally {
      setIsSaving(false);
    }
  };

  const handleChange = (field: string, value: string) => {
    setFormData(prev => ({ ...prev, [field]: value }));
  };

  return (
    <div className="space-y-8">
      <div>
        <h3 className="text-2xl font-bold text-text-primary mb-1">My Profile</h3>
        <p className="text-text-secondary text-sm">Manage your personal information and track your progress.</p>
      </div>

      {/* Gamification Section */}
      <div className="bg-surface-1/5 backdrop-blur-xl border border-border-strong/10 rounded-2xl p-6">
        <h4 className="text-lg font-bold text-text-primary mb-6">🎮 Your Profile</h4>
        <UserLevelCard />
      </div>

      {/* Profile Information Section */}
      <div className="space-y-6">
        <div className="flex items-center justify-between">
          <h4 className="text-lg font-bold text-text-primary">Account Information</h4>
          <Button 
            variant={isEditing ? "secondary" : "ghost"}
            onClick={() => isEditing ? handleSave() : setIsEditing(true)}
            disabled={isSaving}
          >
            {isEditing ? (
              <>
                <Save size={16} className="mr-2" />
                Save Changes
              </>
            ) : 'Edit Profile'}
          </Button>
        </div>

        <div className="flex items-center gap-6 pb-8 border-b border-border-strong/5">
          <div className="w-24 h-24 rounded-full bg-gradient-to-br from-blue-500 to-purple-600 flex items-center justify-center text-3xl font-bold text-text-primary shadow-glow relative group">
            {formData.full_name?.charAt(0) || 'U'}
            <button className="absolute inset-0 rounded-full bg-surface-overlay opacity-0 group-hover:opacity-100 transition-opacity flex items-center justify-center">
              <Camera size={20} className="text-text-primary" />
            </button>
          </div>
          <div>
            <p className="text-sm text-text-primary font-medium mb-2">{user?.role || 'User'}</p>
            <Button variant="secondary" className="text-sm">
              <Camera size={16} className="mr-2" />
              Change Avatar
            </Button>
            <p className="text-xs text-text-muted mt-2">JPG, GIF or PNG. 1MB max.</p>
          </div>
        </div>

        <form className="space-y-6">
          <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
            <Field label="Full Name">
              <Input
                value={formData.full_name}
                onChange={(e) => handleChange('full_name', e.target.value)}
                disabled={!isEditing}
              />
            </Field>
            <Field label="Email Address">
              <Input
                value={formData.email}
                disabled
                className="cursor-not-allowed"
              />
            </Field>
          </div>

          {isEditing && (
            <>
              <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                <Field label="Phone Number">
                  <Input
                    type="tel"
                    placeholder="+1 (555) 000-0000"
                    value={formData.phone}
                    onChange={(e) => handleChange('phone', e.target.value)}
                  />
                </Field>
                <Field label="Department">
                  <Input
                    placeholder="e.g. Security, IT Operations"
                    value={formData.department}
                    onChange={(e) => handleChange('department', e.target.value)}
                  />
                </Field>
              </div>

              <div className="space-y-2">
                <label className="text-sm font-medium text-text-primary">Bio</label>
                <textarea 
                  className="w-full bg-surface-1 border border-border rounded-lg px-4 py-3 text-sm text-text-primary placeholder:text-text-muted focus:ring-2 focus:ring-primary/50 outline-none resize-none"
                  placeholder="Tell us about yourself..."
                  value={formData.bio}
                  onChange={(e) => handleChange('bio', e.target.value)}
                  rows={4}
                />
              </div>

              <div>
                <label className="text-sm font-medium text-text-primary block mb-2">Timezone</label>
                <select 
                  className="w-full bg-surface-1 border border-border rounded-lg px-4 py-2 text-sm text-text-primary focus:ring-2 focus:ring-primary/50 outline-none"
                  value={formData.timezone}
                  onChange={(e) => handleChange('timezone', e.target.value)}
                >
                  <option>UTC</option>
                  <option>America/New_York</option>
                  <option>America/Chicago</option>
                  <option>America/Denver</option>
                  <option>America/Los_Angeles</option>
                  <option>Europe/London</option>
                  <option>Europe/Paris</option>
                  <option>Asia/Tokyo</option>
                  <option>Australia/Sydney</option>
                </select>
              </div>
            </>
          )}

          {isEditing && (
            <div className="flex gap-3 pt-4">
              <Button onClick={handleSave} disabled={isSaving}>
                {isSaving ? 'Saving...' : 'Save Changes'}
              </Button>
              <Button variant="ghost" onClick={() => setIsEditing(false)}>
                Cancel
              </Button>
            </div>
          )}
        </form>
      </div>

      {/* Preferences Section */}
      <div className="space-y-6 border-t border-border-strong/10 pt-8">
        <h4 className="text-lg font-bold text-text-primary">Preferences</h4>
        
        <div className="space-y-4">
          <div className="flex items-center justify-between p-4 bg-surface-1/5 rounded-lg border border-border-strong/5">
            <div>
              <p className="text-sm font-medium text-text-primary">Email Notifications</p>
              <p className="text-xs text-text-secondary">Receive risk alerts and updates</p>
            </div>
            <input type="checkbox" defaultChecked className="w-5 h-5 rounded cursor-pointer" />
          </div>
          
          <div className="flex items-center justify-between p-4 bg-surface-1/5 rounded-lg border border-border-strong/5">
            <div>
              <p className="text-sm font-medium text-text-primary">Desktop Notifications</p>
              <p className="text-xs text-text-secondary">Get notified in real-time</p>
            </div>
            <input type="checkbox" defaultChecked className="w-5 h-5 rounded cursor-pointer" />
          </div>

          <div className="flex items-center justify-between p-4 bg-surface-1/5 rounded-lg border border-border-strong/5">
            <div>
              <p className="text-sm font-medium text-text-primary">Two-Factor Authentication</p>
              <p className="text-xs text-text-secondary">Enhance your account security</p>
            </div>
            <Button variant="ghost" className="text-sm">Enable</Button>
          </div>
        </div>
      </div>
    </div>
  );
};