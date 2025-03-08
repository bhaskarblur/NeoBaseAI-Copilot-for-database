export interface User {
  id: string;
  username: string;
  created_at: string;
}

export interface LoginFormData {
  username: string;
  password: string;
}

export interface SignupFormData {
  username: string;
  password: string;
}

export interface AuthResponse {
  success: boolean;
  message?: string;
  data?: {
    user: User;
  };
}

export interface UserResponse {
  success: boolean;
  message?: string;
  data?: User;
} 