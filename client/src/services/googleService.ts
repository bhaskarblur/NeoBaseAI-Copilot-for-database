import axios from './axiosConfig';

const API_URL = import.meta.env.VITE_API_URL;

interface GoogleTokenResponse {
  access_token: string;
  refresh_token?: string;
  expiry?: string;
}

interface GoogleAuthCodeResponse {
  access_token: string;
  refresh_token: string;
  user?: { email: string };
  user_email?: string;
  expiry?: string;
}

interface SheetInfo {
  title: string;
  sheet_count: number;
  sheets?: string[];
}

const googleService = {
  async refreshGoogleToken(refreshToken: string): Promise<GoogleTokenResponse> {
    try {
      const response = await axios.post(`${API_URL}/google/refresh-token`, {
        refresh_token: refreshToken,
      });
      return response.data;
    } catch (error: any) {
      throw new Error(error.response?.data?.error || 'Failed to refresh token');
    }
  },

  async exchangeAuthCode(code: string, redirectURI: string): Promise<GoogleAuthCodeResponse> {
    try {
      const response = await axios.post(`${API_URL}/auth/google/callback`, {
        code,
        redirect_uri: redirectURI,
        purpose: 'spreadsheet',
      });
      return response.data;
    } catch (error: any) {
      if (error.response) {
        const errorData = error.response.data;
        throw new Error(
          errorData.error || errorData.Error || `Token exchange failed with status ${error.response.status}`
        );
      }
      throw new Error(error.message || 'Failed to exchange authorization code');
    }
  },

  async validateSheet(
    accessToken: string,
    refreshToken: string,
    sheetId: string
  ): Promise<SheetInfo> {
    try {
      const response = await axios.post(`${API_URL}/google/validate-sheet`, {
        access_token: accessToken,
        refresh_token: refreshToken,
        sheet_id: sheetId,
      });
      return response.data;
    } catch (error: any) {
      throw new Error(error.response?.data?.error || 'Failed to validate sheet access');
    }
  },
};

export default googleService;
