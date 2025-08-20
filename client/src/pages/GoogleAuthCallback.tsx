import React, { useEffect, useState } from 'react';
import { useLocation, useNavigate } from 'react-router-dom';
import { Loader2, AlertCircle, CheckCircle } from 'lucide-react';

const GoogleAuthCallback: React.FC = () => {
  const location = useLocation();
  const navigate = useNavigate();
  const [status, setStatus] = useState<'processing' | 'success' | 'error'>('processing');
  const [message, setMessage] = useState('Processing authentication...');

  useEffect(() => {
    const handleCallback = async () => {
      const params = new URLSearchParams(location.search);
      const code = params.get('code');
      const error = params.get('error');

      if (error) {
        setStatus('error');
        setMessage(`Authentication failed: ${error}`);
        
        // Send error to parent window if opened as popup
        if (window.opener) {
          window.opener.postMessage({
            type: 'google-auth-error',
            error: error
          }, window.location.origin);
          
          setTimeout(() => {
            window.close();
          }, 2000);
        }
        return;
      }

      if (!code) {
        setStatus('error');
        setMessage('No authorization code received');
        
        if (window.opener) {
          window.opener.postMessage({
            type: 'google-auth-error',
            error: 'No authorization code received'
          }, window.location.origin);
          
          setTimeout(() => {
            window.close();
          }, 2000);
        }
        return;
      }

      try {
        // Exchange code for tokens
        const response = await fetch(`${import.meta.env.VITE_API_URL}/google/callback?code=${code}`, {
          headers: {
            'Authorization': `Bearer ${localStorage.getItem('token')}`
          }
        });

        if (!response.ok) {
          const error = await response.json();
          throw new Error(error.error || 'Failed to exchange authorization code');
        }

        const data = await response.json();
        
        setStatus('success');
        setMessage('Authentication successful!');
        
        // Send tokens to parent window if opened as popup
        if (window.opener) {
          window.opener.postMessage({
            type: 'google-auth-success',
            access_token: data.access_token,
            refresh_token: data.refresh_token,
            user_email: data.user_email,
            expiry: data.expiry
          }, window.location.origin);
          
          setTimeout(() => {
            window.close();
          }, 1500);
        } else {
          // If not a popup, redirect back to the app
          setTimeout(() => {
            navigate('/');
          }, 1500);
        }
        
      } catch (error: any) {
        setStatus('error');
        setMessage(error.message || 'Authentication failed');
        
        if (window.opener) {
          window.opener.postMessage({
            type: 'google-auth-error',
            error: error.message
          }, window.location.origin);
          
          setTimeout(() => {
            window.close();
          }, 2000);
        }
      }
    };

    handleCallback();
  }, [location, navigate]);

  return (
    <div className="min-h-screen flex items-center justify-center bg-gray-50">
      <div className="max-w-md w-full p-8">
        <div className="bg-white rounded-lg neo-border p-6">
          <div className="flex flex-col items-center space-y-4">
            {status === 'processing' && (
              <>
                <Loader2 className="w-12 h-12 animate-spin text-blue-600" />
                <h2 className="text-xl font-bold">Authenticating with Google</h2>
                <p className="text-gray-600 text-center">{message}</p>
              </>
            )}
            
            {status === 'success' && (
              <>
                <CheckCircle className="w-12 h-12 text-green-600" />
                <h2 className="text-xl font-bold text-green-800">Success!</h2>
                <p className="text-gray-600 text-center">{message}</p>
                <p className="text-sm text-gray-500">Redirecting...</p>
              </>
            )}
            
            {status === 'error' && (
              <>
                <AlertCircle className="w-12 h-12 text-red-600" />
                <h2 className="text-xl font-bold text-red-800">Authentication Failed</h2>
                <p className="text-gray-600 text-center">{message}</p>
                <p className="text-sm text-gray-500">This window will close automatically</p>
              </>
            )}
          </div>
        </div>
      </div>
    </div>
  );
};

export default GoogleAuthCallback;