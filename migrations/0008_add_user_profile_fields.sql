-- Migration: Add profile fields to users table
-- This migration adds personalization fields to the User model

ALTER TABLE users 
ADD COLUMN IF NOT EXISTS bio TEXT DEFAULT '',
ADD COLUMN IF NOT EXISTS phone VARCHAR(20) DEFAULT '',
ADD COLUMN IF NOT EXISTS department VARCHAR(255) DEFAULT '',
ADD COLUMN IF NOT EXISTS timezone VARCHAR(100) DEFAULT 'UTC';

-- Create indexes for better query performance
CREATE INDEX IF NOT EXISTS idx_users_timezone ON users(timezone);
CREATE INDEX IF NOT EXISTS idx_users_department ON users(department);
