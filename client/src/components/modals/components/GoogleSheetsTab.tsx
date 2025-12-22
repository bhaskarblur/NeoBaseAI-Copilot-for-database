import React, { useState, useEffect } from 'react';
import { AlertCircle, CheckCircle, Loader2, ExternalLink, RefreshCw, Info } from 'lucide-react';
import { Connection } from '../../../types/chat';

interface GoogleSheetsTabProps {
  formData: Connection;
  handleChange: (e: React.ChangeEvent<HTMLInputElement | HTMLSelectElement>) => void;
  isEditMode: boolean; // true if editing existing connection, false if creating new
  onGoogleAuthChange: (authData: { 
    google_sheet_id: string; 
    google_sheet_url: string;
    google_auth_token: string; 
    google_refresh_token: string;
  }) => void;
  onRefreshData?: () => void; // Callback to trigger refresh knowledge modal
}

const GoogleSheetsTab: React.FC<GoogleSheetsTabProps> = ({
  formData,
  handleChange,
  isEditMode,
  onGoogleAuthChange,
  onRefreshData,
}) => {
  const [isAuthenticating, setIsAuthenticating] = useState(false);
  const [authError, setAuthError] = useState<string | null>(null);
  const [authSuccess, setAuthSuccess] = useState(false);
  const [sheetUrl, setSheetUrl] = useState('');
  const [sheetInfo, setSheetInfo] = useState<{
    title: string;
    sheet_count: number;
    sheets?: string[];
  } | null>(null);
  const [isValidating, setIsValidating] = useState(false);
  const [userEmail, setUserEmail] = useState<string | null>(null);

  // Load existing URL if available
  useEffect(() => {
    if (formData.google_sheet_url) {
      setSheetUrl(formData.google_sheet_url);
    }
  }, [formData.google_sheet_url]);

  // Helper function to check if token is expired
  const isTokenExpired = (): boolean => {
    const expiry = localStorage.getItem('google_token_expiry');
    if (!expiry) return true;
    
    const expiryTime = new Date(expiry).getTime();
    const now = new Date().getTime();
    // Consider token expired if it expires in less than 5 minutes
    return now > (expiryTime - 5 * 60 * 1000);
  };

  // Helper function to refresh token
  const refreshToken = async (): Promise<boolean> => {
    const refresh = localStorage.getItem('google_refresh_token');
    if (!refresh) return false;
    
    try {
      const response = await fetch(`${import.meta.env.VITE_API_URL}/google/refresh-token`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          'Authorization': `Bearer ${localStorage.getItem('token')}`
        },
        body: JSON.stringify({ refresh_token: refresh })
      });
      
      if (!response.ok) {
        throw new Error('Failed to refresh token');
      }
      
      const data = await response.json();
      localStorage.setItem('google_access_token', data.access_token);
      if (data.refresh_token) {
        localStorage.setItem('google_refresh_token', data.refresh_token);
      }
      if (data.expiry) {
        localStorage.setItem('google_token_expiry', data.expiry);
      }
      
      return true;
    } catch (error) {
      console.error('Failed to refresh token:', error);
      // Clear tokens if refresh fails
      localStorage.removeItem('google_access_token');
      localStorage.removeItem('google_refresh_token');
      localStorage.removeItem('google_user_email');
      localStorage.removeItem('google_token_expiry');
      setAuthSuccess(false);
      setUserEmail(null);
      return false;
    }
  };

  // Check for existing tokens on mount
  useEffect(() => {
    const checkAuth = async () => {
      const accessToken = localStorage.getItem('google_access_token');
      const refresh = localStorage.getItem('google_refresh_token');
      const email = localStorage.getItem('google_user_email');
      
      if (accessToken && refresh && email) {
        // Check if token is expired
        if (isTokenExpired()) {
          // Try to refresh
          const refreshed = await refreshToken();
          if (refreshed) {
            setAuthSuccess(true);
            setUserEmail(email);
          }
        } else {
          setAuthSuccess(true);
          setUserEmail(email);
        }
      }
    };
    
    checkAuth();
  }, []); // Only run on mount

  // Extract sheet ID from URL
  const extractSheetId = (url: string): string | null => {
    const patterns = [
      /\/spreadsheets\/d\/([a-zA-Z0-9-_]+)/,
      /[?&]id=([a-zA-Z0-9-_]+)/,
      /^([a-zA-Z0-9-_]+)$/
    ];
    
    for (const pattern of patterns) {
      const match = url.match(pattern);
      if (match) {
        return match[1];
      }
    }
    
    return null;
  };

  // Build Google OAuth URL for spreadsheet access
  const buildGoogleOAuthURL = (): string => {
    const GOOGLE_CLIENT_ID = import.meta.env.VITE_GOOGLE_CLIENT_ID;
    const GOOGLE_REDIRECT_URI = import.meta.env.VITE_GOOGLE_REDIRECT_URI || 'http://localhost:5173/auth/google/callback';
    
    if (!GOOGLE_CLIENT_ID) {
      throw new Error('Google Client ID is not configured');
    }

    const state = btoa(JSON.stringify({ purpose: 'spreadsheet' }));
    const scope = 'https://www.googleapis.com/auth/spreadsheets https://www.googleapis.com/auth/drive.readonly https://www.googleapis.com/auth/userinfo.email https://www.googleapis.com/auth/userinfo.profile';

    const authUrl = new URL('https://accounts.google.com/o/oauth2/v2/auth');
    authUrl.searchParams.append('client_id', GOOGLE_CLIENT_ID);
    authUrl.searchParams.append('redirect_uri', GOOGLE_REDIRECT_URI);
    authUrl.searchParams.append('response_type', 'code');
    authUrl.searchParams.append('scope', scope);
    authUrl.searchParams.append('access_type', 'offline');
    authUrl.searchParams.append('state', state);
    authUrl.searchParams.append('prompt', 'consent');

    return authUrl.toString();
  };

  // Handle Google authentication for spreadsheet access
  const handleGoogleAuth = async () => {
    try {
      setIsAuthenticating(true);
      setAuthError(null);
      
      // Build OAuth URL
      const oauthUrl = buildGoogleOAuthURL();
      
      // Open a popup window for the OAuth flow
      const width = 600;
      const height = 600;
      const left = window.screenX + (window.outerWidth - width) / 2;
      const top = window.screenY + (window.outerHeight - height) / 2;
      
      const authWindow = window.open(
        oauthUrl, 
        'google-sheets-auth', 
        `width=${width},height=${height},left=${left},top=${top}`
      );
      
      if (!authWindow) {
        throw new Error('Failed to open authentication window');
      }
      
      // Set up message listener before initiating OAuth
      let handleMessage: (event: MessageEvent) => Promise<void>;
      handleMessage = async (event: MessageEvent) => {
        // Verify origin for security
        if (event.origin !== window.location.origin) return;
        
        if (event.data.type === 'google-auth-success') {
          // The spreadsheet OAuth returns the authorization code
          const { code } = event.data;
          
          try {
            // Exchange the code for tokens using the backend endpoint
            const redirectURI = import.meta.env.VITE_GOOGLE_REDIRECT_URI || 'http://localhost:5173/auth/google/callback';
            
            const tokenResponse = await fetch(`${import.meta.env.VITE_API_URL}/auth/google/callback`, {
              method: 'POST',
              headers: {
                'Content-Type': 'application/json',
                'Authorization': `Bearer ${localStorage.getItem('token')}`
              },
              body: JSON.stringify({
                code: code,
                redirect_uri: redirectURI,
                purpose: 'spreadsheet'
              })
            });
            
            if (!tokenResponse.ok) {
              const errorText = await tokenResponse.text();
              console.error('Token exchange error response status:', tokenResponse.status);
              console.error('Token exchange error response text:', errorText);
              try {
                const error = JSON.parse(errorText);
                throw new Error(error.error || error.Error || 'Failed to exchange authorization code');
              } catch (parseError) {
                throw new Error(`Token exchange failed with status ${tokenResponse.status}: ${errorText}`);
              }
            }
            
            const responseText = await tokenResponse.text();
            console.log('Token exchange response status:', tokenResponse.status);
            console.log('Token exchange response text:', responseText);
            console.log('Token exchange response type:', tokenResponse.headers.get('content-type'));
            
            if (!responseText) {
              throw new Error('Empty response from token exchange');
            }
            
            const tokenData = JSON.parse(responseText);
            console.log('Parsed token data:', tokenData);
            
            // Extract email from user object
            const userEmail = tokenData.user?.email || tokenData.user_email;
            if (!userEmail) {
              console.warn('No email found in token response. Token data keys:', Object.keys(tokenData));
              throw new Error('No email returned from Google authentication');
            }
            
            // Store tokens persistently
            localStorage.setItem('google_access_token', tokenData.access_token);
            localStorage.setItem('google_refresh_token', tokenData.refresh_token);
            localStorage.setItem('google_user_email', userEmail);
            if (tokenData.expiry) {
              localStorage.setItem('google_token_expiry', tokenData.expiry);
            }
            
            setUserEmail(userEmail);
            setAuthSuccess(true);
            setIsAuthenticating(false);
            
            // Close the auth window
            if (authWindow) {
              authWindow.close();
            }
            
            window.removeEventListener('message', handleMessage);
          } catch (exchangeError: any) {
            setAuthError(exchangeError.message || 'Failed to exchange authorization code');
            setIsAuthenticating(false);
            if (authWindow) {
              authWindow.close();
            }
            window.removeEventListener('message', handleMessage);
          }
        } else if (event.data.type === 'google-auth-error') {
          setAuthError(event.data.error || 'Authentication failed');
          setIsAuthenticating(false);
          
          if (authWindow) {
            authWindow.close();
          }
          
          window.removeEventListener('message', handleMessage);
        }
      };
      
      window.addEventListener('message', handleMessage);
      
      // Check if window was closed without auth
      const checkClosed = setInterval(() => {
        if (authWindow && authWindow.closed) {
          clearInterval(checkClosed);
          setIsAuthenticating(false);
          window.removeEventListener('message', handleMessage);
        }
      }, 1000);
      
    } catch (error: unknown) {
      const errorMessage = error instanceof Error ? error.message : 'Authentication failed';
      setAuthError(errorMessage);
      setIsAuthenticating(false);
    }
  };

  // Validate Google Sheet access
  const validateSheetAccess = async () => {
    // Check if token needs refresh
    if (isTokenExpired()) {
      const refreshed = await refreshToken();
      if (!refreshed) {
        setAuthError('Authentication expired. Please sign in again.');
        return;
      }
    }
    
    const sheetId = extractSheetId(sheetUrl);
    if (!sheetId) {
      setAuthError('Invalid Google Sheets URL');
      return;
    }
    
    const accessToken = localStorage.getItem('google_access_token');
    const googleRefreshToken = localStorage.getItem('google_refresh_token');
    
    if (!accessToken || !googleRefreshToken) {
      setAuthError('Please authenticate with Google first');
      return;
    }
    
    try {
      setIsValidating(true);
      setAuthError(null);
      
      const response = await fetch(`${import.meta.env.VITE_API_URL}/google/validate-sheet`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          'Authorization': `Bearer ${localStorage.getItem('token')}`
        },
        body: JSON.stringify({
          access_token: accessToken,
          refresh_token: googleRefreshToken,
          sheet_id: sheetId
        })
      });
      
      if (!response.ok) {
        const error = await response.json();
        throw new Error(error.error || 'Failed to validate sheet access');
      }
      
      const data = await response.json();
      setSheetInfo(data);
      
      // Update form data with Google Sheets info
      onGoogleAuthChange({
        google_sheet_id: sheetId,
        google_sheet_url: sheetUrl,
        google_auth_token: accessToken,
        google_refresh_token: googleRefreshToken
      });
      
      // Set database name based on sheet title
      const dbName = data.title.toLowerCase().replace(/[^a-z0-9]/g, '_').substring(0, 50);
      const event = {
        target: {
          name: 'database',
          value: dbName
        }
      } as React.ChangeEvent<HTMLInputElement>;
      handleChange(event);
      
    } catch (error: unknown) {
      const errorMessage = error instanceof Error ? error.message : 'Failed to validate sheet';
      setAuthError(errorMessage);
      setSheetInfo(null);
    } finally {
      setIsValidating(false);
    }
  };

  // Handle sheet URL change
  const handleSheetUrlChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    setSheetUrl(e.target.value);
    setSheetInfo(null);
  };

  // Re-authenticate if needed
  const handleReAuth = () => {
    setAuthSuccess(false);
    setUserEmail(null);
    setSheetInfo(null);
    localStorage.removeItem('google_access_token');
    localStorage.removeItem('google_refresh_token');
    localStorage.removeItem('google_user_email');
    localStorage.removeItem('google_token_expiry');
    handleGoogleAuth();
  };

  return (
    <div className="space-y-6">
      {/* Instructions */}
      <div className="p-4 bg-blue-50 border-2 border-blue-200 rounded-lg">
        <div className="flex items-start gap-2">
          <Info className="w-5 h-5 text-blue-600 mt-0.5 flex-shrink-0" />
          <div className="text-sm text-blue-800">
            <p className="font-semibold mb-2 text-base">How to connect Google Sheets:</p>
            <ol className="list-decimal ml-4 space-y-1">
              <li>Click "Authenticate with Google" to sign in to your Google account</li>
              <li><strong>Grant read-only</strong> access to your Google Sheets</li>
              <li>Paste your Google Sheets URL</li>
              <li>Click "Connect" to connect with the sheet</li>
            </ol>
          </div>
        </div>
      </div>

      {/* Google Authentication */}
      <div>
        <label className="block font-bold mb-2 text-lg">Google Authentication</label>
        <p className="text-gray-600 text-sm mb-4">
          Authenticate with Google to access your spreadsheets
        </p>
        
        {!authSuccess ? (
          <button
            type="button"
            onClick={handleGoogleAuth}
            disabled={isAuthenticating}
            className="neo-button-secondary w-full flex items-center justify-center gap-2"
          >
            {isAuthenticating ? (
              <>
                <Loader2 className="w-5 h-5 animate-spin" />
                <span>Authenticating...</span>
              </>
            ) : (
              <>
                <img src="https://www.google.com/favicon.ico" alt="Google" className="w-5 h-5" />
                <span>Authenticate with Google</span>
              </>
            )}
          </button>
        ) : (
          <div className="p-4 bg-green-50 border-2 border-green-200 rounded-lg">
            <div className="flex items-center justify-between">
              <div className="flex items-center gap-3">
                <CheckCircle className="w-5 h-5 text-green-600" />
                <div>
                  <p className="font-medium text-green-800">Authenticated</p>
                  {userEmail && (
                    <p className="text-sm text-green-600">{userEmail}</p>
                  )}
                </div>
              </div>
              <button
                type="button"
                onClick={handleReAuth}
                className="px-3 py-2 text-sm text-black hover:bg-green-100 hover:text-green-800 rounded-lg transition-colors border border-black hover:border-green-400"
                title="Switch/Reconnect"
              >
                <div className="flex items-center gap-2">
                  <RefreshCw className="w-4 h-4" />
                  <span className="hidden sm:inline">Switch / Reconnect</span>
                </div>
              </button>
            </div>
          </div>
        )}
      </div>

      {/* Google Sheets URL */}
      {authSuccess && (
        <div>
          <label className="block font-bold mb-2 text-lg">Google Sheets URL</label>
          <p className="text-gray-600 text-sm mb-4">
            {isEditMode && formData.google_sheet_url
              ? "Connected Google Sheet URL (read-only)"
              : "Paste the URL of your Google Sheet you want to connect"
            }
          </p>
          
          <div className="flex gap-2">
            <input
              type="text"
              value={sheetUrl}
              onChange={handleSheetUrlChange}
              placeholder="https://docs.google.com/spreadsheets/d/..."
              className={`neo-input flex-1 ${isEditMode && formData.google_sheet_url ? 'bg-gray-100 text-gray-600' : ''}`}
              disabled={isValidating || (isEditMode && Boolean(formData.google_sheet_url))}
              title={isEditMode && formData.google_sheet_url ? "URL cannot be modified when editing existing connections" : ""}
            />
            {isEditMode && formData.google_sheet_url ? (
              <button
                type="button"
                onClick={onRefreshData}
                disabled={!onRefreshData}
                className="neo-button-secondary flex items-center gap-2"
                title="Refresh data from Google Sheets"
              >
                <RefreshCw className="w-4 h-4" />
                Refresh Data
              </button>
            ) : (
              <button
                type="button"
                onClick={validateSheetAccess}
                disabled={!sheetUrl || isValidating}
                className="neo-button-secondary"
                title="Connect to Google Sheets"
              >
                {isValidating ? (
                  <Loader2 className="w-4 h-4 animate-spin" />
                ) : (
                  'Connect'
                )}
              </button>
            )}
          </div>
          
          {sheetUrl && (
            <a
              href={sheetUrl}
              target="_blank"
              rel="noopener noreferrer"
              className="mt-3 inline-flex items-center gap-1 text-sm text-blue-600 hover:text-blue-800"
            >
              Open in Google Sheets
              <ExternalLink className="w-3 h-3" />
            </a>
          )}
        </div>
      )}

      {/* Sheet Information */}
      {sheetInfo && (
        <div className="p-4 bg-gray-50 border-2 border-gray-200 rounded-lg">
          <h4 className="font-bold mb-3">Sheet Information</h4>
          <div className="space-y-2 text-sm">
            <div className="flex gap-1">
              <span className="text-gray-600">Sheet Name:</span>
              <span className="font-medium">{sheetInfo.title}</span>
            </div>
            <div className="flex gap-1">
              <span className="text-gray-600">Number of pages:</span>
              <span className="font-medium">{sheetInfo.sheet_count}</span>
            </div>
            {sheetInfo.sheets && sheetInfo.sheets.length > 0 && (
              <div>
                <span className="text-gray-600">Pages:</span>
                <ul className="mt-1 ml-4 list-disc">
                  {sheetInfo.sheets.map((sheet: string, index: number) => (
                    <li key={index} className="text-gray-800">{sheet}</li>
                  ))}
                </ul>
              </div>
            )}
          </div>
        </div>
      )}

      {/* Hidden fields for connection */}
      <input type="hidden" name="host" value="google-sheets" onChange={handleChange} />
      <input type="hidden" name="port" value="0" onChange={handleChange} />
      <input type="hidden" name="username" value="google-user" onChange={handleChange} />
      <input type="hidden" name="password" value="oauth" onChange={handleChange} />

      {/* Error message */}
      {authError && (
        <div className="p-3 bg-red-50 border-2 border-red-200 rounded-lg">
          <div className="flex items-center gap-2 text-red-600">
            <AlertCircle className="w-4 h-4" />
            <p className="text-sm">{authError}</p>
          </div>
        </div>
      )}

      {/* Data Privacy Notice */}
      <div className="p-4 bg-gray-100 border border-gray-300 rounded-lg">
        <div className="flex items-start gap-2">
          <AlertCircle className="w-5 h-5 text-gray-600 mt-0.5 flex-shrink-0" />
          <div className="text-sm text-gray-700">
            <p className="font-semibold text-base mb-1">Data Security & Privacy</p>
            <p>
              Your Google Sheets data will be synced and encrypted in our secure database. 
              We only request read-only access to your sheets. Authentication tokens are 
              encrypted and you can revoke access at any time from your Google account settings.
            </p>
          </div>
        </div>
      </div>
    </div>
  );
};

export default GoogleSheetsTab;