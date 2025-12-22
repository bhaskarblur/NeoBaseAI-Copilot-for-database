export interface AuthState {
  isAuthenticated: boolean;
  user: User | null;
}

export interface User {
  id: string;
  username: string;
  email: string;
  created_at: string;
}

export interface LoginFormData {
  usernameOrEmail: string;
  password: string;
}

export interface SignupFormData {
  username: string;
  email: string;
  password: string;
  confirmPassword: string;
  userSignupSecret: string;
}


export interface AuthResponse {
  access_token: string;
  refresh_token: string;
  user: {
    id: string;
    username: string;
    email: string;
    created_at: string;
  };
}

export interface AuthResponseWrapper {
  success: boolean;
  data: AuthResponse;
  error?: string;
}

export interface UserResponse {
  success: boolean;
  data: User;
}