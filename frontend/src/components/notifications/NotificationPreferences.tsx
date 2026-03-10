import React, { useEffect, useState } from 'react';
import axios from 'axios';
import './NotificationPreferences.css';

interface Preferences {
  email_on_mitigation_deadline: boolean;
  email_on_critical_risk: boolean;
  email_on_action_assigned: boolean;
  email_deadline_advance_days: number;
  slack_enabled: boolean;
  slack_on_mitigation_deadline: boolean;
  slack_on_critical_risk: boolean;
  slack_on_action_assigned: boolean;
  webhook_enabled: boolean;
  webhook_on_mitigation_deadline: boolean;
  webhook_on_critical_risk: boolean;
  webhook_on_action_assigned: boolean;
  disable_all_notifications: boolean;
  enable_sound_notifications: boolean;
  enable_desktop_notifications: boolean;
}

interface NotificationPreferencesProps {
  authToken: string;
  onClose: () => void;
}

export const NotificationPreferences: React.FC<NotificationPreferencesProps> =
  ({ authToken, onClose }) => {
    const [preferences, setPreferences] = useState<Preferences | null>(null);
    const [loading, setLoading] = useState(true);
    const [saving, setSaving] = useState(false);
    const [error, setError] = useState<string | null>(null);
    const [success, setSuccess] = useState(false);

    useEffect(() => {
      fetchPreferences();
    }, []);

    const fetchPreferences = async () => {
      try {
        setLoading(true);
        const response = await axios.get('/api/v1/notifications/preferences', {
          headers: { Authorization: `Bearer ${authToken}` },
        });
        setPreferences(response.data);
      } catch (err) {
        setError(
          err instanceof Error ? err.message : 'Failed to fetch preferences'
        );
      } finally {
        setLoading(false);
      }
    };

    const handleToggle = (key: keyof Preferences) => {
      if (preferences) {
        setPreferences({
          ...preferences,
          [key]: !preferences[key],
        });
      }
    };

    const handleNumberChange = (key: keyof Preferences, value: number) => {
      if (preferences) {
        setPreferences({
          ...preferences,
          [key]: value,
        });
      }
    };

    const handleSave = async () => {
      if (!preferences) return;

      try {
        setSaving(true);
        setError(null);

        await axios.patch('/api/v1/notifications/preferences', preferences, {
          headers: { Authorization: `Bearer ${authToken}` },
        });

        setSuccess(true);
        setTimeout(() => setSuccess(false), 3000);
      } catch (err) {
        setError(
          err instanceof Error ? err.message : 'Failed to save preferences'
        );
      } finally {
        setSaving(false);
      }
    };

    const handleTestNotification = async (channel: 'email' | 'slack' | 'webhook') => {
      try {
        await axios.post(
          '/api/v1/notifications/test',
          { channel },
          { headers: { Authorization: `Bearer ${authToken}` } }
        );
        setSuccess(true);
        setTimeout(() => setSuccess(false), 3000);
      } catch (err) {
        setError(`Failed to send test notification: ${err}`);
      }
    };

    if (loading) return <div className="preferences-loading">Loading...</div>;

    if (!preferences)
      return <div className="preferences-error">Failed to load preferences</div>;

    return (
      <div className="notification-preferences">
        <div className="preferences-header">
          <h2>Notification Preferences</h2>
          <button
            className="close-button"
            onClick={onClose}
            aria-label="Close preferences"
          >
            ✕
          </button>
        </div>

        {error && <div className="preferences-error">{error}</div>}
        {success && (
          <div className="preferences-success">
            Preferences saved successfully!
          </div>
        )}

        <div className="preferences-content">
          {/* Global Settings */}
          <section className="preferences-section">
            <h3>Global Settings</h3>

            <div className="preference-item">
              <label>
                <input
                  type="checkbox"
                  checked={preferences.disable_all_notifications}
                  onChange={() => handleToggle('disable_all_notifications')}
                />
                <span>Disable all notifications</span>
              </label>
            </div>

            <div className="preference-item">
              <label>
                <input
                  type="checkbox"
                  checked={preferences.enable_sound_notifications}
                  onChange={() => handleToggle('enable_sound_notifications')}
                />
                <span>Enable sound notifications</span>
              </label>
            </div>

            <div className="preference-item">
              <label>
                <input
                  type="checkbox"
                  checked={preferences.enable_desktop_notifications}
                  onChange={() =>
                    handleToggle('enable_desktop_notifications')
                  }
                />
                <span>Enable desktop notifications</span>
              </label>
            </div>
          </section>

          {/* Email Settings */}
          <section className="preferences-section">
            <h3>Email Notifications</h3>

            <div className="preference-item">
              <label>
                <input
                  type="checkbox"
                  checked={preferences.email_on_mitigation_deadline}
                  onChange={() =>
                    handleToggle('email_on_mitigation_deadline')
                  }
                />
                <span>Mitigation Deadline</span>
              </label>
              {preferences.email_on_mitigation_deadline && (
                <div className="sub-setting">
                  <label>
                    Advance notice (days):
                    <input
                      type="number"
                      min="1"
                      max="30"
                      value={preferences.email_deadline_advance_days}
                      onChange={(e) =>
                        handleNumberChange(
                          'email_deadline_advance_days',
                          parseInt(e.target.value)
                        )
                      }
                    />
                  </label>
                </div>
              )}
            </div>

            <div className="preference-item">
              <label>
                <input
                  type="checkbox"
                  checked={preferences.email_on_critical_risk}
                  onChange={() => handleToggle('email_on_critical_risk')}
                />
                <span>Critical Risk Detection</span>
              </label>
            </div>

            <div className="preference-item">
              <label>
                <input
                  type="checkbox"
                  checked={preferences.email_on_action_assigned}
                  onChange={() => handleToggle('email_on_action_assigned')}
                />
                <span>Action Assigned</span>
              </label>
            </div>

            <button
              className="test-button"
              onClick={() => handleTestNotification('email')}
            >
              Send Test Email
            </button>
          </section>

          {/* Slack Settings */}
          <section className="preferences-section">
            <h3>Slack Notifications</h3>

            <div className="preference-item">
              <label>
                <input
                  type="checkbox"
                  checked={preferences.slack_enabled}
                  onChange={() => handleToggle('slack_enabled')}
                />
                <span>Enable Slack Notifications</span>
              </label>
            </div>

            {preferences.slack_enabled && (
              <>
                <div className="preference-item">
                  <label>
                    <input
                      type="checkbox"
                      checked={preferences.slack_on_mitigation_deadline}
                      onChange={() =>
                        handleToggle('slack_on_mitigation_deadline')
                      }
                    />
                    <span>Mitigation Deadline</span>
                  </label>
                </div>

                <div className="preference-item">
                  <label>
                    <input
                      type="checkbox"
                      checked={preferences.slack_on_critical_risk}
                      onChange={() => handleToggle('slack_on_critical_risk')}
                    />
                    <span>Critical Risk Detection</span>
                  </label>
                </div>

                <div className="preference-item">
                  <label>
                    <input
                      type="checkbox"
                      checked={preferences.slack_on_action_assigned}
                      onChange={() =>
                        handleToggle('slack_on_action_assigned')
                      }
                    />
                    <span>Action Assigned</span>
                  </label>
                </div>

                <button
                  className="test-button"
                  onClick={() => handleTestNotification('slack')}
                >
                  Send Test Slack Message
                </button>
              </>
            )}
          </section>

          {/* Webhook Settings */}
          <section className="preferences-section">
            <h3>Webhook Notifications</h3>

            <div className="preference-item">
              <label>
                <input
                  type="checkbox"
                  checked={preferences.webhook_enabled}
                  onChange={() => handleToggle('webhook_enabled')}
                />
                <span>Enable Webhook Notifications</span>
              </label>
            </div>

            {preferences.webhook_enabled && (
              <>
                <div className="preference-item">
                  <label>
                    <input
                      type="checkbox"
                      checked={preferences.webhook_on_mitigation_deadline}
                      onChange={() =>
                        handleToggle('webhook_on_mitigation_deadline')
                      }
                    />
                    <span>Mitigation Deadline</span>
                  </label>
                </div>

                <div className="preference-item">
                  <label>
                    <input
                      type="checkbox"
                      checked={preferences.webhook_on_critical_risk}
                      onChange={() => handleToggle('webhook_on_critical_risk')}
                    />
                    <span>Critical Risk Detection</span>
                  </label>
                </div>

                <div className="preference-item">
                  <label>
                    <input
                      type="checkbox"
                      checked={preferences.webhook_on_action_assigned}
                      onChange={() =>
                        handleToggle('webhook_on_action_assigned')
                      }
                    />
                    <span>Action Assigned</span>
                  </label>
                </div>

                <button
                  className="test-button"
                  onClick={() => handleTestNotification('webhook')}
                >
                  Send Test Webhook
                </button>
              </>
            )}
          </section>
        </div>

        <div className="preferences-footer">
          <button
            className="cancel-button"
            onClick={onClose}
            disabled={saving}
          >
            Cancel
          </button>
          <button
            className="save-button"
            onClick={handleSave}
            disabled={saving}
          >
            {saving ? 'Saving...' : 'Save Preferences'}
          </button>
        </div>
      </div>
    );
  };

export default NotificationPreferences;
