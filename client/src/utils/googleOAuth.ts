const GOOGLE_CLIENT_ID = import.meta.env.VITE_GOOGLE_CLIENT_ID;
const GOOGLE_REDIRECT_URI = import.meta.env.VITE_GOOGLE_REDIRECT_URI || 'http://localhost:5173/auth/google/callback';

export interface GoogleOAuthState {
    purpose: 'auth' | 'spreadsheet';
    action?: 'login' | 'signup'; // For auth purpose only
    signupSecret?: string;
}

export const initiateGoogleOAuth = (purpose: 'auth' | 'spreadsheet', signupSecret?: string, action?: 'login' | 'signup') => {
    if (!GOOGLE_CLIENT_ID) {
        throw new Error('Google Client ID is not configured');
    }

    const state: GoogleOAuthState = { purpose };
    if (action) {
        state.action = action;
    }
    if (signupSecret) {
        state.signupSecret = signupSecret;
    }

    const encodedState = btoa(JSON.stringify(state));
    
    const scope = purpose === 'spreadsheet' 
        ? 'https://www.googleapis.com/auth/spreadsheets https://www.googleapis.com/auth/drive.readonly'
        : 'https://www.googleapis.com/auth/userinfo.email https://www.googleapis.com/auth/userinfo.profile';

    const authUrl = new URL('https://accounts.google.com/o/oauth2/v2/auth');
    authUrl.searchParams.append('client_id', GOOGLE_CLIENT_ID);
    authUrl.searchParams.append('redirect_uri', GOOGLE_REDIRECT_URI);
    authUrl.searchParams.append('response_type', 'code');
    authUrl.searchParams.append('scope', scope);
    authUrl.searchParams.append('access_type', 'offline');
    authUrl.searchParams.append('state', encodedState);
    authUrl.searchParams.append('prompt', 'consent');

    window.location.href = authUrl.toString();
};

export const parseGoogleOAuthCallback = (): { code: string; state: GoogleOAuthState } | null => {
    const params = new URLSearchParams(window.location.search);
    const code = params.get('code');
    const stateParam = params.get('state');

    if (!code) {
        console.error('No authorization code in callback');
        return null;
    }

    // Try to parse state parameter
    let state: GoogleOAuthState = { purpose: 'auth' }; // Default to 'auth' purpose
    
    if (stateParam) {
        try {
            state = JSON.parse(atob(stateParam));
        } catch (error) {
            console.warn('Failed to parse OAuth state, using default:', error);
            // State parsing failed, use default
            state = { purpose: 'auth' };
        }
    } else {
        console.warn('No state parameter in callback, using default auth purpose');
    }

    return { code, state };
};
