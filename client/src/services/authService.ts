import { AuthResponse, LoginFormData, SignupFormData, UserResponse } from '../types/auth';
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
            const response = await axios.post(`${API_URL}/auth/login`, {
                username_or_email: data.usernameOrEmail,
                password: data.password,
            });
            if (response.data.data?.access_token) {
                localStorage.setItem('token', response.data.data.access_token);
                localStorage.setItem('refresh_token', response.data.data.refresh_token);
            }
            return response.data;
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
            const response = await axios.post(`${API_URL}/auth/signup`, {
                username: data.username,
                email: data.email,
                password: data.password,
                user_signup_secret: data.userSignupSecret
            });
            if (response.data.data?.access_token) {
                localStorage.setItem('token', response.data.data.access_token);
                localStorage.setItem('refresh_token', response.data.data.refresh_token);
            }
            return response.data;
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
    }
};

export default authService; 