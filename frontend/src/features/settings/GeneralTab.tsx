import { useState } from 'react';
import { useAuthStore } from '../../hooks/useAuthStore';
import { Button } from '../../components/ui/Button';
import { Input } from '../../components/ui/Input';
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
    <div className="space-y-">
      <div>
        <h className="text-xl font-bold text-white mb-">My Profile</h>
        <p className="text-zinc- text-sm">Manage your personal information and track your progress.</p>
      </div>

      {/ Gamification Section /}
      <div className="bg-white/ backdrop-blur-xl border border-white/ rounded-xl p-">
        <h className="text-lg font-bold text-white mb-"> Your Profile</h>
        <UserLevelCard />
      </div>

      {/ Profile Information Section /}
      <div className="space-y-">
        <div className="flex items-center justify-between">
          <h className="text-lg font-bold text-white">Account Information</h>
          <Button 
            variant={isEditing ? "secondary" : "ghost"}
            onClick={() => isEditing ? handleSave() : setIsEditing(true)}
            disabled={isSaving}
          >
            {isEditing ? (
              <>
                <Save size={} className="mr-" />
                Save Changes
              </>
            ) : 'Edit Profile'}
          </Button>
        </div>

        <div className="flex items-center gap- pb- border-b border-white/">
          <div className="w- h- rounded-full bg-gradient-to-br from-blue- to-purple- flex items-center justify-center text-xl font-bold text-white shadow-glow relative group">
            {formData.full_name?.charAt() || 'U'}
            <button className="absolute inset- rounded-full bg-black/ opacity- group-hover:opacity- transition-opacity flex items-center justify-center">
              <Camera size={} className="text-white" />
            </button>
          </div>
          <div>
            <p className="text-sm text-white font-medium mb-">{user?.role || 'User'}</p>
            <Button variant="secondary" className="text-sm">
              <Camera size={} className="mr-" />
              Change Avatar
            </Button>
            <p className="text-xs text-zinc- mt-">JPG, GIF or PNG. MB max.</p>
          </div>
        </div>

        <form className="space-y-">
          <div className="grid grid-cols- md:grid-cols- gap-">
            <Input 
              label="Full Name" 
              value={formData.full_name}
              onChange={(e) => handleChange('full_name', e.target.value)}
              disabled={!isEditing}
            />
            <Input 
              label="Email Address" 
              value={formData.email}
              disabled
              className="cursor-not-allowed"
            />
          </div>

          {isEditing && (
            <>
              <div className="grid grid-cols- md:grid-cols- gap-">
                <Input 
                  label="Phone Number" 
                  type="tel"
                  placeholder="+ () -"
                  value={formData.phone}
                  onChange={(e) => handleChange('phone', e.target.value)}
                />
                <Input 
                  label="Department" 
                  placeholder="e.g. Security, IT Operations"
                  value={formData.department}
                  onChange={(e) => handleChange('department', e.target.value)}
                />
              </div>

              <div className="space-y-">
                <label className="text-sm font-medium text-white">Bio</label>
                <textarea 
                  className="w-full bg-zinc- border border-border rounded-lg px- py- text-sm text-white placeholder:text-zinc- focus:ring- focus:ring-primary/ outline-none resize-none"
                  placeholder="Tell us about yourself..."
                  value={formData.bio}
                  onChange={(e) => handleChange('bio', e.target.value)}
                  rows={}
                />
              </div>

              <div>
                <label className="text-sm font-medium text-white block mb-">Timezone</label>
                <select 
                  className="w-full bg-zinc- border border-border rounded-lg px- py- text-sm text-white focus:ring- focus:ring-primary/ outline-none"
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
            <div className="flex gap- pt-">
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

      {/ Preferences Section /}
      <div className="space-y- border-t border-white/ pt-">
        <h className="text-lg font-bold text-white">Preferences</h>
        
        <div className="space-y-">
          <div className="flex items-center justify-between p- bg-white/ rounded-lg border border-white/">
            <div>
              <p className="text-sm font-medium text-white">Email Notifications</p>
              <p className="text-xs text-zinc-">Receive risk alerts and updates</p>
            </div>
            <input type="checkbox" defaultChecked className="w- h- rounded cursor-pointer" />
          </div>
          
          <div className="flex items-center justify-between p- bg-white/ rounded-lg border border-white/">
            <div>
              <p className="text-sm font-medium text-white">Desktop Notifications</p>
              <p className="text-xs text-zinc-">Get notified in real-time</p>
            </div>
            <input type="checkbox" defaultChecked className="w- h- rounded cursor-pointer" />
          </div>

          <div className="flex items-center justify-between p- bg-white/ rounded-lg border border-white/">
            <div>
              <p className="text-sm font-medium text-white">Two-Factor Authentication</p>
              <p className="text-xs text-zinc-">Enhance your account security</p>
            </div>
            <Button variant="ghost" className="text-sm">Enable</Button>
          </div>
        </div>
      </div>
    </div>
  );
};