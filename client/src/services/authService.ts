import { AuthResponse, AuthResponseWrapper, LoginFormData, SignupFormData, UserResponse } from '../types/auth';
import axios from './axiosConfig';

const API_URL = import.meta.env.VITE_API_URL;

const authService = {
    async getUser(): Promise<UserResponse> {
        try {
            const response = await axios.get<UserResponse>(`${API_URL}/auth/`, {
                withCredentials: true,
                headers: {
                    'Authorization': `Bearer ${localStorage.getItem('token')}`
                }
            });
            return response.data;
        } catch (error: any) {
            console.error('Get user error:', error);
            throw new Error(error.message || 'Get user failed');
        }
    },
    async login(data: LoginFormData): Promise<AuthResponse> {
        try {
            const response = await axios.post<AuthResponseWrapper>(`${API_URL}/auth/login`, {
                username_or_email: data.usernameOrEmail,
                password: data.password,
            });
            const authData = response.data.data;
            if (authData?.access_token) {
                localStorage.setItem('token', authData.access_token);
                localStorage.setItem('refresh_token', authData.refresh_token);
            }
            return authData;
        } catch (error: any) {
            console.log("login error", error);
            if (error.response?.data?.error) {
                throw new Error(error.response.data.error);
            }
            throw new Error(error.message || 'Login failed');
        }
    },

    async signup(data: SignupFormData): Promise<AuthResponse> {
        try {
            const response = await axios.post<AuthResponseWrapper>(`${API_URL}/auth/signup`, {
                username: data.username,
                email: data.email,
                password: data.password,
                user_signup_secret: data.userSignupSecret
            });
            const authData = response.data.data;
            if (authData?.access_token) {
                localStorage.setItem('token', authData.access_token);
                localStorage.setItem('refresh_token', authData.refresh_token);
            }
            return authData;
        } catch (error: any) {
            if (error.response?.data?.error) {
                throw new Error(error.response.data.error);
            }
            throw new Error(error.message || 'Signup failed');
        }
    },
    async refreshToken(): Promise<string | null> {
        try {
            const refreshToken = localStorage.getItem('refresh_token');
            if (!refreshToken) return null;

            const response = await axios.post(`${API_URL}/auth/refresh-token`, {}, {
                headers: {
                    Authorization: `Bearer ${refreshToken}`
                }
            });

            if (response.data.data?.access_token) {
                localStorage.setItem('token', response.data.data.access_token);
                return response.data.data.access_token;
            }
            return null;
        } catch (error) {
            console.error('Token refresh failed:', error);
            return null;
        }
    },

    logout() {
        localStorage.removeItem('token');
        localStorage.removeItem('refresh_token');
    },

    async validateSignupSecret(secret: string): Promise<boolean> {
        try {
            const response = await axios.post(`${API_URL}/auth/validate-signup-secret`, {
                secret
            });
            return response.data.valid;
        } catch (error: any) {
            console.error('Validate signup secret error:', error);
            throw new Error(error.response?.data?.error || 'Failed to validate signup secret');
        }
    },

    async googleOAuthCallback(code: string, redirectURI: string, userSignupSecret?: string, action?: string): Promise<AuthResponse> {
        try {
            const payload: any = {
                code,
                redirect_uri: redirectURI,
                purpose: 'auth',
                action: action || 'login'
            };
            
            // Only include signup secret if provided
            if (userSignupSecret) {
                payload.user_signup_secret = userSignupSecret;
            }
            
            const response = await axios.post(`${API_URL}/auth/google/callback`, payload);
            console.log('Google OAuth Response:', response.data);
            
            // Response structure from backend: { access_token, refresh_token, user }
            const accessToken = response.data?.access_token;
            const refreshToken = response.data?.refresh_token;
            
            if (accessToken && refreshToken) {
                console.log('Storing tokens...');
                localStorage.setItem('token', accessToken);
                localStorage.setItem('refresh_token', refreshToken);
                console.log('Tokens stored successfully');
            } else {
                console.warn('No tokens found in response:', { accessToken, refreshToken });
            }
            
            return response.data;
        } catch (error: any) {
            console.error('Google OAuth callback error:', error);
            if (error.response?.data?.error) {
                throw new Error(error.response.data.error);
            }
            throw new Error(error.message || 'Google OAuth failed');
        }
    }
};

export default authService; 