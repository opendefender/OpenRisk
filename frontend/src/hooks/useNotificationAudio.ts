import { useCallback, useRef } from 'react';

export interface UseNotificationAudioOptions {
  enabled?: boolean;
  soundUrl?: string;
  volume?: number; // 0 to 1
}

/**
 * Custom hook for handling notification sounds and desktop notifications
 * Provides Web Notifications API integration with fallback to audio
 * 
 * @example
 * const { playSound, sendDesktopNotification, requestPermission } = useNotificationAudio({
 *   enabled: true,
 *   volume: 0.7,
 * });
 * 
 * // Request permission first
 * await requestPermission();
 * 
 * // Play sound
 * playSound();
 * 
 * // Send desktop notification
 * sendDesktopNotification({
 *   title: 'Critical Risk Alert',
 *   options: {
 *     body: 'A critical risk has been detected',
 *     icon: '/icons/alert.png',
 *   },
 * });
 */
export const useNotificationAudio = ({
  enabled = true,
  soundUrl = '/sounds/notification.mp3',
  volume = 0.5,
}: UseNotificationAudioOptions = {}) => {
  const audioRef = useRef<HTMLAudioElement | null>(null);
  const notificationRef = useRef<Notification | null>(null);

  /**
   * Check if browser supports Web Notifications API
   */
  const isNotificationSupported = useCallback(() => {
    return 'Notification' in window;
  }, []);

  /**
   * Check if browser supports audio playback
   */
  const isAudioSupported = useCallback(() => {
    return 'Audio' in window;
  }, []);

  /**
   * Request permission from user for desktop notifications
   */
  const requestPermission = useCallback(async (): Promise<NotificationPermission> => {
    if (!isNotificationSupported()) {
      console.warn('[Notification] Web Notifications API not supported');
      return 'denied';
    }

    if (Notification.permission === 'granted') {
      return 'granted';
    }

    if (Notification.permission !== 'denied') {
      try {
        const permission = await Notification.requestPermission();
        return permission;
      } catch (error) {
        console.error('[Notification] Failed to request permission:', error);
        return 'denied';
      }
    }

    return Notification.permission;
  }, [isNotificationSupported]);

  /**
   * Play notification sound
   */
  const playSound = useCallback(() => {
    if (!enabled || !isAudioSupported()) {
      return;
    }

    try {
      // Create audio element if not exists
      if (!audioRef.current) {
        audioRef.current = new Audio(soundUrl);
        audioRef.current.volume = Math.max(0, Math.min(1, volume)); // Clamp 0-1
      }

      // Reset and play
      audioRef.current.currentTime = 0;
      audioRef.current.play().catch((error) => {
        console.warn('[Audio] Failed to play notification sound:', error);
      });
    } catch (error) {
      console.error('[Audio] Error playing sound:', error);
    }
  }, [enabled, soundUrl, volume, isAudioSupported]);

  /**
   * Stop notification sound
   */
  const stopSound = useCallback(() => {
    if (audioRef.current) {
      audioRef.current.pause();
      audioRef.current.currentTime = 0;
    }
  }, []);

  /**
   * Send desktop notification
   */
  const sendDesktopNotification = useCallback(
    (title: string, options?: NotificationOptions) => {
      if (!enabled || !isNotificationSupported()) {
        return;
      }

      // Check permission
      if (Notification.permission !== 'granted') {
        console.warn('[Notification] Permission not granted for desktop notifications');
        return;
      }

      try {
        // Close previous notification if exists
        if (notificationRef.current) {
          notificationRef.current.close();
        }

        // Create new notification
        notificationRef.current = new Notification(title, {
          icon: '/icons/notification.png',
          badge: '/icons/badge.png',
          ...options,
        });

        // Auto-close after 5 seconds if not clicked
        const autoCloseTimeout = setTimeout(() => {
          if (notificationRef.current) {
            notificationRef.current.close();
          }
        }, 5000);

        // Handle click
        notificationRef.current.onclick = () => {
          clearTimeout(autoCloseTimeout);
          if (notificationRef.current) {
            notificationRef.current.close();
          }
          window.focus();
        };

        // Handle close
        notificationRef.current.onclose = () => {
          clearTimeout(autoCloseTimeout);
        };
      } catch (error) {
        console.error('[Notification] Failed to send desktop notification:', error);
      }
    },
    [enabled, isNotificationSupported]
  );

  /**
   * Close current desktop notification
   */
  const closeDesktopNotification = useCallback(() => {
    if (notificationRef.current) {
      notificationRef.current.close();
      notificationRef.current = null;
    }
  }, []);

  /**
   * Set volume level
   */
  const setVolume = useCallback((level: number) => {
    const clampedVolume = Math.max(0, Math.min(1, level));
    if (audioRef.current) {
      audioRef.current.volume = clampedVolume;
    }
  }, []);

  /**
   * Handle full notification (sound + desktop)
   */
  const handleNotification = useCallback(
    (title: string, options?: NotificationOptions) => {
      // Play sound
      playSound();

      // Send desktop notification
      sendDesktopNotification(title, options);
    },
    [playSound, sendDesktopNotification]
  );

  /**
   * Check current notification permission status
   */
  const getPermissionStatus = useCallback((): NotificationPermission => {
    if (!isNotificationSupported()) {
      return 'denied';
    }
    return Notification.permission;
  }, [isNotificationSupported]);

  return {
    playSound,
    stopSound,
    sendDesktopNotification,
    closeDesktopNotification,
    requestPermission,
    handleNotification,
    setVolume,
    getPermissionStatus,
    isSupported: isNotificationSupported() && isAudioSupported(),
    hasPermission: isNotificationSupported() && Notification.permission === 'granted',
  };
};

/**
 * Utility function to check notification support
 */
export const checkNotificationSupport = (): {
  webNotifications: boolean;
  audio: boolean;
  vibration: boolean;
} => {
  return {
    webNotifications: 'Notification' in window,
    audio: 'Audio' in window,
    vibration: 'vibrate' in navigator,
  };
};

/**
 * Utility function to request vibration (haptic feedback)
 */
export const vibrateNotification = (pattern: number | number[] = [200, 100, 200]) => {
  if ('vibrate' in navigator) {
    navigator.vibrate(pattern);
  }
};

export default useNotificationAudio;
