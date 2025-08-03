import React, { useState } from 'react';
import { LoginFormData, SignupFormData } from '../../types/auth';
import LoginForm from './LoginForm';
import SignupForm from './SignupForm';

interface AuthFormProps {
  onLogin: (data: LoginFormData) => Promise<void>;
  onSignup: (data: SignupFormData) => Promise<void>;
}

export default function AuthForm({ onLogin, onSignup }: AuthFormProps) {
  const [isLogin, setIsLogin] = useState(true);

  const handleSwitchToSignup = () => {
    setIsLogin(false);
  };

  const handleSwitchToLogin = () => {
    setIsLogin(true);
  };

  if (isLogin) {
    return (
      <LoginForm 
        onLogin={onLogin} 
        onSwitchToSignup={handleSwitchToSignup} 
      />
    );
  }

  return (
    <SignupForm 
      onSignup={onSignup} 
      onSwitchToLogin={handleSwitchToLogin} 
    />
  );
}