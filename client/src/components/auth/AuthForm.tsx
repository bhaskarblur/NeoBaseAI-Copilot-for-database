import React, { useState } from 'react';
import { LoginFormData, SignupFormData } from '../../types/auth';
import LoginForm from './LoginForm';
import SignupForm from './SignupForm';
import ForgotPasswordForm from './ForgotPasswordForm';
import ResetPasswordForm from './ResetPasswordForm';

interface AuthFormProps {
  onLogin: (data: LoginFormData) => Promise<void>;
  onSignup: (data: SignupFormData) => Promise<void>;
}

type AuthView = 'login' | 'signup' | 'forgot-password' | 'reset-password';

export default function AuthForm({ onLogin, onSignup }: AuthFormProps) {
  const [currentView, setCurrentView] = useState<AuthView>('login');
  const [resetEmail, setResetEmail] = useState('');

  const handleSwitchToSignup = () => {
    setCurrentView('signup');
  };

  const handleSwitchToLogin = () => {
    setCurrentView('login');
  };

  const handleSwitchToForgotPassword = () => {
    setCurrentView('forgot-password');
  };

  const handleSwitchToResetPassword = (email: string) => {
    setResetEmail(email);
    setCurrentView('reset-password');
  };

  const handlePasswordResetSuccess = () => {
    setCurrentView('login');
  };

  switch (currentView) {
    case 'signup':
      return (
        <SignupForm
          onSignup={onSignup}
          onSwitchToLogin={handleSwitchToLogin}
        />
      );

    case 'forgot-password':
      return (
        <ForgotPasswordForm
          onSwitchToLogin={handleSwitchToLogin}
          onSwitchToResetPassword={handleSwitchToResetPassword}
        />
      );

    case 'reset-password':
      return (
        <ResetPasswordForm
          initialEmail={resetEmail}
          onSwitchToLogin={handleSwitchToLogin}
          onPasswordResetSuccess={handlePasswordResetSuccess}
        />
      );

    default:
      return (
        <LoginForm
          onLogin={onLogin}
          onSwitchToSignup={handleSwitchToSignup}
          onSwitchToForgotPassword={handleSwitchToForgotPassword}
        />
      );
  }
}