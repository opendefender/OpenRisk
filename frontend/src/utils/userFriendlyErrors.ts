/
  User-Friendly Error Messages
  Converts technical errors into clear, actionable messages for end users
 /

export const userFriendlyErrors = {
  // Authentication errors
  AUTH: {
    INVALID_CREDENTIALS: 'Incorrect email or password. Please check and try again.',
    SESSION_EXPIRED: 'Your session has expired. Please log in again.',
    UNAUTHORIZED: 'You don\'t have permission to access this. Contact your administrator.',
    FORBIDDEN: 'Access denied. Please verify your credentials and try again.',
  },

  // Data loading errors
  LOADING: {
    USERS: 'We couldn\'t load the user list. Please refresh the page and try again.',
    REPORTS: 'We couldn\'t load your reports. Please try again or check your connection.',
    INCIDENTS: 'Unable to load incidents. Please refresh the page and try again.',
    THREATS: 'Can\'t retrieve threat information right now. Please check your connection and try again.',
    AUDIT_LOGS: 'We\'re unable to load the audit logs. Please refresh and try again.',
    TOKENS: 'Unable to load your API tokens. Please refresh the page.',
    ASSETS: 'We couldn\'t load your assets. Please try again in a moment.',
    GAMIFICATION: 'We\'re having trouble loading your achievement data. Try again in a moment.',
    DASHBOARD: 'We couldn\'t load your dashboard. Please refresh the page.',
    MARKETPLACE: 'We\'re having trouble loading the marketplace. Please try again.',
  },

  // Creation/Update errors
  CREATE: {
    USER: 'We couldn\'t add the new user. Please verify all information is correct and try again.',
    TOKEN: 'Couldn\'t create the API token. Please check your input and try again.',
    TEAM: 'We couldn\'t create the team. Please enter a valid team name and try again.',
    ASSET: 'We couldn\'t add this asset. Please check your information and try again.',
    GENERIC: 'We couldn\'t save your changes. Please verify your information and try again.',
  },

  // Update errors
  UPDATE: {
    USER_STATUS: 'We couldn\'t update this user\'s status. Please try again.',
    USER_ROLE: 'Couldn\'t change the user\'s role. Please verify the selection and try again.',
    PROFILE: 'We couldn\'t save your profile changes. Please verify your information and try again.',
    TOKEN: 'We couldn\'t update the token. Please try again.',
    GENERIC: 'We couldn\'t save your changes. Please verify your information and try again.',
  },

  // Delete errors
  DELETE: {
    USER: 'We couldn\'t delete this user. Please try again or contact support if the problem persists.',
    TOKEN: 'We couldn\'t remove this token. Please try again.',
    TEAM: 'We couldn\'t delete the team. Please try again or contact support.',
    GENERIC: 'We couldn\'t delete this item. Please try again.',
  },

  // Action errors
  ACTION: {
    REVOKE_TOKEN: 'We couldn\'t disable this token. Please try again.',
    ROTATE_TOKEN: 'We couldn\'t refresh the token. Please try again.',
    REMOVE_MEMBER: 'We couldn\'t remove the team member. Please try again.',
    SYNC_APP: 'We couldn\'t sync the app. Please try again or contact support.',
    INSTALL_APP: 'We couldn\'t install this app. Please try again.',
    UNINSTALL_APP: 'We couldn\'t uninstall this app. Please try again.',
    TOGGLE_APP: 'We couldn\'t toggle this app. Please try again.',
  },

  // Permission errors
  PERMISSION: {
    AUDIT_LOGS: 'You don\'t have permission to view audit logs. Contact an administrator for access.',
    NO_ACCESS: 'You don\'t have permission to perform this action.',
    ADMIN_ONLY: 'This action requires administrator privileges. Contact your administrator.',
  },

  // Network/Connection errors
  NETWORK: {
    OFFLINE: 'You\'re offline. Please check your connection and try again.',
    TIMEOUT: 'The request took too long. Please check your connection and try again.',
    SERVER_ERROR: 'Something went wrong on our end. Please try again in a moment.',
    SERVICE_UNAVAILABLE: 'Our service is temporarily unavailable. Please try again in a few moments.',
  },

  // Validation errors
  VALIDATION: {
    REQUIRED_FIELD: (fieldName: string) => ${fieldName} is required. Please fill it in.,
    INVALID_EMAIL: 'Please enter a valid email address.',
    INVALID_FORMAT: (fieldName: string) => ${fieldName} has an invalid format. Please check and try again.,
    INVALID_SELECTION: 'Please make a valid selection and try again.',
    EMPTY_INPUT: 'Please fill in all required fields.',
  },

  // Generic fallback
  GENERIC: 'Something unexpected happened. Please try again or contact support.',
  TRY_AGAIN: 'Please try again.',
};

/
  Get a user-friendly error message
  @param error - The error object or string
  @param defaultMessage - Fallback message if error can't be parsed
 /
export const getUserFriendlyErrorMessage = (error: any, defaultMessage: string = userFriendlyErrors.GENERIC): string => {
  // If it's already a user-friendly message, return it
  if (typeof error === 'string') {
    // Check if it's already in our error messages
    if (Object.values(userFriendlyErrors).some((category) => 
      typeof category === 'object' && Object.values(category).includes(error)
    )) {
      return error;
    }
    // Check if it's a technical message that needs conversion
    if (error.includes('Failed to')) {
      return userFriendlyErrors.GENERIC;
    }
    return error;
  }

  // Handle axios errors
  if (error?.response?.data?.message) {
    return error.response.data.message;
  }

  // Handle network errors
  if (error?.message?.includes('Network')) {
    return userFriendlyErrors.NETWORK.OFFLINE;
  }

  // Handle timeout errors
  if (error?.message?.includes('timeout')) {
    return userFriendlyErrors.NETWORK.TIMEOUT;
  }

  // Default fallback
  return defaultMessage;
};

/
  Format validation error with field name
 /
export const getValidationError = (fieldName: string, errorType: string = 'required'): string => {
  if (errorType === 'required') {
    return userFriendlyErrors.VALIDATION.REQUIRED_FIELD(fieldName);
  }
  if (errorType === 'invalid') {
    return userFriendlyErrors.VALIDATION.INVALID_FORMAT(fieldName);
  }
  return Please check the ${fieldName} field.;
};
